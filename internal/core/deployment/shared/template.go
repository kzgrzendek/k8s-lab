package shared

import "github.com/kzgrzendek/nova/internal/utils"

// RenderTemplate reads a template file, executes it with the provided data, and returns the rendered content.
// This is a convenience wrapper around utils.RenderTemplate for backward compatibility.
func RenderTemplate(templatePath string, data interface{}) (string, error) {
	return utils.RenderTemplate(templatePath, data)
}
