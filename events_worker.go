package foundation

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	fctx "github.com/ri-nat/foundation/context"
	fkafka "github.com/ri-nat/foundation/kafka"
	"github.com/segmentio/kafka-go"
)

// EventHandler represents an event handler
type EventHandler interface {
	Handle(ctx context.Context, event *Event) ([]*Event, FoundationError)
}

// StartEventsWorkerOptions represents the options for starting an events worker
type StartEventsWorkerOptions struct {
	Handlers map[string][]EventHandler
	Topics   []string
}

func (opts *StartEventsWorkerOptions) GetTopics() []string {
	// If topics are specified in the options, use them
	if len(opts.Topics) > 0 {
		return opts.Topics
	}

	// Otherwise, build topics from events we're handling
	topics := []string{}

	if len(opts.Handlers) == 0 {
		return nil
	}

	for protoName := range opts.Handlers {
		// Collect service names from event message names
		// project.service.SomeEvent -> project.service
		topic := protoName[:strings.LastIndex(protoName, ".")]

		if topic != "" {
			// Add topic to the list if it's not already there
			found := false
			for _, t := range topics {
				if t == topic {
					found = true
					break
				}
			}

			if !found {
				topics = append(topics, topic)
			}
		}
	}

	// Sort topics for consistency
	sort.Strings(topics)

	return topics
}

// StartEventsWorker starts a worker that handles events
func (app *Application) StartEventsWorker(opts *StartEventsWorkerOptions) {
	wOpts := NewStartWorkerOptions()
	wOpts.ModeName = "events_worker"
	wOpts.ProcessFunc = app.newProcessEventFunc(opts.Handlers)
	wOpts.StartComponentsOptions = []StartComponentsOption{
		WithKafkaConsumerTopics(opts.GetTopics()...),
	}

	app.StartWorker(wOpts)
}

func newEventFromKafkaMessage(msg *kafka.Message) *Event {
	headers := make(map[string]string)
	for _, header := range msg.Headers {
		headers[header.Key] = string(header.Value)
	}

	return &Event{
		Topic:     msg.Topic,
		Key:       string(msg.Key),
		Payload:   msg.Value,
		ProtoName: headers[fkafka.HeaderProtoName],
		Headers:   headers,
		CreatedAt: msg.Time,
	}
}

func (app *Application) newProcessEventFunc(handlers map[string][]EventHandler) func(ctx context.Context) FoundationError {
	return func(ctx context.Context) FoundationError {
		// Create 100ms timeout context for reading messages from Kafka
		tCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		msg, err := app.GetKafkaConsumer().FetchMessage(tCtx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				// No messages in Kafka, just return
				return nil
			}

			return NewInternalError(err, "failed to read message from Kafka")
		}

		event := newEventFromKafkaMessage(&msg)

		var handleErr FoundationError

		for _, handler := range handlers[event.ProtoName] {
			handleErr = app.processEvent(ctx, handler, event)

			if handleErr != nil {
				// We just stop all the subsequent handlers from processing the event if one of them failed.
				//
				// TODO: Consider adding a configuration option to allow the user to choose whether to stop after
				// specific handler failed or not. It would require to add ability to return multiple errors from
				// this function.
				break
			}
		}

		// For now, we commit the message even if the handler failed to process it.
		//
		// TODO: add a configuration option to allow the user to choose whether to commit the message or not.
		// Or maybe publish the message to a dead-letter topic.
		if commitErr := app.CommitMessage(ctx, msg); commitErr != nil {
			return commitErr
		}

		return handleErr
	}
}

func (app *Application) processEvent(ctx context.Context, handler EventHandler, event *Event) FoundationError {
	var (
		tx         *sql.Tx
		needCommit bool
		err        error
	)

	if app.Config.DatabaseEnabled {
		tx, err = app.GetPostgreSQL().Begin()
		if err != nil {
			return NewInternalError(err, "failed to begin transaction")
		}
		defer tx.Rollback() // nolint:errcheck
		needCommit = true

		// Add transaction to context
		ctx = fctx.SetTX(ctx, tx)
	}

	// Add correlation ID to context
	ctx = fctx.SetCorrelationID(ctx, event.Headers[fkafka.HeaderCorrelationID])

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

	if needCommit {
		// Commit transaction
		if err = tx.Commit(); err != nil {
			return NewInternalError(err, "failed to commit transaction")
		}
	}

	return nil
}

// CommitMessage tries to commit a Kafka message using the application's KafkaConsumer.
// If the commit operation fails, it retries up to three times with a one-second pause between retries.
// If all attempts fail, the function returns the last occurred error.
func (app *Application) CommitMessage(ctx context.Context, msg kafka.Message) FoundationError {
	var err error

	// TODO: Make something clever here
	for i := 0; i < 3; i++ {
		if err = app.GetKafkaConsumer().CommitMessages(ctx, msg); err != nil {
			if i < 2 {
				time.Sleep(1 * time.Second)
				continue
			}
		}

		return nil
	}

	return NewInternalError(err, "failed to commit message")
}
