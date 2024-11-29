package foundation

import (
	"errors"

	fpg "github.com/foundation-go/foundation/postgresql"
	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (s *Service) GetPostgreSQL() *pgxpool.Pool {
	component := s.GetComponent(fpg.ComponentName)
	if component == nil {
		err := errors.New("PostgreSQL component is not registered")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	pg, ok := component.(*fpg.Component)
	if !ok {
		err := errors.New("PostgreSQL component is not of type *foundation_postgresql.Component")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	return pg.Connection
}
