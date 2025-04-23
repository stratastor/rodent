// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	generalCmd "github.com/stratastor/rodent/internal/command"
	adHandlers "github.com/stratastor/rodent/pkg/ad/handlers"
	"github.com/stratastor/rodent/pkg/facl"
	sharesAPI "github.com/stratastor/rodent/pkg/shares/api"
	"github.com/stratastor/rodent/pkg/shares/smb"
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

	genexec := generalCmd.NewCommandExecutor(true)
	// Create and register AD handler for gRPC using the shared client
	adHandler, err := adHandlers.NewADHandler()
	if err != nil {
		l.Error("Failed to create AD handler", "error", err)
	} else {
		adHandlers.RegisterADGRPCHandlers(adHandler)
	}

	sl, err := logger.NewTag(logger.Config{LogLevel: cfg.Server.LogLevel}, "toggle.shares")
	if err != nil {
		panic(err)
	}

	// Create and register SMB shares handler for gRPC
	aclManager := facl.NewACLManager(sl, nil)
	// Create SMB managers
	smbManager, err := smb.NewManager(sl, genexec, aclManager)
	if err != nil {
		l.Error("Failed to create SMB manager", "error", err)
	} else {
		smbService := smb.NewServiceManager(sl)
		// Create and register shares handler
		sharesHandler := sharesAPI.NewSharesHandler(sl, smbManager, smbService)
		sharesAPI.RegisterSharesGRPCHandlers(sharesHandler)
		l.Info("Registered SMB shares gRPC handlers")
	}
}
