package foundation

import (
	"database/sql"
	"fmt"
)

const Version = "0.0.1"

// Application represents a Foundation application.
type Application struct {
	Name string

	// PG is a PostgreSQL connection pool.
	PG *sql.DB
}

// Init initializes the Foundation application.
func Init(name string) *Application {
	initLogging()

	return &Application{
		Name: name,
	}
}

func (app Application) startComponents() error {
	// PostgreSQL
	if GetEnvOrBool("DATABASE_ENABLED", false) {
		if err := app.connectToPostgreSQL(); err != nil {
			return fmt.Errorf("postgresql: %w", err)
		}
	}

	// Metrics
	go StartMetricsServer()

	return nil
}

func (app Application) stopComponents() {
	// PostgreSQL
	if app.PG != nil {
		app.PG.Close()
	}
}
