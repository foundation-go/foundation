package helpers

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

func AtServiceRoot(paths ...string) string {
	return filepath.Join(getServiceRoot(), filepath.Join(paths...))
}

// getServiceRoot returns the absolute path to the project root directory or panics if it can't be found.
func getServiceRoot() string {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	root, err := findServiceRoot(path)
	if err != nil {
		panic(err)
	}

	return root
}

func findServiceRoot(path string) (string, error) {
	for {
		gomod := filepath.Join(path, "go.mod")
		if _, err := os.Stat(gomod); err == nil {
			return path, nil
		}

		parent := filepath.Dir(path)
		if parent == path {
			break // we've reached the root directory
		}

		path = parent
	}

	return "", errors.New("can't find service root")
}

// findGoMod finds the go.mod file by traversing up the directory tree.
// It returns the absolute path to the go.mod file, or an error if it can't be found.
func findGoMod(path string) (string, error) {
	serviceRoot, err := findServiceRoot(path)
	if err != nil {
		return "", err
	}

	gomod := filepath.Join(serviceRoot, "go.mod")
	if _, err := os.Stat(gomod); err == nil {
		return gomod, nil
	}

	return "", errors.New("no go.mod file found")
}

func GetApplicationRoot() string {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	root, err := findApplicationRoot(path)
	if err != nil {
		panic(err)
	}

	return root
}

func findApplicationRoot(path string) (string, error) {
	foundationToml, err := FindFoundationToml(path)
	if err != nil {
		return "", fmt.Errorf("failed to find foundation.toml: %w", err)
	}

	return filepath.Dir(foundationToml), nil
}

// FindFoundationToml finds the closest foundation.toml file by traversing up the directory tree.
// It returns the absolute path to the foundation.toml file, or an error if it can't be found.
func FindFoundationToml(path string) (string, error) {
	for {
		foundationToml := filepath.Join(path, "foundation.toml")
		if _, err := os.Stat(foundationToml); err == nil {
			return foundationToml, nil
		}

		parent := filepath.Dir(path)
		if parent == path {
			break // we've reached the root directory
		}

		path = parent
	}

	return "", errors.New("no `foundation.toml` file present in the current directory or any of its parents")
}

// checkFoundationDep checks whether the given go.mod file lists Foundation as a dependency.
func checkFoundationDep(gomodPath string) (bool, error) {
	file, err := os.Open(gomodPath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "github.com/foundation-go/foundation") {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, nil
}

func BuiltOnFoundation() bool {
	path, err := os.Getwd()
	if err != nil {
		return false
	}

	gomodPath, err := findGoMod(path)
	if err != nil {
		return false
	}

	foundationDepPresent, err := checkFoundationDep(gomodPath)
	if err != nil || !foundationDepPresent {
		return false
	}

	return true
}

// CopyFile copies src to dst, preserving nothing but the contents.
//
// Used instead of shelling out to `cp`, which does not exist on Windows.
func CopyFile(src, dst string) error {
	content, err := os.ReadFile(src) // #nosec G304 -- paths come from the CLI's own scaffolding
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}

	if err := os.WriteFile(dst, content, 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", dst, err)
	}

	return nil
}

func RunCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func InGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Trimming rather than comparing to "true\n": the line ending is "\r\n" on
	// Windows, where this always reported false.
	return strings.TrimSpace(string(output)) == "true"
}

// FoundationConfig represents the structure of foundation.toml
type FoundationConfig struct {
	Foundation struct {
		Version string `toml:"version"`
	} `toml:"foundation"`
	App struct {
		Name   string `toml:"name"`
		Module string `toml:"module"`
	} `toml:"app"`
}

// ParseFoundationToml parses a foundation.toml file and returns the configuration.
//
// It used to be a hand-rolled line scanner that tracked whether it was inside
// `[app]` and split on the first `=`: it mishandled comments, quoted keys,
// inline tables and multi-line values, and silently returned an empty config
// rather than reporting that it had not understood the file.
func ParseFoundationToml(path string) (*FoundationConfig, error) {
	config := &FoundationConfig{}

	if _, err := toml.DecodeFile(path, config); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return config, nil
}

// GetAppConfig reads the foundation.toml from the application root
func GetAppConfig() (*FoundationConfig, error) {
	appRoot := GetApplicationRoot()
	foundationTomlPath := filepath.Join(appRoot, "foundation.toml")
	return ParseFoundationToml(foundationTomlPath)
}

// ConstructServiceModuleName constructs the full module name for a service
func ConstructServiceModuleName(serviceName string) (string, error) {
	config, err := GetAppConfig()
	if err != nil {
		return "", fmt.Errorf("failed to read foundation.toml: %w", err)
	}

	// If the service name is already a full module path, use it as-is
	if strings.Contains(serviceName, "/") {
		return serviceName, nil
	}

	// Use app.name as the base module path (now stores full module path)
	if config.App.Name != "" {
		// If app name contains "/", it's a full module path - use it as base
		if strings.Contains(config.App.Name, "/") {
			return fmt.Sprintf("%s/%s", config.App.Name, serviceName), nil
		}
		// If app name is just a short name, try to infer from existing modules
		return inferServiceModuleFromWorkspace(config.App.Name, serviceName)
	}

	return "", errors.New("cannot determine module path: foundation.toml missing app name")
}

// inferServiceModuleFromWorkspace tries to infer the full service module path from existing workspace
func inferServiceModuleFromWorkspace(appName, serviceName string) (string, error) {
	appRoot := GetApplicationRoot()

	// Check for go.work first
	goWorkPath := filepath.Join(appRoot, "go.work")
	if _, err := os.Stat(goWorkPath); err == nil {
		if baseModule, err := inferModuleFromGoWork(goWorkPath); err == nil {
			return fmt.Sprintf("%s/%s", baseModule, serviceName), nil
		}
	}

	return "", fmt.Errorf("cannot determine module path: please use full module path when creating app (e.g., foundation new --app --name github.com/myorg/%s)", appName)
}

// inferModuleFromGoWork tries to infer the base module from existing go.work entries
func inferModuleFromGoWork(goWorkPath string) (string, error) {
	file, err := os.Open(goWorkPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "./") {
			// Found a service directory, try to read its go.mod
			servicePath := strings.Trim(line, `"./ `)
			appRoot := filepath.Dir(goWorkPath)
			goModPath := filepath.Join(appRoot, servicePath, "go.mod")

			if _, err := os.Stat(goModPath); err == nil {
				if module, err := readModuleFromGoMod(goModPath); err == nil {
					// Extract base module by removing the service name
					if idx := strings.LastIndex(module, "/"); idx != -1 {
						return module[:idx], nil
					}
				}
			}
		}
	}

	return "", errors.New("could not infer base module from go.work")
}

// readModuleFromGoMod reads the module name from a go.mod file
func readModuleFromGoMod(goModPath string) (string, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", errors.New("no module declaration found in go.mod")
}
