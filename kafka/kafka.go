package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"github.com/sirupsen/logrus"
)

const (
	HeaderCorrelationID = "correlation-id"
	HeaderOriginatorID  = "originator-id"
	HeaderProtoName     = "proto-name"
)

const (
	ConsumerComponentName = "kafka-consumer"
	ProducerComponentName = "kafka-producer"
)

// DefaultHealthTimeout bounds a health check made without a context.
const DefaultHealthTimeout = 2 * time.Second

// newHealthClient builds the client used to probe broker reachability.
func newHealthClient(brokers []string, transport kafka.RoundTripper) *kafka.Client {
	return &kafka.Client{
		Addr:      kafka.TCP(brokers...),
		Timeout:   DefaultHealthTimeout,
		Transport: transport,
	}
}

// pingBrokers performs the cheapest possible round-trip to a broker. Unlike a
// metadata request it does not enumerate topics, so it stays cheap on large
// clusters even when a probe runs every few seconds.
func pingBrokers(ctx context.Context, client *kafka.Client) error {
	if client == nil {
		return errors.New("kafka client is not initialized")
	}

	resp, err := client.ApiVersions(ctx, &kafka.ApiVersionsRequest{})
	if err != nil {
		return fmt.Errorf("kafka brokers are unreachable: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("kafka brokers reported an error: %w", resp.Error)
	}

	return nil
}

// validateBrokers rejects a broker list that cannot possibly work.
//
// strings.Split("", ",") yields a slice holding one empty string, so an unset
// KAFKA_BROKERS used to reach kafka-go as a single broker with an empty
// address, which it retried against forever instead of failing at startup.
func validateBrokers(brokers []string) error {
	if len(brokers) == 0 {
		return errors.New("no Kafka brokers configured: set KAFKA_BROKERS")
	}

	for _, broker := range brokers {
		if strings.TrimSpace(broker) == "" {
			return fmt.Errorf("invalid Kafka broker list %v: contains an empty address", brokers)
		}
	}

	return nil
}

// defaultLogger returns a usable logger for components built without one.
func defaultLogger(logger *logrus.Entry, componentName string) *logrus.Entry {
	if logger == nil {
		return logrus.NewEntry(logrus.StandardLogger()).WithField("component", componentName)
	}

	return logger
}

type ConsumerComponent struct {
	Consumer *kafka.Reader

	appName       string
	brokers       []string
	logger        *logrus.Entry
	saslMechanism sasl.Mechanism
	topics        []string
	tlsDir        string
	group         string
	healthClient  *kafka.Client
}

// ConsumerComponentOption represents an option for the ConsumerComponent
type ConsumerComponentOption func(*ConsumerComponent)

// WithConsumerAppName sets the app name for the ConsumerComponent
func WithConsumerAppName(appName string) ConsumerComponentOption {
	return func(c *ConsumerComponent) {
		c.appName = appName
	}
}

// WithConsumerBrokers sets the brokers for the ConsumerComponent
func WithConsumerBrokers(brokers []string) ConsumerComponentOption {
	return func(c *ConsumerComponent) {
		c.brokers = brokers
	}
}

// WithConsumerLogger sets the logger for the ConsumerComponent
func WithConsumerLogger(logger *logrus.Entry) ConsumerComponentOption {
	return func(c *ConsumerComponent) {
		c.logger = logger.WithField("component", ConsumerComponentName)
	}
}

// WithConsumerTopics sets the topics for the ConsumerComponent
func WithConsumerTopics(topics []string) ConsumerComponentOption {
	return func(c *ConsumerComponent) {
		c.topics = topics
	}
}

// WithConsumerGroupID overrides the Kafka consumer group the component joins.
//
// It defaults to "<app name>-foundation", which puts every events worker of an
// application into the same group: two workers reading different topics then
// fight over partition assignment on every rebalance.
func WithConsumerGroupID(groupID string) ConsumerComponentOption {
	return func(c *ConsumerComponent) {
		c.group = groupID
	}
}

// WithConsumerTLSDir sets the location of the TLS directory for the ConsumerComponent
func WithConsumerTLSDir(tlsDir string) ConsumerComponentOption {
	return func(c *ConsumerComponent) {
		c.tlsDir = tlsDir
	}
}

// WithSASLMechanism sets the sasl mechanism for the ConsumerComponent
func WithSASLMechanism(protocol, username, password string) (ConsumerComponentOption, error) {
	mechanism, err := newSASLMechanism(protocol, username, password)
	if err != nil {
		return nil, err
	}

	return func(c *ConsumerComponent) {
		c.saslMechanism = mechanism
	}, err
}

// NewConsumerComponent returns a new ConsumerComponent
func NewConsumerComponent(opts ...ConsumerComponentOption) *ConsumerComponent {
	c := &ConsumerComponent{}

	for i := range opts {
		opts[i](c)
	}

	return c
}

// log returns the component logger, or a usable default.
func (c *ConsumerComponent) log() *logrus.Entry {
	return defaultLogger(c.logger, ConsumerComponentName)
}

// groupID returns the consumer group the reader joins.
func (c *ConsumerComponent) groupID() string {
	if c.group != "" {
		return c.group
	}

	return fmt.Sprintf("%s-foundation", c.appName)
}

// Start implements the Component interface.
func (c *ConsumerComponent) Start() error {
	if len(c.topics) == 0 {
		return errors.New("you must specify topics during the application initialization using the `WithKafkaConsumerTopics`")
	}

	if err := validateBrokers(c.brokers); err != nil {
		return err
	}

	c.log().Debugf("Kafka consumer topics: %v", c.topics)

	config := kafka.ReaderConfig{
		Brokers:     c.brokers,
		GroupID:     c.groupID(),
		GroupTopics: c.topics,
		ErrorLogger: c.log(),
	}

	dialer, err := newDialer(c.tlsDir, c.saslMechanism)
	if err != nil {
		return err
	}

	config.Dialer = dialer

	transport, err := newTransport(c.tlsDir, c.saslMechanism)
	if err != nil {
		return err
	}

	c.healthClient = newHealthClient(c.brokers, transport)
	c.Consumer = kafka.NewReader(config)

	return nil
}

// Stop implements the Component interface.
func (c *ConsumerComponent) Stop() error {
	if c.Consumer == nil {
		return nil
	}

	return c.Consumer.Close()
}

// Health implements the Component interface.
func (c *ConsumerComponent) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHealthTimeout)
	defer cancel()

	return c.HealthContext(ctx)
}

