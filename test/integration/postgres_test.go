//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/protobuf/proto"

	f "github.com/foundation-go/foundation"
	ferr "github.com/foundation-go/foundation/errors"
	ferrpb "github.com/foundation-go/foundation/errors/proto"
	fmetrics "github.com/foundation-go/foundation/metrics"
	fpg "github.com/foundation-go/foundation/postgresql"
)

// newDatabaseService builds a service with a live PostgreSQL component, through
// the same StartComponents path a real service uses.
func newDatabaseService(t *testing.T) *f.Service {
	t.Helper()

	t.Setenv("DATABASE_URL", postgresURL)
	t.Setenv("METRICS_ENABLED", "false")

	svc := &f.Service{Name: "test", Config: f.NewConfig(), Logger: testLogger()}

	require.NoError(t, svc.StartComponents())
	t.Cleanup(svc.StopComponents)

	return svc
}

func TestPostgresComponentLifecycle(t *testing.T) {
	component := fpg.NewComponent(
		fpg.WithDatabaseURL(postgresURL),
		fpg.WithPoolSize(3),
		fpg.WithLogger(testLogger()),
	)

	require.NoError(t, component.Start())
	require.NotNil(t, component.Connection)

	assert.NoError(t, component.Health())
	assert.NoError(t, component.HealthContext(context.Background()))

	require.NoError(t, component.Stop())

	// After Stop the pool is closed, so the health check has to notice. The
	// component reporting itself healthy here would mean the readiness probe
	// keeps a torn-down service in the load balancer.
	assert.Error(t, component.Health())
}

// The startup ping is bounded; an unreachable database must fail rather than
// hang the process with nothing in the log.
func TestPostgresComponentFailsFastOnAnUnreachableDatabase(t *testing.T) {
	component := fpg.NewComponent(
		// Reserved as invalid by RFC 6890, so the dial can never succeed.
		fpg.WithDatabaseURL("postgres://foundation:foundation@192.0.2.1:5432/foundation?sslmode=disable&connect_timeout=2"),
		fpg.WithPoolSize(1),
		fpg.WithLogger(testLogger()),
	)

	started := time.Now()
	err := component.Start()

	require.Error(t, err)
	assert.Less(t, time.Since(started), fpg.DefaultConnectTimeout+5*time.Second)
}

// Pool exhaustion is a confusing production failure: the service looks idle and
// every request is slow. The collector has to report the real numbers.
func TestPostgresPoolStatsAreExposed(t *testing.T) {
	component := fpg.NewComponent(
		fpg.WithDatabaseURL(postgresURL),
		fpg.WithPoolSize(2),
		fpg.WithLogger(testLogger()),
	)

	require.NoError(t, component.Start())
	t.Cleanup(func() { _ = component.Stop() })

	// Hold both connections.
	first, err := component.Connection.Acquire(context.Background())
	require.NoError(t, err)
	defer first.Release()

	second, err := component.Connection.Acquire(context.Background())
	require.NoError(t, err)
	defer second.Release()

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	values := map[string]float64{}
	for _, family := range families {
		if strings.HasPrefix(family.GetName(), "foundation_database_pool_") && len(family.GetMetric()) > 0 {
			values[family.GetName()] = family.GetMetric()[0].GetGauge().GetValue()
		}
	}

	require.Contains(t, values, "foundation_database_pool_acquired_connections")
	assert.Equal(t, 2.0, values["foundation_database_pool_acquired_connections"])
	assert.Equal(t, 2.0, values["foundation_database_pool_max_connections"])

	// With both connections held, a third acquire has nowhere to go.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err = component.Connection.Acquire(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "an exhausted pool should block, not hand out a third connection")
}

