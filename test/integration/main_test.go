//go:build integration

// Package integration exercises Foundation against the real servers it talks
// to, rather than against fakes.
//
// It exists because a whole class of the framework's behaviour is only true if
// a real Postgres, Redis or Kafka behaves the way the code assumes: that
// `FOR UPDATE SKIP LOCKED` really does let two outbox couriers run without
// publishing the same event twice, that a consumer group's committed offset
// really does advance past messages the worker has no handler for, that a
// Redis URL's database number is really honoured. None of that can be settled
// with a stub.
//
// The tests are behind the `integration` build tag and run against containers:
//
//	go test -tags=integration ./test/integration/...
package integration

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	tclog "github.com/testcontainers/testcontainers-go/log"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foundation-go/foundation/outboxrepo"
)

// Images are pinned so a test failure means the code changed, not the world.
const (
	postgresImage = "postgres:16-alpine"
	redisImage    = "redis:7-alpine"
	kafkaImage    = "confluentinc/confluent-local:7.6.1"
)

// startupTimeout bounds pulling and starting a container.
const startupTimeout = 3 * time.Minute

// Containers are started once for the whole package: Kafka alone takes long
// enough that starting one per test would dominate the run.
var (
	postgresURL  string
	redisURL     string
	kafkaBrokers []string
)

// uniqueSuffix keeps topics, consumer groups and Redis keys from colliding
// between tests that share a container.
var uniqueCounter atomic.Int64

func unique(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), uniqueCounter.Add(1))
}

func TestMain(m *testing.M) {
	// testcontainers is chatty on stdout and would drown the test output.
	tclog.SetDefault(discardLogger{})
	logrus.SetOutput(io.Discard)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	teardown, err := startContainers(ctx)
	if err != nil {
		// Failing is the default on purpose.
		//
		// Exiting 0 here would make a CI runner without a Docker daemon report
		// a green integration job that tested nothing — the same shape of
		// problem as a pipeline that passes because it never looks. Set
		// FOUNDATION_INTEGRATION_SKIP_WITHOUT_DOCKER to opt into the soft skip
		// on a laptop.
		fmt.Fprintf(os.Stderr, "\nintegration tests need a working Docker daemon: %v\n", err)

		if os.Getenv("FOUNDATION_INTEGRATION_SKIP_WITHOUT_DOCKER") != "" {
			fmt.Fprintln(os.Stderr, "skipping, because FOUNDATION_INTEGRATION_SKIP_WITHOUT_DOCKER is set")
			os.Exit(0)
		}

		os.Exit(1)
	}

	code := m.Run()

	teardown()
	os.Exit(code)
}

// discardLogger silences testcontainers.
type discardLogger struct{}

func (discardLogger) Printf(string, ...interface{}) {}

// startContainers brings up Postgres, Redis and Kafka and returns a teardown.
func startContainers(ctx context.Context) (func(), error) {
	var terminators []func()

	teardown := func() {
		for i := len(terminators) - 1; i >= 0; i-- {
			terminators[i]()
		}
	}

	pg, err := tcpostgres.Run(ctx, postgresImage,
		tcpostgres.WithDatabase("foundation"),
		tcpostgres.WithUsername("foundation"),
		tcpostgres.WithPassword("foundation"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(startupTimeout),
		),
	)
	if err != nil {
		teardown()

		return nil, fmt.Errorf("postgres: %w", err)
	}

	terminators = append(terminators, func() { _ = pg.Terminate(context.Background()) })

	postgresURL, err = pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		teardown()

		return nil, fmt.Errorf("postgres connection string: %w", err)
	}

	rd, err := tcredis.Run(ctx, redisImage)
	if err != nil {
		teardown()

		return nil, fmt.Errorf("redis: %w", err)
	}

	terminators = append(terminators, func() { _ = rd.Terminate(context.Background()) })

	redisURL, err = rd.ConnectionString(ctx)
	if err != nil {
		teardown()

		return nil, fmt.Errorf("redis connection string: %w", err)
	}

	kf, err := tckafka.Run(ctx, kafkaImage, tckafka.WithClusterID("foundation-test"))
	if err != nil {
		teardown()

		return nil, fmt.Errorf("kafka: %w", err)
	}

	terminators = append(terminators, func() { _ = kf.Terminate(context.Background()) })

	kafkaBrokers, err = kf.Brokers(ctx)
	if err != nil {
		teardown()

		return nil, fmt.Errorf("kafka brokers: %w", err)
	}

	if err := createOutboxTable(ctx); err != nil {
		teardown()

		return nil, fmt.Errorf("outbox schema: %w", err)
	}

	return teardown, nil
}