// HealthContext implements the HealthCheckerContext interface.
func (c *ConsumerComponent) HealthContext(ctx context.Context) error {
	if c.Consumer == nil {
		return errors.New("reader is not initialized")
	}

	// This used to be a TODO that always returned nil, so an unreachable Kafka
	// never showed up in the readiness probe.
	return pingBrokers(ctx, c.healthClient)
}

// Name implements the Component interface.
func (c *ConsumerComponent) Name() string {
	return ConsumerComponentName
}

// ProducerComponent represents a Kafka producer component
type ProducerComponent struct {
	Producer *kafka.Writer

	brokers              []string
	logger               *logrus.Entry
	tlsDir               string
	batchSize            int
	batchTimeout         time.Duration
	saslMechanism        sasl.Mechanism
	allowAutoTopicCreate bool
	healthClient         *kafka.Client
}

// ProducerComponentOption represents an option for the ProducerComponent
type ProducerComponentOption func(*ProducerComponent)

// WithProducerSASLMechanism sets the sasl mechanism for the ProducerComponent
func WithProducerSASLMechanism(protocol, username, password string) (ProducerComponentOption, error) {
	mechanism, err := newSASLMechanism(protocol, username, password)
	if err != nil {
		return nil, err
	}

	return func(c *ProducerComponent) {
		c.saslMechanism = mechanism
	}, err
}

// WithProducerBrokers sets the brokers for the ProducerComponent
func WithProducerBrokers(brokers []string) ProducerComponentOption {
	return func(c *ProducerComponent) {
		c.brokers = brokers
	}
}

// WithProducerLogger sets the logger for the ProducerComponent
func WithProducerLogger(logger *logrus.Entry) ProducerComponentOption {
	return func(c *ProducerComponent) {
		c.logger = logger.WithField("component", ProducerComponentName)
	}
}

// WithProducerTLSDir sets the location of the TLS files for the ProducerComponent
func WithProducerTLSDir(tlsDir string) ProducerComponentOption {
	return func(c *ProducerComponent) {
		c.tlsDir = tlsDir
	}
}

// WithProducerAllowAutoTopicCreation controls whether writing to an unknown
// topic creates it.
//
// Convenient in development, risky in production: a typo in a topic name
// silently creates a topic with the cluster's default partitioning instead of
// failing.
func WithProducerAllowAutoTopicCreation(allow bool) ProducerComponentOption {
	return func(c *ProducerComponent) {
		c.allowAutoTopicCreate = allow
	}
}

// WithProducerBatchSize sets the batching size for the ProducerComponent
func WithProducerBatchSize(batchSize int) ProducerComponentOption {
	return func(c *ProducerComponent) {
		c.batchSize = batchSize
	}
}

