package commands

import (
	"log"
	"os"
	"os/exec"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	h "github.com/foundation-go/foundation/internal/cli/helpers"
)

var Start = &cobra.Command{
	Use:     "start",
	Aliases: []string{"s"},
	Short:   "Start a service",
	Run: func(_ *cobra.Command, _ []string) {
		if !h.BuiltOnFoundation() {
			log.Fatal("This command must be run from inside a Foundation project")
		}

		// Read all subdirectories under the cmd directory
		files, err := os.ReadDir(h.AtServiceRoot("cmd"))
		if err != nil {
			log.Fatal(err)
		}

		// Collect all directory names
		var binaries []string
		for _, f := range files {
			if f.IsDir() {
				binaries = append(binaries, f.Name())
			}
		}

		// Prompt the user to select a service
		prompt := &survey.Select{
			Message: "Choose a service to start:",
			Options: binaries,
		}
		var binaryName string
		if err = survey.AskOne(prompt, &binaryName); err != nil {
			log.Fatal(err)
		}

		// Run the service
		svc := exec.Command("go", "run", h.AtServiceRoot("cmd", binaryName))
		svc.Stdout = os.Stdout
		svc.Stderr = os.Stderr
		if err = svc.Run(); err != nil {
			log.Fatal(err)
		}
	},
}
