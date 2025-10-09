package api

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all disk API routes
func (h *DiskHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Inventory routes
	router.GET("/inventory", h.GetInventory)
	router.GET("/disks/:device_id", h.GetDisk)
	router.POST("/discovery/trigger", h.TriggerDiscovery)

	// Health routes
	router.POST("/health/check", h.TriggerHealthCheck)
	router.GET("/disks/:device_id/health", h.GetDiskHealth)
	router.GET("/disks/:device_id/smart", h.GetSMARTData)
}
