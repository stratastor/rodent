// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/internal/services/manager"
)

// ServiceHandler provides HTTP endpoints for service management
type ServiceHandler struct {
	manager *manager.ServiceManager
}

// NewServiceHandler creates a new service handler
func NewServiceHandler(manager *manager.ServiceManager) *ServiceHandler {
	return &ServiceHandler{
		manager: manager,
	}
}

// listServices returns a list of all available services
func (h *ServiceHandler) listServices(c *gin.Context) {
	services := h.manager.ListServices()
	c.JSON(http.StatusOK, gin.H{
		"services": services,
	})
}

// getAllServiceStatuses returns the status of all services
func (h *ServiceHandler) getAllServiceStatuses(c *gin.Context) {
	statuses := h.manager.GetAllServiceStatuses(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"statuses": statuses,
	})
}

// getServiceStatus returns the status of a specific service
func (h *ServiceHandler) getServiceStatus(c *gin.Context) {
	name := c.Param("name")

	svc, ok := h.manager.GetService(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "service not found",
		})
		return
	}

	status, err := svc.Status(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":   name,
		"status": status,
	})
}

// startService starts a specific service
func (h *ServiceHandler) startService(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.StartService(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "service started successfully",
	})
}

// stopService stops a specific service
func (h *ServiceHandler) stopService(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.StopService(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "service stopped successfully",
	})
}

// restartService restarts a specific service
func (h *ServiceHandler) restartService(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.RestartService(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "service restarted successfully",
	})
}

// Close cleans up resources used by the service handler
func (h *ServiceHandler) Close() error {
	return h.manager.Close()
}
