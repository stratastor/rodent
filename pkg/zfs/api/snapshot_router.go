// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/snapshot"
)

// RegisterAutoSnapshotRoutes registers the auto-snapshot routes to the dataset router group
// Returns the handler so it can be stored for use by other subsystems (e.g., inventory)
func RegisterAutoSnapshotRoutes(router *gin.RouterGroup, dsManager *dataset.Manager) (*snapshot.Handler, error) {
	// Create handler
	handler, err := snapshot.NewHandler(dsManager)
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