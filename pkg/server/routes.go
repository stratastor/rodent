// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/constants"
	svcAPI "github.com/stratastor/rodent/internal/services/api"
	svcManager "github.com/stratastor/rodent/internal/services/manager"
	"github.com/stratastor/rodent/pkg/ad/handlers"
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
