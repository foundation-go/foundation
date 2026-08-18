//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	f "github.com/foundation-go/foundation"
	ferr "github.com/foundation-go/foundation/errors"
	ferrpb "github.com/foundation-go/foundation/errors/proto"
	fkafka "github.com/foundation-go/foundation/kafka"
)

// The events worker derives its topic from the proto package, so a message from
// `foundation.errors` lands on `foundation.errors`.
const errorsTopic = "foundation.errors"

func TestKafkaProducerComponentLifecycle(t *testing.T) {
	component := fkafka.NewProducerComponent(
		fkafka.WithProducerBrokers(kafkaBrokers),
		fkafka.WithProducerLogger(testLogger()),
	)

	require.NoError(t, component.Start())
	require.NotNil(t, component.Producer)

	assert.NoError(t, component.Health(), "a reachable broker must report healthy")
	assert.NoError(t, component.HealthContext(context.Background()))

	require.NoError(t, component.Stop())
}

func TestKafkaConsumerComponentLifecycle(t *testing.T) {
	topic := unique("lifecycle")
	createTopic(t, topic, 1)

	component := fkafka.NewConsumerComponent(
		fkafka.WithConsumerAppName("test"),
		fkafka.WithConsumerBrokers(kafkaBrokers),
		fkafka.WithConsumerTopics([]string{topic}),
		fkafka.WithConsumerLogger(testLogger()),
	)

	require.NoError(t, component.Start())
	require.NotNil(t, component.Consumer)

	// The consumer's health check used to be a TODO that always returned nil,
	// so an unreachable cluster never showed up in the readiness probe.
	assert.NoError(t, component.Health())

	require.NoError(t, component.Stop())
}

