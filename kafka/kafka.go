package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/segmentio/kafka-go"
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

type ConsumerComponent struct {
	Consumer *kafka.Reader

	appName string
	brokers []string
	logger  *logrus.Entry
	topics  []string
	tlsDir  string
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

// WithConsumerTLSDir sets the location of the TLS directory for the ConsumerComponent
func WithConsumerTLSDir(tlsDir string) ConsumerComponentOption {
	return func(c *ConsumerComponent) {
		c.tlsDir = tlsDir
	}
}

// NewConsumerComponent returns a new ConsumerComponent
func NewConsumerComponent(opts ...ConsumerComponentOption) *ConsumerComponent {
	c := &ConsumerComponent{}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Start implements the Component interface.
func (c *ConsumerComponent) Start() error {
	if len(c.topics) == 0 {
		return errors.New("you must specify topics during the application initialization using the `WithKafkaConsumerTopics`")
	}
	c.logger.Debugf("Kafka consumer topics: %v", c.topics)

	config := kafka.ReaderConfig{
		Brokers:     c.brokers,
		GroupID:     fmt.Sprintf("%s-foundation", c.appName),
		GroupTopics: c.topics,
		ErrorLogger: c.logger,
	}

	dialer, err := newDialer(c.tlsDir)
	if err != nil {
		return err
	}

	config.Dialer = dialer

	consumer := kafka.NewReader(config)

	c.Consumer = consumer

	return nil
}

// Stop implements the Component interface.
func (c *ConsumerComponent) Stop() error {
	return c.Consumer.Close()
}

// Health implements the Component interface.
func (c *ConsumerComponent) Health() error {
	if c.Consumer == nil {
		return errors.New("reader is not initialized")
	}

	// TODO: find a way to check the health of the consumer. Maybe by adding the metadata fet

	return nil
}

// Name implements the Component interface.
func (c *ConsumerComponent) Name() string {
	return ConsumerComponentName
}

// ProducerComponent represents a Kafka producer component
type ProducerComponent struct {
	Producer *kafka.Writer

	brokers []string
	logger  *logrus.Entry
	tlsDir  string
}

// ProducerComponentOption represents an option for the ProducerComponent
type ProducerComponentOption func(*ProducerComponent)

// WithProducerBrokers sets the brokers for the ProducerComponent
func WithProducerBrokers(brokers []string) ProducerComponentOption {
	return func(c *ProducerComponent) {
		c.brokers = brokers
	}
}

// WithProducerLogger sets the logger for the ProducerComponent
func WithProducerLogger(logger *logrus.Entry) ProducerComponentOption {
	return func(c *ProducerComponent) {
		c.logger = logger
	}
}

// WithProducerTLSDir sets the location of the TLS files for the ProducerComponent
func WithProducerTLSDir(tlsDir string) ProducerComponentOption {
	return func(c *ProducerComponent) {
		c.tlsDir = tlsDir
	}
}

// NewProducerComponent returns a new ProducerComponent
func NewProducerComponent(opts ...ProducerComponentOption) *ProducerComponent {
	c := &ProducerComponent{}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Start implements the Component interface.
func (c *ProducerComponent) Start() error {
	transport, err := newTransport(c.tlsDir)
	if err != nil {
		return err
	}

	producer := &kafka.Writer{
		Addr:                   kafka.TCP(c.brokers...),
		AllowAutoTopicCreation: true,
		BatchSize:              1,               // TODO: make this configurable
		BatchTimeout:           1 * time.Second, // TODO: make this configurable
		Logger:                 c.logger,
		Transport:              transport,
	}

	c.Producer = producer

	return nil
}

// Stop implements the Component interface.
func (c *ProducerComponent) Stop() error {
	c.Producer.Close()

	return nil
}

// Health implements the Component interface.
func (c *ProducerComponent) Health() error {
	if c.Producer == nil {
		return errors.New("writer is not initialized")
	}

	return nil
}

// Name implements the Component interface.
func (c *ProducerComponent) Name() string {
	return ProducerComponentName
}

func newDialer(tlsDir string) (*kafka.Dialer, error) {
	if tlsDir == "" {
		return nil, nil
	}

	tlsConfig, err := newTLSConfig(tlsDir)
	if err != nil {
		return nil, err
	}

	return &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
		TLS:       tlsConfig,
	}, nil
}

func newTransport(tlsDir string) (*kafka.Transport, error) {
	if tlsDir == "" {
		return nil, nil
	}

	tlsConfig, err := newTLSConfig(tlsDir)
	if err != nil {
		return nil, err
	}

	return &kafka.Transport{
		TLS: tlsConfig,
	}, nil
}

func newTLSConfig(dir string) (*tls.Config, error) {
	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")
	caFile := filepath.Join(dir, "ca.crt")

	tlsConfig := &tls.Config{}

	// Load client cert
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	tlsConfig.Certificates = []tls.Certificate{cert}

	// Load CA cert
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	tlsConfig.RootCAs = caCertPool

	return tlsConfig, nil
}
