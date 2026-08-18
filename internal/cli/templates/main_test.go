package templates

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func templateData() map[string]interface{} {
	return map[string]interface{}{
		"FoundationVersion": "0.3.0",
		"Name":              "github.com/example/myapp/chats",
		"ShortName":         "chats",
	}
}

func render(t *testing.T, folder, name string) string {
	t.Helper()

	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o750))
	require.NoError(t, CreateFromTemplate(dir, folder, name, templateData()))

	content, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)

	return string(content)
}

// The scaffolded service imported `foundation/examples/clubchat/protos/chats`
// and referenced a symbol that does not exist there, so `foundation new
// --service` produced a directory that could not be built.
func TestServiceMainTemplateIsValidGo(t *testing.T) {
	source := render(t, "service", "cmd/grpc/main.go")

	_, err := parser.ParseFile(token.NewFileSet(), "main.go", source, parser.AllErrors)
	require.NoError(t, err, "the generated main.go must parse")

	assert.NotContains(t, source, "examples/", "generated code must not depend on the framework's examples")
	assert.NotContains(t, source, "{{", "every placeholder must be substituted")
	assert.Contains(t, source, `f.InitGRPCServer("chats")`)
}

// Templates receive both the full name and the short one; using the full module
// path where a short name belongs produced a database called
// "github.com/example/myapp/chats".
func TestServiceEnvTemplateUsesTheShortName(t *testing.T) {
	source := render(t, "service", ".env.example")

	assert.Contains(t, source, "/chats")
	assert.NotContains(t, source, "github.com")
	assert.NotContains(t, source, "{{")
}

func TestApplicationTemplates(t *testing.T) {
	toml := render(t, "application", "foundation.toml")

	assert.Contains(t, toml, `version = "0.3.0"`)
	assert.Contains(t, toml, `name = "github.com/example/myapp/chats"`)
	assert.NotContains(t, toml, "{{")

	for _, name := range []string{"README.md", ".gitignore"} {
		assert.NotContains(t, render(t, "application", name), "{{")
	}
}

func TestServiceReadmeTemplate(t *testing.T) {
	assert.NotContains(t, render(t, "service", "README.md"), "{{")
}

func TestReadTemplateReportsAMissingTemplate(t *testing.T) {
	_, err := ReadTemplate("service/nope")

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "failed to read file"))
}
