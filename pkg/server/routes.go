// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	generalCmd "github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/constants"
	"github.com/stratastor/rodent/internal/events"
	"github.com/stratastor/rodent/internal/managers"
	svcAPI "github.com/stratastor/rodent/internal/services/api"
	svcManager "github.com/stratastor/rodent/internal/services/manager"
	"github.com/stratastor/rodent/pkg/ad"
	"github.com/stratastor/rodent/pkg/ad/handlers"
	"github.com/stratastor/rodent/pkg/disk"
	diskAPI "github.com/stratastor/rodent/pkg/disk/api"
	"github.com/stratastor/rodent/pkg/facl"
	aclAPI "github.com/stratastor/rodent/pkg/facl/api"
	"github.com/stratastor/rodent/pkg/inventory"
	sshAPI "github.com/stratastor/rodent/pkg/keys/ssh/api"
	"github.com/stratastor/rodent/pkg/netmage"
	netmageAPI "github.com/stratastor/rodent/pkg/netmage/api"
	"github.com/stratastor/rodent/pkg/netmage/types"
	"github.com/stratastor/rodent/pkg/shares"
	sharesAPI "github.com/stratastor/rodent/pkg/shares/api"
	"github.com/stratastor/rodent/pkg/shares/smb"
	"github.com/stratastor/rodent/pkg/system"
	systemAPI "github.com/stratastor/rodent/pkg/system/api"
	"github.com/stratastor/rodent/pkg/zfs/api"
	"github.com/stratastor/rodent/pkg/zfs/autosnapshots"
	"github.com/stratastor/rodent/pkg/zfs/autotransfers"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

// Shared manager instances for stateful subsystems
var (
	// sharedDiskManager holds the stateful disk manager instance
	// Used by inventory and other subsystems that need access to disk state
	sharedDiskManager *disk.Manager

	// sharedSharesManager holds the shares manager instance (SMB manager)
	// Used by inventory to collect shares information
	sharedSharesManager shares.SharesManager

	// sharedSnapshotHandler holds the snapshot handler which provides access to the snapshot manager
	// Used by inventory to collect snapshot policies information
	sharedSnapshotHandler *autosnapshots.Handler

	// sharedTransferPolicyHandler holds the transfer policy handler
	// Used by inventory to collect transfer policies information
	sharedTransferPolicyHandler *autotransfers.Handler

	// sharedTransferManager holds the transfer manager instance
	// Used for shutdown to gracefully terminate active transfers
	sharedTransferManager *dataset.TransferManager
)

func registerZFSRoutes(engine *gin.Engine) (error error) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	cfg := config.GetConfig()
	// Create command executor with sudo support
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: cfg.Server.LogLevel})

	// Initialize managers
	datasetManager := dataset.NewManager(executor)
	managers.SetDatasetManager(datasetManager)

	var datasetHandler *api.DatasetHandler
	transferManager, err := dataset.NewTransferManager(logger.Config{LogLevel: cfg.Server.LogLevel})
	if err != nil {
		return fmt.Errorf("failed to create dataset transfer manager: %w", err)

	} else {
		// Store shared instance for use by shutdown handler and gRPC handlers
		sharedTransferManager = transferManager
		managers.SetTransferManager(transferManager)

		// Create dataset handler with transfer manager
		datasetHandler, err = api.NewDatasetHandler(datasetManager, transferManager)
		if err != nil {
			return fmt.Errorf("failed to create dataset handler: %w", err)
		}
	}

	poolManager := pool.NewManager(executor)
	poolHandler := api.NewPoolHandler(poolManager)

	// API group with version
	v1 := engine.Group(constants.APIZFS)
	{
		// Register ZFS routes
		datasetHandler.RegisterRoutes(v1)
		poolHandler.RegisterRoutes(v1)

		schedulers := v1.Group("/schedulers")
		{
			// Register auto-snapshot routes and store handler for use by other subsystems (e.g., inventory)
			snapshotHandler, err := api.RegisterAutoSnapshotRoutes(schedulers, datasetManager)
			if err == nil {
				sharedSnapshotHandler = snapshotHandler
				managers.SetSnapshotManager(snapshotHandler.Manager())
			}
			// If err != nil, sharedSnapshotHandler remains nil and inventory won't include snapshot policies

			// Register transfer policy routes
			if snapshotHandler != nil && transferManager != nil {
				transferPolicyHandler, err := api.RegisterTransferPolicyRoutes(
					schedulers,
					transferManager,
					snapshotHandler,
				)
				if err != nil {
					// Log the error but don't fail startup
					cfg := config.GetConfig()
					if l, lerr := logger.NewTag(logger.Config{LogLevel: cfg.Server.LogLevel}, "routes"); lerr == nil {
						l.Warn("Failed to register transfer policy routes", "error", err)
					}
				} else {
					sharedTransferPolicyHandler = transferPolicyHandler
					managers.SetTransferPolicyManager(transferPolicyHandler.Manager())
				}
			}
		}

		// Health check routes
		// v1.GET("/health", healthCheck)
	}
	return nil
}

