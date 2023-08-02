package kafka

import (
	"errors"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
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

const (
	healthCheckTimeout = 1 * time.Second
)

type ConsumerComponent struct {
	Consumer *kafka.Consumer

	appName string
	brokers string
	logger  *logrus.Entry
	topics  []string
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
func WithConsumerBrokers(brokers string) ConsumerComponentOption {
	return func(c *ConsumerComponent) {
		c.brokers = brokers
	}
}

// WithConsumerLogger sets the logger for the ConsumerComponent
func WithConsumerLogger(logger *logrus.Entry) ConsumerComponentOption {
	return func(c *ConsumerComponent) {
		c.logger = logger
	}
}

// WithConsumerTopics sets the topics for the ConsumerComponent
func WithConsumerTopics(topics []string) ConsumerComponentOption {
	return func(c *ConsumerComponent) {
		c.topics = topics
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

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  c.brokers,
		"group.id":           fmt.Sprintf("%s-consumer", c.appName),
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": false,
	})
	if err != nil {
		return err
	}

	c.logger.Debugf("Kafka consumer topics: %v", c.topics)

	if err = consumer.SubscribeTopics(c.topics, nil); err != nil {
		return err
	}

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
		return errors.New("consumer is not initialized")
	}

	if _, err := c.Consumer.GetMetadata(nil, true, 1000); err != nil {
		return err
	}

	return nil
}

// Name implements the Component interface.
func (c *ConsumerComponent) Name() string {
	return ConsumerComponentName
}

// ProducerComponent represents a Kafka producer component
type ProducerComponent struct {
	Producer *kafka.Producer

	brokers string
	logger  *logrus.Entry
}

// ProducerComponentOption represents an option for the ProducerComponent
type ProducerComponentOption func(*ProducerComponent)

// WithProducerBrokers sets the brokers for the ProducerComponent
func WithProducerBrokers(brokers string) ProducerComponentOption {
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
	producer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": c.brokers})
	if err != nil {
		return err
	}

	// Test the connection
	if _, err = producer.GetMetadata(nil, true, int(healthCheckTimeout.Milliseconds())); err != nil {
		return err
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
		return errors.New("producer is not initialized")
	}

	if _, err := c.Producer.GetMetadata(nil, true, int(healthCheckTimeout.Milliseconds())); err != nil {
		return err
	}

	return nil
}

// Name implements the Component interface.
func (c *ProducerComponent) Name() string {
	return ProducerComponentName
}
