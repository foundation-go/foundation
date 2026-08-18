package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
			log.Fatalf("Failed to get `--service` flag: %v", err)
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
		log.Print("Git repository already exists, skipping initialization")
	} else if err := helpers.RunCommand(shortName, "git", "init"); err != nil {
		log.Fatalf("Failed to initialize Git repository: %v", err)
	}
}

func newService(input *newInput) {
	appRoot := helpers.GetApplicationRoot()

	if err := os.Chdir(appRoot); err != nil {
		log.Fatalf("Failed to change directory to application root: %v", err)
	}

	newEntity(input, "service", newServiceFiles)

	shortName := extractShortName(input.Name)

	// Construct the proper module name for the service
	moduleName, err := helpers.ConstructServiceModuleName(input.Name)
	if err != nil {
		log.Fatalf("Failed to construct service module name: %v", err)
	}

	// Initialize Go module with the constructed module name
	if err := helpers.RunCommand(shortName, "go", "mod", "init", moduleName); err != nil {
		log.Fatalf("Failed to initialize Go module: %v", err)
	}

	// Ensure go.work exists and add the service to it
	if err := ensureGoWorkspace(appRoot, shortName); err != nil {
		log.Fatalf("Failed to setup Go workspace: %v", err)
	}

	if err := helpers.RunCommand(shortName, "go", "mod", "tidy"); err != nil {
		log.Fatalf("Failed to `go mod tidy`: %v", err)
	}

	// Copying with Go rather than shelling out to `cp`, which does not exist on
	// Windows.
	if err := helpers.CopyFile(
		filepath.Join(shortName, ".env.example"),
		filepath.Join(shortName, ".env"),
	); err != nil {
		log.Fatalf("Failed to copy `.env.example` to `.env`: %v", err)
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
	if index := strings.LastIndex(name, "/"); index != -1 {
		return name[index+1:]
	}

	return name
}

func newEntity(input *newInput, entity string, files []string) {
	shortName := extractShortName(input.Name)
	log.Printf("Creating a new Foundation %s: %s", entity, shortName)

	if _, err := os.Stat(shortName); !os.IsNotExist(err) {
		log.Fatalf("Directory already exists: %s", shortName)
	}

	if err := os.Mkdir(shortName, 0755); err != nil {
		log.Fatalf("Failed to create directory (%s): %v", shortName, err)
	}

	templateData := map[string]interface{}{
		"FoundationVersion": input.FoundationVersion,
		// Name is what the user typed, which may be a full module path;
		// ShortName is the directory and the identifier used in code. Templates
		// that used Name for both produced a database called
		// "github.com/org/service".
		"Name":      input.Name,
		"ShortName": shortName,
	}

	log.Print("Creating files:")
	for _, file := range files {
		if strings.Contains(file, "/") {
			dir := filepath.Dir(file)
			if err := os.MkdirAll(filepath.Join(shortName, dir), 0755); err != nil {
				log.Fatalf("Failed to create directory (%s): %v", dir, err)
			}
		}

		if err := templates.CreateFromTemplate(shortName, entity, file, templateData); err != nil {
			log.Fatalf("Failed to create file (%s): %v", file, err)
		}

		log.Printf(" - %s", file)
	}
}

func init() {
	New.Flags().StringP("name", "n", "", "Name of the new application or service")
	if err := New.MarkFlagRequired("name"); err != nil {
		log.Fatalf("Failed to mark `--name` flag as required: %v", err)
	}

	New.Flags().BoolP("app", "a", false, "Create a new application")
	New.Flags().BoolP("service", "s", false, "Create a new service")
	New.MarkFlagsMutuallyExclusive("app", "service")
}
