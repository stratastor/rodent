// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package templates

import (
	"embed"
)

//go:embed traefik/*.tmpl
var TraefikFS embed.FS

// GetTraefikTemplate returns the content of a Traefik template by name
func GetTraefikTemplate(name string) (string, error) {
	content, err := TraefikFS.ReadFile("traefik/" + name)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
