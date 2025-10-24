// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// APIResponse represents a standardized API response format
type APIResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents error information in API responses
type APIError struct {
	Code    int                    `json:"code"`
	Domain  string                 `json:"domain"`
	Message string                 `json:"message"`
	Details string                 `json:"details,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// DiskHandler handles disk management gRPC and REST API requests
type DiskHandler struct {
	manager *disk.Manager
	logger  logger.Logger
}

// NewDiskHandler creates a new disk API handler
func NewDiskHandler(manager *disk.Manager, logger logger.Logger) *DiskHandler {
	return &DiskHandler{
		manager: manager,
		logger:  logger,
	}
}

// sendSuccess sends a successful response with the standardized format
func (h *DiskHandler) sendSuccess(c *gin.Context, statusCode int, result interface{}) {
	response := APIResponse{
		Success: true,
		Result:  result,
	}
	c.JSON(statusCode, response)
}

// sendError sends an error response with the standardized format
func (h *DiskHandler) sendError(c *gin.Context, err error) {
	response := APIResponse{
		Success: false,
	}

	if rodentErr, ok := err.(*errors.RodentError); ok {
		response.Error = &APIError{
			Code:    int(rodentErr.Code),
			Domain:  string(rodentErr.Domain),
			Message: rodentErr.Message,
			Details: rodentErr.Details,
			Meta:    make(map[string]interface{}),
		}

		// Add metadata if available
		if rodentErr.Metadata != nil {
			for k, v := range rodentErr.Metadata {
				response.Error.Meta[k] = v
			}
		}

		c.JSON(rodentErr.HTTPStatus, response)
	} else {
		// Generic error
		response.Error = &APIError{
			Code:    http.StatusInternalServerError,
			Domain:  "DISK",
			Message: "Internal server error",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, response)
	}
}

// Inventory handlers

func (h *DiskHandler) GetInventory(c *gin.Context) {
	// Parse optional filter from query params
	var filter *types.DiskFilter

	// Check if any filter params are provided
	if c.Request.URL.Query().Encode() != "" {
		filter = &types.DiskFilter{}

		// Parse state filter (comma-separated)
		if states := c.Query("states"); states != "" {
			filter.States = []types.DiskState{}
			for _, s := range strings.Split(states, ",") {
				filter.States = append(filter.States, types.DiskState(strings.TrimSpace(s)))
			}
		}

		// Parse pool_name filter
		if poolName := c.Query("pool_name"); poolName != "" {
			filter.PoolName = poolName
		}

		// Parse available filter
		if availableStr := c.Query("available"); availableStr != "" {
			available := availableStr == "true"
			filter.Available = &available
		}

		// Parse min_size filter
		if minSizeStr := c.Query("min_size"); minSizeStr != "" {
			if minSize, err := strconv.ParseUint(minSizeStr, 10, 64); err == nil {
				filter.MinSize = minSize
			}
		}

		// Parse max_size filter
		if maxSizeStr := c.Query("max_size"); maxSizeStr != "" {
			if maxSize, err := strconv.ParseUint(maxSizeStr, 10, 64); err == nil {
				filter.MaxSize = maxSize
			}
		}
	}

	disks := h.manager.GetInventory(filter)

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"disks": disks,
		"count": len(disks),
	})
}

func (h *DiskHandler) GetDisk(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	disk, err := h.manager.GetDisk(deviceID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, disk)
}

func (h *DiskHandler) TriggerDiscovery(c *gin.Context) {
	if err := h.manager.TriggerDiscovery(c.Request.Context()); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Disk discovery triggered",
	})
}

// Health handlers

func (h *DiskHandler) TriggerHealthCheck(c *gin.Context) {
	if err := h.manager.TriggerHealthCheck(c.Request.Context()); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Health check triggered",
	})
}

func (h *DiskHandler) GetDiskHealth(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	disk, err := h.manager.GetDisk(deviceID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	healthData := map[string]interface{}{
		"device_id":     disk.DeviceID,
		"health_status": disk.Health,
		"health_reason": disk.HealthReason,
		"smart_status":  "",
		"updated_at":    disk.UpdatedAt,
	}

	if disk.SMARTInfo != nil {
		healthData["smart_status"] = disk.SMARTInfo.OverallStatus
		healthData["temperature"] = disk.SMARTInfo.Temperature
	}

	h.sendSuccess(c, http.StatusOK, healthData)
}

func (h *DiskHandler) GetSMARTData(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	disk, err := h.manager.GetDisk(deviceID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	if disk.SMARTInfo == nil {
		h.sendError(
			c,
			errors.New(errors.DiskSMARTNotAvailable, "SMART data not available for this device"),
		)
		return
	}

	h.sendSuccess(c, http.StatusOK, disk.SMARTInfo)
}

func (h *DiskHandler) RefreshSMART(c *gin.Context) {
	if err := h.manager.TriggerHealthCheck(c.Request.Context()); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "SMART data refresh triggered",
	})
}

// Probe handlers

func (h *DiskHandler) StartProbe(c *gin.Context) {
	var request struct {
		DeviceID  string `json:"device_id" binding:"required"`
		ProbeType string `json:"probe_type,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	probeType := types.ProbeType(request.ProbeType)
	if probeType == "" {
		probeType = types.ProbeTypeQuick
	}

	probeID, err := h.manager.TriggerProbe(c.Request.Context(), request.DeviceID, probeType)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"probe_id": probeID,
	})
}

