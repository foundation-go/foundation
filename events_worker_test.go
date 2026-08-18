package foundation

import (
	"context"
	"io"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	ferr "github.com/foundation-go/foundation/errors"
	ferrpb "github.com/foundation-go/foundation/errors/proto"
	fkafka "github.com/foundation-go/foundation/kafka"
)

// recordingHandler captures the events it is given and optionally fails.
type recordingHandler struct {
	calls int
	err   ferr.FoundationError
}

func (h *recordingHandler) Handle(context.Context, *Event, proto.Message) ([]*Event, ferr.FoundationError) {
	h.calls++

	return nil, h.err
}

// shutdownProbe reports whether Shutdown has been called on a service.
type shutdownProbe struct {
	service *Service
}

func (p shutdownProbe) called() bool {
	select {
	case <-p.service.stopSignal():
		return true
	default:
		return false
	}
}

func newTestEventsWorker(t *testing.T, handlers map[proto.Message][]EventHandler) (*EventsWorker, shutdownProbe) {
	t.Helper()

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	svc := &Service{
		Name: "test",
		Config: &Config{
			Database: &DatabaseConfig{},
			Outbox:   &OutboxConfig{},
			Sentry:   &SentryConfig{},
		},
		Logger: logrus.NewEntry(logger),
	}

	registry, err := (&EventsWorkerOptions{Handlers: handlers}).Registry()
	require.NoError(t, err)

	return &EventsWorker{
		SpinWorker: &SpinWorker{Service: svc},
		registry:   registry,
	}, shutdownProbe{service: svc}
}

func newTestMessage(t *testing.T, msg proto.Message, headers map[string]string) kafka.Message {
	t.Helper()

	payload, err := proto.Marshal(msg)
	require.NoError(t, err)

	return newRawTestMessage(payload, ProtoToName(msg), headers)
}

func newRawTestMessage(payload []byte, protoName string, headers map[string]string) kafka.Message {
	kafkaHeaders := []kafka.Header{{Key: fkafka.HeaderProtoName, Value: []byte(protoName)}}
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{Key: k, Value: []byte(v)})
	}

	return kafka.Message{
		Topic:     "foundation.errors",
		Partition: 0,
		Offset:    42,
		Value:     payload,
		Headers:   kafkaHeaders,
	}
}

func TestHandleMessageCommitsHandledEvent(t *testing.T) {
	handler := &recordingHandler{}
	template := &ferrpb.NotFoundError{}

	worker, shutdown := newTestEventsWorker(t, map[proto.Message][]EventHandler{
		template: {handler},
	})

	msg := newTestMessage(t, &ferrpb.NotFoundError{Kind: "chat", Id: "1"}, nil)

	shouldCommit, err := worker.handleMessage(context.Background(), msg, IgnoreError)

	assert.True(t, shouldCommit)
	assert.Nil(t, err)
	assert.Equal(t, 1, handler.calls)
	assert.False(t, shutdown.called())
}

// A worker subscribes to whole topics, so it constantly reads event types it
// does not handle. Those offsets were never committed, which pinned the
// committed offset behind the read position: the lag grew without bound and
// every restart replayed the tail of the topic.
func TestHandleMessageCommitsEventWithoutHandlers(t *testing.T) {
	worker, shutdown := newTestEventsWorker(t, map[proto.Message][]EventHandler{
		&ferrpb.NotFoundError{}: {&recordingHandler{}},
	})

	msg := newTestMessage(t, &ferrpb.StaleObjectError{Kind: "chat"}, nil)

	shouldCommit, err := worker.handleMessage(context.Background(), msg, IgnoreError)

	assert.True(t, shouldCommit, "an unhandled event must still advance the committed offset")
	assert.Nil(t, err)
	assert.False(t, shutdown.called())
}

func TestHandleMessageCommitsEventWithoutProtoNameHeader(t *testing.T) {
	worker, _ := newTestEventsWorker(t, map[proto.Message][]EventHandler{
		&ferrpb.NotFoundError{}: {&recordingHandler{}},
	})

	msg := newRawTestMessage([]byte("whatever"), "", nil)

	shouldCommit, err := worker.handleMessage(context.Background(), msg, IgnoreError)

	assert.True(t, shouldCommit)
	assert.Nil(t, err)
}

