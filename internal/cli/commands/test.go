package commands

import (
	"log"
	"os"
	"os/exec"
	"strings"

	f "github.com/ri-nat/foundation"
	h "github.com/ri-nat/foundation/internal/cli/helpers"
	"github.com/spf13/cobra"
)

var Test = &cobra.Command{
	Use:     "test",
	Aliases: []string{"t"},
	Short:   "Run tests",
	Run: func(cmd *cobra.Command, args []string) {
		if !h.BuiltOnFoundation() {
			log.Fatal("This command must be run from inside a Foundation project")
		}

		if f.IsProductionEnv() {
			log.Fatal("You're trying to run tests in production environment")
		}

		env := os.Environ()
		env = append(env, "FOUNDATION_ENV=test")

		opts := []string{
			"test",
		}
		if cmd.Flag("opts") != nil {
			opts = append(opts, strings.Split(cmd.Flag("opts").Value.String(), " ")...)
		}
		opts = append(opts, h.AtProjectRoot("..."))

		test := exec.Command("go", opts...)
		test.Stdout = os.Stdout
		test.Stderr = os.Stderr
		test.Env = env
		if err := test.Run(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	Test.Flags().StringP("opts", "o", "-cover", "Options to pass to go test")
}