func (h *DiskHandler) CancelProbe(c *gin.Context) {
	probeID := c.Param("probe_id")
	if probeID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "probe_id is required"))
		return
	}

	if err := h.manager.CancelProbe(probeID); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Probe cancelled",
	})
}

func (h *DiskHandler) GetProbe(c *gin.Context) {
	probeID := c.Param("probe_id")
	if probeID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "probe_id is required"))
		return
	}

	probe, err := h.manager.GetProbeExecution(probeID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, probe)
}

func (h *DiskHandler) ListProbes(c *gin.Context) {
	probes := h.manager.GetActiveProbes()
	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"probes": probes,
		"count":  len(probes),
	})
}

func (h *DiskHandler) GetProbeHistory(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	limit := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	history, err := h.manager.GetProbeHistory(deviceID, limit)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// Probe schedule handlers

func (h *DiskHandler) ListProbeSchedules(c *gin.Context) {
	schedules := h.manager.GetProbeSchedules()
	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"schedules": schedules,
		"count":     len(schedules),
	})
}

func (h *DiskHandler) GetProbeSchedule(c *gin.Context) {
	scheduleID := c.Param("schedule_id")
	if scheduleID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "schedule_id is required"))
		return
	}

	schedule, err := h.manager.GetProbeSchedule(scheduleID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, schedule)
}

func (h *DiskHandler) CreateProbeSchedule(c *gin.Context) {
	var schedule types.ProbeSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		h.sendError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.CreateProbeSchedule(&schedule); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusCreated, schedule)
}

func (h *DiskHandler) UpdateProbeSchedule(c *gin.Context) {
	scheduleID := c.Param("schedule_id")
	if scheduleID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "schedule_id is required"))
		return
	}

	var schedule types.ProbeSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		h.sendError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.UpdateProbeSchedule(scheduleID, &schedule); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Probe schedule updated",
	})
}

func (h *DiskHandler) DeleteProbeSchedule(c *gin.Context) {
	scheduleID := c.Param("schedule_id")
	if scheduleID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "schedule_id is required"))
		return
	}

	if err := h.manager.DeleteProbeSchedule(scheduleID); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Probe schedule deleted",
	})
}

func (h *DiskHandler) EnableProbeSchedule(c *gin.Context) {
	scheduleID := c.Param("schedule_id")
	if scheduleID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "schedule_id is required"))
		return
	}

	if err := h.manager.EnableProbeSchedule(scheduleID); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Probe schedule enabled",
	})
}

