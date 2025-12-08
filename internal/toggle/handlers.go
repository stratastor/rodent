// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	generalCmd "github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/managers"
	servicesAPI "github.com/stratastor/rodent/internal/services/api"
	servicesMgr "github.com/stratastor/rodent/internal/services/manager"
	adHandlers "github.com/stratastor/rodent/pkg/ad/handlers"
	"github.com/stratastor/rodent/pkg/facl"
	faclAPI "github.com/stratastor/rodent/pkg/facl/api"
	sharesAPI "github.com/stratastor/rodent/pkg/shares/api"
	"github.com/stratastor/rodent/pkg/shares/smb"
	zfsAPI "github.com/stratastor/rodent/pkg/zfs/api"
	"github.com/stratastor/rodent/pkg/zfs/autosnapshots"
	"github.com/stratastor/rodent/pkg/zfs/autotransfers"
	"github.com/stratastor/rodent/pkg/zfs/command"
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

	// Get shared managers (set by HTTP routes during server startup)
	datasetManager := managers.GetDatasetManager()
	transferManager := managers.GetTransferManager()

	// Create dataset handler using shared managers
	var datasetHandler *zfsAPI.DatasetHandler
	if datasetManager != nil && transferManager != nil {
		datasetHandler, err = zfsAPI.NewDatasetHandler(datasetManager, transferManager)
		if err != nil {
			l.Error("Failed to create dataset handler", "error", err)
		}
	} else {
		l.Warn("Shared managers not available, creating dataset handler without transfer support")
		// Fallback: create minimal handler (datasetManager may be nil)
		datasetHandler, _ = zfsAPI.NewDatasetHandler(datasetManager, nil)
	}

	poolManager := pool.NewManager(executor)
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

	// Register auto-snapshot gRPC handlers using shared manager
	snapshotManager := managers.GetSnapshotManager()
	if snapshotManager != nil {
		snapHandler := autosnapshots.NewGRPCHandlerWithManager(snapshotManager)
		// Don't start the manager - it's already started by HTTP routes
		autosnapshots.RegisterAutosnapshotGRPCHandlers(snapHandler)
		l.Info("Registered auto-snapshot gRPC handlers")
	} else {
		l.Warn("Snapshot manager not available, skipping auto-snapshot gRPC handlers")
	}

	// Register auto-transfer gRPC handlers using shared manager
	transferPolicyManager := managers.GetTransferPolicyManager()
	if transferPolicyManager != nil {
		transferHandler := autotransfers.NewGRPCHandlerWithManager(transferPolicyManager)
		// Don't start the manager - it's already started by HTTP routes
		autotransfers.RegisterTransferPolicyGRPCHandlers(transferHandler)
		l.Info("Registered auto-transfer gRPC handlers")
	} else {
		l.Warn("Transfer policy manager not available, skipping auto-transfer gRPC handlers")
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

	// Note: Network gRPC handlers are registered in pkg/server/routes.go
	// within registerNetworkRoutes() function, not here. This is because
	// the network manager needs to be created during server startup
	// with proper context and renderer configuration.
}
