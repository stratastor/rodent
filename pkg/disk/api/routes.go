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
	router.POST("/smart/refresh", h.RefreshSMART)

	// Probe routes
	router.POST("/probes/start", h.StartProbe)
	router.POST("/probes/:probe_id/cancel", h.CancelProbe)
	router.GET("/probes/:probe_id", h.GetProbe)
	router.GET("/probes", h.ListProbes)
	router.GET("/disks/:device_id/probe-history", h.GetProbeHistory)

	// Probe schedule routes
	router.GET("/probe-schedules", h.ListProbeSchedules)
	router.GET("/probe-schedules/:schedule_id", h.GetProbeSchedule)
	router.POST("/probe-schedules", h.CreateProbeSchedule)
	router.PUT("/probe-schedules/:schedule_id", h.UpdateProbeSchedule)
	router.DELETE("/probe-schedules/:schedule_id", h.DeleteProbeSchedule)
	router.POST("/probe-schedules/:schedule_id/enable", h.EnableProbeSchedule)
	router.POST("/probe-schedules/:schedule_id/disable", h.DisableProbeSchedule)

	// Topology routes
	router.GET("/topology", h.GetTopology)
	router.POST("/topology/refresh", h.RefreshTopology)
	router.GET("/topology/controllers", h.GetControllers)
	router.GET("/topology/enclosures", h.GetEnclosures)

	// State management routes
	router.GET("/disks/:device_id/state", h.GetDeviceState)
	router.PUT("/disks/:device_id/state", h.SetDeviceState)
	router.POST("/disks/:device_id/validate", h.ValidateDisk)
	router.POST("/disks/:device_id/quarantine", h.QuarantineDisk)

	// Metadata routes
	router.PUT("/disks/:device_id/tags", h.SetDiskTags)
	router.DELETE("/disks/:device_id/tags", h.DeleteDiskTags)
	router.PUT("/disks/:device_id/notes", h.SetDiskNotes)

	// Statistics routes
	router.GET("/disks/:device_id/statistics", h.GetDeviceStatistics)
	router.GET("/statistics/global", h.GetGlobalStatistics)

	// Monitoring routes
	router.GET("/monitoring/config", h.GetMonitoringConfig)
	router.PUT("/monitoring/config", h.SetMonitoringConfig)

	// Configuration routes
	router.GET("/config", h.GetConfiguration)
	router.PUT("/config", h.UpdateConfiguration)
	router.POST("/config/reload", h.ReloadConfiguration)
}
