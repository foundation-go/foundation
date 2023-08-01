package foundation

import (
	"fmt"
	"net/http"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

const (
	kafkaHealthCheckTimeout = 1 * time.Second
)

func (app *Application) healthHandler(w http.ResponseWriter, r *http.Request) {
	if app.Config.DatabaseEnabled {
		if err := app.checkPostgreSQLConnection(); err != nil {
			app.Logger.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if app.Config.KafkaConsumerEnabled {
		if err := app.checkKafkaConnection("kafka consumer", app.KafkaConsumer); err != nil {
			app.Logger.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if app.Config.KafkaProducerEnabled {
		if err := app.checkKafkaConnection("kafka producer", app.KafkaProducer); err != nil {
			app.Logger.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// kafkaMetadataProvider is an interface that allows us to use both
// `*kafka.Producer` and `*kafka.Consumer` as a parameter to `checkKafkaConnection`.
type kafkaMetadataProvider interface {
	GetMetadata(topic *string, allTopics bool, timeoutMs int) (*kafka.Metadata, error)
}

func (app *Application) checkKafkaConnection(name string, kfk kafkaMetadataProvider) error {
	return app.checkConnection(name, kfk, func() error {
		// We don't care about the metadata, we just want to make sure connection is alive.
		_, err := kfk.GetMetadata(nil, false, int(kafkaHealthCheckTimeout.Milliseconds()))
		return err
	})
}

func (app *Application) checkPostgreSQLConnection() error {
	return app.checkConnection("postgresql", app.PG, func() error {
		return app.PG.Ping()
	})
}

func (app *Application) checkConnection(name string, conn interface{}, ping func() error) error {
	if conn == nil {
		return fmt.Errorf("%s enabled, but not initialized", name)
	}

	started := time.Now()
	if err := ping(); err != nil {
		return fmt.Errorf("failed to ping %s: %w", name, err)
	}

	app.Logger.Debugf("%s connection check took %vms", name, time.Since(started).Milliseconds())

	return nil
}
