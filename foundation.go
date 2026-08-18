package foundation

import (
	"context"
	"fmt"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	fjobs "github.com/foundation-go/foundation/jobs"
	fkafka "github.com/foundation-go/foundation/kafka"
	fmetrics "github.com/foundation-go/foundation/metrics"
	fpg "github.com/foundation-go/foundation/postgresql"
	fredis "github.com/foundation-go/foundation/redis"
	fsentry "github.com/foundation-go/foundation/sentry"
)

// Version is the Foundation release the service was built against.
//
// The constant is the fallback: when the framework is consumed as a module the
// real version comes from the build information, so it no longer drifts from
// whatever the constant was last set to by hand.
var Version = versionFromBuildInfo(defaultVersion)

// defaultVersion is reported when the build carries no module information, as
// in `go run` and in the framework's own tests.
const defaultVersion = "0.3.0"

// versionFromBuildInfo reads the version of the Foundation module from the
// running binary's build information.
func versionFromBuildInfo(fallback string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fallback
	}

	if info.Main.Path == modulePath && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return normalizeVersion(info.Main.Version)
	}

	for _, dep := range info.Deps {
		if dep.Path == modulePath && dep.Version != "" {
			return normalizeVersion(dep.Version)
		}
	}

	return fallback
}

// normalizeVersion drops the leading "v" module versions carry.
//
// Everything that prints Version already adds its own — the startup banner says
// "Foundation: v%s" — so passing "v0.3.0" straight through produced "vv0.3.0"
// and a version label inconsistent with what the same code emitted before.
func normalizeVersion(version string) string {
	return strings.TrimPrefix(version, "v")
}

// modulePath is this module's import path.
const modulePath = "github.com/foundation-go/foundation"

// DefaultShutdownTimeout is how long the service spends draining work in flight
// before stopping its components.
const DefaultShutdownTimeout = 30 * time.Second

// DefaultPort is the port server modes listen on when PORT is unset.
const DefaultPort = 51051

// DefaultRedisPoolSize is the pool size used when none is configured.
const DefaultRedisPoolSize = 5

// Service represents a single microservice - part of the bigger Foundation-based application, which implements
// an isolated domain of the application logic.
type Service struct {
	Name       string
	Config     *Config
	Components []Component
	ModeName   string

	// stopRequested is closed by Shutdown. It replaces a cancelFunc field that
	// Start assigned and other goroutines read — a data race, and a silent
	// no-op for anything that called Shutdown before Start got that far.
	stopRequestedOnce sync.Once
	stopOnce          sync.Once
	stopRequested     chan struct{}

	// draining is set once shutdown begins, so that the readiness probe can
	// report the service as unavailable before anything is torn down.
	draining atomic.Bool

	Logger *logrus.Entry
}

// Config represents the configuration of a Service.
type Config struct {
	Database     *DatabaseConfig
	EventsWorker *EventsWorkerConfig
	GRPC         *GRPCConfig
	Kafka        *KafkaConfig
	Metrics      *MetricsConfig
	Outbox       *OutboxConfig
	Redis        *RedisConfig
	Sentry       *SentryConfig
	JobsEnqueuer *JobsEnqueuerConfig
	HTTP         *HTTPConfig

	// ShutdownTimeout bounds how long the service spends draining work in
	// flight before it stops its components. Set it comfortably below the
	// supervisor's own grace period (Kubernetes' terminationGracePeriodSeconds,
	// for instance), so that the service finishes on its own terms.
	ShutdownTimeout time.Duration

	// DrainDelay is how long the service keeps serving after it has started
	// failing its readiness probe, before it begins shutting down. It gives
	// load balancers time to notice and stop routing new requests here.
	// Default: 0, i.e. shut down immediately.
	DrainDelay time.Duration

	// HealthCheckTimeout bounds a single readiness probe.
	HealthCheckTimeout time.Duration

	// LogPayloads enables logging full gRPC request and response bodies at
	// Debug level. Off by default: a protobuf message printed whole contains
	// whatever the caller sent, and turning on LOG_LEVEL=debug in production to
	// investigate something unrelated should not start writing passwords and
	// tokens to the log.
	LogPayloads bool
}

