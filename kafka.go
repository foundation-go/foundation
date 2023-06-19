package foundation

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// WithKafkaConsumerTopics sets the Kafka consumer topics.
func WithKafkaConsumerTopics(topics ...string) StartComponentsOption {
	return func(app *Application) {
		app.kafkaConsumerTopics = topics
	}
}

func (app *Application) connectKafkaConsumer() error {
	app.Logger.Info("Connecting to Kafka as a consumer...")

	brokers, err := getBrokers()
	if err != nil {
		return err
	}

	if len(app.kafkaConsumerTopics) == 0 {
		return errors.New("you must set topics during the application initialization")
	}

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"group.id":           fmt.Sprintf("%s-consumer", app.Name),
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": false,
	})
	if err != nil {
		return err
	}

	err = consumer.SubscribeTopics(app.kafkaConsumerTopics, nil)
	if err != nil {
		return err
	}

	app.KafkaConsumer = consumer

	return nil
}

func (app *Application) connectKafkaProducer() error {
	app.Logger.Info("Connecting to Kafka as a producer...")

	brokers, err := getBrokers()
	if err != nil {
		return err
	}

	producer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
	if err != nil {
		return err
	}

	// Test the connection
	_, err = producer.GetMetadata(nil, true, 1000)
	if err != nil {
		return err
	}

	app.KafkaProducer = producer

	return nil
}

func getBrokers() (string, error) {
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		return "", errors.New("KAFKA_BROKERS variable is not set")
	}

	return strings.TrimSpace(kafkaBrokers), nil
}
