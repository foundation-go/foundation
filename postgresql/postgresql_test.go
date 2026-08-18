package postgresql

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Stop dereferenced a Connection that only exists after a successful Start, and
// logged through a logger that is nil unless WithLogger was passed.
func TestComponentIsSafeBeforeStart(t *testing.T) {
	c := NewComponent()

	require.NotPanics(t, func() { assert.NoError(t, c.Stop()) })
	require.NotPanics(t, func() { assert.ErrorContains(t, c.Health(), "not initialized") })
	require.NotPanics(t, func() {
		assert.ErrorContains(t, c.HealthContext(context.Background()), "not initialized")
	})
}

func TestStartRejectsANonPositivePoolSize(t *testing.T) {
	c := NewComponent(WithDatabaseURL("postgres://localhost:5432/test"), WithPoolSize(0))

	err := c.Start()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pool size")
}

func TestStartRejectsAMalformedURL(t *testing.T) {
	c := NewComponent(WithDatabaseURL("://nonsense"), WithPoolSize(1))

	assert.Error(t, c.Start())
}

func TestName(t *testing.T) {
	assert.Equal(t, ComponentName, NewComponent().Name())
}

func TestNewNullTimeFromPbTimestamp(t *testing.T) {
	assert.False(t, NewNullTimeFromPbTimestamp(nil).Valid)

	ts := timestamppb.New(timestamppb.Now().AsTime())
	assert.True(t, NewNullTimeFromPbTimestamp(ts).Valid)
}

func TestNullHelpers(t *testing.T) {
	assert.False(t, NewNullInt32(nil).Valid)
	assert.False(t, NewNullInt64(nil).Valid)
	assert.False(t, NewNullString(nil).Valid)
	assert.False(t, NewNullUUID(nil).Valid)

	i32 := int32(1)
	i64 := int64(2)
	str := "three"
	id := "0f8fad5b-d9cb-469f-a165-70867728950e"

	assert.True(t, NewNullInt32(&i32).Valid)
	assert.True(t, NewNullInt64(&i64).Valid)
	assert.True(t, NewNullString(&str).Valid)
	assert.True(t, NewNullUUID(&id).Valid)
}

// A metric labelled by SQL statement would grow a time series per distinct
// query text; the leading verb keeps the cardinality bounded.
func TestQueryOperation(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"select", "SELECT * FROM chats", "SELECT"},
		{"lowercase", "insert into chats (id) values ($1)", "INSERT"},
		{"leading whitespace", "\n\t  UPDATE chats SET name = $1", "UPDATE"},
		{"cte", "WITH recent AS (SELECT 1) SELECT * FROM recent", "WITH"},
		{"transaction control", "begin", "BEGIN"},
		// sqlc prefixes its generated statements with a name comment.
		{"sqlc comment", "-- name: ListOutboxEvents :many\nSELECT id FROM t", "SELECT"},
		{"several comments", "-- one\n-- two\nDELETE FROM t", "DELETE"},
		{"only a comment", "-- name: X :exec", "unknown"},
		{"empty", "   ", "unknown"},
		// An unrecognised verb collapses into one series rather than being
		// echoed back as a label value.
		{"unknown verb", "VACUUM ANALYZE", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, queryOperation(tt.sql))
		})
	}
}

func TestPoolStatsCollectorIsSafeBeforeStart(t *testing.T) {
	collector := poolStatsCollector{component: NewComponent()}

	descs := make(chan *prometheus.Desc, 8)
	collector.Describe(descs)
	close(descs)
	assert.Len(t, descs, 4)

	metrics := make(chan prometheus.Metric, 8)
	require.NotPanics(t, func() { collector.Collect(metrics) })
	close(metrics)
	assert.Empty(t, metrics, "an unstarted pool has nothing to report")
}

func TestTracingIsEnabledByDefault(t *testing.T) {
	assert.True(t, NewComponent().tracingEnabled)
	assert.False(t, NewComponent(WithTracing(false)).tracingEnabled)
}

// sqlc is configured with sql_package "pgx/v5", so generated code takes pgtype
// values; the database/sql helpers were the wrong family for this project and
// left callers converting by hand.
func TestPgTypeHelpers(t *testing.T) {
	assert.False(t, NewPgTimestamptzFromPb(nil).Valid)
	assert.True(t, NewPgTimestamptzFromPb(timestamppb.New(time.Unix(1, 0))).Valid)

	assert.False(t, NewPgTimestamptz(time.Time{}).Valid)
	assert.True(t, NewPgTimestamptz(time.Unix(1, 0)).Valid)

	assert.False(t, NewPgText(nil).Valid)
	assert.False(t, NewPgInt4(nil).Valid)
	assert.False(t, NewPgInt8(nil).Valid)
	assert.False(t, NewPgBool(nil).Valid)
	assert.False(t, NewPgUUID(nil).Valid)

	str := "value"
	i32 := int32(1)
	i64 := int64(2)
	flag := false
	id := "0f8fad5b-d9cb-469f-a165-70867728950e"
	bad := "not-a-uuid"

	assert.Equal(t, "value", NewPgText(&str).String)
	assert.True(t, NewPgText(&str).Valid)
	assert.Equal(t, int32(1), NewPgInt4(&i32).Int32)
	assert.Equal(t, int64(2), NewPgInt8(&i64).Int64)

	// A pointer to false is a value, not an absence.
	pgBool := NewPgBool(&flag)
	assert.True(t, pgBool.Valid)
	assert.False(t, pgBool.Bool)

	assert.True(t, NewPgUUID(&id).Valid)
	assert.False(t, NewPgUUID(&bad).Valid, "an unparsable UUID is NULL, not a corrupt value")
}