func (h *DiskHandler) DisableProbeSchedule(c *gin.Context) {
	scheduleID := c.Param("schedule_id")
	if scheduleID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "schedule_id is required"))
		return
	}

	if err := h.manager.DisableProbeSchedule(scheduleID); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Probe schedule disabled",
	})
}

// Topology handlers

func (h *DiskHandler) GetTopology(c *gin.Context) {
	topology, err := h.manager.GetTopology()
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, topology)
}

func (h *DiskHandler) RefreshTopology(c *gin.Context) {
	if err := h.manager.RefreshTopology(c.Request.Context()); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Topology refresh triggered",
	})
}

func (h *DiskHandler) GetControllers(c *gin.Context) {
	controllers, err := h.manager.GetControllers()
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"controllers": controllers,
		"count":       len(controllers),
	})
}

func (h *DiskHandler) GetEnclosures(c *gin.Context) {
	enclosures, err := h.manager.GetEnclosures()
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"enclosures": enclosures,
		"count":      len(enclosures),
	})
}

// State management handlers

func (h *DiskHandler) GetDeviceState(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	deviceState, err := h.manager.GetDeviceState(deviceID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, deviceState)
}

func (h *DiskHandler) SetDeviceState(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	var request struct {
		State  string `json:"state" binding:"required"`
		Reason string `json:"reason,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	diskState := types.DiskState(request.State)
	if err := h.manager.SetDiskState(deviceID, diskState, request.Reason); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Device state updated",
	})
}

func (h *DiskHandler) ValidateDisk(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	result, err := h.manager.ValidateDisk(deviceID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, result)
}

func (h *DiskHandler) QuarantineDisk(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	var request struct {
		Reason string `json:"reason,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.QuarantineDisk(deviceID, request.Reason); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Disk quarantined",
	})
}

// Metadata handlers

func (h *DiskHandler) SetDiskTags(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	var request struct {
		Tags map[string]string `json:"tags" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.SetDiskTags(deviceID, request.Tags); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Disk tags updated",
	})
}

func (h *DiskHandler) DeleteDiskTags(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	var request struct {
		TagKeys []string `json:"tag_keys" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.DeleteDiskTags(deviceID, request.TagKeys); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Disk tags deleted",
	})
}

func (h *DiskHandler) SetDiskNotes(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	var request struct {
		Notes string `json:"notes"` // Allow empty string for cleanup
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.SetDiskNotes(deviceID, request.Notes); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Disk notes updated",
	})
}

// Statistics handlers

func (h *DiskHandler) GetDeviceStatistics(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		h.sendError(c, errors.New(errors.ServerRequestValidation, "device_id is required"))
		return
	}

	stats, err := h.manager.GetDeviceStatistics(deviceID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, stats)
}

func (h *DiskHandler) GetGlobalStatistics(c *gin.Context) {
	stats := h.manager.GetStatistics()
	h.sendSuccess(c, http.StatusOK, stats)
}

// Monitoring handlers

func (h *DiskHandler) GetMonitoringConfig(c *gin.Context) {
	config := h.manager.GetMonitoringConfig()
	h.sendSuccess(c, http.StatusOK, config)
}

func (h *DiskHandler) SetMonitoringConfig(c *gin.Context) {
	var config types.MonitoringConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		h.sendError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.SetMonitoringConfig(&config); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Monitoring configuration updated",
	})
}

// Configuration handlers

func (h *DiskHandler) GetConfiguration(c *gin.Context) {
	config := h.manager.GetConfig()
	h.sendSuccess(c, http.StatusOK, config)
}

func (h *DiskHandler) UpdateConfiguration(c *gin.Context) {
	var config types.DiskManagerConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		h.sendError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.UpdateConfig(&config); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Configuration updated",
	})
}

func (h *DiskHandler) ReloadConfiguration(c *gin.Context) {
	if err := h.manager.ReloadConfig(); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Configuration reloaded",
	})
}
