package foundation

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"

	cablecourier "github.com/foundation-go/foundation/cable/courier"
	ferr "github.com/foundation-go/foundation/errors"
	ferrpb "github.com/foundation-go/foundation/errors/proto"
	fkafka "github.com/foundation-go/foundation/kafka"
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
	Resolver     CableMessageResolver
	Logger       *logrus.Entry
	Service      *Service
	RedisChannel string
}

// CableCourierOptions contains configuration options to instantiate a CableCourier.
// It maps protocol names to their corresponding message resolvers.
type CableCourierOptions struct {
	// Resolvers map protocol names to lists of CableMessageResolvers.
	Resolvers map[proto.Message][]CableMessageResolver
	// RedisChannel is the name of the Redis PubSub channel to publish events to.
	RedisChannel string
}

// EventHandlers takes the resolvers defined in CableCourierOptions and wraps them
// into event handlers.
func (opts *CableCourierOptions) EventHandlers(s *Service) map[proto.Message][]EventHandler {
	handlers := make(map[proto.Message][]EventHandler)

	// A courier constructed as &CableCourierOptions{} has a nil map, and the
	// default error resolvers below used to be written straight into it:
	// "assignment to entry in nil map".
	if opts.Resolvers == nil {
		opts.Resolvers = make(CableCourierResolvers)
	}

	errors := []proto.Message{
		&ferrpb.InternalError{},
		&ferrpb.UnauthenticatedError{},
		&ferrpb.StaleObjectError{},
		&ferrpb.NotFoundError{},
		&ferrpb.PermissionDeniedError{},
		&ferrpb.InvalidArgumentError{},
	}

	// Add default resolvers for the error types the caller has not covered.
	//
	// The check has to be by proto name, not by map key. The keys here are
	// pointers, so a caller who registered their own `&ferrpb.InternalError{}`
	// holds a different key from the one built above: looking up by key missed
	// it and added a second entry for the same event type. That used to mean
	// one of the two resolvers was silently dropped; now that the events worker
	// rejects duplicate registrations, it would stop the courier from starting
	// at all.
	registered := make(map[string]bool, len(opts.Resolvers))
	for protoObj := range opts.Resolvers {
		registered[ProtoToName(protoObj)] = true
	}

	for _, err := range errors {
		if registered[ProtoToName(err)] {
			continue
		}

		opts.Resolvers[err] = []CableMessageResolver{CableDefaultErrorResolver}
		registered[ProtoToName(err)] = true
	}

	// Wrap resolvers into event handlers
	for protoObj, resolvers := range opts.Resolvers {
		for _, resolver := range resolvers {
			handlers[protoObj] = append(handlers[protoObj], &CableMessageEventHandler{
				Resolver:     resolver,
				Logger:       s.Logger.WithField("proto", ProtoToName(protoObj)),
				Service:      s,
				RedisChannel: opts.RedisChannel,
			})
		}
	}

	return handlers
}

// DefaultAnyCableRedisChannel is the Redis PubSub channel AnyCable listens on.
const DefaultAnyCableRedisChannel = "__anycable__"

// Start runs a cable_courier worker using the given CableCourierOptions.
func (c *CableCourier) Start(opts *CableCourierOptions) {
	// The nil check used to guard only the channel default, while
	// opts.EventHandlers below was called unconditionally — so a nil opts
	// produced a nil dereference two lines later instead of a clear error.
	if opts == nil {
		opts = &CableCourierOptions{}
	}

	if opts.RedisChannel == "" {
		opts.RedisChannel = GetEnvOrString("ANYCABLE_REDIS_CHANNEL", DefaultAnyCableRedisChannel)
	}

	ewOpts := &EventsWorkerOptions{
		ModeName: "cable_courier",
		Handlers: opts.EventHandlers(c.Service),
		StartComponentsOptions: []StartComponentsOption{
			WithRedis(),
		},
	}

	// Errors are published to EVENTS_WORKER_ERRORS_TOPIC when it is set, which
	// is not a topic any handled proto name would produce. Subscribe to it too,
	// or the courier would never see the errors it exists to deliver.
	if errorsTopic := c.Config.EventsWorker.ErrorsTopic; errorsTopic != "" {
		ewOpts.Topics = append(ewOpts.GetTopics(), errorsTopic)
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
		err = fmt.Errorf("failed to get stream for event: %w", err)
		sentry.CaptureException(err)
		h.Logger.Error(err)

		return nil, nil
	}

	// If the stream is empty, we don't want to broadcast the message.
	if stream == "" {
		return nil, nil
	}

	// Broadcast the message to the stream.
	// If the broadcast fails, we log the error, capture it with Sentry and go on to avoid infinite loops.
	err = cablecourier.NewClient(h.Service.GetRedis(), h.RedisChannel).BroadcastMessage(
		ctx,
		event.ProtoName,
		msg,
		stream,
		event.Headers[fkafka.HeaderCorrelationID],
	)
	if err != nil {
		err = fmt.Errorf("failed to broadcast message: %w", err)
		sentry.CaptureException(err)
		h.Logger.Error(err)

		return nil, nil
	}

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
