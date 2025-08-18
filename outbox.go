package foundation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/foundation-go/foundation/outboxrepo"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	fctx "github.com/foundation-go/foundation/context"
	ferr "github.com/foundation-go/foundation/errors"
	fkafka "github.com/foundation-go/foundation/kafka"
)

// Event represents an event to be published to the outbox
type Event struct {
	Topic     string
	Key       string
	Payload   []byte
	ProtoName string
	Headers   map[string]string
	CreatedAt time.Time
}

// Unmarshal unmarshals the event payload into a protobuf message
func (e *Event) Unmarshal(msg proto.Message) ferr.FoundationError {
	if err := proto.Unmarshal(e.Payload, msg); err != nil {
		return ferr.NewInternalError(err, "failed to unmarshal Event payload")
	}

	return nil
}

// NewEventFromProto creates a new event from a protobuf message
func NewEventFromProto(msg proto.Message, key string, headers map[string]string) (*Event, ferr.FoundationError) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, ferr.NewInternalError(err, "failed to marshal message")
	}

	// Get proto name
	protoName := string(msg.ProtoReflect().Descriptor().FullName())
	// Construct topic name from proto name
	topic := ProtoNameToTopic(protoName)

	if headers == nil {
		headers = make(map[string]string)
	}

	return &Event{
		Topic:     topic,
		Key:       key,
		Payload:   payload,
		ProtoName: protoName,
		Headers:   headers,
		CreatedAt: time.Now(),
	}, nil
}

func addDefaultHeaders(ctx context.Context, event *Event) *Event {
	if event.Headers == nil {
		event.Headers = make(map[string]string)
	}

	event.Headers[fkafka.HeaderProtoName] = event.ProtoName
	event.Headers[fkafka.HeaderCorrelationID] = fctx.GetCorrelationID(ctx)

	return event
}

// publishEventToOutbox publishes an event to the outbox.
func (s *Service) publishEventToOutbox(ctx context.Context, event *Event, tx pgx.Tx) ferr.FoundationError {
	var (
		err error

		commitNeeded = false
	)

	if tx == nil {
		// Start transaction
		tx, err = s.GetPostgreSQL().Begin(ctx)
		if err != nil {
			return ferr.NewInternalError(err, "failed to begin transaction")
		}
		defer tx.Rollback(ctx) // nolint:errcheck
		commitNeeded = true
	}

	// Marshal headers to JSON
	headers, err := json.Marshal(event.Headers)
	if err != nil {
		return ferr.NewInternalError(err, "failed to marshal headers")
	}

	queries := outboxrepo.New(tx)
	params := outboxrepo.CreateOutboxEventParams{
		Topic:   event.Topic,
		Key:     event.Key,
		Payload: event.Payload,
		Headers: headers,
	}
	// Publish event
	if err = queries.CreateOutboxEvent(ctx, params); err != nil {
		return ferr.NewInternalError(err, "failed to insert event into outbox")
	}

	if commitNeeded {
		if err = tx.Commit(ctx); err != nil {
			return ferr.NewInternalError(err, "failed to commit transaction")
		}
	}

	return nil
}

// publishEventToKafka publishes an event to the Kafka topic.
func (s *Service) publishEventToKafka(ctx context.Context, event *Event) ferr.FoundationError {
	message, err := NewMessageFromEvent(event)
	if err != nil {
		return ferr.NewInternalError(err, "failed to create message from event")
	}

	if err := s.GetKafkaProducer().WriteMessages(ctx, *message); err != nil {
		return ferr.NewInternalError(err, "failed to publish event to Kafka")
	}

	return nil
}

// PublishEvent publishes an event to the outbox, starting a new transaction,
// or straight to the Kafka topic if `OUTBOX_ENABLED` is not set.
func (s *Service) PublishEvent(ctx context.Context, event *Event, tx pgx.Tx) ferr.FoundationError {
	event = addDefaultHeaders(ctx, event)

	if s.Config.Outbox.Enabled {
		return s.publishEventToOutbox(ctx, event, tx)
	}

	return s.publishEventToKafka(ctx, event)
}

