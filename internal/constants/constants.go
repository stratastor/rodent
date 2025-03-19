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

	// config
	DefaultTraefikCertDir = "/home/ubuntu/rodent/scripts/traefik/certs"
	DefaultTraefikTLSPath = "/home/ubuntu/rodent/scripts/traefik/tls.yml"

	// Docker compose file paths
	DefaultDockerComposeDir   = "/home/ubuntu/rodent/scripts"
	DefaultTraefikComposePath = "/home/ubuntu/rodent/scripts/traefik/dc-traefik.yml"
)
