package foundation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	fctx "github.com/foundation-go/foundation/context"
	ferr "github.com/foundation-go/foundation/errors"
	fkafka "github.com/foundation-go/foundation/kafka"
	fmetrics "github.com/foundation-go/foundation/metrics"
)

type EventsWorker struct {
	*SpinWorker

	registry map[string]*handlerEntry
}

// handlerEntry holds everything needed to process one event type: a template
// message to clone before unmarshalling, and the handlers registered for it.
type handlerEntry struct {
	template proto.Message
	handlers []EventHandler
}

// EventHandler represents an event handler
type EventHandler interface {
	Handle(context.Context, *Event, proto.Message) ([]*Event, ferr.FoundationError)
}

// ErrorHandlingStrategy defines the EventsWorker behavior when errors occur while handle event
type ErrorHandlingStrategy int

const (
	// Default strategy: commit the message and skip the event
	IgnoreError ErrorHandlingStrategy = iota

	// ShutdownOnError stops the worker on error
	ShutdownOnError

	// RetryOnError retries to handle event on error. TODO: implement RetryOnError.
	// RetryOnError
)

// EventsWorkerOptions represents the options for starting an events worker
type EventsWorkerOptions struct {
	Handlers               map[proto.Message][]EventHandler
	Topics                 []string
	ModeName               string
	ErrorHandlingStrategy  ErrorHandlingStrategy
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

	if len(opts.Handlers) == 0 {
		return nil
	}

	// Otherwise, build topics from the events we handle: the message name
	// without its last segment, so project.service.SomeEvent yields
	// project.service.
	seen := make(map[string]bool, len(opts.Handlers))
	topics := []string{}

	for protoMsg := range opts.Handlers {
		topic := ProtoToTopic(protoMsg)
		if topic == "" || seen[topic] {
			continue
		}

		seen[topic] = true
		topics = append(topics, topic)
	}

	// Sort topics for consistency
	sort.Strings(topics)

	return topics
}

// Registry indexes the handlers by the full proto name of the event they
// handle, and reports registrations that cannot work.
//
// The Handlers map is keyed by proto.Message, i.e. by pointer identity. Two
// separately constructed `&pb.UserCreated{}` values are different keys holding
// the same event type, and the name-keyed lookup used at runtime silently kept
// only one of them — so half the handlers never ran, with nothing to indicate
// it. That is now a startup error rather than a mystery in production.
func (opts *EventsWorkerOptions) Registry() (map[string]*handlerEntry, error) {
	registry := make(map[string]*handlerEntry, len(opts.Handlers))

	for msg, handlers := range opts.Handlers {
		name := ProtoToName(msg)

		if _, duplicate := registry[name]; duplicate {
			return nil, fmt.Errorf(
				"event `%s` is registered under two different proto.Message keys; "+
					"merge them into a single entry, or the handlers of one of them will never run",
				name,
			)
		}

		// A message with no proto package yields no topic. That only matters
		// when the topics are being derived from the handled types: a worker
		// that lists its topics explicitly — consuming a legacy or third-party
		// topic, say — has no need for the derivation at all.
		if len(opts.Topics) == 0 && ProtoNameToTopic(name) == "" {
			return nil, fmt.Errorf(
				"event `%s` has no package, so no Kafka topic can be derived from it; "+
					"declare the message inside a proto package, or set EventsWorkerOptions.Topics explicitly",
				name,
			)
		}

		registry[name] = &handlerEntry{template: msg, handlers: handlers}
	}

	return registry, nil
}

