package foundation

import (
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"

	fkafka "github.com/ri-nat/foundation/kafka"
	fpg "github.com/ri-nat/foundation/postgresql"
	fsentry "github.com/ri-nat/foundation/sentry"
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
	MetricsEnabled       bool
	MetricsPort          int
	KafkaBrokers         []string
	KafkaConsumerEnabled bool
	KafkaConsumerTopics  []string
	KafkaProducerEnabled bool
	SentryDSN            string
	SentryEnabled        bool
}

// NewConfig returns a new Config with values populated from environment variables.
func NewConfig() *Config {
	return &Config{
		DatabaseEnabled:      len(GetEnvOrString("DATABASE_URL", "")) > 0,
		DatabasePool:         GetEnvOrInt("DATABASE_POOL", 5),
		DatabaseURL:          GetEnvOrString("DATABASE_URL", ""),
		MetricsEnabled:       GetEnvOrBool("METRICS_ENABLED", true),
		MetricsPort:          GetEnvOrInt("METRICS_PORT", 51077),
		KafkaBrokers:         strings.Split(GetEnvOrString("KAFKA_BROKERS", ""), ","),
		KafkaConsumerEnabled: GetEnvOrBool("KAFKA_CONSUMER_ENABLED", false),
		KafkaConsumerTopics:  nil,
		KafkaProducerEnabled: GetEnvOrBool("KAFKA_PRODUCER_ENABLED", false),
		SentryDSN:            GetEnvOrString("SENTRY_DSN", ""),
		SentryEnabled:        len(GetEnvOrString("SENTRY_DSN", "")) > 0,
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

	// Sentry
	if app.Config.SentryEnabled {
		app.Components = append(app.Components, fsentry.NewComponent(app.Config.SentryDSN))
	}

	// PostgreSQL
	if app.Config.DatabaseEnabled {
		app.Components = append(app.Components, fpg.NewPostgreSQLComponent(
			fpg.WithDatabaseURL(app.Config.DatabaseURL),
			fpg.WithPoolSize(app.Config.DatabasePool),
			fpg.WithLogger(app.Logger),
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

	// Metrics server
	if app.Config.MetricsEnabled {
		app.Components = append(app.Components, NewMetricsServerComponent(
			WithMetricsServerHealthHandler(app.healthHandler),
			WithMetricsServerLogger(app.Logger),
			WithMetricsServerPort(app.Config.MetricsPort),
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
			return fmt.Errorf("%s: %w", component.Name(), err)
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
			err = fmt.Errorf("failed to stop component `%s`: %w", app.Components[i].Name(), err)
			sentry.CaptureException(err)
			app.Logger.Error(err)
		}
	}
}
