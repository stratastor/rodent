// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	RodentVersion     = "v0.0.1"
	RodentPIDFilePath = "/home/rodent/.rodent/rodent.pid"

	// config
	ConfigFileName = "rodent.yml"
	StateFileName  = "rodent_state.yml"

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

	// APINetwork is the base path for network management API endpoints
	APINetwork = APIBase + "/network"

	// APISystem is the base path for system management API endpoints
	APISystem = APIBase + "/system"

	// APIDisk is the base path for disk management API endpoints
	APIDisk = APIBase + "/disk"

	// Template paths - relative paths
	TemplatesBasePath = "internal/templates"
)