// WithProducerBatchTimeout sets the batching timeout for the ProducerComponent
func WithProducerBatchTimeout(batchTimeout time.Duration) ProducerComponentOption {
	return func(c *ProducerComponent) {
		c.batchTimeout = batchTimeout
	}
}

// NewProducerComponent returns a new ProducerComponent
func NewProducerComponent(opts ...ProducerComponentOption) *ProducerComponent {
	c := &ProducerComponent{}

	for i := range opts {
		opts[i](c)
	}

	return c
}

// log returns the component logger, or a usable default.
func (c *ProducerComponent) log() *logrus.Entry {
	return defaultLogger(c.logger, ProducerComponentName)
}

// Start implements the Component interface.
func (c *ProducerComponent) Start() error {
	if err := validateBrokers(c.brokers); err != nil {
		return err
	}

	transport, err := newTransport(c.tlsDir, c.saslMechanism)
	if err != nil {
		return err
	}

	c.healthClient = newHealthClient(c.brokers, transport)

	c.Producer = &kafka.Writer{
		Addr:                   kafka.TCP(c.brokers...),
		AllowAutoTopicCreation: c.allowAutoTopicCreate,
		BatchSize:              c.batchSize,
		BatchTimeout:           c.batchTimeout,
		Logger:                 c.log(),
		Transport:              transport,
		Balancer:               &kafka.Hash{}, // distribute messages to partitions based on the hash of the key, round-robin if no key
	}

	return nil
}

// Stop implements the Component interface.
func (c *ProducerComponent) Stop() error {
	if c.Producer == nil {
		return nil
	}

	return c.Producer.Close()
}

// Health implements the Component interface.
func (c *ProducerComponent) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHealthTimeout)
	defer cancel()

	return c.HealthContext(ctx)
}

// HealthContext implements the HealthCheckerContext interface.
func (c *ProducerComponent) HealthContext(ctx context.Context) error {
	if c.Producer == nil {
		return errors.New("writer is not initialized")
	}

	return pingBrokers(ctx, c.healthClient)
}

// Name implements the Component interface.
func (c *ProducerComponent) Name() string {
	return ProducerComponentName
}

func newDialer(tlsDir string, saslMechanism sasl.Mechanism) (*kafka.Dialer, error) {
	if tlsDir == "" && saslMechanism == nil {
		return nil, nil
	}

	var (
		tlsConfig *tls.Config
		err       error
	)

	if tlsDir != "" {
		tlsConfig, err = newTLSConfig(tlsDir)
	}

	if err != nil {
		return nil, err
	}

	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS:           tlsConfig,
		SASLMechanism: saslMechanism,
	}

	return dialer, nil
}

func newTransport(tlsDir string, saslMechanism sasl.Mechanism) (*kafka.Transport, error) {
	if tlsDir == "" && saslMechanism == nil {
		return &kafka.Transport{}, nil
	}

	var (
		tlsConfig *tls.Config
		err       error
	)

	if tlsDir != "" {
		tlsConfig, err = newTLSConfig(tlsDir)
	}

	if err != nil {
		return nil, err
	}

	return &kafka.Transport{
		TLS:  tlsConfig,
		SASL: saslMechanism,
	}, nil
}

// newTLSConfig loads the client certificate, key and CA from dir.
//
// The file names follow the Kubernetes TLS secret convention: `tls.crt`,
// `tls.key` and `ca.crt`.
func newTLSConfig(dir string) (*tls.Config, error) {
	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")
	caFile := filepath.Join(dir, "ca.crt")

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load the Kafka client certificate from %s: %w", dir, err)
	}

	// #nosec G304 -- the path is assembled from an operator-supplied
	// certificate directory, which is configuration rather than user input.
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read the Kafka CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	// The result was discarded, so a malformed CA file produced an empty pool
	// and every connection failed the handshake with nothing to suggest the CA
	// had simply failed to parse.
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("no certificates could be parsed from %s", caFile)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// newSASLMechanism return a SASL mechanism
// Available values for protocol are "plain" or "scram-sha-512"
func newSASLMechanism(protocol, username, password string) (sasl.Mechanism, error) {
	var (
		mechanism sasl.Mechanism
		err       error
	)

	switch strings.ToLower(protocol) {
	case "scram-sha-512":
		mechanism, err = scram.Mechanism(scram.SHA512, username, password)
	case "plain":
		mechanism = plain.Mechanism{
			Username: username,
			Password: password,
		}
	default:
		err = fmt.Errorf("unknown protocol %s. available values for protocol are \"plain\" or \"scram-sha-512\"", protocol)
	}

	return mechanism, err
}
