package foundation

import (
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	ferrpb "github.com/foundation-go/foundation/errors/proto"
)

func courierTestService() *Service {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	return &Service{
		Name:   "test",
		Config: NewConfig(),
		Logger: logrus.NewEntry(logger),
	}
}

func resolverStub(_ ...int) CableMessageResolver {
	return CableDefaultErrorResolver
}

// The default error resolvers were added under the framework's own pointer, so
// a caller who registered a resolver for the same error type held a different
// key: the map ended up with two entries for one event. That used to drop one
// of them silently, and once the events worker started rejecting duplicate
// registrations it would have stopped the courier from booting at all.
func TestEventHandlersDoNotDuplicateOverriddenErrorResolvers(t *testing.T) {
	opts := &CableCourierOptions{
		Resolvers: CableCourierResolvers{
			&ferrpb.InvalidArgumentError{}: {resolverStub()},
			&ferrpb.InternalError{}:        {resolverStub()},
		},
	}

	handlers := opts.EventHandlers(courierTestService())

	// One entry per event type, whoever registered it.
	byName := map[string]int{}
	for protoObj := range handlers {
		byName[ProtoToName(protoObj)]++
	}

	for name, count := range byName {
		assert.Equal(t, 1, count, "%s is registered %d times", name, count)
	}

	// And the worker accepts the result.
	_, err := (&EventsWorkerOptions{Handlers: handlers}).Registry()
	require.NoError(t, err)
}

func TestEventHandlersAddDefaultsForUncoveredErrors(t *testing.T) {
	opts := &CableCourierOptions{Resolvers: CableCourierResolvers{}}

	handlers := opts.EventHandlers(courierTestService())

	names := map[string]bool{}
	for protoObj := range handlers {
		names[ProtoToName(protoObj)] = true
	}

	for _, name := range []string{
		"foundation.errors.InternalError",
		"foundation.errors.UnauthenticatedError",
		"foundation.errors.StaleObjectError",
		"foundation.errors.NotFoundError",
		"foundation.errors.PermissionDeniedError",
		"foundation.errors.InvalidArgumentError",
	} {
		assert.True(t, names[name], "%s should have a default resolver", name)
	}
}

func TestEventHandlersWithNilResolvers(t *testing.T) {
	opts := &CableCourierOptions{}

	var handlers map[proto.Message][]EventHandler

	require.NotPanics(t, func() { handlers = opts.EventHandlers(courierTestService()) })

	assert.Len(t, handlers, 6)
}
