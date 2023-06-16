package foundation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	fg "github.com/ri-nat/foundation/grpc"
	"github.com/ri-nat/foundation/internal/outboxrepo"
	"google.golang.org/protobuf/proto"
)

// Event represents an event to be published to the outbox
type Event struct {
	Topic     string
	Key       string
	Payload   []byte
	Headers   map[string]string
	CreatedAt time.Time
}

// NewEvent creates a new event
func NewEvent(msg proto.Message, key string, headers map[string]string) (*Event, error) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Get proto name
	protoName := string(msg.ProtoReflect().Descriptor().FullName())
	// Construct topic name from proto name
	topic := protoNameToTopic(protoName)

	return &Event{
		Topic:     topic,
		Key:       key,
		Payload:   payload,
		Headers:   headers,
		CreatedAt: time.Now(),
	}, nil
}

// PublishEvent publishes an event to the outbox within a provided transaction
func (app Application) PublishEvent(ctx context.Context, tx *sql.Tx, event *Event) error {
	// Marshal headers to JSON
	headers, err := json.Marshal(event.Headers)
	if err != nil {
		return fmt.Errorf("failed to marshal headers: %w", err)
	}

	// Instantiate outbox repo
	queries := outboxrepo.New(tx)

	params := outboxrepo.CreateOutboxEventParams{
		Topic:   event.Topic,
		Key:     event.Key,
		Payload: event.Payload,
		Headers: headers,
	}
	if err := queries.CreateOutboxEvent(ctx, params); err != nil {
		return fmt.Errorf("failed to insert event into outbox: %w", err)
	}

	return nil
}

// PublishEventTx publishes an event to the outbox, starting a new transaction
func (app Application) PublishEventTx(ctx context.Context, event *Event) error {
	// Start transaction
	tx, err := app.PG.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // nolint:errcheck

	// Publish event
	if err := app.PublishEvent(ctx, tx, event); err != nil {
		return err
	}

	return tx.Commit()
}

// NewAndPublishEvent creates a new event and publishes it to the outbox
func (app Application) NewAndPublishEvent(ctx context.Context, tx *sql.Tx, msg proto.Message, key string, headers map[string]string) error {
	event, err := NewEvent(msg, key, headers)
	if err != nil {
		return err
	}

	return app.PublishEvent(ctx, tx, event)
}

// NewAndPublishEventTx creates a new event and publishes it to the outbox within a transaction
func (app Application) NewAndPublishEventTx(ctx context.Context, msg proto.Message, key string, headers map[string]string) error {
	event, err := NewEvent(msg, key, headers)
	if err != nil {
		return err
	}

	return app.PublishEventTx(ctx, event)
}

// WithTransaction executes the given function in a transaction. If the function
// returns an event, it will be published.
func (app Application) WithTransaction(ctx context.Context, f func(tx *sql.Tx) (*Event, error)) error {
	// Start transaction
	tx, err := app.PG.Begin()
	if err != nil {
		return fg.NewInternalError(err, "failed to begin transaction")
	}
	defer tx.Rollback() // nolint: errcheck

	// Execute function
	event, err := f(tx)
	if err != nil {
		return err
	}

	// Publish event (if any)
	if event != nil {
		if err = app.PublishEvent(ctx, tx, event); err != nil {
			return fg.NewInternalError(err, "failed to publish event")
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fg.NewInternalError(err, "failed to commit transaction")
	}

	return nil
}

func protoNameToTopic(protoName string) string {
	topicParts := strings.Split(protoName, ".")
	topicParts = topicParts[:len(topicParts)-1]

	return strings.Join(topicParts, ".")
}
