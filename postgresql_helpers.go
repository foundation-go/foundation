package foundation

import (
	"database/sql"

	fpg "github.com/ri-nat/foundation/postgresql"
)

func (app *Application) GetPostgreSQL() *sql.DB {
	component := app.GetComponent(fpg.ComponentName)
	if component == nil {
		app.Logger.Fatal("PostgreSQL component is not registered")
	}

	pg, ok := component.(*fpg.PostgreSQLComponent)
	if !ok {
		app.Logger.Fatal("PostgreSQL component is not of type *fpg.PostgreSQLComponent")
	}

	return pg.Connection
}
