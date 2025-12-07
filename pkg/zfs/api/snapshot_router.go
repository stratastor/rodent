// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/zfs/autosnapshots"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
)

// RegisterAutoSnapshotRoutes registers the auto-snapshot routes to the dataset router group
// Returns the handler so it can be stored for use by other subsystems (e.g., inventory)
func RegisterAutoSnapshotRoutes(
	router *gin.RouterGroup,
	dsManager *dataset.Manager,
) (*autosnapshots.Handler, error) {
	// Create handler
	handler, err := autosnapshots.NewHandler(dsManager)
	if err != nil {
		return nil, err
	}

	// Start the manager
	if err := handler.StartManager(); err != nil {
		return nil, err
	}

	// Register routes
	handler.RegisterRoutes(router)

	return handler, nil
}