// Start runs the worker that handles events
func (w *EventsWorker) Start(opts *EventsWorkerOptions) {
	registry, err := opts.Registry()
	if err != nil {
		w.Fatal(err, "invalid events worker configuration")
	}

	w.registry = registry

	wOpts := NewSpinWorkerOptions()
	wOpts.ModeName = opts.ModeName
	wOpts.ProcessFunc = w.newProcessEventFunc(opts.ErrorHandlingStrategy)
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

func (w *EventsWorker) newProcessEventFunc(
	errorMode ErrorHandlingStrategy,
) func(ctx context.Context) ferr.FoundationError {
	return func(ctx context.Context) ferr.FoundationError {
		msg, err := w.GetKafkaConsumer().FetchMessage(ctx)
		if err != nil {
			// A cancelled context is how a fetch ends during a normal
			// shutdown, not a failure worth reporting.
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
				return nil
			}

			return ferr.NewInternalError(err, "failed to read message from Kafka")
		}

		shouldCommit, handleErr := w.handleMessage(ctx, msg, errorMode)

		if shouldCommit {
			if commitErr := w.CommitMessage(ctx, msg); commitErr != nil {
				return commitErr
			}
		}

		return handleErr
	}
}

// handleMessage processes a single fetched Kafka message and reports whether
// its offset should be committed.
//
// It is deliberately separate from fetching and committing so that the
// decisions below can be tested without a broker.
func (w *EventsWorker) handleMessage(
	ctx context.Context,
	msg kafka.Message,
	errorMode ErrorHandlingStrategy,
) (bool, ferr.FoundationError) {
	event := newEventFromKafkaMessage(&msg)

	// Enrich the context with the correlation ID of the incoming event, so
	// that everything published while handling it — including the error
	// event below — carries the same ID.
	ctx = fctx.WithCorrelationID(ctx, event.Headers[fkafka.HeaderCorrelationID])

	log := w.Logger.WithFields(map[string]interface{}{
		"correlation_id": event.Headers[fkafka.HeaderCorrelationID],
		"event":          event.ProtoName,
		"topic":          msg.Topic,
		"partition":      msg.Partition,
		"offset":         msg.Offset,
	})
	log.Info("Received event")

	if !event.CreatedAt.IsZero() {
		fmetrics.EventLag.WithLabelValues(msg.Topic).Observe(time.Since(event.CreatedAt).Seconds())
	}

	entry, ok := w.registry[event.ProtoName]
	if !ok {
		// A worker subscribes to whole topics, so most of what it reads may be
		// of types it does not handle. Those offsets still have to be
		// committed: skipping the commit leaves the committed offset behind
		// the read position, so the consumer lag grows without bound and every
		// restart replays the tail of the topic.
		log.Debugf("Skipping event without handlers: `%s`", event.ProtoName)
		fmetrics.EventsProcessed.
			WithLabelValues(msg.Topic, event.ProtoName, fmetrics.ResultSkipped).
			Inc()

		return true, nil
	}

	protoMsg := proto.Clone(entry.template)
	if err := proto.Unmarshal(event.Payload, protoMsg); err != nil {
		// The coordinates go into the error itself, not just the log fields:
		// this error is reported once, by the caller, and whoever reads it in
		// Sentry needs to know which offset to re-drive.
		unmarshalErr := ferr.NewInternalError(err, fmt.Sprintf(
			"failed to unmarshal payload of `%s` at %s/%d offset %d",
			event.ProtoName, msg.Topic, msg.Partition, msg.Offset,
		))

		if errorMode == ShutdownOnError {
			log.Error("Cannot unmarshal event, shutting down")
			w.Shutdown()

			return false, unmarshalErr
		}

		fmetrics.EventsProcessed.
			WithLabelValues(msg.Topic, event.ProtoName, fmetrics.ResultError).
			Inc()

		// A payload that cannot be parsed now will not become parsable later,
		// so commit and move on rather than blocking the partition forever.
		// The error is returned rather than reported here: the worker loop
		// reports whatever it gets back, and reporting in both places sent two
		// Sentry events per message, doubling the volume during exactly the
		// poison-message storm where the count matters.
		return true, unmarshalErr
	}

	var handleErr ferr.FoundationError

	started := time.Now()

	defer func() {
		fmetrics.EventProcessingDuration.
			WithLabelValues(event.ProtoName).
			Observe(time.Since(started).Seconds())

		fmetrics.EventsProcessed.
			WithLabelValues(msg.Topic, event.ProtoName, fmetrics.ResultOf(handleErr)).
			Inc()
	}()

	for _, handler := range entry.handlers {
		handlerLog := log.WithField("handler", fmt.Sprintf("%T", handler))
		handlerLog.Info("Processing event")

		handleErr = w.processEvent(ctx, handler, event, protoMsg)
		if handleErr != nil {
			handlerLog.WithError(handleErr).Errorf("Failed to process event `%s`", event.ProtoName)

			// Publish the error to the errors topic, for delivery back to
			// whoever originated the request over WebSocket.
			if err := w.publishHandlerError(ctx, event, handleErr); err != nil {
				return false, err
			}

			// We just stop all the subsequent handlers from processing the event if one of them failed.
			//
			// TODO: Consider adding a configuration option to allow the user to choose whether to stop after
			// specific handler failed or not. It would require to add ability to return multiple errors from
			// this function.
			break
		}

		handlerLog.Info("Event processed successfully")
	}

	if handleErr != nil && errorMode == ShutdownOnError {
		log.Errorf("Cannot process event: %v", handleErr)
		w.Shutdown()

		return false, handleErr
	}

	return true, handleErr
}

