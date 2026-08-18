package foundation

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envVarPattern finds the environment variables the framework reads.
var envVarPattern = regexp.MustCompile(`(?:GetEnvOr[A-Za-z]*|os\.Getenv)\("([A-Z][A-Z0-9_]*)"`)

// undocumentedEnvVars are read by the code but deliberately absent from ENV.md.
var undocumentedEnvVars = map[string]bool{
	// Only ever set by the test suite.
	"NON_EXISTING_ENV_VAR": true,
	// Set by the Go toolchain, not by an operator.
	"GOMODCACHE": true,
}

// ENV.md documented settings that did not exist and missed settings that did:
// `EVENTS_WORKER_ERRORS_TOPIC` was described in detail while nothing read it,
// the Kafka TLS file names were wrong, the tracing sampler default contradicted
// the code, and the SASL variables were not mentioned at all. Following the
// documentation literally produced a service that did not work.
func TestEveryEnvironmentVariableIsDocumented(t *testing.T) {
	docs, err := os.ReadFile("ENV.md")
	require.NoError(t, err)

	documented := string(docs)

	var undocumented []string

	for _, name := range envVarsReadByTheCode(t) {
		if undocumentedEnvVars[name] {
			continue
		}

		if !strings.Contains(documented, name) {
			undocumented = append(undocumented, name)
		}
	}

	sort.Strings(undocumented)

	assert.Empty(t, undocumented, "these are read by the code but missing from ENV.md")
}

// envVarsReadByTheCode walks the module and collects every environment variable
// name the non-test sources read.
func envVarsReadByTheCode(t *testing.T) []string {
	t.Helper()

	seen := map[string]bool{}

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// The examples and generated protobuf are not part of the
			// framework's configuration surface.
			if info.Name() == "examples" || info.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path) // #nosec G304 -- walking this module's own sources
		if err != nil {
			return err
		}

		for _, match := range envVarPattern.FindAllStringSubmatch(string(content), -1) {
			seen[match[1]] = true
		}

		return nil
	})
	require.NoError(t, err)

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}

	sort.Strings(names)

	require.NotEmpty(t, names, "the scan found nothing, which means it is broken")

	return names
}
