package commands

import (
	"log"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"
)

const forceVersionFlag = "version"

// DBForce clears the dirty flag golang-migrate sets when a migration fails
// partway.
//
// Without it there was no way out of that state through the CLI: every
// subsequent db:migrate refuses with "Dirty database version N", and the
// operator had to edit schema_migrations by hand.
var DBForce = &cobra.Command{
	Use:   "db:force",
	Short: "Force the migration version, clearing the dirty flag",
	Long: "Sets the migration version without running anything, which clears the dirty flag " +
		"golang-migrate sets when a migration fails partway. Verify the schema first: this " +
		"tells the migrator what to believe, it does not change any tables.",
	Run: func(cmd *cobra.Command, _ []string) {
		dir, err := migrationsDir(cmd.Flag("dir").Value.String())
		if err != nil {
			log.Fatal(err)
		}

		version, err := cmd.Flags().GetInt(forceVersionFlag)
		if err != nil {
			log.Fatal(err)
		}

		if version < 0 {
			log.Fatalf("`--%s` must not be negative", forceVersionFlag)
		}

		m, err := newMigrator(dir)
		if err != nil {
			log.Fatal(err)
		}
		defer closeMigrator(m)

		if err := m.Force(version); err != nil {
			log.Fatal(err)
		}

		log.Printf("✅ Migration version forced to %d", version)
	},
}

func init() {
	DBForce.Flags().StringP("dir", "d", MigrationsDirectory, "Directory containing migrations (only applicable in production)")
	if err := DBForce.MarkFlagDirname("dir"); err != nil {
		log.Fatal(err)
	}

	DBForce.Flags().IntP(forceVersionFlag, "v", 0, "Migration version to record")

	if err := DBForce.MarkFlagRequired(forceVersionFlag); err != nil {
		log.Fatal(err)
	}
}
