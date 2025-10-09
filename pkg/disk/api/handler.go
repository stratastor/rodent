// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk"
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
	// TODO: Parse optional filter from query params
	disks := h.manager.GetInventory(nil)

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