func registerADRoutes(engine *gin.Engine) (adHandler *handlers.ADHandler, err error) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	// Create AD handler
	adHandler, err = handlers.NewADHandler()
	if err != nil {
		// Log the error but don't fail startup
		// TODO: Handle this differently?
		return nil, err
	}

	// API group with version
	v1 := engine.Group(constants.APIAD)
	{
		// Register AD routes
		adHandler.RegisterRoutes(v1)
	}
	return adHandler, nil
}

func registerServiceRoutes(engine *gin.Engine) (serviceHandler *svcAPI.ServiceHandler, err error) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	// Create logger
	l, err := logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "services")
	if err != nil {
		return nil, err
	}

	// Create service manager
	serviceManager, err := svcManager.NewServiceManager(l)
	if err != nil {
		return nil, fmt.Errorf("failed to create service manager: %w", err)
	}

	// Create service handler
	serviceHandler = svcAPI.NewServiceHandler(serviceManager)

	// API group with version
	v1 := engine.Group(constants.APIServices)
	{
		// Register service routes
		serviceHandler.RegisterRoutes(v1)
	}
	return serviceHandler, nil
}

func registerFaclRoutes(engine *gin.Engine) (*aclAPI.ACLHandler, error) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	// Create logger
	l, err := logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "facl")
	if err != nil {
		return nil, err
	}

	adClient, err := ad.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create AD client: %w", err)
	}

	// Create ACL manager
	aclManager := facl.NewACLManager(l, adClient)

	// Create ACL handler
	aclHandler := aclAPI.NewACLHandler(aclManager, l)

	// API group with version
	v1 := engine.Group(constants.APIFACL)
	{
		// Register ACL routes
		aclHandler.RegisterRoutes(v1)
	}

	return aclHandler, nil
}

// RegisterSharesRoutes registers shares API routes
func registerSharesRoutes(engine *gin.Engine) error {
	l, err := logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "shares")
	if err != nil {
		return err
	}
	// Create the SMB manager
	executor := generalCmd.NewCommandExecutor(true)

	// Create SMB manager (passing nil for fileOps to use default paths)
	smbManager, err := smb.NewManager(l, executor, nil)
	if err != nil {
		return fmt.Errorf("failed to create SMB manager: %w", err)
	}

	// Attempt to generate config on startup by importing existing smb.conf if needed
	// This is done in a goroutine to avoid blocking initialization
	go func() {
		// Create a background context for this operation
		ctx := context.Background()
		if err := smbManager.GenerateConfig(ctx); err != nil {
			l.Warn("Failed to generate initial SMB configuration", "error", err)
		}
	}()

	// Create SMB service manager
	smbService := smb.NewServiceManager(l)

	// Store shared instance for use by other subsystems (e.g., inventory)
	sharedSharesManager = smbManager

	// Create the shares handler
	sharesHandler := sharesAPI.NewSharesHandler(l, smbManager, smbService)

	// Register routes
	v1 := engine.Group(constants.APIShares)
	{
		sharesHandler.RegisterRoutes(v1)
	}

	return nil
}

// registerSSHKeyRoutes registers SSH key management API routes
func registerSSHKeyRoutes(engine *gin.Engine) (*sshAPI.SSHKeyHandler, error) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	// Create logger
	l, err := logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "ssh")
	if err != nil {
		return nil, err
	}

	// Create SSH key handler
	sshKeyHandler, err := sshAPI.NewSSHKeyHandler(l)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH key handler: %w", err)
	}

	// API group with version
	v1 := engine.Group(constants.APISSHKeys)
	{
		// Register SSH key routes
		sshKeyHandler.RegisterRoutes(v1)
	}

	// Register gRPC handlers
	sshAPI.RegisterSSHKeyGRPCHandlers(sshKeyHandler)

	return sshKeyHandler, nil
}

// registerNetworkRoutes registers network management API routes
func registerNetworkRoutes(engine *gin.Engine) (*netmageAPI.NetworkHandler, error) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	// Create logger
	l, err := logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "network")
	if err != nil {
		return nil, err
	}

	// Create network manager with networkd renderer (default)
	ctx := context.Background()
	networkManager, err := netmage.NewManager(ctx, l, types.RendererNetworkd)
	if err != nil {
		return nil, fmt.Errorf("failed to create network manager: %w", err)
	}

	// Create network handler
	networkHandler := netmageAPI.NewNetworkHandler(networkManager, l)

	// API group with version
	v1 := engine.Group(constants.APINetwork)
	{
		// Register network routes
		networkHandler.RegisterRoutes(v1)
	}

	// Register gRPC handlers
	netmageAPI.RegisterNetworkGRPCHandlers(networkHandler)

	return networkHandler, nil
}

