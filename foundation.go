package foundation

import (
	"database/sql"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/sirupsen/logrus"
)

const Version = "0.0.1"

// Application represents a Foundation application.
type Application struct {
	Name string

	PG              *sql.DB
	DatabaseEnabled bool

	KafkaConsumer        *kafka.Consumer
	KafkaConsumerEnabled bool
	kafkaConsumerTopics  []string
	KafkaProducer        *kafka.Producer
	KafkaProducerEnabled bool

	Logger *logrus.Entry
}

// Init initializes the Foundation application.
func Init(name string) *Application {
	return &Application{
		Name:                 name,
		kafkaConsumerTopics:  []string{},
		Logger:               initLogger(name),
		DatabaseEnabled:      GetEnvOrBool("DATABASE_ENABLED", false),
		KafkaConsumerEnabled: GetEnvOrBool("KAFKA_CONSUMER_ENABLED", false),
		KafkaProducerEnabled: GetEnvOrBool("KAFKA_PRODUCER_ENABLED", false),
	}
}

// StartComponentsOption is an option to `StartComponents`.
type StartComponentsOption func(*Application)

// StartComponents starts the default Foundation application components.
func (app *Application) StartComponents(opts ...StartComponentsOption) error {
	// Apply options
	for _, opt := range opts {
		opt(app)
	}

	// PostgreSQL
	if app.DatabaseEnabled {
		if err := app.connectToPostgreSQL(); err != nil {
			return fmt.Errorf("postgresql: %w", err)
		}
	}

	// Kafka consumer
	if app.KafkaConsumerEnabled {
		if err := app.connectKafkaConsumer(); err != nil {
			return fmt.Errorf("kafka consumer: %w", err)
		}
	}

	// Kafka producer
	if app.KafkaProducerEnabled {
		if err := app.connectKafkaProducer(); err != nil {
			return fmt.Errorf("kafka producer: %w", err)
		}
	}

	// Metrics
	go app.StartMetricsServer()

	return nil
}

// StopComponents stops the default Foundation application components.
func (app *Application) StopComponents() {
	// PostgreSQL
	if app.PG != nil {
		app.PG.Close()
	}

	// Kafka consumer
	if app.KafkaConsumer != nil {
		app.KafkaConsumer.Close()
	}

	// Kafka producer
	if app.KafkaProducer != nil {
		app.KafkaProducer.Close()
	}
}
