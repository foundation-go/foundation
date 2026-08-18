package commands

import (
	"log"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"
)

var DBMigrate = &cobra.Command{
	Use:     "db:migrate",
	Aliases: []string{"dbm"},
	Short:   "Run database migrations",
	Run: func(cmd *cobra.Command, _ []string) {
		dir, err := migrationsDir(cmd.Flag("dir").Value.String())
		if err != nil {
			log.Fatal(err)
		}

		m, err := newMigrator(dir)
		if err != nil {
			log.Fatal(err)
		}
		defer closeMigrator(m)

		reportMigrationResult(m.Up(), "Nothing to migrate")
	},
}

func init() {
	DBMigrate.Flags().StringP("dir", "d", MigrationsDirectory, "Directory containing migrations (only applicable in production)")
	if err := DBMigrate.MarkFlagDirname("dir"); err != nil {
		log.Fatal(err)
	}
}