// The database used to be a blind spot: a request trace jumped from handler to
// response with no sign of how much of the time went on SQL.
func TestPostgresQueriesAreTracedAndMeasured(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(previous) })

	svc := newDatabaseService(t)
	pool := svc.GetPostgreSQL()

	before := testutil.ToFloat64(fmetrics.DatabaseQueries.WithLabelValues("SELECT", fmetrics.ResultSuccess))

	var answer int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT 42").Scan(&answer))
	assert.Equal(t, 42, answer)

	after := testutil.ToFloat64(fmetrics.DatabaseQueries.WithLabelValues("SELECT", fmetrics.ResultSuccess))
	assert.Equal(t, 1.0, after-before, "the query should have been counted")

	spans := exporter.GetSpans()
	require.NotEmpty(t, spans, "the query should have produced a span")

	var found bool

	for _, span := range spans {
		if span.Name != "postgresql SELECT" {
			continue
		}

		found = true

		attributes := map[string]string{}
		for _, attr := range span.Attributes {
			attributes[string(attr.Key)] = attr.Value.String()
		}

		assert.Equal(t, "postgresql", attributes["db.system"])
		assert.Equal(t, "SELECT", attributes["db.operation"])
		assert.Equal(t, "SELECT 42", attributes["db.statement"])
	}

	assert.True(t, found, "expected a `postgresql SELECT` span, got %v", spanNames(spans))
}

func TestPostgresFailedQueriesAreCountedAsErrors(t *testing.T) {
	svc := newDatabaseService(t)
	pool := svc.GetPostgreSQL()

	before := testutil.ToFloat64(fmetrics.DatabaseQueries.WithLabelValues("SELECT", fmetrics.ResultError))

	_, err := pool.Exec(context.Background(), "SELECT from_a_table_that_does_not_exist()")
	require.Error(t, err)

	after := testutil.ToFloat64(fmetrics.DatabaseQueries.WithLabelValues("SELECT", fmetrics.ResultError))
	assert.Equal(t, 1.0, after-before)
}

// WithTransaction has to commit on success and roll back on failure, against a
// real transactional database rather than a stub that always agrees.
func TestWithTransactionCommitsAndRollsBack(t *testing.T) {
	svc := newDatabaseService(t)
	pool := svc.GetPostgreSQL()
	truncateOutbox(t, pool)

	t.Run("commits on success", func(t *testing.T) {
		err := svc.WithTransaction(context.Background(), func(tx pgx.Tx) ([]*f.Event, ferr.FoundationError) {
			_, execErr := tx.Exec(context.Background(),
				"INSERT INTO foundation_outbox_events (topic, key, payload, headers, created_at) VALUES ($1, $2, $3, $4, NOW())",
				"chats", "committed", []byte("payload"), []byte("{}"),
			)
			require.NoError(t, execErr)

			return nil, nil
		})
		require.Nil(t, err)

		assert.Equal(t, 1, countOutboxRows(t, pool, "committed"))
	})

	t.Run("rolls back on failure", func(t *testing.T) {
		err := svc.WithTransaction(context.Background(), func(tx pgx.Tx) ([]*f.Event, ferr.FoundationError) {
			_, execErr := tx.Exec(context.Background(),
				"INSERT INTO foundation_outbox_events (topic, key, payload, headers, created_at) VALUES ($1, $2, $3, $4, NOW())",
				"chats", "rolled-back", []byte("payload"), []byte("{}"),
			)
			require.NoError(t, execErr)

			return nil, ferr.NewInternalError(errors.New("nope"), "handler failed")
		})
		require.NotNil(t, err)

		assert.Zero(t, countOutboxRows(t, pool, "rolled-back"),
			"a failed handler must leave nothing behind")
	})
}

func TestWithResponseTransactionReturnsTheResponse(t *testing.T) {
	svc := newDatabaseService(t)
	truncateOutbox(t, svc.GetPostgreSQL())

	response, err := svc.WithResponseTransaction(context.Background(),
		func(tx pgx.Tx) (proto.Message, []*f.Event, ferr.FoundationError) {
			var one int
			require.NoError(t, tx.QueryRow(context.Background(), "SELECT 1").Scan(&one))

			return &ferrpb.NotFoundError{Kind: "chat", Id: "1"}, nil, nil
		})

	require.Nil(t, err)
	require.NotNil(t, response)

	notFound, ok := response.(*ferrpb.NotFoundError)
	require.True(t, ok)
	assert.Equal(t, "chat", notFound.Kind)
}

// countOutboxRows counts the outbox rows with the given key.
func countOutboxRows(t *testing.T, pool *pgxpool.Pool, key string) int {
	t.Helper()

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM foundation_outbox_events WHERE key = $1", key).Scan(&count))

	return count
}

func spanNames(spans tracetest.SpanStubs) []string {
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name)
	}

	return names
}