// registerSystemRoutes registers system management API routes
func registerSystemRoutes(engine *gin.Engine) (*systemAPI.SystemHandler, error) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	// Create logger
	l, err := logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "system")
	if err != nil {
		return nil, err
	}

	// Create system manager
	systemManager := system.NewManager(l)

	// Create system handler
	systemHandler := systemAPI.NewSystemHandler(systemManager, l)

	// API group with version
	v1 := engine.Group(constants.APISystem)
	{
		// Register system routes
		systemHandler.RegisterRoutes(v1)
	}

	// Register gRPC handlers
	systemAPI.RegisterSystemGRPCHandlers(systemHandler)

	return systemHandler, nil
}

// registerDiskRoutes registers disk management API routes
func registerDiskRoutes(engine *gin.Engine) (*diskAPI.DiskHandler, error) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	// Create logger
	l, err := logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "disk")
	if err != nil {
		return nil, err
	}

	// Create command executor with sudo support
	executor := generalCmd.NewCommandExecutor(true)

	// Use global event bus (may be nil if not initialized yet)
	eventBus := events.GlobalEventBus
	if eventBus == nil {
		l.Warn("Global event bus not initialized, disk events will be logged only")
	}

	// Create disk manager (manages its own zpool executor for conflict detection)
	diskManager, err := disk.NewManager(l, executor, eventBus)
	if err != nil {
		return nil, fmt.Errorf("failed to create disk manager: %w", err)
	}

	// Start disk manager
	ctx := context.Background()
	if err := diskManager.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start disk manager: %w", err)
	}

	// Store shared instance for use by other subsystems (e.g., inventory)
	sharedDiskManager = diskManager

	// Create disk handler
	diskHandler := diskAPI.NewDiskHandler(diskManager, l)

	// API group with version
	v1 := engine.Group(constants.APIDisk)
	{
		// Register disk routes
		diskHandler.RegisterRoutes(v1)
	}

	// Register gRPC handlers
	diskAPI.RegisterDiskGRPCHandlers(diskHandler)

	return diskHandler, nil
}

// registerInventoryRoutes registers inventory API routes
// Creates new manager instances for stateless managers (System, ZFS, Network)
// Uses shared disk manager instance for stateful disk operations
func registerInventoryRoutes(engine *gin.Engine) (*inventory.Handler, error) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	cfg := config.GetConfig()

	// Create logger
	l, err := logger.NewTag(config.NewLoggerConfig(cfg), "inventory")
	if err != nil {
		return nil, err
	}

	// Create stateless managers for inventory collection
	// These managers only read state and don't maintain persistent state

	// System Manager - reads /proc, /sys, etc.
	systemMgr := system.NewManager(l)

	// ZFS Managers - wrap zpool/zfs commands
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: cfg.Server.LogLevel})
	poolMgr := pool.NewManager(executor)
	datasetMgr := dataset.NewManager(executor)

	// Network Manager - reads network configuration
	ctx := context.Background()
	networkMgr, err := netmage.NewManager(ctx, l, types.RendererNetworkd)
	if err != nil {
		l.Warn("Failed to create network manager for inventory", "error", err)
		networkMgr = nil // Continue without network data
	}

	// Disk Manager - use shared stateful instance
	// May be nil if disk routes haven't been registered yet
	diskMgr := sharedDiskManager
	if diskMgr == nil {
		l.Warn("Shared disk manager not available, disk inventory will be unavailable")
	}

	// Shares Manager - use shared instance
	// May be nil if shares routes haven't been registered yet
	sharesMgr := sharedSharesManager
	if sharesMgr == nil {
		l.Warn("Shared shares manager not available, shares inventory will be unavailable")
	}

	// Snapshot Manager - extract from shared handler
	// May be nil if snapshot routes haven't been registered yet
	var snapshotMgr autosnapshots.SchedulerInterface
	if sharedSnapshotHandler != nil {
		snapshotMgr = sharedSnapshotHandler // Handler implements SchedulerInterface
	} else {
		l.Warn("Shared snapshot handler not available, snapshot policies inventory will be unavailable")
	}

	// Transfer Policy Manager - extract from shared handler
	// May be nil if transfer policy routes haven't been registered yet
	var transferPolicyMgr *autotransfers.Manager
	if sharedTransferPolicyHandler != nil {
		transferPolicyMgr = sharedTransferPolicyHandler.Manager()
	} else {
		l.Warn("Shared transfer policy handler not available, transfer policies inventory will be unavailable")
	}

	// Create inventory collector
	collector := inventory.NewCollector(
		diskMgr,
		poolMgr,
		datasetMgr,
		networkMgr,
		systemMgr,
		sharesMgr,
		snapshotMgr,
		transferPolicyMgr,
		l,
	)

	// Create inventory handler
	inventoryHandler := inventory.NewHandler(collector, l)

	// API group with version
	v1 := engine.Group(constants.APIInventory)
	{
		// Register inventory routes
		inventoryHandler.RegisterRoutes(v1)
	}

	// Register gRPC handlers
	inventory.RegisterInventoryGRPCHandlers(inventoryHandler)

	return inventoryHandler, nil
}
