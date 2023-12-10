package commands

import (
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"

	f "github.com/foundation-go/foundation"
	h "github.com/foundation-go/foundation/internal/cli/helpers"
)

const (
	MigrationsDirectory = "db/migrations"
)

var DBMigrate = &cobra.Command{
	Use:     "db:migrate",
	Aliases: []string{"dbm"},
	Short:   "Run database migrations",
	Run: func(cmd *cobra.Command, _ []string) {
		var dir string
		databaseURL := f.GetEnvOrString("DATABASE_URL", "")

		if f.IsProductionEnv() {
			dir = cmd.Flag("dir").Value.String()
			if dir == "" {
				log.Fatal("You should specify the directory containing migrations with the `--dir` flag")
			}
		} else {
			if !h.BuiltOnFoundation() {
				log.Fatal("This command must be run from inside a Foundation project")
			}

			dir = h.AtServiceRoot(MigrationsDirectory)
		}

		// Check if migrations directory exists
		_, err := os.Stat(dir)
		if os.IsNotExist(err) {
			log.Fatalf("Migrations directory `%s` does not exist", dir)
		}

		if databaseURL == "" {
			log.Fatal("`DATABASE_URL` environment variable is not set")
		}

		m, err := migrate.New(fmt.Sprintf("file://%s", dir), databaseURL)
		if err != nil {
			log.Fatal(err)
		}

		if err = m.Up(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	DBMigrate.Flags().StringP("dir", "d", MigrationsDirectory, "Directory containing migrations (only applicable in production)")
	DBMigrate.MarkFlagDirname("dir")
}