// publishHandlerError delivers a failed handler's error back to the originator
// of the request, honouring EVENTS_WORKER_DELIVER_ERRORS and
// EVENTS_WORKER_ERRORS_TOPIC — both of which were read into the config and then
// never consulted.
func (w *EventsWorker) publishHandlerError(ctx context.Context, event *Event, handleErr ferr.FoundationError) ferr.FoundationError {
	originatorID := event.Headers[fkafka.HeaderOriginatorID]
	if originatorID == "" {
		return nil
	}

	if w.Config != nil && w.Config.EventsWorker != nil && !w.Config.EventsWorker.DeliverErrors {
		return nil
	}

	errorEvent, err := NewEventFromProto(handleErr.MarshalProto(), originatorID, nil)
	if err != nil {
		return err
	}

	if w.Config != nil {
		errorEvent.Topic = w.Config.EventsWorker.TopicFor(errorEvent.Topic)
	}

	return w.PublishEvent(ctx, errorEvent, nil)
}

func (w *EventsWorker) processEvent(ctx context.Context, handler EventHandler, event *Event, msg proto.Message) ferr.FoundationError {
	var (
		tx         pgx.Tx
		needCommit bool
		err        error
	)

	if w.Config.Database.Enabled {
		tx, err = w.GetPostgreSQL().Begin(ctx)
		if err != nil {
			return ferr.NewInternalError(err, "failed to begin transaction")
		}
		defer tx.Rollback(ctx) // nolint:errcheck
		needCommit = true

		// Add transaction to context
		ctx = fctx.WithTX(ctx, tx)
	}

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
		if err = tx.Commit(ctx); err != nil {
			return ferr.NewInternalError(err, "failed to commit transaction")
		}
	}

	return nil
}

const (
	// commitMessageAttempts is how many times a Kafka offset commit is tried
	// before giving up.
	commitMessageAttempts = 3

	// commitMessageBaseDelay is the first backoff step between commit attempts;
	// it doubles on every retry.
	commitMessageBaseDelay = 250 * time.Millisecond
)

// CommitMessage commits a Kafka message using the service's KafkaConsumer,
// retrying a few times with exponential backoff. The wait between attempts is
// interruptible, so a shutdown does not have to sit through it.
func (s *Service) CommitMessage(ctx context.Context, msg kafka.Message) ferr.FoundationError {
	var lastErr error

	for attempt := 0; attempt < commitMessageAttempts; attempt++ {
		if attempt > 0 {
			delay := commitMessageBaseDelay << (attempt - 1)

			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ferr.NewInternalError(lastErr, "failed to commit message before shutdown")
			case <-timer.C:
			}
		}

		if err := s.GetKafkaConsumer().CommitMessages(ctx, msg); err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	return ferr.NewInternalError(lastErr, "failed to commit message")
}
