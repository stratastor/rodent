package api

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all disk API routes
func (h *DiskHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Disk collection and resource routes
	router.GET("/", h.GetInventory)
	router.GET("/available", h.GetAvailableDisks)
	router.GET("/:device_id", h.GetDisk)
	router.GET("/:device_id/health", h.GetDiskHealth)
	router.GET("/:device_id/smart", h.GetSMARTData)
	router.GET("/:device_id/state", h.GetDeviceState)
	router.PUT("/:device_id/state", h.SetDeviceState)
	router.PUT("/:device_id/tags", h.SetDiskTags)
	router.DELETE("/:device_id/tags", h.DeleteDiskTags)
	router.PUT("/:device_id/notes", h.SetDiskNotes)
	router.GET("/:device_id/statistics", h.GetDeviceStatistics)
	router.POST("/:device_id/validate", h.ValidateDisk)
	router.POST("/:device_id/quarantine", h.QuarantineDisk)
	router.GET("/:device_id/probes/history", h.GetProbeHistory)

	// Discovery routes
	router.POST("/discovery/trigger", h.TriggerDiscovery)

	// Health routes
	router.POST("/health/check", h.TriggerHealthCheck)
	router.POST("/smart/refresh", h.RefreshSMART)

	// Probe routes
	probes := router.Group("/probes")
	{
		probes.GET("", h.ListProbes)
		probes.POST("/start", h.StartProbe)
		probes.GET("/:probe_id", h.GetProbe)
		probes.POST("/:probe_id/cancel", h.CancelProbe)

		// Probe schedules
		schedules := probes.Group("/schedules")
		{
			schedules.GET("", h.ListProbeSchedules)
			schedules.POST("", h.CreateProbeSchedule)
			schedules.GET("/:schedule_id", h.GetProbeSchedule)
			schedules.PUT("/:schedule_id", h.UpdateProbeSchedule)
			schedules.DELETE("/:schedule_id", h.DeleteProbeSchedule)
			schedules.POST("/:schedule_id/enable", h.EnableProbeSchedule)
			schedules.POST("/:schedule_id/disable", h.DisableProbeSchedule)
		}
	}

	// Topology routes
	topology := router.Group("/topology")
	{
		topology.GET("", h.GetTopology)
		topology.POST("/refresh", h.RefreshTopology)
		topology.GET("/controllers", h.GetControllers)
		topology.GET("/enclosures", h.GetEnclosures)
	}

	// Statistics routes
	statistics := router.Group("/statistics")
	{
		statistics.GET("/global", h.GetGlobalStatistics)
	}

	// Configuration routes
	config := router.Group("/config")
	{
		config.GET("", h.GetConfiguration)
		config.PUT("", h.UpdateConfiguration)
		config.POST("/reload", h.ReloadConfiguration)
		config.GET("/monitoring", h.GetMonitoringConfig)
		config.PUT("/monitoring", h.SetMonitoringConfig)
	}
}
