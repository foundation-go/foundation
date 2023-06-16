package foundation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ri-nat/foundation/internal/outboxrepo"
)

// Event represents an event to be published to the outbox
type Event struct {
	Topic     string
	Partition int32
	Payload   []byte
	Headers   map[string]string
	CreatedAt time.Time
}

// NewEvent creates a new event
func NewEvent(topic string, partition int32, payload []byte, headers map[string]string) *Event {
	return &Event{
		Topic:     topic,
		Partition: partition,
		Payload:   payload,
		Headers:   headers,
		CreatedAt: time.Now(),
	}
}

// PublishEvent publishes an event to the outbox
func (app Application) PublishEvent(ctx context.Context, db *sql.DB, event *Event) error {
	// Marshal headers to JSON
	headers, err := json.Marshal(event.Headers)
	if err != nil {
		return fmt.Errorf("failed to marshal headers: %w", err)
	}

	// Instantiate outbox repo
	queries := outboxrepo.New(db)

	params := outboxrepo.CreateOutboxEventParams{
		Topic:     event.Topic,
		Partition: event.Partition,
		Payload:   event.Payload,
		Headers:   headers,
		CreatedAt: event.CreatedAt,
	}
	if err := queries.CreateOutboxEvent(ctx, params); err != nil {
		return fmt.Errorf("failed to insert event into outbox: %w", err)
	}

	return nil
}

// PublishEventTx publishes an event to the outbox within a transaction
func (app Application) PublishEventTx(ctx context.Context, db *sql.DB, event *Event) error {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // nolint:errcheck

	// Publish event
	if err := app.PublishEvent(ctx, db, event); err != nil {
		return err
	}

	return tx.Commit()
}

// NewAndPublishEvent creates a new event and publishes it to the outbox
func (app Application) NewAndPublishEvent(ctx context.Context, db *sql.DB, topic string, partition int32, payload []byte, headers map[string]string) error {
	event := NewEvent(topic, partition, payload, headers)
	return app.PublishEvent(ctx, db, event)
}

// NewAndPublishEventTx creates a new event and publishes it to the outbox within a transaction
func (app Application) NewAndPublishEventTx(ctx context.Context, db *sql.DB, topic string, partition int32, payload []byte, headers map[string]string) error {
	event := NewEvent(topic, partition, payload, headers)
	return app.PublishEventTx(ctx, db, event)
}
