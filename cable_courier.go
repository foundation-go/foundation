package foundation

import (
	"context"
	"fmt"

	cablecourier "github.com/foundation-go/foundation/cable/courier"
	ferr "github.com/foundation-go/foundation/errors"
	ferrpb "github.com/foundation-go/foundation/errors/proto"
	fkafka "github.com/foundation-go/foundation/kafka"
	"github.com/getsentry/sentry-go"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// CableCourier is a mode in which events are received from Kafka and published
// to Redis PubSub channels for AnyCable.
type CableCourier struct {
	*EventsWorker

	Options *CableCourierOptions
}

// InitCableCourier initializes a new CableCourier.
func InitCableCourier(name string) *CableCourier {
	return &CableCourier{
		EventsWorker: InitEventsWorker(name),
	}
}

// CableMessageResolver is a function that resolves the stream name for a given event.
type CableMessageResolver func(context.Context, *Event, proto.Message) (string, error)

// CableCourierResolvers maps proto.Message types to their corresponding resolvers.
type CableCourierResolvers map[proto.Message][]CableMessageResolver

// CableMessageEventHandler is a concrete implementation of EventHandler that uses
// a CableMessageResolver to handle events.
type CableMessageEventHandler struct {
	// Resolver is used to resolve the stream name for a given event.
	Resolver CableMessageResolver
	Logger   *logrus.Entry
	Redis    *redis.Client
}

// CableCourierOptions contains configuration options to instantiate a CableCourier.
// It maps protocol names to their corresponding message resolvers.
type CableCourierOptions struct {
	// Resolvers map protocol names to lists of CableMessageResolvers.
	Resolvers map[proto.Message][]CableMessageResolver
}

// EventHandlers takes the resolvers defined in CableCourierOptions and wraps them
// into event handlers.
func (opts *CableCourierOptions) EventHandlers(s *Service) map[proto.Message][]EventHandler {
	handlers := make(map[proto.Message][]EventHandler)

	errors := []proto.Message{
		&ferrpb.InternalError{},
		&ferrpb.UnauthenticatedError{},
		&ferrpb.StaleObjectError{},
		&ferrpb.NotFoundError{},
		&ferrpb.PermissionDeniedError{},
		&ferrpb.InvalidArgumentError{},
	}

	// Add default resolvers for errors, if not already defined
	for _, err := range errors {
		if _, ok := opts.Resolvers[err]; !ok {
			opts.Resolvers[err] = []CableMessageResolver{
				CableDefaultErrorResolver,
			}
		}
	}

	// Wrap resolvers into event handlers
	for protoObj, resolvers := range opts.Resolvers {
		for _, resolver := range resolvers {
			handlers[protoObj] = append(handlers[protoObj], &CableMessageEventHandler{
				Resolver: resolver,
				Logger:   s.Logger.WithField("proto", ProtoToName(protoObj)),
				Redis:    s.GetRedis(),
			})
		}
	}

	return handlers
}

// Start runs a cable_courier worker using the given CableCourierOptions.
func (c *CableCourier) Start(opts *CableCourierOptions) {
	ewOpts := &EventsWorkerOptions{
		ModeName: "cable_courier",
		Handlers: opts.EventHandlers(c.Service),
	}

	c.EventsWorker.Start(ewOpts)
}

// Handle uses the associated CableMessageResolver to determine the appropriate stream
// for the event and broadcasts the message to that stream.
func (h *CableMessageEventHandler) Handle(ctx context.Context, event *Event, msg proto.Message) ([]*Event, ferr.FoundationError) {
	stream, err := h.Resolver(ctx, event, msg)
	if err != nil {
		// We don't want the event_worker to broadcast any errors from the cable_courier
		// in order to avoid infinite loops of error messages. Instead, we log the error
		// and capture it with Sentry.
		err := fmt.Errorf("failed to get stream for event: %w", err)
		sentry.CaptureException(err)
		h.Logger.Error(err)

		return nil, nil
	}

	// If the stream is empty, we don't want to broadcast the message.
	if stream == "" {
		return nil, nil
	}

	// Broadcast the message to the stream.
	cablecourier.NewClient(h.Redis).BroadcastMessage(
		event.ProtoName,
		msg,
		stream,
		event.Headers[fkafka.HeaderCorrelationID],
	)

	return nil, nil
}

// CableDefaultErrorResolver is a default resolver for errors that returns a stream
// name based on the user ID in the event headers.
func CableDefaultErrorResolver(_ context.Context, event *Event, _ proto.Message) (string, error) {
	userID := event.Headers[fkafka.HeaderOriginatorID]

	if userID == "" {
		return "", nil
	}

	return fmt.Sprintf("user:%s", event.Headers[fkafka.HeaderOriginatorID]), nil
}
