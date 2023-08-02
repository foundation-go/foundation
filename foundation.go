package foundation

import (
	"fmt"

	"github.com/ri-nat/foundation/postgresql"
	"github.com/sirupsen/logrus"

	fkafka "github.com/ri-nat/foundation/kafka"
)

const Version = "0.1.0"

// Application represents a Foundation application.
type Application struct {
	Name       string
	Config     *Config
	Components []Component

	Logger *logrus.Entry
}

type Config struct {
	DatabaseEnabled      bool
	DatabasePool         int
	DatabaseURL          string
	KafkaBrokers         string
	KafkaConsumerEnabled bool
	KafkaConsumerTopics  []string
	KafkaProducerEnabled bool
	InsightEnabled       bool
	InsightPort          int
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
		InsightEnabled:       GetEnvOrBool("INSIGHT_ENABLED", true),
		InsightPort:          GetEnvOrInt("INSIGHT_PORT", 51077),
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

// WithKafkaConsumerTopics sets the Kafka consumer topics.
func WithKafkaConsumerTopics(topics ...string) StartComponentsOption {
	return func(app *Application) {
		app.Config.KafkaConsumerTopics = topics
	}
}

func (app *Application) addSystemComponents() error {
	// Remove user-defined components in order to add system components first.
	existedComponents := app.Components
	app.Components = []Component{}

	// PostgreSQL
	if app.Config.DatabaseEnabled {
		app.Components = append(app.Components, postgresql.NewPostgreSQLComponent(
			postgresql.WithDatabaseURL(app.Config.DatabaseURL),
			postgresql.WithPoolSize(app.Config.DatabasePool),
			postgresql.WithLogger(app.Logger),
		))
	}

	// Kafka consumer
	if app.Config.KafkaConsumerEnabled {
		app.Components = append(app.Components, fkafka.NewConsumerComponent(
			fkafka.WithConsumerAppName(app.Name),
			fkafka.WithConsumerBrokers(app.Config.KafkaBrokers),
			fkafka.WithConsumerTopics(app.Config.KafkaConsumerTopics),
			fkafka.WithConsumerLogger(app.Logger),
		))
	}

	// Kafka producer
	if app.Config.KafkaProducerEnabled {
		app.Components = append(app.Components, fkafka.NewProducerComponent(
			fkafka.WithProducerBrokers(app.Config.KafkaBrokers),
			fkafka.WithProducerLogger(app.Logger),
		))
	}

	// Insight server
	if app.Config.InsightEnabled {
		app.Components = append(app.Components, NewInsightServerComponent(
			WithInsightServerHealthHandler(app.healthHandler),
			WithInsightServerLogger(app.Logger),
			WithInsightServerPort(app.Config.InsightPort),
		))
	}

	// Add user-defined components back
	app.Components = append(app.Components, existedComponents...)

	return nil
}

// StartComponents starts the default Foundation application components.
func (app *Application) StartComponents(opts ...StartComponentsOption) error {
	// Apply options
	for _, opt := range opts {
		opt(app)
	}

	if err := app.addSystemComponents(); err != nil {
		return err
	}

	app.Logger.Info("Starting components:")

	for _, component := range app.Components {
		app.Logger.Infof(" - %s", component.Name())

		if err := component.Start(); err != nil {
			return fmt.Errorf("failed to start component `%s`: %w", component.Name(), err)
		}
	}

	return nil
}

// StopComponents stops the default Foundation application components.
func (app *Application) StopComponents() {
	app.Logger.Info("Stopping components:")

	// Stop components in reverse order, so that dependencies are stopped first
	for i := len(app.Components) - 1; i >= 0; i-- {
		app.Logger.Infof(" - %s", app.Components[i].Name())

		if err := app.Components[i].Stop(); err != nil {
			app.Logger.Errorf("failed to stop component `%s`: %s", app.Components[i].Name(), err)
		}
	}
}
