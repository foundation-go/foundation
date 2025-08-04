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

		rawName := cmd.Flag("name").Value.String()
		input := &newInput{
			FoundationVersion: f.Version,
			Name:              rawName,
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

	shortName := extractShortName(input.Name)
	if helpers.InGitRepository() {
		log.Info().Msg("Git repository already exists, skipping initialization")
	} else if err := helpers.RunCommand(shortName, "git", "init"); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Git repository")
	}
}

func newService(input *newInput) {
	appRoot := helpers.GetApplicationRoot()

	if err := os.Chdir(appRoot); err != nil {
		log.Fatal().Err(err).Msg("Failed to change directory to application root")
	}

	newEntity(input, "service", newServiceFiles)

	shortName := extractShortName(input.Name)
	
	// Construct the proper module name for the service
	moduleName, err := helpers.ConstructServiceModuleName(input.Name)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to construct service module name")
	}

	// Initialize Go module with the constructed module name
	if err := helpers.RunCommand(shortName, "go", "mod", "init", moduleName); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Go module")
	}

	// Ensure go.work exists and add the service to it
	if err := ensureGoWorkspace(appRoot, shortName); err != nil {
		log.Fatal().Err(err).Msg("Failed to setup Go workspace")
	}

	if err := helpers.RunCommand(shortName, "go", "mod", "tidy"); err != nil {
		log.Fatal().Err(err).Msg("Failed to `go mod tidy`")
	}

	if err := helpers.RunCommand(shortName, "cp", ".env.example", ".env"); err != nil {
		log.Fatal().Err(err).Msg("Failed to copy `.env.example` to `.env`")
	}
}

// ensureGoWorkspace ensures that a go.work file exists and includes the service
func ensureGoWorkspace(appRoot, serviceName string) error {
	goWorkPath := filepath.Join(appRoot, "go.work")
	
	// Check if go.work already exists
	if _, err := os.Stat(goWorkPath); os.IsNotExist(err) {
		// Create new go.work file
		if err := helpers.RunCommand(appRoot, "go", "work", "init"); err != nil {
			return fmt.Errorf("failed to initialize go.work: %w", err)
		}
	}
	
	// Add the service to the workspace
	serviceRelPath := "./" + serviceName
	if err := helpers.RunCommand(appRoot, "go", "work", "use", serviceRelPath); err != nil {
		return fmt.Errorf("failed to add service to workspace: %w", err)
	}
	
	return nil
}

// extractShortName extracts the short name from a full module path
// e.g., "github.com/paylitech/backend" -> "backend"
func extractShortName(name string) string {
	if !strings.Contains(name, "/") {
		return name
	}
	
	parts := strings.Split(name, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	
	log.Fatal().Str("name", name).Msg("Failed to extract short name from module path")
	return name
}

func newEntity(input *newInput, entity string, files []string) {
	shortName := extractShortName(input.Name)
	log.Info().Str("name", shortName).Msg(fmt.Sprintf("Creating a new Foundation %s", entity))
	log.Info().Msg("")

	if _, err := os.Stat(shortName); !os.IsNotExist(err) {
		log.Fatal().Str("dirname", shortName).Msg("Directory already exists")
	}

	if err := os.Mkdir(shortName, 0755); err != nil {
		log.Fatal().Err(err).Str("dirname", shortName).Msg("Failed to create directory")
	}

	templateData := map[string]interface{}{
		"FoundationVersion": input.FoundationVersion,
		"Name":              input.Name,
	}

	log.Info().Msg("Creating files:")
	for _, file := range files {
		if strings.Contains(file, "/") {
			dir := filepath.Dir(file)
			if err := os.MkdirAll(filepath.Join(shortName, dir), 0755); err != nil {
				log.Fatal().Err(err).Str("dirname", dir).Msg("Failed to create directory")
			}
		}

		if err := templates.CreateFromTemplate(shortName, entity, file, templateData); err != nil {
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
