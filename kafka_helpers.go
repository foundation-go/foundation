package foundation

import (
	"errors"

	"github.com/getsentry/sentry-go"
	fkafka "github.com/ri-nat/foundation/kafka"
	"github.com/segmentio/kafka-go"
)

// NewMessageFromEvent creates a new Kafka message from a Foundation Outbox event
func NewMessageFromEvent(event *Event) (*kafka.Message, error) {
	message := &kafka.Message{
		Topic:   event.Topic,
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

func (s *Service) GetKafkaConsumer() *kafka.Reader {
	component := s.GetComponent(fkafka.ConsumerComponentName)
	if component == nil {
		err := errors.New("kafka consumer component is not registered")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	consumer, ok := component.(*fkafka.ConsumerComponent)
	if !ok {
		err := errors.New("kafka consumer component is not of type *fkafka.ConsumerComponent")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	return consumer.Consumer
}

func (s *Service) GetKafkaProducer() *kafka.Writer {
	component := s.GetComponent(fkafka.ProducerComponentName)
	if component == nil {
		err := errors.New("kafka producer component is not registered")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	producer, ok := component.(*fkafka.ProducerComponent)
	if !ok {
		err := errors.New("kafka producer component is not of type *fkafka.ProducerComponent")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	return producer.Producer
}

func (s *Service) getKafkaBrokers() ([]string, error) {
	if len(s.Config.Kafka.Brokers) == 0 {
		return nil, errors.New("KAFKA_BROKERS variable is not set")
	}

	return s.Config.Kafka.Brokers, nil
}
