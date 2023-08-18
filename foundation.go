package foundation

import (
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"

	fkafka "github.com/ri-nat/foundation/kafka"
	fpg "github.com/ri-nat/foundation/postgresql"
	fredis "github.com/ri-nat/foundation/redis"
	fsentry "github.com/ri-nat/foundation/sentry"
)

const Version = "0.1.1"

// Service represents a single microservice - part of the bigger Foundation-based application, which implements
// an isolated domain of the application logic.
type Service struct {
	Name       string
	Config     *Config
	Components []Component
	ModeName   string

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
	// events worker handlers should be published.
	ErrorsTopic string

	// DeliverErrors determines whether errors from events worker handlers
	// should be published to the errors topic (and thus, delivered
	// to originator, aka user) or not.
	DeliverErrors bool
}

// GRPCConfig represents the configuration of a gRPC server.
type GRPCConfig struct {
	TLSDir string
}

// KafkaConfig represents the configuration of a Kafka client.
type KafkaConfig struct {
	Brokers  []string
	Consumer *KafkaConsumerConfig
	Producer *KafkaProducerConfig
	TLSDir   string
}

// KafkaConsumerConfig represents the configuration of a Kafka consumer.
type KafkaConsumerConfig struct {
	Enabled bool
	Topics  []string
}

// KafkaProducerConfig represents the configuration of a Kafka producer.
type KafkaProducerConfig struct {
	Enabled bool
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

// NewConfig returns a new Config with values populated from environment variables.
func NewConfig() *Config {
	return &Config{
		Database: &DatabaseConfig{
			Enabled: len(GetEnvOrString("DATABASE_URL", "")) > 0,
			Pool:    GetEnvOrInt("DATABASE_POOL", 5),
			URL:     GetEnvOrString("DATABASE_URL", ""),
		},
		EventsWorker: &EventsWorkerConfig{
			ErrorsTopic:   GetEnvOrString("EVENTS_WORKER_ERRORS_TOPIC", "foundation.events_worker.errors"),
			DeliverErrors: GetEnvOrBool("EVENTS_WORKER_DELIVER_ERRORS", true),
		},
		GRPC: &GRPCConfig{
			TLSDir: GetEnvOrString("GRPC_TLS_DIR", ""),
		},
		Kafka: &KafkaConfig{
			Brokers: strings.Split(GetEnvOrString("KAFKA_BROKERS", ""), ","),
			Consumer: &KafkaConsumerConfig{
				Enabled: false,
				Topics:  nil,
			},
			Producer: &KafkaProducerConfig{
				Enabled: false,
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
			DSN:     GetEnvOrString("SENTRY_DSN", ""),
			Enabled: len(GetEnvOrString("SENTRY_DSN", "")) > 0,
		},
	}
}

// Init initializes the Foundation service.
func Init(name string) *Service {
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

func (s *Service) addSystemComponents() error {
	// Remove user-defined components in order to add system components first.
	existedComponents := s.Components
	s.Components = []Component{}

	// Sentry
	if s.Config.Sentry.Enabled {
		s.Components = append(s.Components, fsentry.NewComponent(s.Config.Sentry.DSN))
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
		s.Components = append(s.Components, fkafka.NewConsumerComponent(
			fkafka.WithConsumerAppName(s.Name),
			fkafka.WithConsumerBrokers(s.Config.Kafka.Brokers),
			fkafka.WithConsumerLogger(s.Logger),
			fkafka.WithConsumerTLSDir(s.Config.Kafka.TLSDir),
			fkafka.WithConsumerTopics(s.Config.Kafka.Consumer.Topics),
		))
	}

	// Kafka producer
	if s.Config.Kafka.Producer.Enabled {
		s.Components = append(s.Components, fkafka.NewProducerComponent(
			fkafka.WithProducerBrokers(s.Config.Kafka.Brokers),
			fkafka.WithProducerLogger(s.Logger),
			fkafka.WithProducerTLSDir(s.Config.Kafka.TLSDir),
		))
	}

	// Metrics server
	if s.Config.Metrics.Enabled {
		s.Components = append(s.Components, NewMetricsServerComponent(
			WithMetricsServerHealthHandler(s.healthHandler),
			WithMetricsServerLogger(s.Logger),
			WithMetricsServerPort(s.Config.Metrics.Port),
		))
	}

	// Redis
	if s.Config.Redis.Enabled {
		s.Components = append(s.Components, fredis.NewComponent(
			fredis.WithLogger(s.Logger),
			fredis.WithURL(s.Config.Redis.URL),
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

	for _, component := range s.Components {
		s.Logger.Infof(" - %s", component.Name())

		if err := component.Start(); err != nil {
			return fmt.Errorf("%s: %w", component.Name(), err)
		}
	}

	return nil
}

// StopComponents stops the default Foundation service components.
func (s *Service) StopComponents() {
	s.Logger.Info("Stopping components:")

	// Stop components in reverse order, so that dependencies are stopped first
	for i := len(s.Components) - 1; i >= 0; i-- {
		s.Logger.Infof(" - %s", s.Components[i].Name())

		if err := s.Components[i].Stop(); err != nil {
			err = fmt.Errorf("failed to stop component `%s`: %w", s.Components[i].Name(), err)
			sentry.CaptureException(err)
			s.Logger.Error(err)
		}
	}
}

type StartOptions struct {
	ModeName               string
	StartComponentsOptions []StartComponentsOption
	ServiceFunc            func(ctx context.Context) error
}

// Start starts the Foundation service.
func (s *Service) Start(opts *StartOptions) {
	s.ModeName = opts.ModeName

	// Set running mode to logger
	s.Logger = s.Logger.WithField("mode", s.ModeName)

	// Log application startup
	s.logStartup()

	// Start common components
	if err := s.StartComponents(opts.StartComponentsOptions...); err != nil {
		err = fmt.Errorf("failed to start components: %w", err)
		sentry.CaptureException(err)
		s.Logger.Fatalf("Failed to start components: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Run the actual service code
	if err := opts.ServiceFunc(ctx); err != nil {
		err = fmt.Errorf("failed to start service: %w", err)
		sentry.CaptureException(err)
		s.Logger.Fatalf("Failed to start service: %v", err)
	}

	<-ctx.Done()
	s.Logger.Println("Shutting down service...")

	s.StopComponents()

	s.Logger.Println("Service gracefully stopped")
}