// NewAndPublishEvent creates a new event and publishes it to the outbox within a transaction
func (s *Service) NewAndPublishEvent(ctx context.Context, msg proto.Message, key string, headers map[string]string, tx pgx.Tx) ferr.FoundationError {
	event, err := NewEventFromProto(msg, key, headers)
	if err != nil {
		return err
	}

	return s.PublishEvent(ctx, event, tx)
}

// WithTransaction executes the given function in a transaction. If the function
// returns an event, it will be published.
func (s *Service) WithTransaction(ctx context.Context, f func(tx pgx.Tx) ([]*Event, ferr.FoundationError)) ferr.FoundationError {
	// Start transaction
	tx, err := s.GetPostgreSQL().Begin(ctx)
	if err != nil {
		return ferr.NewInternalError(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx) // nolint: errcheck

	// Execute function
	events, fErr := f(tx)
	if fErr != nil {
		return fErr
	}

	// Publish events (if any)
	if len(events) > 0 {
		for i, event := range events {
			if err = s.PublishEvent(ctx, event, tx); err != nil {
				return ferr.NewInternalError(
					err,
					fmt.Sprintf("failed to publish event %s: %d out of %d", event.ProtoName, i+1, len(events)),
				)
			}
		}
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return ferr.NewInternalError(err, "failed to commit transaction")
	}

	return nil
}

// WithResponseTransaction executes the given function in a transaction. If the function
// returns an event, it will be published. If the function returns a response, it will be returned.
func (s *Service) WithResponseTransaction(ctx context.Context, f func(tx pgx.Tx) (proto.Message, []*Event, ferr.FoundationError)) (proto.Message, ferr.FoundationError) {
	tx, err := s.GetPostgreSQL().Begin(ctx)
	if err != nil {
		return nil, ferr.NewInternalError(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx) // nolint: errcheck

	response, events, fErr := f(tx)
	if fErr != nil {
		return nil, fErr
	}

	if len(events) > 0 {
		for i, event := range events {
			if err = s.PublishEvent(ctx, event, tx); err != nil {
				return nil, ferr.NewInternalError(err, fmt.Sprintf("failed to publish event %s: %d out of %d", event.ProtoName, i+1, len(events)))
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, ferr.NewInternalError(err, "failed to commit transaction")
	}

	return response, nil
}

// ListOutboxEvents returns a list of outbox events in the order they were created.
func (s *Service) ListOutboxEvents(ctx context.Context, tx pgx.Tx, limit int32) ([]outboxrepo.FoundationOutboxEvent, ferr.FoundationError) {
	queries := outboxrepo.New(tx)

	events, err := queries.ListOutboxEvents(ctx, limit)
	if err != nil {
		return nil, ferr.NewInternalError(err, "failed to `ListOutboxEvents`")
	}

	return events, nil
}

// DeleteOutboxEvents deletes outbox events up to (and including) the given ID.
func (s *Service) DeleteOutboxEvents(ctx context.Context, tx pgx.Tx, maxID int64) ferr.FoundationError {
	queries := outboxrepo.New(tx)

	if err := queries.DeleteOutboxEvents(ctx, maxID); err != nil {
		return ferr.NewInternalError(err, "failed to `DeleteOutboxEvents`")
	}

	return nil
}

// TODO: extract these functions to a more appropriate place
func ProtoNameToTopic(protoName string) string {
	// TODO: Respect `EVENTS_WORKER_ERRORS_TOPIC` for Foundation errors
	topicParts := strings.Split(protoName, ".")
	topicParts = topicParts[:len(topicParts)-1]

	return strings.Join(topicParts, ".")
}

func ProtoToTopic(msg proto.Message) string {
	return ProtoNameToTopic(ProtoToName(msg))
}

func ProtoToName(msg proto.Message) string {
	return string(msg.ProtoReflect().Descriptor().FullName())
}
