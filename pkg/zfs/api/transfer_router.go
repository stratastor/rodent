// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/snapshot"
	"github.com/stratastor/rodent/pkg/zfs/transfers"
)

// RegisterTransferPolicyRoutes registers the transfer policy routes to the scheduler router group
// Returns the handler so it can be stored for use by other subsystems
func RegisterTransferPolicyRoutes(
	router *gin.RouterGroup,
	transferManager *dataset.TransferManager,
	snapshotHandler *snapshot.Handler,
) (*transfers.Handler, error) {
	// Get snapshot manager from handler
	snapshotMgr, err := snapshot.GetManager(nil, "")
	if err != nil {
		return nil, err
	}

	// Get transfer policy manager
	cfg := config.GetConfig()
	logCfg := logger.Config{LogLevel: cfg.Server.LogLevel}
	policyManager, err := transfers.GetManager(snapshotMgr, transferManager, logCfg)
	if err != nil {
		return nil, err
	}

	// Create handler with the manager
	handler := transfers.NewHandlerWithManager(policyManager)

	// Start the manager
	if err := handler.StartManager(); err != nil {
		return nil, err
	}

	// Register routes
	handler.RegisterRoutes(router)

	return handler, nil
}
