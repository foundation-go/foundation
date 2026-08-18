package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	ComponentName = "postgresql"

	// DefaultConnectTimeout bounds the connectivity check performed at startup.
	DefaultConnectTimeout = 10 * time.Second

	// DefaultHealthTimeout bounds a health check made without a context.
	DefaultHealthTimeout = 2 * time.Second
)

type Component struct {
	Connection *pgxpool.Pool

	databaseURL    string
	poolSize       int
	logger         *logrus.Entry
	tracingEnabled bool

	// poolStats is registered on Start and unregistered on Stop, so that the
	// gauges belong to the pool that is actually open. Registering once per
	// process instead meant the first component ever started owned them
	// forever: after it was stopped its collector kept reporting, from a closed
	// pool, and any later component's numbers were invisible.
	poolStats prometheus.Collector
}

// ComponentOption is an option to `PostgreSQLComponent`.
type ComponentOption func(*Component)

// WithLogger sets the logger for the PostgreSQL component.
func WithLogger(logger *logrus.Entry) ComponentOption {
	return func(c *Component) {
		c.logger = logger.WithField("component", c.Name())
	}
}

// WithDatabaseURL sets the database URL for the PostgreSQL component.
func WithDatabaseURL(databaseURL string) ComponentOption {
	return func(c *Component) {
		c.databaseURL = databaseURL
	}
}

// WithPoolSize sets the pool size for the PostgreSQL component.
func WithPoolSize(poolSize int) ComponentOption {
	return func(c *Component) {
		c.poolSize = poolSize
	}
}

// WithTracing controls whether queries emit OpenTelemetry spans and duration
// metrics. Enabled by default.
func WithTracing(enabled bool) ComponentOption {
	return func(c *Component) {
		c.tracingEnabled = enabled
	}
}

func NewComponent(opts ...ComponentOption) *Component {
	c := &Component{tracingEnabled: true}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// log returns the component logger, or a usable default when the component was
// built without one.
func (c *Component) log() *logrus.Entry {
	if c.logger == nil {
		return logrus.NewEntry(logrus.StandardLogger()).WithField("component", c.Name())
	}

	return c.logger
}

// Start implements the Component interface.
func (c *Component) Start() error {
	config, err := pgxpool.ParseConfig(c.databaseURL)
	if err != nil {
		return err
	}

	if c.poolSize <= 0 {
		return fmt.Errorf("pool size must be positive, got %d", c.poolSize)
	}

	config.MaxConns = int32(c.poolSize) //nolint:gosec // guarded above and bounded by configuration

	if c.tracingEnabled {
		config.ConnConfig.Tracer = QueryTracer{}
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return err
	}

	// Bound the connectivity check: an unreachable database would otherwise
	// hang startup indefinitely, with no log line explaining why.
	ctx, cancel := context.WithTimeout(context.Background(), DefaultConnectTimeout)
	defer cancel()

	if err = pool.Ping(ctx); err != nil {
		pool.Close()

		return fmt.Errorf("failed to reach the database within %s: %w", DefaultConnectTimeout, err)
	}

	c.Connection = pool

	c.registerPoolStats()

	return nil
}

// registerPoolStats exposes this pool's counters to Prometheus.
func (c *Component) registerPoolStats() {
	if c.poolStats != nil {
		return
	}

	collector := poolStatsCollector{component: c}

	if err := prometheus.Register(collector); err != nil {
		// Another live component already owns the gauges. That is a
		// misconfiguration — a service has one database pool — and the warning
		// is more useful than a second set of numbers would be.
		c.log().Warnf("Failed to register the connection pool collector: %v", err)

		return
	}

	c.poolStats = collector
}

// unregisterPoolStats stops reporting this pool's counters.
func (c *Component) unregisterPoolStats() {
	if c.poolStats == nil {
		return
	}

	prometheus.Unregister(c.poolStats)
	c.poolStats = nil
}

// Stop implements the Component interface.
func (c *Component) Stop() error {
	c.unregisterPoolStats()

	if c.Connection == nil {
		return nil
	}

	c.log().Info("Disconnecting from PostgreSQL...")

	c.Connection.Close()
	c.Connection = nil

	return nil
}

// Health implements the Component interface.
func (c *Component) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHealthTimeout)
	defer cancel()

	return c.HealthContext(ctx)
}

