package commands

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	h "github.com/foundation-go/foundation/internal/cli/helpers"
)

var InitOutbox = &cobra.Command{
	Use:   "init-outbox",
	Short: "Initialize outbox pattern for the current service",
	Long: `Initialize outbox pattern by copying outbox migration files from foundation framework
to the current service's db/migrations directory. This creates the foundation_outbox_events
table required for the transactional outbox pattern.`,
	Run: func(cmd *cobra.Command, _ []string) {
		if !h.BuiltOnFoundation() {
			log.Fatal("This command must be run from inside a Foundation service")
		}

		serviceRoot := h.AtServiceRoot()
		migrationsDir := filepath.Join(serviceRoot, MigrationsDirectory)

		// Create migrations directory if it doesn't exist
		if err := os.MkdirAll(migrationsDir, 0755); err != nil {
			log.Fatalf("Failed to create migrations directory: %v", err)
		}

		// Generate timestamp for migration files
		timestamp := time.Now().Format("20060102150405")
		migrationName := "create_foundation_outbox_events"

		// Source files from foundation framework
		// We need to get the foundation module path
		foundationPath, err := getFoundationModulePath()
		if err != nil {
			log.Fatalf("Failed to locate foundation module: %v", err)
		}

		sourceMigrationsPath := filepath.Join(foundationPath, "outboxrepo", "migrations")

		// Check if source migrations exist
		if _, err := os.Stat(sourceMigrationsPath); os.IsNotExist(err) {
			log.Fatalf("Foundation outbox migrations not found at: %s", sourceMigrationsPath)
		}

		// Copy migration files
		upFile := fmt.Sprintf("%s_%s.up.sql", timestamp, migrationName)
		downFile := fmt.Sprintf("%s_%s.down.sql", timestamp, migrationName)

		if err := copyMigrationFile(
			filepath.Join(sourceMigrationsPath, "000001_create_foundation_outbox_events.up.sql"),
			filepath.Join(migrationsDir, upFile),
		); err != nil {
			log.Fatalf("Failed to copy up migration: %v", err)
		}

		if err := copyMigrationFile(
			filepath.Join(sourceMigrationsPath, "000001_create_foundation_outbox_events.down.sql"),
			filepath.Join(migrationsDir, downFile),
		); err != nil {
			log.Fatalf("Failed to copy down migration: %v", err)
		}

		log.Printf("✅ Outbox migrations created successfully:")
		log.Printf("   - %s", upFile)
		log.Printf("   - %s", downFile)
		log.Printf("")
		log.Printf("Next steps:")
		log.Printf("1. Run migrations: foundation db:migrate")
		log.Printf("2. Enable outbox in your service: f.WithOutbox()")
		log.Printf("3. Create outbox courier: foundation new --service --name %s-outbox", getServiceName())
		log.Printf("4. Use WithResponseTransaction in handlers to publish events")
	},
}

func copyMigrationFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func getFoundationModulePath() (string, error) {
	// Try to find foundation module in GOPATH/pkg/mod or go mod cache
	gomodcache := os.Getenv("GOMODCACHE")
	if gomodcache == "" {
		// Default go mod cache location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		gomodcache = filepath.Join(homeDir, "go", "pkg", "mod")
	}

	// Look for foundation module in mod cache
	// Pattern: github.com/foundation-go/foundation@version
	foundationPattern := filepath.Join(gomodcache, "github.com", "foundation-go")

	if _, err := os.Stat(foundationPattern); os.IsNotExist(err) {
		return "", fmt.Errorf("foundation module not found in go mod cache")
	}

	// Find the actual versioned directory
	entries, err := os.ReadDir(foundationPattern)
	if err != nil {
		return "", fmt.Errorf("failed to read foundation module directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "" {
			foundationPath := filepath.Join(foundationPattern, entry.Name())
			// Check if this directory contains outboxrepo/migrations
			outboxMigrationPath := filepath.Join(foundationPath, "outboxrepo", "migrations")
			if _, err := os.Stat(outboxMigrationPath); err == nil {
				return foundationPath, nil
			}
		}
	}

	return "", fmt.Errorf("foundation outbox migrations not found in any version")
}

func getServiceName() string {
	serviceRoot := h.AtServiceRoot()
	return filepath.Base(serviceRoot)
}
