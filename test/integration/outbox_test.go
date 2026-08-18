//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	f "github.com/foundation-go/foundation"
	ferrpb "github.com/foundation-go/foundation/errors/proto"
)

// The transactional outbox is the framework's central delivery guarantee, and
// every one of these properties depends on how Postgres and Kafka actually
// behave under concurrency.

func TestOutboxRoundTrip(t *testing.T) {
	topic := unique("outbox")
	createTopic(t, topic, 1)

	pool := newPool(t)
	truncateOutbox(t, pool)

	// A service with the outbox enabled writes the event to the table rather
	// than to Kafka.
	producer := newOutboxProducerService(t)

	event, ferrErr := f.NewEventFromProto(&ferrpb.NotFoundError{Kind: "chat", Id: "1"}, "chat-1", nil)
	require.Nil(t, ferrErr)
	event.Topic = topic

	require.Nil(t, producer.PublishEvent(context.Background(), event, nil))

	assert.Equal(t, 1, countAllOutboxRows(t, pool), "the event should be waiting in the outbox")
	assertNoMessages(t, topic, 2*time.Second)

	// Now the courier moves it.
	courier := startOutboxCourier(t, 100, 10*time.Millisecond)
	defer courier.stop()

	messages := readMessages(t, topic, 1, 60*time.Second)
	require.Len(t, messages, 1)
	assert.Equal(t, "chat-1", string(messages[0].Key))

	eventually(t, 30*time.Second, "the outbox to be emptied", func() bool {
		return countAllOutboxRows(t, pool) == 0
	})
}

// The headline property, and the one that cannot be established by reading the
// code: `FOR UPDATE SKIP LOCKED` really does let two couriers share the work
// without either publishing an event the other already published.
//
// Before the lock, both replicas read the same rows and both published them:
// every event was delivered twice for as long as both were up.
func TestTwoConcurrentCouriersPublishEachEventExactlyOnce(t *testing.T) {
	const events = 300

	topic := unique("concurrent")
	createTopic(t, topic, 1)

	pool := newPool(t)
	truncateOutbox(t, pool)

	seedOutbox(t, pool, topic, events)

	// Small batches and a tight interval, so the two couriers genuinely
	// interleave rather than one draining the table before the other wakes.
	first := startOutboxCourier(t, 20, 5*time.Millisecond)
	defer first.stop()

	second := startOutboxCourier(t, 20, 5*time.Millisecond)
	defer second.stop()

	messages := readMessages(t, topic, events, 120*time.Second)

	counts := map[string]int{}
	for _, message := range messages {
		counts[string(message.Key)]++
	}

	var duplicated, missing []string

	for i := 0; i < events; i++ {
		key := fmt.Sprintf("event-%d", i)

		switch counts[key] {
		case 1:
		case 0:
			missing = append(missing, key)
		default:
			duplicated = append(duplicated, fmt.Sprintf("%s×%d", key, counts[key]))
		}
	}

	assert.Empty(t, duplicated, "no event may be published twice")
	assert.Empty(t, missing, "no event may be lost")
	assert.Len(t, counts, events)

	eventually(t, 30*time.Second, "the outbox to be emptied", func() bool {
		return countAllOutboxRows(t, pool) == 0
	})
}

// Deleting everything up to a maximum id would remove rows a concurrent courier
// had locked but not yet committed — and if that courier rolled back, those
// events would be gone without ever having been published. The courier deletes
// exactly the rows it published, by id.
func TestCourierDeletesOnlyWhatItPublished(t *testing.T) {
	topic := unique("delete")
	createTopic(t, topic, 1)

	pool := newPool(t)
	truncateOutbox(t, pool)

	seedOutbox(t, pool, topic, 10)

	// Hold the lowest-id row in an open transaction, as a stalled courier
	// would. A courier that deletes "everything up to the highest id it saw"
	// would take this row with it.
	tx, err := pool.Begin(context.Background())
	require.NoError(t, err)

	defer tx.Rollback(context.Background()) //nolint:errcheck // rolled back below

	var lockedID int64
	require.NoError(t, tx.QueryRow(context.Background(),
		"SELECT id FROM foundation_outbox_events ORDER BY id ASC LIMIT 1 FOR UPDATE").Scan(&lockedID))

	courier := startOutboxCourier(t, 100, 5*time.Millisecond)
	defer courier.stop()

	// The other nine are published and removed...
	eventually(t, 60*time.Second, "the unlocked rows to be published", func() bool {
		return countAllOutboxRows(t, pool) == 1
	})

	// ...and the locked one is still there, untouched.
	var remaining int64
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT id FROM foundation_outbox_events").Scan(&remaining))
	assert.Equal(t, lockedID, remaining, "a row another transaction holds must survive")

	// Once released, it is picked up like any other.
	require.NoError(t, tx.Rollback(context.Background()))

	eventually(t, 60*time.Second, "the released row to be published", func() bool {
		return countAllOutboxRows(t, pool) == 0
	})

	messages := readMessages(t, topic, 10, 60*time.Second)
	assert.Len(t, messages, 10)
}

