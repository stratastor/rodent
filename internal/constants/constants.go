// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	RodentVersion     = "v0.0.1"
	RodentPIDFilePath = "/var/run/rodent.pid"

	// config
	SystemConfigDir = "/etc/rodent"
	UserConfigDir   = "~/.rodent"
	ConfigFileName  = "rodent.yml"
	StateFileName   = "rodent_state.yml"

	// routes
	APIVersion = "v1"
	APIBase    = "/api/" + APIVersion + "/rodent"
	APIZFS     = APIBase + "/zfs"
	APIPool    = APIZFS + "/pool"
	APIDataset = APIZFS + "/dataset"

	APIAD = APIBase + "/ad"

	// APIServices is the base path for service management API endpoints
	APIServices = APIBase + "/services"

	// APIFACL is the base path for filesystem ACL management API endpoints
	APIFACL = APIBase + "/facl"

	APIShares = APIBase + "/shares"

	APIKeys    = APIBase + "/keys"
	APISSHKeys = APIKeys + "/ssh"

	// Updated path constants

	// Template paths - relative paths
	// Base paths - these should be set at runtime based on executable location
	TemplatesBasePath = "internal/templates"
	ScriptsBasePath   = "/home/ubuntu/rodent/scripts"

	// Traefik paths
	TraefikTemplateDir = TemplatesBasePath + "/traefik"
	TraefikConfigDir   = ScriptsBasePath + "/traefik"

	// Template file names (no paths needed as they are embedded)
	TraefikComposeTemplate = "dc-traefik.yml.tmpl"
	TraefikConfigTemplate  = "config.yml.tmpl"
	TraefikTLSTemplate     = "tls.yml.tmpl"

	// Runtime paths for files
	DefaultTraefikCertDir     = ScriptsBasePath + "/traefik/certs"
	DefaultTraefikTLSPath     = ScriptsBasePath + "/traefik/tls.yml"
	DefaultTraefikComposePath = ScriptsBasePath + "/traefik/dc-traefik.yml"
	DefaultTraefikConfigPath  = ScriptsBasePath + "/traefik/config.yml"
)
