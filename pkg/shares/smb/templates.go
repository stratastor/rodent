// Package smb provides SMB share management functionality
package smb

import (
	"embed"
)

//go:embed share.tmpl global.tmpl
var templateFS embed.FS

// DefaultTemplateContent returns the content for the default share template
func DefaultTemplateContent() string {
	content, err := templateFS.ReadFile("share.tmpl")
	if err != nil {
		// Fallback to hardcoded template
		return `[{{.Name}}]
    path = {{.Path}}
    comment = {{.Description}}
    read only = {{if .ReadOnly}}yes{{else}}no{{end}}
    browsable = {{if .Browsable}}yes{{else}}no{{end}}
    {{if .ValidUsers}}valid users = {{join .ValidUsers ", "}}{{end}}
    {{if .InheritACLs}}inherit acls = yes{{end}}
    {{if .MapACLInherit}}map acl inherit = yes{{end}}
    {{range $key, $value := .CustomParameters}}
    {{$key}} = {{$value}}
    {{end}}`
	}
	return string(content)
}

// GlobalTemplateContent returns the global configuration template content
func GlobalTemplateContent() string {
	content, _ := templateFS.ReadFile("global.tmpl")
	return string(content)
}
