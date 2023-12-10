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

var DBRollback = &cobra.Command{
	Use:     "db:rollback",
	Aliases: []string{"dbr"},
	Short:   "Rollback database migrations",
	Long:    "Rollback database migrations by a given number of steps, e.g.: `foundation db:rollback --steps 2`",
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

		// Parse `steps` flag
		steps, err := cmd.Flags().GetInt("step")
		if err != nil || steps <= 0 {
			log.Fatal("You should set `--steps` flag to a positive integer")
		}

		// Check if `DATABASE_URL` environment variable is set
		if databaseURL == "" {
			log.Fatal("`DATABASE_URL` environment variable is not set")
		}

		// Initialize migrator
		m, err := migrate.New(fmt.Sprintf("file://%s", dir), databaseURL)
		if err != nil {
			log.Fatal(err)
		}

		// Rollback migrations
		if err = m.Steps(-1 * steps); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	DBRollback.Flags().StringP("dir", "d", MigrationsDirectory, "Directory containing migrations (only applicable in production)")
	if err := DBRollback.MarkFlagDirname("dir"); err != nil {
		log.Fatal(err)
	}

	DBRollback.Flags().Int32P("steps", "s", 1, "Number of migrations to rollback")
}
