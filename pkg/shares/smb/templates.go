// Package smb provides SMB share management functionality
package smb

import (
	"embed"
	"text/template"
)

//go:embed share.tmpl global.tmpl
var templateFS embed.FS

// GetTemplate loads a template from the embedded filesystem
func getTemplate(name string) (*template.Template, error) {
	content, err := templateFS.ReadFile(name)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

// DefaultTemplateContent returns the default share template content
func DefaultTemplateContent() string {
	content, _ := templateFS.ReadFile("share.tmpl")
	return string(content)
}

// GlobalTemplateContent returns the global configuration template content
func GlobalTemplateContent() string {
	content, _ := templateFS.ReadFile("global.tmpl")
	return string(content)
}
