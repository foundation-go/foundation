package foundation

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	fctx "github.com/foundation-go/foundation/context"
	ferr "github.com/foundation-go/foundation/errors"
	fkafka "github.com/foundation-go/foundation/kafka"
	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

type EventsWorker struct {
	*SpinWorker

	protoNamesToMessages map[string]proto.Message
}

// EventHandler represents an event handler
type EventHandler interface {
	Handle(context.Context, *Event, proto.Message) ([]*Event, ferr.FoundationError)
}

// EventsWorkerOptions represents the options for starting an events worker
type EventsWorkerOptions struct {
	Handlers               map[proto.Message][]EventHandler
	Topics                 []string
	ModeName               string
	StartComponentsOptions []StartComponentsOption
}

func InitEventsWorker(name string) *EventsWorker {
	return &EventsWorker{
		SpinWorker: InitSpinWorker(name),
	}
}

func (opts *EventsWorkerOptions) GetTopics() []string {
	// If topics are specified in the options, use them
	if len(opts.Topics) > 0 {
		return opts.Topics
	}

	// Otherwise, build topics from events we're handling
	topics := []string{}

	if len(opts.Handlers) == 0 {
		return nil
	}

	for protoMsg := range opts.Handlers {
		protoName := ProtoToName(protoMsg)
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

func (opts *EventsWorkerOptions) ProtoNamesToMessages() map[string]proto.Message {
	protoNamesToMessages := make(map[string]proto.Message)

	for msg := range opts.Handlers {
		protoNamesToMessages[ProtoToName(msg)] = msg
	}

	return protoNamesToMessages
}

// Start runs the worker that handles events
func (w *EventsWorker) Start(opts *EventsWorkerOptions) {
	w.protoNamesToMessages = opts.ProtoNamesToMessages()

	wOpts := NewSpinWorkerOptions()
	wOpts.ModeName = opts.ModeName
	wOpts.ProcessFunc = w.newProcessEventFunc(opts.Handlers)
	wOpts.StartComponentsOptions = append(opts.StartComponentsOptions,
		WithKafkaConsumer(),
		WithKafkaConsumerTopics(opts.GetTopics()...),
	)

	w.SpinWorker.Start(wOpts)
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

func (w *EventsWorker) newProcessEventFunc(handlers map[proto.Message][]EventHandler) func(ctx context.Context) ferr.FoundationError {
	return func(ctx context.Context) ferr.FoundationError {
		msg, err := w.GetKafkaConsumer().FetchMessage(ctx)
		if err != nil {
			return ferr.NewInternalError(err, "failed to read message from Kafka")
		}

		event := newEventFromKafkaMessage(&msg)

		var handleErr ferr.FoundationError

		log := w.Logger.WithFields(map[string]interface{}{
			"correlation_id": event.Headers[fkafka.HeaderCorrelationID],
			"event":          event.ProtoName,
		})
		log.Info("Received event")

		templateProtoMsg, ok := w.protoNamesToMessages[event.ProtoName]
		if !ok {
			log.Debugf("Skip event without handlers: `%s`", event.ProtoName)
			return nil
		}

		// Add explicit handlers
		protoMsg := proto.Clone(templateProtoMsg)
		curHandlers := handlers[templateProtoMsg]
		err = proto.Unmarshal(event.Payload, protoMsg)
		if err != nil {
			return ferr.NewInternalError(err, "failed to unmarshal event payload")
		}

		for _, handler := range curHandlers {
			log := log.WithField("handler", fmt.Sprintf("%T", handler))
			log.Info("Processing event")

			handleErr = w.processEvent(ctx, handler, event, protoMsg)
			if handleErr != nil {
				log.WithError(handleErr).Errorf("Failed to process event `%s`", event.ProtoName)

				// We publish the error event to the error topic for further delivery to the user via WebSocket.
				if event.Headers[fkafka.HeaderOriginatorID] != "" {
					err := w.NewAndPublishEvent(ctx, handleErr.MarshalProto(), event.Headers[fkafka.HeaderOriginatorID], nil, nil)
					if err != nil {
						return err
					}
				}

				// We just stop all the subsequent handlers from processing the event if one of them failed.
				//
				// TODO: Consider adding a configuration option to allow the user to choose whether to stop after
				// specific handler failed or not. It would require to add ability to return multiple errors from
				// this function.
				break
			}

			log.Info("Event processed successfully")
		}

		// For now, we commit the message even if the handler failed to process it.
		//
		// TODO: add a configuration option to allow the user to choose whether to commit the message or not.
		// Or maybe even publish the message to a dead-letter topic?
		if commitErr := w.CommitMessage(ctx, msg); commitErr != nil {
			return commitErr
		}

		return handleErr
	}
}

func (w *EventsWorker) processEvent(ctx context.Context, handler EventHandler, event *Event, msg proto.Message) ferr.FoundationError {
	var (
		tx         *sql.Tx
		needCommit bool
		err        error
	)

	if w.Config.Database.Enabled {
		tx, err = w.GetPostgreSQL().Begin()
		if err != nil {
			return ferr.NewInternalError(err, "failed to begin transaction")
		}
		defer tx.Rollback() // nolint:errcheck
		needCommit = true

		// Add transaction to context
		ctx = fctx.WithTX(ctx, tx)
	}

	// Add correlation ID to context
	ctx = fctx.WithCorrelationID(ctx, event.Headers[fkafka.HeaderCorrelationID])

	// Handle event
	events, handleErr := handler.Handle(ctx, event, msg)
	if handleErr != nil {
		return handleErr
	}

	// Publish outgoing events
	for _, e := range events {
		if publishErr := w.PublishEvent(ctx, e, tx); publishErr != nil {
			return publishErr
		}
	}

	if needCommit {
		// Commit transaction
		if err = tx.Commit(); err != nil {
			return ferr.NewInternalError(err, "failed to commit transaction")
		}
	}

	return nil
}

// CommitMessage tries to commit a Kafka message using the service's KafkaConsumer.
// If the commit operation fails, it retries up to three times with a one-second pause between retries.
// If all attempts fail, the function returns the last occurred error.
func (s *Service) CommitMessage(ctx context.Context, msg kafka.Message) ferr.FoundationError {
	// TODO: Make something clever here, like exponential backoff
	for i := 0; i < 3; i++ {
		err := s.GetKafkaConsumer().CommitMessages(ctx, msg)
		if err == nil {
			return nil
		}

		if i == 2 {
			return ferr.NewInternalError(err, "failed to commit message")
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}
