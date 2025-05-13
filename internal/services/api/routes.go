// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all service-related routes with the given router group
func (h *ServiceHandler) RegisterRoutes(router *gin.RouterGroup) {
	// List all services
	router.GET("", h.listServices)

	// Get status of all services
	router.GET("/status", h.getAllServiceStatuses)

	// Service-specific operations
	serviceGroup := router.Group("/:name")
	{
		// Runtime operations
		serviceGroup.GET("/status", h.getServiceStatus)
		serviceGroup.POST("/start", h.startService)
		serviceGroup.POST("/stop", h.stopService)
		serviceGroup.POST("/restart", h.restartService)
		
		// Startup management operations
		serviceGroup.GET("/startup", h.getStartupStatus)    // Get enabled/disabled status
		serviceGroup.POST("/enable", h.enableService)       // Enable service to start at boot
		serviceGroup.POST("/disable", h.disableService)     // Disable service from starting at boot
	}
}
