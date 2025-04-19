// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"github.com/stratastor/rodent/pkg/zfs/api"
)

// RegisterAllHandlers registers all domain-specific command handlers
func RegisterAllHandlers() {
	// Register ZFS-related handlers
	api.RegisterZFSGRPCHandlers()

	// Add registrations for other domains here
	// Example:
	// shares.RegisterSMBHandlers()
	// ad.RegisterADHandlers()
	// etc.
}
