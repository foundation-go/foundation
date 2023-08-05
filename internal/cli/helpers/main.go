package helpers

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func AtProjectRoot(paths ...string) string {
	return filepath.Join(getProjectRoot(), filepath.Join(paths...))
}

// getProjectRoot returns the absolute path to the project root directory or panics if it can't be found.
func getProjectRoot() string {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	root, err := findProjectRoot(path)
	if err != nil {
		panic(err)
	}

	return root
}

func findProjectRoot(path string) (string, error) {
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

	return "", errors.New("can't find project root")
}

// findGoMod finds the go.mod file by traversing up the directory tree.
// It returns the absolute path to the go.mod file, or an error if it can't be found.
func findGoMod(path string) (string, error) {
	projectRoot, err := findProjectRoot(path)
	if err != nil {
		return "", err
	}

	gomod := filepath.Join(projectRoot, "go.mod")
	if _, err := os.Stat(gomod); err == nil {
		return gomod, nil
	}

	return "", errors.New("no go.mod file found")
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
		if strings.Contains(scanner.Text(), "github.com/ri-nat/foundation") {
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
