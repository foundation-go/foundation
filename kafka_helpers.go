package foundation

import (
	"errors"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/getsentry/sentry-go"
	fkafka "github.com/ri-nat/foundation/kafka"
)

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

func (app *Application) GetKafkaConsumer() *kafka.Consumer {
	component := app.GetComponent(fkafka.ConsumerComponentName)
	if component == nil {
		err := errors.New("kafka consumer component is not registered")
		sentry.CaptureException(err)
		app.Logger.Fatal(err)
	}

	consumer, ok := component.(*fkafka.ConsumerComponent)
	if !ok {
		err := errors.New("kafka consumer component is not of type *fkafka.ConsumerComponent")
		sentry.CaptureException(err)
		app.Logger.Fatal(err)
	}

	return consumer.Consumer
}

func (app *Application) GetKafkaProducer() *kafka.Producer {
	component := app.GetComponent(fkafka.ProducerComponentName)
	if component == nil {
		err := errors.New("kafka producer component is not registered")
		sentry.CaptureException(err)
		app.Logger.Fatal(err)
	}

	producer, ok := component.(*fkafka.ProducerComponent)
	if !ok {
		err := errors.New("kafka producer component is not of type *fkafka.ProducerComponent")
		sentry.CaptureException(err)
		app.Logger.Fatal(err)
	}

	return producer.Producer
}

func (app *Application) getKafkaBrokers() (string, error) {
	if app.Config.KafkaBrokers == "" {
		return "", errors.New("KAFKA_BROKERS variable is not set")
	}

	return strings.TrimSpace(app.Config.KafkaBrokers), nil
}
