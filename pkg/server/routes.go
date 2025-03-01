// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/constants"
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

func registerADRoutes(engine *gin.Engine) {
	// Add error handler middleware
	engine.Use(ErrorHandler())

	// Create AD handler
	adHandler, err := handlers.NewADHandler()
	if err != nil {
		// Log the error but don't fail startup
		// TODO: Handle this differently?
		log.Printf("Failed to initialize AD handler: %v", err)
		return
	}

	// Register cleanup on server shutdown
	defer adHandler.Close()

	// API group with version
	v1 := engine.Group(constants.APIAD)
	{
		// Register AD routes
		adHandler.RegisterRoutes(v1)
	}
}