// DatabaseConfig represents the configuration of a PostgreSQL database.
type DatabaseConfig struct {
	Enabled bool
	Pool    int
	URL     string
}

// EventsWorkerConfig represents the configuration of an event bus.
type EventsWorkerConfig struct {
	// ErrorsTopic is the name of the Kafka topic to which errors from the
	// events worker handlers are published.
	//
	// Empty — the default — means the topic is derived from the error's proto
	// name like any other event, which puts Foundation errors on
	// `foundation.errors`.
	ErrorsTopic string

	// DeliverErrors determines whether errors from events worker handlers
	// should be published to the errors topic (and thus, delivered
	// to originator, aka user) or not.
	DeliverErrors bool
}

// TopicFor returns the topic an error event should be published to: the
// configured errors topic, or the one derived from the message name.
func (c *EventsWorkerConfig) TopicFor(derived string) string {
	if c != nil && c.ErrorsTopic != "" {
		return c.ErrorsTopic
	}

	return derived
}

// GRPCConfig represents the configuration of a gRPC server.
type GRPCConfig struct {
	TLSDir string
}

// KafkaConfig represents the configuration of a Kafka client.
type KafkaConfig struct {
	Brokers  []string
	SASL     *KafkaSASLConfig
	Consumer *KafkaConsumerConfig
	Producer *KafkaProducerConfig
	TLSDir   string
}

// KafkaSASLConfig represents the configuration of a Kafka consumer.
type KafkaSASLConfig struct {
	Username string
	Password string
	Protocol string
}

// KafkaConsumerConfig represents the configuration of a Kafka consumer.
type KafkaConsumerConfig struct {
	Enabled bool
	Topics  []string
	// GroupID is the Kafka consumer group to join. Defaults to
	// "<service name>-foundation", which is wrong as soon as an application
	// runs two workers that read different topics: they end up in one group
	// and take partitions away from each other on every rebalance.
	GroupID string
}

// KafkaProducerConfig represents the configuration of a Kafka producer.
type KafkaProducerConfig struct {
	Enabled   bool
	BatchSize int
	// BatchFlushInterval is how long the writer waits for a batch to fill
	// before sending it. Accepts a duration (`50ms`); a bare number in the
	// environment is read as seconds, as it was before.
	//
	// It replaces a `BatchTimeout int` field that meant seconds. Renaming it
	// rather than changing its type is deliberate: `BatchTimeout = 5` would
	// still have compiled against a time.Duration and quietly become five
	// nanoseconds, turning off batching with no error and no log line.
	BatchFlushInterval time.Duration
	// AllowAutoTopicCreation lets a write to an unknown topic create it.
	// Defaults to true outside production, where a typo in a topic name should
	// fail rather than quietly create a topic.
	AllowAutoTopicCreation bool
}

// MetricsConfig represents the configuration of a metrics server.
type MetricsConfig struct {
	Enabled bool
	Port    int
}

// SentryConfig represents the configuration of a Sentry client.
type SentryConfig struct {
	DSN     string
	Enabled bool
	// Environment separates events coming from development, test and
	// production. Defaults to FOUNDATION_ENV.
	Environment string
	// Release is reported with every event. Defaults to SENTRY_RELEASE.
	Release string
	// FlushTimeout is how long the service waits for buffered events to reach
	// Sentry before exiting.
	FlushTimeout time.Duration
}

// OutboxConfig represents the configuration of an outbox.
type OutboxConfig struct {
	Enabled bool
}

// RedisConfig represents the configuration of a Redis client.
type RedisConfig struct {
	Enabled bool
	URL     string
}

// JobsEnqueuerConfig represents the configuration of a jobs enqueuer.
type JobsEnqueuerConfig struct {
	Enabled   bool
	URL       string
	Pool      int
	Namespace string
}

