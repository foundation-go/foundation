package commands

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"

	f "github.com/foundation-go/foundation"
	h "github.com/foundation-go/foundation/internal/cli/helpers"
)

// MigrationsDirectory is where a service keeps its migrations.
const MigrationsDirectory = "db/migrations"

// migrationsDir resolves the directory to read migrations from.
//
// In production the caller has to say where they are, since there is no service
// checkout to infer it from; elsewhere it is the service's own directory.
func migrationsDir(flagValue string) (string, error) {
	dir := flagValue

	if !f.IsProductionEnv() {
		if !h.BuiltOnFoundation() {
			return "", errors.New("this command must be run from inside a Foundation service")
		}

		dir = h.AtServiceRoot(MigrationsDirectory)
	}

	if dir == "" {
		return "", errors.New("specify the directory containing migrations with the `--dir` flag")
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("migrations directory `%s` does not exist", dir)
	}

	return dir, nil
}

// newMigrator opens a migrator over the given directory and DATABASE_URL.
func newMigrator(dir string) (*migrate.Migrate, error) {
	databaseURL := f.GetEnvOrString("DATABASE_URL", "")
	if databaseURL == "" {
		return nil, errors.New("`DATABASE_URL` environment variable is not set")
	}

	return migrate.New(fmt.Sprintf("file://%s", dir), databaseURL)
}

// closeMigrator releases the source and database handles.
//
// Neither command used to close them, so every invocation leaked a connection
// and left the advisory lock to the server to clean up.
func closeMigrator(m *migrate.Migrate) {
	sourceErr, databaseErr := m.Close()
	if sourceErr != nil {
		log.Printf("Failed to close the migration source: %v", sourceErr)
	}

	if databaseErr != nil {
		log.Printf("Failed to close the database connection: %v", databaseErr)
	}
}

// reportMigrationResult turns a migrator error into an exit status.
//
// migrate.ErrNoChange means there was nothing to do, which is the normal
// outcome of re-running migrations. Treating it as a failure — as this used to
// — turns every redeploy that adds no migrations into a red pipeline.
func reportMigrationResult(err error, nothingToDo string) {
	switch {
	case err == nil:
		log.Print("✅ Done")
	case errors.Is(err, migrate.ErrNoChange):
		log.Print(nothingToDo)
	default:
		log.Fatal(err)
	}
}
