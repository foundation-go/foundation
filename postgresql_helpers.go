package foundation

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	fpg "github.com/foundation-go/foundation/postgresql"
)

// GetPostgreSQL returns the PostgreSQL connection pool.
//
// It panics when the pool is unavailable. That is a wiring mistake — the
// component was never enabled, or the caller ran before Start — not a runtime
// condition, and the recovery middleware and interceptors turn the panic into a
// reported 500 rather than a dead process. Use TryGetPostgreSQL where the
// absence has to be handled.
func (s *Service) GetPostgreSQL() *pgxpool.Pool {
	pool, err := s.TryGetPostgreSQL()
	if err != nil {
		panic(err)
	}

	return pool
}

// TryGetPostgreSQL returns the PostgreSQL connection pool, or an error
// explaining why it is not available.
func (s *Service) TryGetPostgreSQL() (*pgxpool.Pool, error) {
	component := s.GetComponent(fpg.ComponentName)
	if component == nil {
		return nil, errors.New("no PostgreSQL component is registered: set DATABASE_URL to enable it")
	}

	pg, ok := component.(*fpg.Component)
	if !ok {
		return nil, fmt.Errorf("component `%s` is a %T, not a *postgresql.Component", fpg.ComponentName, component)
	}

	if pg.Connection == nil {
		return nil, errors.New("the PostgreSQL component has not been started yet")
	}

	return pg.Connection, nil
}