// NewConfig returns a new Config with values populated from environment variables.
func NewConfig() *Config {
	return &Config{
		Database: &DatabaseConfig{
			Enabled: len(GetEnvOrString("DATABASE_URL", "")) > 0,
			Pool:    GetEnvOrInt("DATABASE_POOL", 5),
			URL:     GetEnvOrString("DATABASE_URL", ""),
		},
		EventsWorker: &EventsWorkerConfig{
			ErrorsTopic:   GetEnvOrString("EVENTS_WORKER_ERRORS_TOPIC", ""),
			DeliverErrors: GetEnvOrBool("EVENTS_WORKER_DELIVER_ERRORS", true),
		},
		GRPC: &GRPCConfig{
			TLSDir: GetEnvOrString("GRPC_TLS_DIR", ""),
		},
		Kafka: &KafkaConfig{
			Brokers: GetEnvOrStrings("KAFKA_BROKERS", ",", nil),
			SASL: &KafkaSASLConfig{
				Username: GetEnvOrString("KAFKA_SASL_USERNAME", ""),
				Password: GetEnvOrString("KAFKA_SASL_PASSWORD", ""),
				Protocol: GetEnvOrString("KAFKA_SASL_PROTOCOL", ""),
			},
			Consumer: &KafkaConsumerConfig{
				Enabled: false,
				Topics:  nil,
				GroupID: GetEnvOrString("KAFKA_CONSUMER_GROUP", ""),
			},
			Producer: &KafkaProducerConfig{
				Enabled:                false,
				BatchSize:              GetEnvOrInt("KAFKA_PRODUCER_BATCH_SIZE", 1),
				BatchFlushInterval:     GetEnvOrDurationSeconds("KAFKA_PRODUCER_BATCH_TIMEOUT", time.Second),
				AllowAutoTopicCreation: GetEnvOrBool("KAFKA_ALLOW_AUTO_TOPIC_CREATION", !IsProductionEnv()),
			},
			TLSDir: GetEnvOrString("KAFKA_TLS_DIR", ""),
		},
		Metrics: &MetricsConfig{
			Enabled: GetEnvOrBool("METRICS_ENABLED", true),
			Port:    GetEnvOrInt("METRICS_PORT", 51077),
		},
		Outbox: &OutboxConfig{
			Enabled: false,
		},
		Redis: &RedisConfig{
			Enabled: len(GetEnvOrString("REDIS_URL", "")) > 0,
			URL:     GetEnvOrString("REDIS_URL", ""),
		},
		Sentry: &SentryConfig{
			DSN:          GetEnvOrString("SENTRY_DSN", ""),
			Enabled:      len(GetEnvOrString("SENTRY_DSN", "")) > 0,
			Environment:  GetEnvOrString("SENTRY_ENVIRONMENT", string(FoundationEnv())),
			Release:      GetEnvOrString("SENTRY_RELEASE", ""),
			FlushTimeout: GetEnvOrDuration("SENTRY_FLUSH_TIMEOUT", fsentry.DefaultFlushTimeout),
		},
		JobsEnqueuer: &JobsEnqueuerConfig{
			Enabled:   false,
			URL:       GetEnvOrString("REDIS_URL", ""),
			Pool:      GetEnvOrInt("REDIS_POOL", 5),
			Namespace: GetEnvOrString("REDIS_NAMESPACE", ""),
		},
		HTTP:               NewHTTPConfig(),
		ShutdownTimeout:    GetEnvOrDuration("SHUTDOWN_TIMEOUT", DefaultShutdownTimeout),
		DrainDelay:         GetEnvOrDuration("DRAIN_DELAY", 0),
		HealthCheckTimeout: GetEnvOrDuration("HEALTH_CHECK_TIMEOUT", DefaultHealthCheckTimeout),
		LogPayloads:        GetEnvOrBool("LOG_PAYLOADS", false),
	}
}

// shutdownTimeout returns the configured shutdown budget, falling back to the
// default for services constructed without a Config.
func (s *Service) shutdownTimeout() time.Duration {
	if s.Config != nil && s.Config.ShutdownTimeout > 0 {
		return s.Config.ShutdownTimeout
	}

	return DefaultShutdownTimeout
}

