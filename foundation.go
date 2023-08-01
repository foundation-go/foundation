package foundation

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/sirupsen/logrus"
)

const Version = "0.0.2"

// Application represents a Foundation application.
type Application struct {
	Name   string
	Config *Config

	PG *sql.DB

	KafkaConsumer *kafka.Consumer
	KafkaProducer *kafka.Producer

	Logger *logrus.Entry

	MetricsServer *http.Server
}

type Config struct {
	DatabaseEnabled      bool
	DatabasePool         int
	DatabaseURL          string
	KafkaBrokers         string
	KafkaConsumerEnabled bool
	KafkaConsumerTopics  []string
	KafkaProducerEnabled bool
	MetricsEnabled       bool
	MetricsPort          int
}

// NewConfig returns a new Config with values populated from environment variables.
func NewConfig() *Config {
	return &Config{
		DatabaseEnabled:      len(GetEnvOrString("DATABASE_URL", "")) > 0,
		DatabasePool:         GetEnvOrInt("DATABASE_POOL", 5),
		DatabaseURL:          GetEnvOrString("DATABASE_URL", ""),
		KafkaBrokers:         GetEnvOrString("KAFKA_BROKERS", ""),
		KafkaConsumerEnabled: GetEnvOrBool("KAFKA_CONSUMER_ENABLED", false),
		KafkaConsumerTopics:  nil,
		KafkaProducerEnabled: GetEnvOrBool("KAFKA_PRODUCER_ENABLED", false),
		MetricsEnabled:       GetEnvOrBool("METRICS_ENABLED", false),
		MetricsPort:          GetEnvOrInt("METRICS_PORT", 51077),
	}
}

// Init initializes the Foundation application.
func Init(name string) *Application {
	return &Application{
		Name:   name,
		Config: NewConfig(),
		Logger: initLogger(name),
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
	if app.Config.DatabaseEnabled {
		if err := app.connectToPostgreSQL(); err != nil {
			return fmt.Errorf("postgresql: %w", err)
		}
	}

	// Kafka consumer
	if app.Config.KafkaConsumerEnabled {
		if err := app.connectKafkaConsumer(); err != nil {
			return fmt.Errorf("kafka consumer: %w", err)
		}
	}

	// Kafka producer
	if app.Config.KafkaProducerEnabled {
		if err := app.connectKafkaProducer(); err != nil {
			return fmt.Errorf("kafka producer: %w", err)
		}
	}

	// Metrics
	if app.Config.MetricsEnabled {
		app.StartMetricsServer()
	}

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

	// Metrics
	if app.MetricsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := app.MetricsServer.Shutdown(ctx); err != nil {
			app.Logger.Errorf("Failed to shut down metrics server: %v", err)
		}
	}
}