// The pending gauge used to report the size of the batch just read, which can
// never exceed the limit it was read with — so an alert on a growing backlog
// never fired.
func TestCountOutboxEventsReportsTheWholeBacklog(t *testing.T) {
	pool := newPool(t)
	truncateOutbox(t, pool)

	seedOutbox(t, pool, unique("backlog"), 250)

	svc := newDatabaseService(t)

	tx, err := svc.GetPostgreSQL().Begin(context.Background())
	require.NoError(t, err)

	defer tx.Rollback(context.Background()) //nolint:errcheck // read-only

	count, ferrErr := svc.CountOutboxEvents(context.Background(), tx)
	require.Nil(t, ferrErr)

	assert.EqualValues(t, 250, count, "the backlog is what an alert needs, not the batch size")
}

// A courier that is stopped mid-run must leave the table consistent: an event
// is either published and deleted, or neither.
func TestCourierShutdownLeavesNoHalfDoneWork(t *testing.T) {
	const events = 100

	topic := unique("shutdown")
	createTopic(t, topic, 1)

	pool := newPool(t)
	truncateOutbox(t, pool)

	seedOutbox(t, pool, topic, events)

	courier := startOutboxCourier(t, 10, 5*time.Millisecond)

	// Stop it while it is working.
	eventually(t, 60*time.Second, "the courier to start draining the outbox", func() bool {
		return countAllOutboxRows(t, pool) < events
	})

	courier.stop()

	published := len(readMessages(t, topic, events, 5*time.Second))
	remaining := countAllOutboxRows(t, pool)

	// Every event is accounted for exactly once: published, or still waiting.
	// A row deleted without being published would show up as a shortfall.
	assert.Equal(t, events, published+remaining,
		"published %d and %d still in the outbox, from %d seeded", published, remaining, events)
}

//
// Helpers
//

// newOutboxProducerService builds a service that writes events to the outbox.
func newOutboxProducerService(t *testing.T) *f.Service {
	t.Helper()

	t.Setenv("DATABASE_URL", postgresURL)
	t.Setenv("METRICS_ENABLED", "false")

	svc := &f.Service{Name: "test", Config: f.NewConfig(), Logger: testLogger()}
	svc.Config.Outbox.Enabled = true

	require.NoError(t, svc.StartComponents())
	t.Cleanup(svc.StopComponents)

	return svc
}

// runningCourier is an outbox courier started in the background.
type runningCourier struct {
	courier *f.OutboxCourier
	stopped chan struct{}
	once    sync.Once
}

func (c *runningCourier) stop() {
	c.once.Do(func() {
		c.courier.Shutdown()

		select {
		case <-c.stopped:
		case <-time.After(30 * time.Second):
		}
	})
}

// startOutboxCourier starts a real courier in the background, through the same
// Service.Start path a deployed courier uses.
func startOutboxCourier(t *testing.T, batchSize int32, interval time.Duration) *runningCourier {
	t.Helper()

	t.Setenv("DATABASE_URL", postgresURL)
	t.Setenv("KAFKA_BROKERS", kafkaBrokers[0])
	t.Setenv("METRICS_ENABLED", "false")
	t.Setenv("SHUTDOWN_TIMEOUT", "10s")

	courier := f.InitOutboxCourier("test")
	courier.Logger = testLogger()

	running := &runningCourier{courier: courier, stopped: make(chan struct{})}

	go func() {
		defer close(running.stopped)

		opts := f.NewOutboxCourierOptions()
		opts.BatchSize = batchSize
		opts.Interval = interval

		courier.Start(opts)
	}()

	t.Cleanup(running.stop)

	return running
}

// seedOutbox inserts events straight into the table, as a service publishing
// inside a transaction would.
func seedOutbox(t *testing.T, pool *pgxpool.Pool, topic string, count int) {
	t.Helper()

	// created_at is NOT NULL without a default: the framework's own INSERT
	// always supplies NOW(), so the seed has to as well.
	createdAt := time.Now()

	batch := make([][]interface{}, 0, count)
	for i := 0; i < count; i++ {
		batch = append(batch, []interface{}{
			topic,
			fmt.Sprintf("event-%d", i),
			[]byte(fmt.Sprintf("payload-%d", i)),
			[]byte(`{"proto-name":"foundation.errors.NotFoundError"}`),
			createdAt,
		})
	}

	_, err := pool.CopyFrom(
		context.Background(),
		pgx.Identifier{"foundation_outbox_events"},
		[]string{"topic", "key", "payload", "headers", "created_at"},
		pgx.CopyFromRows(batch),
	)
	require.NoError(t, err)
}

func countAllOutboxRows(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM foundation_outbox_events").Scan(&count))

	return count
}

// readMessages reads up to want messages from the start of a topic, returning
// whatever arrived before the deadline.
func readMessages(t *testing.T, topic string, want int, timeout time.Duration) []kafka.Message {
	t.Helper()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     kafkaBrokers,
		Topic:       topic,
		Partition:   0,
		StartOffset: kafka.FirstOffset,
		MaxWait:     200 * time.Millisecond,
	})
	defer reader.Close() //nolint:errcheck // test helper

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	messages := make([]kafka.Message, 0, want)

	for len(messages) < want {
		message, err := reader.ReadMessage(ctx)
		if err != nil {
			break
		}

		messages = append(messages, message)
	}

	return messages
}

// assertNoMessages fails if anything shows up on the topic within the window.
func assertNoMessages(t *testing.T, topic string, window time.Duration) {
	t.Helper()

	assert.Empty(t, readMessages(t, topic, 1, window),
		"an event with the outbox enabled must not go straight to Kafka")
}
