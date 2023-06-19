package foundation

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (app *Application) connectToPostgreSQL() error {
	app.Logger.Info("Connecting to PostgreSQL...")

	dbURL := os.Getenv("DATABASE_URL")
	connConfig, err := pgx.ParseConfig(dbURL)
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

	db.SetMaxOpenConns(GetEnvOrInt("DATABASE_POOL", 5))
	app.PG = db

	return nil
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
