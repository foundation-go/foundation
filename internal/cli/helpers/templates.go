package helpers

import (
	"fmt"
	"os"
	"text/template"
)

func CreateFromTemplate(dir string, tmplFolder string, tmplName string, input interface{}) error {
	file, err := os.Create(dir + "/" + tmplName)
	if err != nil {
		return fmt.Errorf("failed to create file `%s`: %v", tmplName, err)
	}
	defer file.Close()

	tmpl, err := ReadTemplate(tmplFolder + "/" + tmplName)
	if err != nil {
		return fmt.Errorf("failed to read template: %v", err)
	}

	err = tmpl.Execute(file, input)
	if err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	return nil
}

func ReadTemplate(name string) (*template.Template, error) {
	file, err := os.Open("internal/cli/templates/" + name + ".tmpl")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(name).Parse(string(buf[:n]))
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}
