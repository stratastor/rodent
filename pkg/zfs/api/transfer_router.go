// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/zfs/autosnapshots"
	"github.com/stratastor/rodent/pkg/zfs/autotransfers"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
)

// RegisterTransferPolicyRoutes registers the transfer policy routes to the scheduler router group
// Returns the handler so it can be stored for use by other subsystems
func RegisterTransferPolicyRoutes(
	router *gin.RouterGroup,
	transferManager *dataset.TransferManager,
	snapshotHandler *autosnapshots.Handler,
) (*autotransfers.Handler, error) {
	// Get snapshot manager from handler
	snapshotMgr, err := autosnapshots.GetManager(nil, "")
	if err != nil {
		return nil, err
	}

	// Get transfer policy manager
	cfg := config.GetConfig()
	logCfg := logger.Config{LogLevel: cfg.Server.LogLevel}
	policyManager, err := autotransfers.GetManager(snapshotMgr, transferManager, logCfg)
	if err != nil {
		return nil, err
	}

	// Create handler with the manager
	handler := autotransfers.NewHandlerWithManager(policyManager)

	// Start the manager
	if err := handler.StartManager(); err != nil {
		return nil, err
	}

	// Register routes
	handler.RegisterRoutes(router)

	return handler, nil
}
