// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	generalCmd "github.com/stratastor/rodent/internal/command"
	servicesAPI "github.com/stratastor/rodent/internal/services/api"
	servicesMgr "github.com/stratastor/rodent/internal/services/manager"
	adHandlers "github.com/stratastor/rodent/pkg/ad/handlers"
	"github.com/stratastor/rodent/pkg/facl"
	faclAPI "github.com/stratastor/rodent/pkg/facl/api"
	sharesAPI "github.com/stratastor/rodent/pkg/shares/api"
	"github.com/stratastor/rodent/pkg/shares/smb"
	zfsAPI "github.com/stratastor/rodent/pkg/zfs/api"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
	"github.com/stratastor/rodent/pkg/zfs/snapshot"
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

	// Create and register FACL handler for gRPC
	faclLogger, err := logger.NewTag(logger.Config{LogLevel: cfg.Server.LogLevel}, "toggle.facl")
	if err != nil {
		panic(err)
	}
	aclManager := facl.NewACLManager(faclLogger, nil)
	aclHandler := faclAPI.NewACLHandler(aclManager, faclLogger)
	faclAPI.RegisterFACLGRPCHandlers(aclHandler)
	l.Info("Registered FACL gRPC handlers")

	// Create SMB managers and register SMB shares handler for gRPC
	smbManager, err := smb.NewManager(sl, genexec, nil)
	if err != nil {
		l.Error("Failed to create SMB manager", "error", err)
	} else {
		smbService := smb.NewServiceManager(sl)
		// Create and register shares handler
		sharesHandler := sharesAPI.NewSharesHandler(sl, smbManager, smbService)
		sharesAPI.RegisterSharesGRPCHandlers(sharesHandler)
		l.Info("Registered SMB shares gRPC handlers")
	}

	// Create and register auto-snapshot handler for gRPC
	snapHandler, err := snapshot.NewGRPCHandler(datasetManager)
	if err != nil {
		l.Error("Failed to create auto-snapshot handler", "error", err)
	} else {
		// Start the snapshot manager
		if err := snapHandler.StartManager(); err != nil {
			l.Error("Failed to start auto-snapshot manager", "error", err)
		}

		// Register the handlers
		snapshot.RegisterAutosnapshotGRPCHandlers(snapHandler)
		l.Info("Registered auto-snapshot gRPC handlers")
	}

	// Create and register services handler for gRPC
	servicesLogger, err := logger.NewTag(
		logger.Config{LogLevel: cfg.Server.LogLevel},
		"toggle.services",
	)
	if err != nil {
		l.Error("Failed to create services logger", "error", err)
		panic(err)
	} else {
		serviceManager, err := servicesMgr.NewServiceManager(servicesLogger)
		if err != nil {
			l.Error("Failed to create service manager", "error", err)
			panic(err)
		} else {
			serviceHandler := servicesAPI.NewServiceHandler(serviceManager)
			servicesAPI.RegisterServiceGRPCHandlers(serviceHandler)
			l.Info("Registered services gRPC handlers")
		}
	}
}