// HealthContext implements the HealthCheckerContext interface.
func (c *Component) HealthContext(ctx context.Context) error {
	if c.Connection == nil {
		return errors.New("connection is not initialized")
	}

	return c.Connection.Ping(ctx)
}

// Name implements the Component interface.
func (c *Component) Name() string {
	return ComponentName
}

func NewNullTimeFromPbTimestamp(timestamp *timestamppb.Timestamp) sql.NullTime {
	result := sql.NullTime{}

	if timestamp != nil && (timestamp.GetSeconds() > 0 || timestamp.GetNanos() > 0) {
		_ = result.Scan(timestamp.AsTime())
	}

	return result
}

func NewNullInt32(num *int32) sql.NullInt32 {
	result := sql.NullInt32{}

	if num != nil {
		_ = result.Scan(*num)
	}

	return result
}

func NewNullInt64(num *int64) sql.NullInt64 {
	result := sql.NullInt64{}

	if num != nil {
		_ = result.Scan(*num)
	}

	return result
}

func NewNullString(str *string) sql.NullString {
	result := sql.NullString{}

	if str != nil {
		_ = result.Scan(*str)
	}

	return result
}

func NewNullUUID(uuidStr *string) uuid.NullUUID {
	result := uuid.NullUUID{}

	if uuidStr != nil {
		_ = result.Scan(*uuidStr)
	}

	return result
}

//
// pgtype helpers
//
// The helpers above return database/sql null types, which is the wrong family
// for this project: sqlc.yaml configures sql_package "pgx/v5", so generated
// code takes pgtype values. Converting between the two at every call site is
// exactly the busywork these helpers exist to remove. The database/sql versions
// are kept for callers that already use them.
//

// NewPgTimestamptzFromPb converts a protobuf timestamp to a pgtype value.
func NewPgTimestamptzFromPb(timestamp *timestamppb.Timestamp) pgtype.Timestamptz {
	if timestamp == nil || !timestamp.IsValid() {
		return pgtype.Timestamptz{}
	}

	return pgtype.Timestamptz{Time: timestamp.AsTime(), Valid: true}
}

// NewPgTimestamptz converts a time to a pgtype value, treating the zero time as
// NULL.
func NewPgTimestamptz(value time.Time) pgtype.Timestamptz {
	if value.IsZero() {
		return pgtype.Timestamptz{}
	}

	return pgtype.Timestamptz{Time: value, Valid: true}
}

// NewPgText converts an optional string to a pgtype value.
func NewPgText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}

	return pgtype.Text{String: *value, Valid: true}
}

// NewPgInt4 converts an optional int32 to a pgtype value.
func NewPgInt4(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}

	return pgtype.Int4{Int32: *value, Valid: true}
}

// NewPgInt8 converts an optional int64 to a pgtype value.
func NewPgInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}

	return pgtype.Int8{Int64: *value, Valid: true}
}

// NewPgBool converts an optional bool to a pgtype value.
func NewPgBool(value *bool) pgtype.Bool {
	if value == nil {
		return pgtype.Bool{}
	}

	return pgtype.Bool{Bool: *value, Valid: true}
}

// NewPgUUID converts an optional UUID string to a pgtype value. An unparsable
// value is treated as NULL.
func NewPgUUID(value *string) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}

	parsed, err := uuid.Parse(*value)
	if err != nil {
		return pgtype.UUID{}
	}

	return pgtype.UUID{Bytes: parsed, Valid: true}
}
