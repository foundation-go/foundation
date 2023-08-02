package postgresql

import (
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	ComponentName = "postgresql"
)

type PostgreSQLComponent struct {
	Connection *sql.DB

	databaseURL string
	poolSize    int
	logger      *logrus.Entry
}

// PostgreSQLComponentOption is an option to `PostgreSQLComponent`.
type PostgreSQLComponentOption func(*PostgreSQLComponent)

// WithLogger sets the logger for the PostgreSQL component.
func WithLogger(logger *logrus.Entry) PostgreSQLComponentOption {
	return func(c *PostgreSQLComponent) {
		c.logger = logger.WithField("component", c.Name())
	}
}

// WithDatabaseURL sets the database URL for the PostgreSQL component.
func WithDatabaseURL(databaseURL string) PostgreSQLComponentOption {
	return func(c *PostgreSQLComponent) {
		c.databaseURL = databaseURL
	}
}

// WithPoolSize sets the pool size for the PostgreSQL component.
func WithPoolSize(poolSize int) PostgreSQLComponentOption {
	return func(c *PostgreSQLComponent) {
		c.poolSize = poolSize
	}
}

func NewPostgreSQLComponent(opts ...PostgreSQLComponentOption) *PostgreSQLComponent {
	c := &PostgreSQLComponent{}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Start implements the Component interface.
func (c *PostgreSQLComponent) Start() error {
	c.logger.Info("Connecting to PostgreSQL...")

	connConfig, err := pgx.ParseConfig(c.databaseURL)
	c.logger.Debugf("PostgreSQL connection config: %+v", connConfig)
	if err != nil {
		return fmt.Errorf("can't parse DATABASE_URL variable: %w", err)
	}

	connStr := stdlib.RegisterConnConfig(connConfig)
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}

	if err = db.Ping(); err != nil {
		return err
	}

	db.SetMaxOpenConns(c.poolSize)
	c.Connection = db

	return nil
}

// Stop implements the Component interface.
func (c *PostgreSQLComponent) Stop() error {
	c.logger.Info("Disconnecting from PostgreSQL...")

	return c.Connection.Close()
}

// Health implements the Component interface.
func (c *PostgreSQLComponent) Health() error {
	if c.Connection == nil {
		return fmt.Errorf("connection is not initialized")
	}

	return c.Connection.Ping()
}

// Name implements the Component interface.
func (c *PostgreSQLComponent) Name() string {
	return ComponentName
}

func NewNullTimeFromPbTimestamp(timestamp *timestamppb.Timestamp) (res sql.NullTime) {
	if timestamp.GetSeconds() > 0 || timestamp.GetNanos() > 0 {
		res.Scan(timestamp.AsTime()) // nolint: errcheck
	}

	return
}

func NewNullInt32(num *int32) (res sql.NullInt32) {
	if num != nil {
		res.Scan(*num) // nolint: errcheck
	}

	return
}

func NewNullInt64(num *int64) (res sql.NullInt64) {
	if num != nil {
		res.Scan(*num) // nolint: errcheck
	}

	return
}

func NewNullString(str *string) (res sql.NullString) {
	if str != nil {
		res.Scan(*str) // nolint: errcheck
	}

	return
}
