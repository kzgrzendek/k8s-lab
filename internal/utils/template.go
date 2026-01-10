package utils

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

// RenderTemplate reads a template file, executes it with the provided data, and returns the rendered content.
func RenderTemplate(templatePath string, data interface{}) (string, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file %s: %w", templatePath, err)
	}

	tmpl, err := template.New("template").Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templatePath, err)
	}

	return buf.String(), nil
}
