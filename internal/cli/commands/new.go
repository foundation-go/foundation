package commands

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/ri-nat/foundation/internal/cli/helpers"
)

type newInput struct {
	Name string
}

var (
	newAppFiles = []string{
		"README.md",
		".gitignore",
	}
)

var New = &cobra.Command{
	Use:     "new",
	Aliases: []string{"n"},
	Short:   "Create a new Foundation application or service",
	Run: func(cmd *cobra.Command, _ []string) {
		serviceFlag, err := cmd.Flags().GetBool("service")
		if err != nil {
			log.Fatal().Err(err)
		}

		input := &newInput{
			Name: cmd.Flag("name").Value.String(),
		}

		if serviceFlag {
			// TODO: Create a new service
			log.Fatal().Msg("Service creation is not yet supported")
		} else {
			newApplication(input)
		}
	},
}

func newApplication(input *newInput) {
	log.Info().Msg(fmt.Sprintf("Creating a new Foundation application `%s`", input.Name))
	log.Info().Msg("")

	// TODO: check if we're in a git repository (stop if yes)

	if _, err := os.Stat(input.Name); !os.IsNotExist(err) {
		log.Fatal().Msg(fmt.Sprintf("Directory `%s` already exists", input.Name))
	}

	if err := os.Mkdir(input.Name, 0755); err != nil {
		log.Fatal().Err(err)
	}

	log.Info().Msg("Creating files:")
	for _, file := range newAppFiles {
		if err := helpers.CreateFromTemplate(input.Name, "app", file, input); err != nil {
			log.Fatal().Err(err)
		}

		log.Info().Msg(fmt.Sprintf(" - %s", file))
	}

	// Initialize git repository
	if err := helpers.RunCommand(input.Name, "git", "init"); err != nil {
		log.Fatal().Err(err)
	}
}

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	New.Flags().StringP("name", "n", "", "Name of the new application or service")
	New.MarkFlagRequired("name")

	New.Flags().BoolP("app", "a", true, "Create a new application")
	New.Flags().BoolP("service", "s", false, "Create a new service")
	New.MarkFlagsMutuallyExclusive("app", "service")
}