// An unparsable payload will not become parsable on a retry. It has to be
// reported and committed, otherwise it blocks its partition forever.
func TestHandleMessageCommitsUnparsablePayload(t *testing.T) {
	handler := &recordingHandler{}
	template := &ferrpb.NotFoundError{}
	handlers := map[proto.Message][]EventHandler{template: {handler}}

	worker, shutdown := newTestEventsWorker(t, handlers)

	msg := newRawTestMessage([]byte{0xff, 0xff, 0xff, 0xff}, ProtoToName(template), nil)

	shouldCommit, err := worker.handleMessage(context.Background(), msg, IgnoreError)

	assert.True(t, shouldCommit)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal payload")
	// The offset has to travel with the error: it is reported once, by the
	// caller, and whoever reads it needs to know what to re-drive.
	assert.Contains(t, err.Error(), "foundation.errors/0 offset 42")
	assert.Equal(t, 0, handler.calls)
	assert.False(t, shutdown.called())
}

func TestHandleMessageShutsDownOnUnparsablePayloadWhenAsked(t *testing.T) {
	handler := &recordingHandler{}
	template := &ferrpb.NotFoundError{}
	handlers := map[proto.Message][]EventHandler{template: {handler}}

	worker, shutdown := newTestEventsWorker(t, handlers)

	msg := newRawTestMessage([]byte{0xff, 0xff, 0xff, 0xff}, ProtoToName(template), nil)

	shouldCommit, err := worker.handleMessage(context.Background(), msg, ShutdownOnError)

	assert.False(t, shouldCommit, "ShutdownOnError must leave the offset for investigation")
	assert.NotNil(t, err)
	assert.True(t, shutdown.called())
}

func TestHandleMessageCommitsAfterHandlerErrorByDefault(t *testing.T) {
	handler := &recordingHandler{err: ferr.NewNotFoundError(nil, "chat", "1")}
	template := &ferrpb.NotFoundError{}
	handlers := map[proto.Message][]EventHandler{template: {handler}}

	worker, shutdown := newTestEventsWorker(t, handlers)

	msg := newTestMessage(t, &ferrpb.NotFoundError{Kind: "chat", Id: "1"}, nil)

	shouldCommit, err := worker.handleMessage(context.Background(), msg, IgnoreError)

	assert.True(t, shouldCommit)
	assert.NotNil(t, err)
	assert.Equal(t, 1, handler.calls)
	assert.False(t, shutdown.called())
}

func TestHandleMessageShutsDownOnHandlerError(t *testing.T) {
	handler := &recordingHandler{err: ferr.NewNotFoundError(nil, "chat", "1")}
	template := &ferrpb.NotFoundError{}
	handlers := map[proto.Message][]EventHandler{template: {handler}}

	worker, shutdown := newTestEventsWorker(t, handlers)

	msg := newTestMessage(t, &ferrpb.NotFoundError{Kind: "chat", Id: "1"}, nil)

	shouldCommit, err := worker.handleMessage(context.Background(), msg, ShutdownOnError)

	assert.False(t, shouldCommit)
	assert.NotNil(t, err)
	assert.True(t, shutdown.called())
}

// The first failing handler stops the chain.
func TestHandleMessageStopsAfterFirstFailingHandler(t *testing.T) {
	failing := &recordingHandler{err: ferr.NewNotFoundError(nil, "chat", "1")}
	next := &recordingHandler{}
	template := &ferrpb.NotFoundError{}
	handlers := map[proto.Message][]EventHandler{template: {failing, next}}

	worker, _ := newTestEventsWorker(t, handlers)

	msg := newTestMessage(t, &ferrpb.NotFoundError{Kind: "chat", Id: "1"}, nil)

	_, err := worker.handleMessage(context.Background(), msg, IgnoreError)

	assert.NotNil(t, err)
	assert.Equal(t, 1, failing.calls)
	assert.Equal(t, 0, next.calls, "handlers after a failure must not run")
}

func TestEventsWorkerGetTopics(t *testing.T) {
	t.Run("explicit topics win", func(t *testing.T) {
		opts := &EventsWorkerOptions{Topics: []string{"b", "a"}}
		assert.Equal(t, []string{"b", "a"}, opts.GetTopics())
	})

	t.Run("derived from handled protos, sorted and deduplicated", func(t *testing.T) {
		opts := &EventsWorkerOptions{
			Handlers: map[proto.Message][]EventHandler{
				&ferrpb.NotFoundError{}:    {&recordingHandler{}},
				&ferrpb.StaleObjectError{}: {&recordingHandler{}},
			},
		}

		assert.Equal(t, []string{"foundation.errors"}, opts.GetTopics())
	})

	t.Run("no handlers, no topics", func(t *testing.T) {
		assert.Nil(t, (&EventsWorkerOptions{}).GetTopics())
	})
}