// Init initializes the Foundation service.
func Init(name string) *Service {
	LoadEnv()

	return &Service{
		Name:   name,
		Config: NewConfig(),
		Logger: initLogger(name),
	}
}

// StartComponentsOption is an option to `StartComponents`.
type StartComponentsOption func(*Service)

// WithKafkaConsumer sets the Kafka consumer enabled flag.
func WithKafkaConsumer() StartComponentsOption {
	return func(s *Service) {
		s.Config.Kafka.Consumer.Enabled = true
	}
}

// WithKafkaProducer sets the Kafka producer enabled flag.
func WithKafkaProducer() StartComponentsOption {
	return func(s *Service) {
		s.Config.Kafka.Producer.Enabled = true
	}
}

// WithKafkaConsumerTopics sets the Kafka consumer topics.
func WithKafkaConsumerTopics(topics ...string) StartComponentsOption {
	return func(s *Service) {
		s.Config.Kafka.Consumer.Topics = topics
	}
}

// WithOutbox sets the outbox enabled flag.
func WithOutbox() StartComponentsOption {
	return func(s *Service) {
		s.Config.Outbox.Enabled = true
	}
}

// WithRedis sets the redis enabled flag.
func WithRedis() StartComponentsOption {
	return func(s *Service) {
		s.Config.Redis.Enabled = true
	}
}

// WithJobsEnqueuer sets the jobs enqueuer enabled flag.
func WithJobsEnqueuer() StartComponentsOption {
	return func(s *Service) {
		s.Config.JobsEnqueuer.Enabled = true
	}
}

// warnAboutPlaintextSASL points out that SASL credentials are about to travel
// in the clear. SASL/PLAIN over a plaintext connection sends the password
// verbatim.
func (s *Service) warnAboutPlaintextSASL() {
	if s.Config.Kafka.TLSDir != "" {
		return
	}

	s.Logger.Warn(
		"Kafka SASL is configured without TLS (KAFKA_TLS_DIR is empty): credentials will be " +
			"sent over an unencrypted connection",
	)
}

