package foundation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

const (
	KafkaHeaderCorrelationID = "correlation-id"
	KafkaHeaderOriginatorID  = "originator-id"
	KafkaHeaderProtoName     = "proto-name"
)

// WithKafkaConsumerTopics sets the Kafka consumer topics.
func WithKafkaConsumerTopics(topics ...string) StartComponentsOption {
	return func(app *Application) {
		app.Config.KafkaConsumerTopics = topics
	}
}

func (app *Application) connectKafkaConsumer() error {
	app.Logger.Info("Connecting to Kafka as a consumer...")

	brokers, err := app.getBrokers()
	if err != nil {
		return err
	}

	if len(app.Config.KafkaConsumerTopics) == 0 {
		return errors.New("you must specify topics during the application initialization using the `WithKafkaConsumerTopics`")
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

	app.Logger.Debugf("Kafka consumer topics: %v", app.Config.KafkaConsumerTopics)

	if err = consumer.SubscribeTopics(app.Config.KafkaConsumerTopics, nil); err != nil {
		return err
	}

	app.KafkaConsumer = consumer

	return nil
}

func (app *Application) connectKafkaProducer() error {
	app.Logger.Info("Connecting to Kafka as a producer...")

	brokers, err := app.getBrokers()
	if err != nil {
		return err
	}

	producer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
	if err != nil {
		return err
	}

	// Test the connection
	if _, err = producer.GetMetadata(nil, true, 1000); err != nil {
		return err
	}

	app.KafkaProducer = producer

	return nil
}

func (app *Application) getBrokers() (string, error) {
	if app.Config.KafkaBrokers == "" {
		return "", errors.New("KAFKA_BROKERS variable is not set")
	}

	return strings.TrimSpace(app.Config.KafkaBrokers), nil
}

// NewMessageFromEvent creates a new Kafka message from a Foundation Outbox event
func NewMessageFromEvent(event *Event) (*kafka.Message, error) {
	message := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &event.Topic,
			Partition: kafka.PartitionAny,
		},
		Value:   event.Payload,
		Key:     []byte(event.Key),
		Headers: []kafka.Header{},
	}

	for k, v := range event.Headers {
		message.Headers = append(message.Headers, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	return message, nil
}