func TestShutdownIsSafeWithoutStart(t *testing.T) {
	assert.NotPanics(t, func() { (&Service{}).Shutdown() })
}

// The Handlers map is keyed by pointer identity, so two separately constructed
// values of the same message type are different keys. The name-keyed lookup
// used at runtime silently kept only one of them, and half the handlers never
// ran.
func TestRegistryRejectsDuplicateEventTypes(t *testing.T) {
	opts := &EventsWorkerOptions{
		Handlers: map[proto.Message][]EventHandler{
			&ferrpb.NotFoundError{}: {&recordingHandler{}},
			// A second, distinct pointer to the same message type.
			&ferrpb.NotFoundError{}: {&recordingHandler{}},
		},
	}

	// Two composite-literal keys of the same type are distinct pointers, so the
	// map really does hold two entries.
	require.Len(t, opts.Handlers, 2)

	_, err := opts.Registry()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "two different proto.Message keys")
	assert.Contains(t, err.Error(), "foundation.errors.NotFoundError")
}

func TestRegistryAcceptsDistinctEventTypes(t *testing.T) {
	registry, err := (&EventsWorkerOptions{
		Handlers: map[proto.Message][]EventHandler{
			&ferrpb.NotFoundError{}:    {&recordingHandler{}},
			&ferrpb.StaleObjectError{}: {&recordingHandler{}, &recordingHandler{}},
		},
	}).Registry()

	require.NoError(t, err)
	require.Len(t, registry, 2)
	assert.Len(t, registry["foundation.errors.StaleObjectError"].handlers, 2)
	assert.NotNil(t, registry["foundation.errors.NotFoundError"].template)
}

// A message with no proto package yields an empty topic. Publishing to a topic
// named "" fails at the broker, far from the cause.
func TestProtoNameToTopic(t *testing.T) {
	assert.Equal(t, "project.service", ProtoNameToTopic("project.service.SomeEvent"))
	assert.Equal(t, "service", ProtoNameToTopic("service.SomeEvent"))
	assert.Equal(t, "", ProtoNameToTopic("SomeEvent"))
	assert.Equal(t, "", ProtoNameToTopic(""))
	assert.Equal(t, "", ProtoNameToTopic(".SomeEvent"))
}

// EVENTS_WORKER_ERRORS_TOPIC and EVENTS_WORKER_DELIVER_ERRORS were read into
// the config, documented in ENV.md, and then never consulted.
func TestEventsWorkerConfigTopicFor(t *testing.T) {
	assert.Equal(t, "foundation.errors", (&EventsWorkerConfig{}).TopicFor("foundation.errors"))
	assert.Equal(t, "errors.v2", (&EventsWorkerConfig{ErrorsTopic: "errors.v2"}).TopicFor("foundation.errors"))

	var nilConfig *EventsWorkerConfig
	assert.Equal(t, "foundation.errors", nilConfig.TopicFor("foundation.errors"))
}

func TestPublishHandlerErrorRespectsDeliverErrors(t *testing.T) {
	worker, _ := newTestEventsWorker(t, map[proto.Message][]EventHandler{
		&ferrpb.NotFoundError{}: {&recordingHandler{}},
	})
	worker.Config.EventsWorker = &EventsWorkerConfig{DeliverErrors: false}

	event := &Event{Headers: map[string]string{fkafka.HeaderOriginatorID: "user-1"}}

	// With delivery disabled nothing is published, so the missing Kafka
	// producer is never reached.
	require.NotPanics(t, func() {
		assert.Nil(t, worker.publishHandlerError(
			context.Background(), event, ferr.NewNotFoundError(nil, "chat", "1"),
		))
	})
}

// Without an originator there is nobody to deliver the error to.
func TestPublishHandlerErrorWithoutAnOriginator(t *testing.T) {
	worker, _ := newTestEventsWorker(t, map[proto.Message][]EventHandler{
		&ferrpb.NotFoundError{}: {&recordingHandler{}},
	})
	worker.Config.EventsWorker = &EventsWorkerConfig{DeliverErrors: true}

	require.NotPanics(t, func() {
		assert.Nil(t, worker.publishHandlerError(
			context.Background(), &Event{Headers: map[string]string{}}, ferr.NewNotFoundError(nil, "chat", "1"),
		))
	})
}
