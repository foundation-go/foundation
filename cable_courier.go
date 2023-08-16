package foundation

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	cable_courier "github.com/ri-nat/foundation/cable/courier"
	fctx "github.com/ri-nat/foundation/context"
	fkafka "github.com/ri-nat/foundation/kafka"
	"google.golang.org/protobuf/proto"
)

// CableMessageResolver provides an interface to translate messages from Kafka
// into corresponding streams for AnyCable.
type CableMessageResolver interface {
	// GetStream determines the appropriate stream name for a given event.
	GetStream(ctx context.Context, event *Event) (string, error)
	GetMessage(ctx context.Context, event *Event) (proto.Message, error)
}

// CableMessageEventHandler is a concrete implementation of EventHandler that uses
// a CableMessageResolver to handle events.
type CableMessageEventHandler struct {
	// Resolver is used to resolve the stream name for a given event.
	Resolver CableMessageResolver
}

// CableCourierOptions contains configuration options to instantiate a CableCourier.
// It maps protocol names to their corresponding message resolvers.
type CableCourierOptions struct {
	// Resolvers map protocol names to lists of CableMessageResolvers.
	Resolvers map[string][]CableMessageResolver
}

// GetEventHandlers takes the resolvers defined in CableCourierOptions and wraps them
// into event handlers.
func (opts *CableCourierOptions) GetEventHandlers() map[string][]EventHandler {
	handlers := make(map[string][]EventHandler)

	for protoName, resolvers := range opts.Resolvers {
		for _, resolver := range resolvers {
			handlers[protoName] = append(handlers[protoName], &CableMessageEventHandler{
				Resolver: resolver,
			})
		}
	}

	return handlers
}

// StartCableCourier initializes a cable_courier worker using the given CableCourierOptions.
func (s *Service) StartCableCourier(opts *CableCourierOptions) {
	ewOpts := &EventsWorkerOptions{
		ModeName: "cable_courier",
		Handlers: opts.GetEventHandlers(),
	}

	s.StartEventsWorker(ewOpts)
}

// Handle uses the associated CableMessageResolver to determine the appropriate stream
// for the event and broadcasts the message to that stream.
func (h *CableMessageEventHandler) Handle(ctx context.Context, event *Event) ([]*Event, FoundationError) {
	logger := fctx.GetLogger(ctx)

	stream, err := h.Resolver.GetStream(ctx, event)
	if err != nil {
		// We don't want the event_worker to broadcast any errors from the cable_courier
		// in order to avoid infinite loops of error messages. Instead, we log the error
		// and capture it with Sentry.
		err := fmt.Errorf("failed to get stream for event: %w", err)
		sentry.CaptureException(err)
		logger.Error(err)

		return nil, nil
	}

	msg, err := h.Resolver.GetMessage(ctx, event)
	if err != nil {
		err := fmt.Errorf("failed to get message for event: %w", err)
		sentry.CaptureException(err)
		logger.Error(err)

		return nil, nil
	}

	// Broadcast the message to the stream.
	cable_courier.NewClient(fctx.GetRedis(ctx)).BroadcastMessage(
		event.ProtoName,
		msg,
		stream,
		event.Headers[fkafka.HeaderCorrelationID],
	)

	return nil, nil
}
