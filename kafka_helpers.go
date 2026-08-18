package foundation

import (
	"errors"
	"fmt"

	"github.com/segmentio/kafka-go"

	fkafka "github.com/foundation-go/foundation/kafka"
)

// NewMessageFromEvent creates a new Kafka message from a Foundation Outbox event
func NewMessageFromEvent(event *Event) (*kafka.Message, error) {
	message := &kafka.Message{
		Topic:   event.Topic,
		Value:   event.Payload,
		Key:     []byte(event.Key),
		Headers: make([]kafka.Header, 0, len(event.Headers)),
		// Carry the event's own timestamp rather than letting the broker stamp
		// the moment of delivery: an event that sat in the outbox for a minute
		// would otherwise look a minute younger than it is, and the same event
		// would have different times depending on whether it went through the
		// outbox or straight to Kafka.
		Time: event.CreatedAt,
	}

	for k, v := range event.Headers {
		message.Headers = append(message.Headers, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	return message, nil
}

// GetKafkaConsumer returns the Kafka reader.
//
// It panics when the reader is unavailable; see GetPostgreSQL for why, and use
// TryGetKafkaConsumer where the absence has to be handled.
func (s *Service) GetKafkaConsumer() *kafka.Reader {
	reader, err := s.TryGetKafkaConsumer()
	if err != nil {
		panic(err)
	}

	return reader
}

// TryGetKafkaConsumer returns the Kafka reader, or an error explaining why it
// is not available.
func (s *Service) TryGetKafkaConsumer() (*kafka.Reader, error) {
	component := s.GetComponent(fkafka.ConsumerComponentName)
	if component == nil {
		return nil, errors.New("no Kafka consumer component is registered: use foundation.WithKafkaConsumer()")
	}

	consumer, ok := component.(*fkafka.ConsumerComponent)
	if !ok {
		return nil, fmt.Errorf(
			"component `%s` is a %T, not a *kafka.ConsumerComponent", fkafka.ConsumerComponentName, component,
		)
	}

	if consumer.Consumer == nil {
		return nil, errors.New("the Kafka consumer component has not been started yet")
	}

	return consumer.Consumer, nil
}

// GetKafkaProducer returns the Kafka writer.
//
// It panics when the writer is unavailable; see GetPostgreSQL for why, and use
// TryGetKafkaProducer where the absence has to be handled.
func (s *Service) GetKafkaProducer() *kafka.Writer {
	writer, err := s.TryGetKafkaProducer()
	if err != nil {
		panic(err)
	}

	return writer
}

// TryGetKafkaProducer returns the Kafka writer, or an error explaining why it
// is not available.
func (s *Service) TryGetKafkaProducer() (*kafka.Writer, error) {
	component := s.GetComponent(fkafka.ProducerComponentName)
	if component == nil {
		return nil, errors.New("no Kafka producer component is registered: use foundation.WithKafkaProducer()")
	}

	producer, ok := component.(*fkafka.ProducerComponent)
	if !ok {
		return nil, fmt.Errorf(
			"component `%s` is a %T, not a *kafka.ProducerComponent", fkafka.ProducerComponentName, component,
		)
	}

	if producer.Producer == nil {
		return nil, errors.New("the Kafka producer component has not been started yet")
	}

	return producer.Producer, nil
}
