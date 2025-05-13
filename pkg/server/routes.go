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
	svcAPI "github.com/stratastor/rodent/internal/services/api"
	svcManager "github.com/stratastor/rodent/internal/services/manager"
	"github.com/stratastor/rodent/pkg/ad"
	"github.com/stratastor/rodent/pkg/ad/handlers"
	"github.com/stratastor/rodent/pkg/facl"
	aclAPI "github.com/stratastor/rodent/pkg/facl/api"
	sshAPI "github.com/stratastor/rodent/pkg/keys/ssh/api"
	sharesAPI "github.com/stratastor/rodent/pkg/shares/api"
	"github.com/stratastor/rodent/pkg/shares/smb"
	"github.com/stratastor/rodent/pkg/zfs/api"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

func registerZFSRoutes(engine *gin.Engine) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	cfg := config.GetConfig()
	// Create command executor with sudo support
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: cfg.Server.LogLevel})

	// Initialize managers
	datasetManager := dataset.NewManager(executor)
	poolManager := pool.NewManager(executor)

	// Create API handlers
	datasetHandler := api.NewDatasetHandler(datasetManager)
	poolHandler := api.NewPoolHandler(poolManager)

	// API group with version
	v1 := engine.Group(constants.APIZFS)
	{
		// Register ZFS routes
		datasetHandler.RegisterRoutes(v1)
		poolHandler.RegisterRoutes(v1)

		schedulers := v1.Group("/schedulers")
		{
			// Register auto-snapshot routes
			_ = api.RegisterAutoSnapshotRoutes(schedulers, datasetManager)
		}

		// Health check routes
		// v1.GET("/health", healthCheck)
	}
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