func (s *Service) addSystemComponents() error {
	// Remove user-defined components in order to add system components first.
	existedComponents := s.Components
	s.Components = []Component{}

	// Sentry
	if s.Config.Sentry.Enabled {
		s.Components = append(s.Components, fsentry.NewComponent(
			s.Config.Sentry.DSN,
			fsentry.WithEnvironment(s.Config.Sentry.Environment),
			fsentry.WithRelease(s.Config.Sentry.Release),
			fsentry.WithFlushTimeout(s.Config.Sentry.FlushTimeout),
		))
	}

	// PostgreSQL
	if s.Config.Database.Enabled {
		s.Components = append(s.Components, fpg.NewComponent(
			fpg.WithDatabaseURL(s.Config.Database.URL),
			fpg.WithLogger(s.Logger),
			fpg.WithPoolSize(s.Config.Database.Pool),
		))
	}

	// Kafka consumer
	if s.Config.Kafka.Consumer.Enabled {
		consumerOptions := []fkafka.ConsumerComponentOption{
			fkafka.WithConsumerAppName(s.Name),
			fkafka.WithConsumerBrokers(s.Config.Kafka.Brokers),
			fkafka.WithConsumerLogger(s.Logger),
			fkafka.WithConsumerTLSDir(s.Config.Kafka.TLSDir),
			fkafka.WithConsumerTopics(s.Config.Kafka.Consumer.Topics),
			fkafka.WithConsumerGroupID(s.Config.Kafka.Consumer.GroupID),
		}

		if s.Config.Kafka.SASL.Username != "" && s.Config.Kafka.SASL.Password != "" {
			s.warnAboutPlaintextSASL()

			saslComponent, err := fkafka.WithSASLMechanism(s.Config.Kafka.SASL.Protocol, s.Config.Kafka.SASL.Username, s.Config.Kafka.SASL.Password)
			if err != nil {
				return err
			}

			consumerOptions = append(consumerOptions, saslComponent)
		}

		s.Components = append(s.Components, fkafka.NewConsumerComponent(consumerOptions...))
	}

	// Kafka producer
	if s.Config.Kafka.Producer.Enabled {
		producerOptions := []fkafka.ProducerComponentOption{
			fkafka.WithProducerBrokers(s.Config.Kafka.Brokers),
			fkafka.WithProducerLogger(s.Logger),
			fkafka.WithProducerTLSDir(s.Config.Kafka.TLSDir),
			fkafka.WithProducerBatchSize(s.Config.Kafka.Producer.BatchSize),
			fkafka.WithProducerBatchTimeout(s.Config.Kafka.Producer.BatchFlushInterval),
			fkafka.WithProducerAllowAutoTopicCreation(s.Config.Kafka.Producer.AllowAutoTopicCreation),
		}

		if s.Config.Kafka.SASL.Username != "" && s.Config.Kafka.SASL.Password != "" {
			s.warnAboutPlaintextSASL()

			producerSASLComponent, err := fkafka.WithProducerSASLMechanism(s.Config.Kafka.SASL.Protocol, s.Config.Kafka.SASL.Username, s.Config.Kafka.SASL.Password)
			if err != nil {
				return err
			}

			producerOptions = append(producerOptions, producerSASLComponent)
		}

		s.Components = append(s.Components, fkafka.NewProducerComponent(producerOptions...))
	}

	// Metrics server
	if s.Config.Metrics.Enabled {
		s.Components = append(s.Components, NewMetricsServerComponent(
			WithMetricsServerHealthHandler(s.healthHandler),
			WithMetricsServerLivenessHandler(s.livenessHandler),
			WithMetricsServerReadinessHandler(s.readinessHandler),
			WithMetricsServerLogger(s.Logger),
			WithMetricsServerPort(s.Config.Metrics.Port),
			WithMetricsServerHTTPConfig(s.httpConfig()),
		))
	}

	// Redis
	if s.Config.Redis.Enabled {
		s.Components = append(s.Components, fredis.NewComponent(
			fredis.WithLogger(s.Logger),
			fredis.WithURL(s.Config.Redis.URL),
		))
	}

	if s.Config.JobsEnqueuer.Enabled {
		redisPool, err := BuildRedisPool(s.Config.JobsEnqueuer.URL, s.Config.JobsEnqueuer.Pool)
		if err != nil {
			return fmt.Errorf("failed to initialize redis pool: %w", err)
		}

		s.Components = append(s.Components, fjobs.NewComponent(
			fjobs.WithLogger(s.Logger),
			fjobs.WithRedisPool(redisPool),
			fjobs.WithNamespace(s.Config.JobsEnqueuer.Namespace),
		))
	}

	// Add user-defined components back
	s.Components = append(s.Components, existedComponents...)

	return nil
}

// StartComponents starts the default Foundation service components.
func (s *Service) StartComponents(opts ...StartComponentsOption) error {
	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	if err := s.addSystemComponents(); err != nil {
		return err
	}

	s.Logger.Info("Starting components:")

	started := make([]Component, 0, len(s.Components))

	for _, component := range s.Components {
		s.Logger.Infof(" - %s", component.Name())

		if err := component.Start(); err != nil {
			// Unwind what is already running. Otherwise a service that fails to
			// start halfway through leaves database connections, Kafka readers
			// and listening sockets behind it.
			s.stopComponents(started)

			return fmt.Errorf("%s: %w", component.Name(), err)
		}

		started = append(started, component)
	}

	return nil
}

// stopSignal returns the channel closed when a shutdown is requested.
func (s *Service) stopSignal() <-chan struct{} {
	s.stopRequestedOnce.Do(func() {
		s.stopRequested = make(chan struct{})
	})

	return s.stopRequested
}

