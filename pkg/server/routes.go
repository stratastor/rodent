package server

import (
	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/zfs/api"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

func registerRoutes(engine *gin.Engine) {
	// Create command executor with sudo support
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "warn"})

	// Initialize managers
	datasetManager := dataset.NewManager(executor)
	poolManager := pool.NewManager(executor)

	// Create API handlers
	datasetHandler := api.NewDatasetHandler(datasetManager)
	poolHandler := api.NewPoolHandler(poolManager)

	// API group with version
	v1 := engine.Group("/api/v1")
	{
		// Register ZFS routes
		datasetHandler.RegisterRoutes(v1)
		poolHandler.RegisterRoutes(v1)

		// Health check routes
		// v1.GET("/health", healthCheck)
	}
}
