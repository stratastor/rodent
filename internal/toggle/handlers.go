// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	adHandlers "github.com/stratastor/rodent/pkg/ad/handlers"
	zfsAPI "github.com/stratastor/rodent/pkg/zfs/api"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

// RegisterAllHandlers registers all domain-specific command handlers
func RegisterAllHandlers() {
	cfg := config.GetConfig()
	l, err := logger.NewTag(logger.Config{LogLevel: cfg.Server.LogLevel}, "toggle")
	if err != nil {
		panic(err)
	}

	// Create command executor with sudo support
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: cfg.Server.LogLevel})

	// Initialize managers
	datasetManager := dataset.NewManager(executor)
	poolManager := pool.NewManager(executor)

	// Create API handlers
	datasetHandler := zfsAPI.NewDatasetHandler(datasetManager)
	poolHandler := zfsAPI.NewPoolHandler(poolManager)

	// Register ZFS-related handlers
	zfsAPI.RegisterZFSGRPCHandlers(poolHandler, datasetHandler)

	// Create and register AD handler for gRPC using the shared client
	adHandler, err := adHandlers.NewADHandler()
	if err != nil {
		l.Error("Failed to create AD handler", "error", err)
	} else {
		adHandlers.RegisterADGRPCHandlers(adHandler)
	}

	// Add registrations for other domains here as needed
	// Example:
	// shares.RegisterSMBGRPCHandlers()
	// etc.
}
