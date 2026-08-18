package main

import (
	"github.com/spf13/cobra"

	f "github.com/foundation-go/foundation"
	c "github.com/foundation-go/foundation/internal/cli/commands"
)

func main() {
	// The CLI reads DATABASE_URL and friends without going through
	// foundation.Init, so it loads the service's .env itself.
	f.LoadEnv()

	rootCmd := &cobra.Command{
		Use:   "foundation",
		Short: "Manage Foundation applications and services",
	}
	rootCmd.AddCommand(
		c.DBForce,
		c.DBMigrate,
		c.DBRollback,
		c.InitOutbox,
		c.New,
		c.Start,
		c.Test,
	)

	cobra.CheckErr(rootCmd.Execute())
}
