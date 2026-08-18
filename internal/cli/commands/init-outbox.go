package commands

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/foundation-go/foundation/internal/cli/helpers"
	"github.com/foundation-go/foundation/outboxrepo"
)

// outboxMigrationName is the suffix given to the copied migration files.
const outboxMigrationName = "create_foundation_outbox_events"

var InitOutbox = &cobra.Command{
	Use:   "init-outbox",
	Short: "Initialize outbox pattern for the current service",
	Long: `Initialize outbox pattern by copying outbox migration files from foundation framework
to the current service's db/migrations directory. This creates the foundation_outbox_events
table required for the transactional outbox pattern.`,
	Run: func(_ *cobra.Command, _ []string) {
		if !helpers.BuiltOnFoundation() {
			log.Fatal("This command must be run from inside a Foundation service")
		}

		serviceRoot := helpers.AtServiceRoot()
		migrationsDir := filepath.Join(serviceRoot, MigrationsDirectory)

		if err := os.MkdirAll(migrationsDir, 0o750); err != nil {
			log.Fatalf("Failed to create migrations directory: %v", err)
		}

		// The timestamp orders these among the service's own migrations.
		timestamp := time.Now().Format("20060102150405")

		written, err := copyOutboxMigrations(migrationsDir, timestamp)
		if err != nil {
			log.Fatalf("Failed to copy the outbox migrations: %v", err)
		}

		log.Print("✅ Outbox migrations created successfully:")
		for _, name := range written {
			log.Printf("   - %s", name)
		}

		log.Print("")
		log.Print("Next steps:")
		log.Print("1. Run migrations: foundation db:migrate")
		log.Print("2. Enable outbox in your service: f.WithOutbox()")
		log.Printf("3. Create outbox courier: foundation new --service --name %s-outbox", filepath.Base(serviceRoot))
		log.Print("4. Use WithResponseTransaction in handlers to publish events")
	},
}

// copyOutboxMigrations writes the embedded migrations into dir, renaming them
// with the given timestamp, and returns the names it wrote.
func copyOutboxMigrations(dir, timestamp string) ([]string, error) {
	entries, err := fs.ReadDir(outboxrepo.Migrations, outboxrepo.MigrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read the embedded migrations: %w", err)
	}

	var written []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// path.Join, not filepath.Join: an embed.FS is always addressed with
		// forward slashes, so building the path with the OS separator made
		// every read fail on Windows.
		content, err := fs.ReadFile(outboxrepo.Migrations, path.Join(outboxrepo.MigrationsDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", entry.Name(), err)
		}

		name := fmt.Sprintf("%s_%s%s", timestamp, outboxMigrationName, migrationSuffix(entry.Name()))

		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", name, err)
		}

		written = append(written, name)
	}

	if len(written) == 0 {
		return nil, fmt.Errorf("no migrations are embedded in the foundation module")
	}

	return written, nil
}

// migrationSuffix returns the ".up.sql" / ".down.sql" tail of a migration file.
func migrationSuffix(name string) string {
	if index := strings.Index(name, "."); index != -1 {
		return name[index:]
	}

	return ".sql"
}
