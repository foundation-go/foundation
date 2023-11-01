package main

import (
	"github.com/spf13/cobra"

	c "github.com/ri-nat/foundation/internal/cli/commands"
)

func main() {
	var rootCmd = &cobra.Command{Use: "foundation"}
	rootCmd.AddCommand(
		c.DBMigrate,
		c.DBRollback,
		c.New,
		c.Start,
		c.Test,
	)

	cobra.CheckErr(rootCmd.Execute())
}
