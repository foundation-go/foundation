package helpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The parser used to be a hand-rolled line scanner: it mishandled comments,
// quoted keys and inline tables, and returned an empty config rather than
// reporting that it had not understood the file.
func TestParseFoundationToml(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foundation.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
# A comment mentioning name = "wrong"
[foundation]
version = "0.3.0"

[app]
name = "github.com/example/myapp"   # trailing comment
module = "github.com/example/myapp"
`), 0o600))

	config, err := ParseFoundationToml(path)
	require.NoError(t, err)

	assert.Equal(t, "0.3.0", config.Foundation.Version)
	assert.Equal(t, "github.com/example/myapp", config.App.Name)
	assert.Equal(t, "github.com/example/myapp", config.App.Module)
}

func TestParseFoundationTomlReportsMalformedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foundation.toml")
	require.NoError(t, os.WriteFile(path, []byte("[app\nname = "), 0o600))

	_, err := ParseFoundationToml(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestParseFoundationTomlReportsMissingFiles(t *testing.T) {
	_, err := ParseFoundationToml(filepath.Join(t.TempDir(), "nope.toml"))

	assert.Error(t, err)
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	require.NoError(t, os.WriteFile(src, []byte("contents"), 0o600))
	require.NoError(t, CopyFile(src, dst))

	content, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "contents", string(content))

	assert.Error(t, CopyFile(filepath.Join(dir, "missing"), dst))
}
