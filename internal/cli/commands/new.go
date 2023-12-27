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
	"github.com/foundation-go/foundation/internal/cli/templates"
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
		".env.example",
		"README.md",
		"cmd/grpc/main.go",
	}
)

var New = &cobra.Command{
	Use:     "new",
	Aliases: []string{"n"},
	Short:   "Create a new Foundation application or service",
	Run: func(cmd *cobra.Command, _ []string) {
		serviceFlag, err := cmd.Flags().GetBool("service")
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get `--service` flag")
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
		log.Fatal().Err(err).Msg("Failed to initialize Git repository")
	}
}

func newService(input *newInput) {
	appRoot := helpers.GetApplicationRoot()

	if err := os.Chdir(appRoot); err != nil {
		log.Fatal().Err(err).Msg("Failed to change directory to application root")
	}

	newEntity(input, "service", newServiceFiles)

	if err := helpers.RunCommand(input.Name, "go", "mod", "init"); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Go module")
	}

	if err := helpers.RunCommand(input.Name, "go", "mod", "tidy"); err != nil {
		log.Fatal().Err(err).Msg("Failed to `go mod tidy`")
	}

	if err := helpers.RunCommand(input.Name, "cp", ".env.example", ".env"); err != nil {
		log.Fatal().Err(err).Msg("Failed to copy `.env.example` to `.env`")
	}
}

func newEntity(input *newInput, entity string, files []string) {
	log.Info().Str("name", input.Name).Msg(fmt.Sprintf("Creating a new Foundation %s", entity))
	log.Info().Msg("")

	if _, err := os.Stat(input.Name); !os.IsNotExist(err) {
		log.Fatal().Str("dirname", input.Name).Msg("Directory already exists")
	}

	if err := os.Mkdir(input.Name, 0755); err != nil {
		log.Fatal().Err(err).Str("dirname", input.Name).Msg("Failed to create directory")
	}

	log.Info().Msg("Creating files:")
	for _, file := range files {
		if strings.Contains(file, "/") {
			dir := filepath.Dir(file)
			if err := os.MkdirAll(filepath.Join(input.Name, dir), 0755); err != nil {
				log.Fatal().Err(err).Str("dirname", dir).Msg("Failed to create directory")
			}
		}

		if err := templates.CreateFromTemplate(input.Name, entity, file, input); err != nil {
			log.Fatal().Err(err).Str("filename", file).Msg("Failed to create file")
		}

		log.Info().Msg(fmt.Sprintf(" - %s", file))
	}
}

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	New.Flags().StringP("name", "n", "", "Name of the new application or service")
	if err := New.MarkFlagRequired("name"); err != nil {
		log.Fatal().Err(err).Msg("Failed to mark `--name` flag as required")
	}

	New.Flags().BoolP("app", "a", false, "Create a new application")
	New.Flags().BoolP("service", "s", false, "Create a new service")
	New.MarkFlagsMutuallyExclusive("app", "service")
}
