package foundation

import (
	"context"
	"database/sql"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	fctx "github.com/ri-nat/foundation/context"
)

// EventHandler represents an event handler
type EventHandler interface {
	Handle(ctx context.Context, event *Event) ([]*Event, FoundationError)
}

// StartEventsWorkerOptions represents the options for starting an events worker
type StartEventsWorkerOptions struct {
	Handlers map[string][]EventHandler
}

// StartEventsWorker starts a worker that handles events
func (app *Application) StartEventsWorker(opts *StartEventsWorkerOptions) {
	wOpts := NewStartWorkerOptions()
	wOpts.ModeName = "events_worker"
	wOpts.ProcessFunc = app.newHandleEventsFunc(opts.Handlers)

	app.StartWorker(wOpts)
}

func newEventFromKafkaMessage(msg *kafka.Message) *Event {
	headers := make(map[string]string)
	for _, header := range msg.Headers {
		headers[header.Key] = string(header.Value)
	}

	return &Event{
		Topic:     *msg.TopicPartition.Topic,
		Key:       string(msg.Key),
		Payload:   msg.Value,
		ProtoName: headers[KafkaHeaderProtoName],
		Headers:   headers,
		CreatedAt: msg.Timestamp,
	}
}

func (app *Application) newHandleEventsFunc(handlers map[string][]EventHandler) func(ctx context.Context) FoundationError {
	return func(ctx context.Context) FoundationError {
		msg, err := app.KafkaConsumer.ReadMessage(-1)
		if err != nil {
			return NewInternalError(err, "failed to read message from Kafka")
		}

		event := newEventFromKafkaMessage(msg)

		for _, handler := range handlers[event.ProtoName] {
			if handleErr := app.handleEvent(ctx, handler, event); handleErr != nil {
				// For now, we commit the message even if the handler failed to process it.
				//
				// TODO: add a configuration option to allow the user to choose whether to commit the message or not.
				// Or maybe publish the message to a dead-letter topic.
				if commitErr := app.CommitMessage(msg); commitErr != nil {
					return commitErr
				}

				return handleErr
			}
		}

		// Commit message
		if commitErr := app.CommitMessage(msg); commitErr != nil {
			return commitErr
		}

		return nil
	}
}

func (app *Application) handleEvent(ctx context.Context, handler EventHandler, event *Event) FoundationError {
	var tx *sql.Tx
	commitNeeded := false

	if app.DatabaseEnabled {
		tx, err := app.PG.Begin()
		if err != nil {
			return NewInternalError(err, "failed to begin transaction")
		}
		defer tx.Rollback() // nolint:errcheck
		commitNeeded = true

		// Add transaction to context
		ctx = fctx.SetTX(ctx, tx)
	}

	// Add correlation ID to context
	ctx = fctx.SetCorrelationID(ctx, event.Headers[KafkaHeaderCorrelationID])

	// Handle event
	events, handleErr := handler.Handle(ctx, event)
	if handleErr != nil {
		return handleErr
	}

	// Publish outgoing events
	for _, e := range events {
		if publishErr := app.PublishEvent(ctx, e, tx); publishErr != nil {
			return publishErr
		}
	}

	if commitNeeded {
		// Commit transaction
		if err := tx.Commit(); err != nil {
			return NewInternalError(err, "failed to commit transaction")
		}
	}

	return nil
}

// CommitMessage tries to commit a Kafka message using the application's KafkaConsumer.
// If the commit operation fails, it retries up to three times with a one-second pause between retries.
// If all attempts fail, the function returns the last occurred error.
func (app *Application) CommitMessage(msg *kafka.Message) FoundationError {
	var err error

	// TODO: Make something clever here
	for i := 0; i < 3; i++ {
		if _, err = app.KafkaConsumer.CommitMessage(msg); err != nil {
			if i < 2 {
				time.Sleep(1 * time.Second)
				continue
			}
		}

		return nil
	}

	return NewInternalError(err, "failed to commit message")
}
