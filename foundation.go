package foundation

import (
	"database/sql"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

const Version = "0.0.1"

// Application represents a Foundation application.
type Application struct {
	Name string

	// PG is a PostgreSQL connection pool.
	PG *sql.DB

	// KafkaConsumer is a Kafka consumer group.
	KafkaConsumer       *kafka.Consumer
	kafkaConsumerTopics []string
	// KafkaProducer is a Kafka producer.
	KafkaProducer *kafka.Producer
}

// Init initializes the Foundation application.
func Init(name string) *Application {
	initLogging()

	return &Application{
		Name:                name,
		kafkaConsumerTopics: []string{},
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
	if GetEnvOrBool("DATABASE_ENABLED", false) {
		if err := app.connectToPostgreSQL(); err != nil {
			return fmt.Errorf("postgresql: %w", err)
		}
	}

	// Kafka consumer
	if GetEnvOrBool("KAFKA_CONSUMER_ENABLED", false) {
		if err := app.connectKafkaConsumer(); err != nil {
			return fmt.Errorf("kafka consumer: %w", err)
		}
	}

	// Kafka producer
	if GetEnvOrBool("KAFKA_PRODUCER_ENABLED", false) {
		if err := app.connectKafkaProducer(); err != nil {
			return fmt.Errorf("kafka producer: %w", err)
		}
	}

	// Metrics
	go StartMetricsServer()

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