func TestKafkaHealthFailsAgainstAStoppedCluster(t *testing.T) {
	component := fkafka.NewProducerComponent(
		// Reserved as invalid by RFC 6890.
		fkafka.WithProducerBrokers([]string{"192.0.2.1:9092"}),
		fkafka.WithProducerLogger(testLogger()),
	)

	require.NoError(t, component.Start())
	t.Cleanup(func() { _ = component.Stop() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	assert.Error(t, component.HealthContext(ctx))
}

// The Kafka message carries the event's own timestamp. Leaving it unset let the
// broker stamp the moment of delivery, so an event that waited in the outbox
// looked younger than it was.
func TestPublishEventReachesKafkaWithItsHeadersAndTime(t *testing.T) {
	topic := unique("publish")
	createTopic(t, topic, 1)

	svc := newKafkaProducerService(t)

	createdAt := time.Now().Add(-90 * time.Second).Truncate(time.Millisecond)

	event, ferrErr := f.NewEventFromProto(&ferrpb.NotFoundError{Kind: "chat", Id: "1"}, "chat-1", nil)
	require.Nil(t, ferrErr)

	event.Topic = topic
	event.CreatedAt = createdAt

	ctx := context.Background()
	require.Nil(t, svc.PublishEvent(ctx, event, nil))

	message := readOne(t, topic)

	assert.Equal(t, "chat-1", string(message.Key))
	assert.WithinDuration(t, createdAt, message.Time, time.Second,
		"the broker must not restamp the event's creation time")

	headers := map[string]string{}
	for _, header := range message.Headers {
		headers[header.Key] = string(header.Value)
	}

	assert.Equal(t, "foundation.errors.NotFoundError", headers[fkafka.HeaderProtoName])

	var payload ferrpb.NotFoundError
	require.NoError(t, proto.Unmarshal(message.Value, &payload))
	assert.Equal(t, "chat", payload.Kind)
}

// The whole events worker, started the way a service starts it, against a real
// broker: handlers run, and the consumer group's committed offset advances.
func TestEventsWorkerHandlesEventsAndCommitsOffsets(t *testing.T) {
	createTopic(t, errorsTopic, 1)

	group := unique("worker")

	var (
		mu       sync.Mutex
		received []string
	)

	handler := handlerFunc(func(_ context.Context, _ *f.Event, msg proto.Message) ([]*f.Event, ferr.FoundationError) {
		mu.Lock()
		defer mu.Unlock()

		received = append(received, msg.(*ferrpb.NotFoundError).Id)

		return nil, nil
	})

	worker := startEventsWorker(t, group, map[proto.Message][]f.EventHandler{
		&ferrpb.NotFoundError{}: {handler},
	}, f.IgnoreError)
	defer worker.stop()

	produce(t, errorsTopic, "foundation.errors.NotFoundError", &ferrpb.NotFoundError{Kind: "chat", Id: "1"})
	produce(t, errorsTopic, "foundation.errors.NotFoundError", &ferrpb.NotFoundError{Kind: "chat", Id: "2"})

	eventually(t, 60*time.Second, "both events to be handled", func() bool {
		mu.Lock()
		defer mu.Unlock()

		return len(received) == 2
	})

	eventually(t, 30*time.Second, "the committed offset to reach 2", func() bool {
		return committedOffset(t, group, errorsTopic, 0) >= 2
	})

	mu.Lock()
	defer mu.Unlock()
	assert.ElementsMatch(t, []string{"1", "2"}, received)
}

// A worker subscribes to a whole topic, so most of what it reads is often of
// types it does not handle. Those offsets were never committed: the committed
// offset stopped advancing, consumer lag grew without bound, and every restart
// replayed the tail of the topic.
//
// The unit tests assert the commit *decision* against a fake. This asserts that
// a real consumer group's offset actually moves.
func TestEventsWorkerCommitsOffsetsForUnhandledEventTypes(t *testing.T) {
	createTopic(t, errorsTopic, 1)

	group := unique("unhandled")

	var handled int64

	handler := handlerFunc(func(context.Context, *f.Event, proto.Message) ([]*f.Event, ferr.FoundationError) {
		handled++

		return nil, nil
	})

	// The worker handles NotFoundError and nothing else.
	worker := startEventsWorker(t, group, map[proto.Message][]f.EventHandler{
		&ferrpb.NotFoundError{}: {handler},
	}, f.IgnoreError)
	defer worker.stop()

	// One handled event, then a tail of types the worker knows nothing about.
	produce(t, errorsTopic, "foundation.errors.NotFoundError", &ferrpb.NotFoundError{Kind: "chat", Id: "1"})

	const tail = 5
	for i := 0; i < tail; i++ {
		produce(t, errorsTopic, "foundation.errors.StaleObjectError", &ferrpb.StaleObjectError{Kind: "chat", Id: "x"})
	}

	// The committed offset has to reach the end of the tail, not stop at the
	// last event the worker had a handler for.
	eventually(t, 60*time.Second, "the committed offset to pass the unhandled tail", func() bool {
		return committedOffset(t, group, errorsTopic, 0) >= tail+1
	})

	assert.EqualValues(t, 1, handled, "only the handled type should reach the handler")
}

// A payload that cannot be parsed will not become parsable on a retry, so it
// must be committed rather than left to block its partition forever.
func TestEventsWorkerCommitsPastAnUnparsablePayload(t *testing.T) {
	createTopic(t, errorsTopic, 1)

	group := unique("poison")

	var handled int64

	handler := handlerFunc(func(context.Context, *f.Event, proto.Message) ([]*f.Event, ferr.FoundationError) {
		handled++

		return nil, nil
	})

	worker := startEventsWorker(t, group, map[proto.Message][]f.EventHandler{
		&ferrpb.NotFoundError{}: {handler},
	}, f.IgnoreError)
	defer worker.stop()

	// A message that claims to be a handled type but is not valid protobuf.
	writeRaw(t, errorsTopic, "foundation.errors.NotFoundError", []byte{0xff, 0xff, 0xff, 0xff})
	produce(t, errorsTopic, "foundation.errors.NotFoundError", &ferrpb.NotFoundError{Kind: "chat", Id: "after-poison"})

	eventually(t, 60*time.Second, "the event after the poison message to be handled", func() bool {
		return handled >= 1
	})

	eventually(t, 30*time.Second, "the committed offset to pass both messages", func() bool {
		return committedOffset(t, group, errorsTopic, 0) >= 2
	})
}

// KAFKA_CONSUMER_GROUP was hardcoded to `<app>-foundation`, so two workers of
// one application reading different topics took partitions from each other.
func TestConsumerGroupIsConfigurable(t *testing.T) {
	topic := unique("group")
	createTopic(t, topic, 1)

	group := unique("custom-group")

	component := fkafka.NewConsumerComponent(
		fkafka.WithConsumerAppName("test"),
		fkafka.WithConsumerBrokers(kafkaBrokers),
		fkafka.WithConsumerTopics([]string{topic}),
		fkafka.WithConsumerGroupID(group),
		fkafka.WithConsumerLogger(testLogger()),
	)

	require.NoError(t, component.Start())
	t.Cleanup(func() { _ = component.Stop() })

	writeRaw(t, topic, "x.Y", []byte("payload"))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	message, err := component.Consumer.FetchMessage(ctx)
	require.NoError(t, err)
	require.NoError(t, component.Consumer.CommitMessages(ctx, message))

	assert.GreaterOrEqual(t, committedOffset(t, group, topic, 0), int64(1),
		"the offset should be committed under the configured group")
}

//
// Helpers
//

// handlerFunc adapts a function to the EventHandler interface.
type handlerFunc func(context.Context, *f.Event, proto.Message) ([]*f.Event, ferr.FoundationError)

func (h handlerFunc) Handle(ctx context.Context, event *f.Event, msg proto.Message) ([]*f.Event, ferr.FoundationError) {
	return h(ctx, event, msg)
}

// newKafkaProducerService builds a service with a live Kafka producer.
func newKafkaProducerService(t *testing.T) *f.Service {
	t.Helper()

	t.Setenv("KAFKA_BROKERS", kafkaBrokers[0])
	t.Setenv("METRICS_ENABLED", "false")

	svc := &f.Service{Name: "test", Config: f.NewConfig(), Logger: testLogger()}
	svc.Config.Kafka.Producer.Enabled = true

	require.NoError(t, svc.StartComponents())
	t.Cleanup(svc.StopComponents)

	return svc
}

// runningWorker is a worker started in the background.
type runningWorker struct {
	worker  *f.EventsWorker
	stopped chan struct{}
	once    sync.Once
}

// stop asks the worker to shut down and waits for it.
func (w *runningWorker) stop() {
	w.once.Do(func() {
		w.worker.Shutdown()

		select {
		case <-w.stopped:
		case <-time.After(30 * time.Second):
		}
	})
}

// startEventsWorker starts a real events worker in the background.
//
// This runs the whole Service.Start path — components, signal handling, the
// spin loop and the drain — rather than calling the worker's internals, so the
// test covers what a deployed worker actually does.
func startEventsWorker(
	t *testing.T,
	group string,
	handlers map[proto.Message][]f.EventHandler,
	strategy f.ErrorHandlingStrategy,
) *runningWorker {
	t.Helper()

	t.Setenv("KAFKA_BROKERS", kafkaBrokers[0])
	t.Setenv("KAFKA_CONSUMER_GROUP", group)
	t.Setenv("METRICS_ENABLED", "false")
	t.Setenv("SHUTDOWN_TIMEOUT", "10s")

	worker := f.InitEventsWorker("test")
	worker.Logger = testLogger()

	running := &runningWorker{worker: worker, stopped: make(chan struct{})}

	go func() {
		defer close(running.stopped)

		worker.Start(&f.EventsWorkerOptions{
			ModeName:              "events_worker",
			Handlers:              handlers,
			ErrorHandlingStrategy: strategy,
		})
	}()

	t.Cleanup(running.stop)

	return running
}

// produce writes a protobuf event to a topic the way the framework does.
func produce(t *testing.T, topic, protoName string, msg proto.Message) {
	t.Helper()

	payload, err := proto.Marshal(msg)
	require.NoError(t, err)

	writeRaw(t, topic, protoName, payload)
}

// writeRaw writes an arbitrary payload, so that a test can produce something the
// worker cannot parse.
func writeRaw(t *testing.T, topic, protoName string, payload []byte) {
	t.Helper()

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(kafkaBrokers...),
		Topic:                  topic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	defer writer.Close() //nolint:errcheck // test helper

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	require.NoError(t, writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte("key"),
		Value: payload,
		Headers: []kafka.Header{
			{Key: fkafka.HeaderProtoName, Value: []byte(protoName)},
		},
	}))
}

// readOne reads a single message from the beginning of a topic.
func readOne(t *testing.T, topic string) kafka.Message {
	t.Helper()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   kafkaBrokers,
		Topic:     topic,
		Partition: 0,
		MaxWait:   time.Second,
	})
	defer reader.Close() //nolint:errcheck // test helper

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	message, err := reader.ReadMessage(ctx)
	require.NoError(t, err)

	return message
}
