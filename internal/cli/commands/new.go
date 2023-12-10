package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	f "github.com/foundation-go/foundation"
	"github.com/foundation-go/foundation/internal/cli/helpers"
)

type newInput struct {
	FoundationVersion string
	Name              string
}

var (
	newAppFiles = []string{
		"README.md",
		".gitignore",
		"foundation.toml",
	}

	newServiceFiles = []string{
		"README.md",
		"go.mod",
		"cmd/api/main.go",
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
			FoundationVersion: f.Version,
			Name:              cmd.Flag("name").Value.String(),
		}

		if serviceFlag {
			newService(input)
		} else {
			newApplication(input)
		}
	},
}

func newApplication(input *newInput) {
	newEntity(input, "application", newAppFiles)

	if helpers.InGitRepository() {
		log.Info().Msg("Git repository already exists, skipping initialization")
	} else if err := helpers.RunCommand(input.Name, "git", "init"); err != nil {
		log.Fatal().Err(err)
	}
}

func newService(input *newInput) {
	appRoot := helpers.GetApplicationRoot()

	if err := os.Chdir(appRoot); err != nil {
		log.Fatal().Err(err)
	}

	newEntity(input, "service", newServiceFiles)

	if err := helpers.RunCommand(input.Name, "go", "mod", "tidy"); err != nil {
		log.Fatal().Err(err)
	}
}

func newEntity(input *newInput, entity string, files []string) {
	log.Info().Msg(fmt.Sprintf("Creating a new Foundation %s `%s`", entity, input.Name))
	log.Info().Msg("")

	if _, err := os.Stat(input.Name); !os.IsNotExist(err) {
		log.Fatal().Msg(fmt.Sprintf("Directory `%s` already exists", input.Name))
	}

	if err := os.Mkdir(input.Name, 0755); err != nil {
		log.Fatal().Err(err)
	}

	log.Info().Msg("Creating files:")
	for _, file := range files {
		if strings.Contains(file, "/") {
			dir := filepath.Dir(file)
			if err := os.MkdirAll(filepath.Join(input.Name, dir), 0755); err != nil {
				log.Fatal().Err(err)
			}
		}

		if err := helpers.CreateFromTemplate(input.Name, "service", file, input); err != nil {
			log.Fatal().Err(err)
		}

		log.Info().Msg(fmt.Sprintf(" - %s", file))
	}
}

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	New.Flags().StringP("name", "n", "", "Name of the new application or service")
	New.MarkFlagRequired("name")

	New.Flags().BoolP("app", "a", false, "Create a new application")
	New.Flags().BoolP("service", "s", false, "Create a new service")
	New.MarkFlagsMutuallyExclusive("app", "service")
}
