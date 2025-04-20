// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/zfs/api"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

// RegisterAllHandlers registers all domain-specific command handlers
func RegisterAllHandlers() {
	cfg := config.GetConfig()

	// Create command executor with sudo support
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: cfg.Server.LogLevel})

	// Initialize managers
	datasetManager := dataset.NewManager(executor)
	poolManager := pool.NewManager(executor)

	// Create API handlers
	datasetHandler := api.NewDatasetHandler(datasetManager)
	poolHandler := api.NewPoolHandler(poolManager)

	// Register ZFS-related handlers
	api.RegisterZFSGRPCHandlers(poolHandler, datasetHandler)

	// Add registrations for other domains here as needed
	// Example:
	// shares.RegisterSMBGRPCHandlers()
	// ad.RegisterADGRPCHandlers()
	// etc.
}
