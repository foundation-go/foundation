package commands

import (
	"fmt"
	"log"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"
)

// rollbackStepsFlag is referenced both when the flag is registered and when it
// is read, so the two cannot drift apart again.
const rollbackStepsFlag = "steps"

var DBRollback = &cobra.Command{
	Use:     "db:rollback",
	Aliases: []string{"dbr"},
	Short:   "Rollback database migrations",
	Long:    "Rollback database migrations by a given number of steps, e.g.: `foundation db:rollback --steps 2`",
	Run: func(cmd *cobra.Command, _ []string) {
		dir, err := migrationsDir(cmd.Flag("dir").Value.String())
		if err != nil {
			log.Fatal(err)
		}

		steps, err := rollbackSteps(cmd)
		if err != nil {
			log.Fatal(err)
		}

		m, err := newMigrator(dir)
		if err != nil {
			log.Fatal(err)
		}
		defer closeMigrator(m)

		reportMigrationResult(m.Steps(-1*steps), "Nothing to roll back")
	},
}

// rollbackSteps reads and validates the number of migrations to roll back.
func rollbackSteps(cmd *cobra.Command) (int, error) {
	steps, err := cmd.Flags().GetInt(rollbackStepsFlag)
	if err != nil {
		return 0, fmt.Errorf("failed to read `--%s`: %w", rollbackStepsFlag, err)
	}

	if steps <= 0 {
		return 0, fmt.Errorf("`--%s` must be a positive integer, got %d", rollbackStepsFlag, steps)
	}

	return steps, nil
}

func init() {
	DBRollback.Flags().StringP("dir", "d", MigrationsDirectory, "Directory containing migrations (only applicable in production)")
	if err := DBRollback.MarkFlagDirname("dir"); err != nil {
		log.Fatal(err)
	}

	DBRollback.Flags().IntP(rollbackStepsFlag, "s", 1, "Number of migrations to rollback")
}
