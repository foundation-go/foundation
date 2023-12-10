package helpers

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	for {
		foundationToml := filepath.Join(path, "foundation.toml")
		if _, err := os.Stat(foundationToml); err == nil {
			return path, nil
		}

		parent := filepath.Dir(path)
		if parent == path {
			break // we've reached the root directory
		}

		path = parent
	}

	return "", errors.New("can't find application root")
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

	return "", errors.New("can't find foundation.toml")
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

func RunCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func InGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	output, err := cmd.Output()

	return err == nil && string(output) == "true\n"
}