// Shutdown asks the service to stop.
//
// Unlike SIGTERM it does not go through the drain delay: a programmatic
// shutdown is a decision the service has already made — an events worker with
// ShutdownOnError hitting a message it must not skip past, for instance — and
// waiting out a delay meant for load balancers would let it keep doing the very
// thing it stopped for.
//
// It is safe to call before Start and to call more than once.
func (s *Service) Shutdown() {
	s.stopRequestedOnce.Do(func() {
		s.stopRequested = make(chan struct{})
	})

	s.stopOnce.Do(func() {
		close(s.stopRequested)
	})
}

// beginDraining marks the service unavailable to the readiness probe.
func (s *Service) beginDraining() {
	s.draining.Store(true)
	fmetrics.SetDraining(true)
}

// StopComponents stops the default Foundation service components.
//
// It is bounded by Config.ShutdownTimeout: a component that refuses to stop
// must not keep the process alive until the supervisor kills it, because then
// nothing after this point — flushing Sentry, for one — gets to run.
func (s *Service) StopComponents() {
	done := make(chan struct{})

	go func() {
		defer close(done)

		s.stopComponents(s.Components)
	}()

	timeout := s.shutdownTimeout()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		s.Logger.Errorf("Components did not stop within %s; exiting anyway", timeout)
	}
}

// stopComponents stops the given components in reverse order, so that
// dependents go down before their dependencies.
func (s *Service) stopComponents(components []Component) {
	if len(components) == 0 {
		return
	}

	s.Logger.Info("Stopping components:")

	for i := len(components) - 1; i >= 0; i-- {
		s.stopComponent(components[i])
	}
}

// stopComponent stops a single component, containing both errors and panics:
// one misbehaving component must not prevent the rest from shutting down.
func (s *Service) stopComponent(component Component) {
	name := component.Name()

	defer func() {
		if r := recover(); r != nil {
			s.CaptureError(
				fmt.Errorf("panic while stopping component `%s`: %v", name, r),
				"",
			)
		}
	}()

	s.Logger.Infof(" - %s", name)

	if err := component.Stop(); err != nil {
		s.CaptureError(err, fmt.Sprintf("failed to stop component `%s`", name))
	}
}

type StartOptions struct {
	ModeName               string
	StartComponentsOptions []StartComponentsOption
	ServiceFunc            func(ctx context.Context) error
}

// Start runs the Foundation service.
func (s *Service) Start(opts *StartOptions) {
	s.ModeName = opts.ModeName

	// Set running mode to logger
	s.Logger = s.Logger.WithField("mode", s.ModeName)

	// Log application startup
	s.logStartup()

	fmetrics.Info.WithLabelValues(s.Name, s.ModeName, string(FoundationEnv()), Version).Set(1)

	// Tracing is initialised for every mode. It used to be set up inside the
	// gateway alone, so a trace stopped at the first hop: gRPC services and
	// every worker were invisible.
	tracingShutdown := s.initTracing()
	defer tracingShutdown()

	// Start common components
	if err := s.StartComponents(opts.StartComponentsOptions...); err != nil {
		s.Fatal(err, "failed to start components")
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The service context lags the signal by the drain delay. Readiness starts
	// failing the moment the signal arrives, but servers keep accepting for a
	// little longer, so that load balancers notice before connections are
	// refused rather than after.
	serviceCtx, stopServing := context.WithCancel(context.WithoutCancel(signalCtx))
	defer stopServing()

	go func() {
		select {
		case <-signalCtx.Done():
			s.beginDraining()

			// A programmatic Shutdown during the drain cuts it short.
			if delay := s.drainDelay(); delay > 0 {
				s.Logger.Infof("Draining for %s before shutting down", delay)

				timer := time.NewTimer(delay)
				defer timer.Stop()

				select {
				case <-timer.C:
				case <-s.stopSignal():
				}
			}
		case <-s.stopSignal():
			// Requested from inside the service: stop now.
			s.beginDraining()
		}

		stopServing()
	}()

	// Run the actual service code
	if err := opts.ServiceFunc(serviceCtx); err != nil {
		s.Fatal(err, "failed to start service")
	}

	<-serviceCtx.Done()
	s.Logger.Println("Shutting down service...")

	s.StopComponents()

	s.Logger.Println("Service gracefully stopped")
}