// createOutboxTable applies the framework's own embedded migration, so the
// tests run against the schema Foundation actually ships.
func createOutboxTable(ctx context.Context) error {
	statement, err := outboxrepo.Migrations.ReadFile(
		path.Join(outboxrepo.MigrationsDir, "000001_create_foundation_outbox_events.up.sql"),
	)
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, postgresURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, string(statement))

	return err
}

//
// Per-test helpers
//

// newPool opens a connection pool for a test and closes it afterwards.
func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), postgresURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

// truncateOutbox empties the outbox table, since every test shares one database.
func truncateOutbox(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(context.Background(), "TRUNCATE foundation_outbox_events"); err != nil {
		t.Fatalf("failed to truncate the outbox: %v", err)
	}
}

// testLogger returns a logger that keeps quiet.
func testLogger() *logrus.Entry {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	return logrus.NewEntry(logger)
}

// kafkaClient returns a client for inspecting the broker from a test.
func kafkaClient() *kafka.Client {
	return &kafka.Client{
		Addr:    kafka.TCP(kafkaBrokers...),
		Timeout: 10 * time.Second,
	}
}

// createTopic creates a topic and removes it when the test ends.
//
// Explicit creation rather than auto-creation: a topic that springs into
// existence on first write has one partition and races with the metadata
// refresh, which makes consumer-group tests flaky for reasons that have nothing
// to do with the code under test.
func createTopic(t *testing.T, topic string, partitions int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := kafkaClient()

	resp, err := client.CreateTopics(ctx, &kafka.CreateTopicsRequest{
		Topics: []kafka.TopicConfig{{
			Topic:             topic,
			NumPartitions:     partitions,
			ReplicationFactor: 1,
		}},
	})
	if err != nil {
		t.Fatalf("failed to create topic %s: %v", topic, err)
	}

	for name, topicErr := range resp.Errors {
		if topicErr != nil {
			t.Fatalf("failed to create topic %s: %v", name, topicErr)
		}
	}

	t.Cleanup(func() {
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer deleteCancel()

		_, _ = client.DeleteTopics(deleteCtx, &kafka.DeleteTopicsRequest{Topics: []string{topic}})
	})

	waitForTopic(t, topic)
}

// waitForTopic blocks until the broker reports the topic, so that a producer
// does not race the metadata propagation.
func waitForTopic(t *testing.T, topic string) {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := kafkaClient().Metadata(ctx, &kafka.MetadataRequest{Topics: []string{topic}})
		cancel()

		if err == nil {
			for _, tp := range resp.Topics {
				if tp.Name == topic && tp.Error == nil && len(tp.Partitions) > 0 {
					return
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("topic %s did not become available", topic)
}

// committedOffset returns the offset a consumer group has committed for a
// partition, or -1 when it has committed nothing.
func committedOffset(t *testing.T, groupID, topic string, partition int) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := kafkaClient().OffsetFetch(ctx, &kafka.OffsetFetchRequest{
		GroupID: groupID,
		Topics:  map[string][]int{topic: {partition}},
	})
	if err != nil {
		t.Fatalf("failed to fetch the committed offset: %v", err)
	}

	for _, partitions := range resp.Topics {
		for _, p := range partitions {
			if p.Partition == partition {
				return p.CommittedOffset
			}
		}
	}

	return -1
}

// eventually polls until condition holds or the deadline passes.
func eventually(t *testing.T, timeout time.Duration, description string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("timed out after %s waiting for: %s", timeout, description)
}
