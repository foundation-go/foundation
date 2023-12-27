package templates

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	//go:embed application application/.gitignore.tmpl
	//go:embed service service/.env.example.tmpl
	fileTemplates embed.FS
)

func CreateFromTemplate(dir string, tmplFolder string, tmplName string, input interface{}) error {
	file, err := os.Create(filepath.Join(dir, tmplName))
	if err != nil {
		return fmt.Errorf("failed to create file `%s`: %w", tmplName, err)
	}
	defer file.Close()

	tmpl, err := ReadTemplate(tmplFolder + "/" + tmplName)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	err = tmpl.Execute(file, input)
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func ReadTemplate(name string) (*template.Template, error) {
	tmplContent, err := fileTemplates.ReadFile(name + ".tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	fnMap := template.FuncMap{
		"Title": cases.Title(language.English).String,
		"Lower": cases.Lower(language.English).String,
	}

	tmpl, err := template.New(name).Funcs(fnMap).Parse(string(tmplContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return tmpl, nil
}
