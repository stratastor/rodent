// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/netmage/types"
)

// NetworkHandler handles REST API requests for network management
type NetworkHandler struct {
	manager types.Manager
	logger  logger.Logger
}

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

// NewNetworkHandler creates a new network API handler
func NewNetworkHandler(manager types.Manager, logger logger.Logger) *NetworkHandler {
	return &NetworkHandler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes registers the network management routes
func (h *NetworkHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Interface management routes
	interfaces := router.Group("/interfaces")
	{
		interfaces.GET("", h.ListInterfaces)
		interfaces.GET("/:iface_name", h.GetInterface)
		interfaces.PUT("/:iface_name/state", h.SetInterfaceState)
		interfaces.GET("/:iface_name/statistics", h.GetInterfaceStatistics)
	}

	// IP address management routes
	addresses := router.Group("/addresses")
	{
		addresses.POST("", h.AddIPAddress)
		addresses.DELETE("", h.RemoveIPAddress)
		addresses.GET("/:iface_name", h.GetIPAddresses)
	}

	// Route management routes
	routes := router.Group("/routes")
	{
		routes.GET("", h.GetRoutes)
		routes.POST("", h.AddRoute)
		routes.DELETE("", h.RemoveRoute)
	}

	// Netplan configuration routes
	netplan := router.Group("/netplan")
	{
		netplan.GET("/config", h.GetNetplanConfig)
		netplan.PUT("/config", h.SetNetplanConfig)
		netplan.POST("/apply", h.ApplyNetplanConfig)
		netplan.POST("/try", h.TryNetplanConfig)
		netplan.POST("/safe-apply", h.SafeApplyConfig)
		netplan.GET("/status", h.GetNetplanStatus)
		netplan.GET("/status/:iface_name", h.GetNetplanStatusInterface)
		netplan.GET("/diff", h.GetNetplanDiff)
	}

	// Backup and restore routes
	backups := router.Group("/backups")
	{
		backups.GET("", h.ListBackups)
		backups.POST("", h.CreateBackup)
		backups.POST("/:backup_id/restore", h.RestoreBackup)
	}

	// System information routes
	router.GET("/system", h.GetSystemNetworkInfo)

	// Global DNS management routes
	dns := router.Group("/dns")
	{
		dns.GET("/global", h.GetGlobalDNS)
		dns.PUT("/global", h.SetGlobalDNS)
	}

	// Validation routes
	validation := router.Group("/validate")
	{
		validation.POST("/ip", h.ValidateIPAddress)
		validation.POST("/interface-name", h.ValidateInterfaceName)
		validation.POST("/netplan-config", h.ValidateNetplanConfig)
	}
}

// sendSuccess sends a successful response with the standardized format
func (h *NetworkHandler) sendSuccess(c *gin.Context, statusCode int, result interface{}) {
	response := APIResponse{
		Success: true,
		Result:  result,
	}
	c.JSON(statusCode, response)
}

// sendError sends an error response with the standardized format
func (h *NetworkHandler) sendError(c *gin.Context, err error) {
	response := APIResponse{
		Success: false,
	}

	if rodentErr, ok := err.(*errors.RodentError); ok {
		h.logger.Error("Network API error",
			"error", err,
			"code", rodentErr.Code,
			"domain", rodentErr.Domain,
			"path", c.Request.URL.Path)

		response.Error = &APIError{
			Code:    int(rodentErr.Code),
			Domain:  string(rodentErr.Domain),
			Message: rodentErr.Message,
			Details: rodentErr.Details,
		}

		// Add metadata if available
		if len(rodentErr.Metadata) > 0 {
			response.Error.Meta = make(map[string]interface{})
			for k, v := range rodentErr.Metadata {
				response.Error.Meta[k] = v
			}
		}

		c.JSON(rodentErr.HTTPStatus, response)
		return
	}

	// Fallback for non-RodentError
	h.logger.Error("Network API error", "error", err, "path", c.Request.URL.Path)
	response.Error = &APIError{
		Code:    500,
		Domain:  "NETWORK",
		Message: "Internal server error",
		Details: err.Error(),
	}
	c.JSON(http.StatusInternalServerError, response)
}

// ListInterfaces handles GET /interfaces
func (h *NetworkHandler) ListInterfaces(c *gin.Context) {
	ctx := c.Request.Context()

	interfaces, err := h.manager.ListInterfaces(ctx)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"interfaces": interfaces,
		"count":      len(interfaces),
	})
}

// GetInterface handles GET /interfaces/:iface_name
func (h *NetworkHandler) GetInterface(c *gin.Context) {
	name := c.Param("iface_name")
	ctx := c.Request.Context()

	iface, err := h.manager.GetInterface(ctx, name)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, iface)
}

// SetInterfaceState handles PUT /interfaces/:iface_name/state
func (h *NetworkHandler) SetInterfaceState(c *gin.Context) {
	name := c.Param("iface_name")
	ctx := c.Request.Context()

	var req types.InterfaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.SetInterfaceState(ctx, name, req.State); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":   "Interface state updated successfully",
		"interface": name,
		"state":     req.State,
	})
}

// GetInterfaceStatistics handles GET /interfaces/:iface_name/statistics
func (h *NetworkHandler) GetInterfaceStatistics(c *gin.Context) {
	name := c.Param("iface_name")
	ctx := c.Request.Context()

	stats, err := h.manager.GetInterfaceStatistics(ctx, name)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"interface":  name,
		"statistics": stats,
	})
}

// AddIPAddress handles POST /addresses
func (h *NetworkHandler) AddIPAddress(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.AddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.AddIPAddress(ctx, req.Interface, req.Address); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusCreated, map[string]interface{}{
		"message":   "IP address added successfully",
		"interface": req.Interface,
		"address":   req.Address,
	})
}

// RemoveIPAddress handles DELETE /addresses
func (h *NetworkHandler) RemoveIPAddress(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.AddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.RemoveIPAddress(ctx, req.Interface, req.Address); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":   "IP address removed successfully",
		"interface": req.Interface,
		"address":   req.Address,
	})
}

// GetIPAddresses handles GET /addresses/:iface_name
func (h *NetworkHandler) GetIPAddresses(c *gin.Context) {
	iface := c.Param("iface_name")
	ctx := c.Request.Context()

	addresses, err := h.manager.GetIPAddresses(ctx, iface)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"interface": iface,
		"addresses": addresses,
		"count":     len(addresses),
	})
}

// GetRoutes handles GET /routes
func (h *NetworkHandler) GetRoutes(c *gin.Context) {
	table := c.Query("table")
	ctx := c.Request.Context()

	routes, err := h.manager.GetRoutes(ctx, table)
	if err != nil {
		h.sendError(c, err)
		return
	}

	result := map[string]interface{}{
		"routes": routes,
		"count":  len(routes),
	}
	if table != "" {
		result["table"] = table
	}

	h.sendSuccess(c, http.StatusOK, result)
}

// AddRoute handles POST /routes
func (h *NetworkHandler) AddRoute(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.RouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	route := &types.Route{
		To:     req.To,
		Via:    req.Via,
		From:   req.From,
		Device: req.Device,
		Table:  req.Table,
		Metric: req.Metric,
	}

	if err := h.manager.AddRoute(ctx, route); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusCreated, map[string]interface{}{
		"message": "Route added successfully",
		"route":   route,
	})
}

// RemoveRoute handles DELETE /routes
func (h *NetworkHandler) RemoveRoute(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.RouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	route := &types.Route{
		To:     req.To,
		Via:    req.Via,
		From:   req.From,
		Device: req.Device,
		Table:  req.Table,
		Metric: req.Metric,
	}

	if err := h.manager.RemoveRoute(ctx, route); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Route removed successfully",
		"route":   route,
	})
}

// GetNetplanConfig handles GET /netplan/config
func (h *NetworkHandler) GetNetplanConfig(c *gin.Context) {
	ctx := c.Request.Context()

	config, err := h.manager.GetNetplanConfig(ctx)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, config)
}

// SetNetplanConfig handles PUT /netplan/config
func (h *NetworkHandler) SetNetplanConfig(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.NetplanConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	// Create backup if description provided
	var backupID string
	if req.BackupDesc != "" {
		id, err := h.manager.BackupNetplanConfig(ctx)
		if err != nil {
			h.logger.Warn("Failed to create backup before config update", "error", err)
		} else {
			backupID = id
		}
	}

	if err := h.manager.SetNetplanConfig(ctx, req.Config); err != nil {
		h.sendError(c, err)
		return
	}

	result := map[string]interface{}{
		"message": "Netplan configuration updated successfully",
	}
	if backupID != "" {
		result["backup_id"] = backupID
	}

	h.sendSuccess(c, http.StatusOK, result)
}

// ApplyNetplanConfig handles POST /netplan/apply
func (h *NetworkHandler) ApplyNetplanConfig(c *gin.Context) {
	ctx := c.Request.Context()

	if err := h.manager.ApplyNetplanConfig(ctx); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Netplan configuration applied successfully",
	})
}

// TryNetplanConfig handles POST /netplan/try
func (h *NetworkHandler) TryNetplanConfig(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.NetplanTryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	timeout := time.Duration(req.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second // Default timeout
	}

	// If config is provided, set it first
	if req.Config != nil {
		if err := h.manager.SetNetplanConfig(ctx, req.Config); err != nil {
			h.sendError(c, err)
			return
		}
	}

	result, err := h.manager.TryNetplanConfig(ctx, timeout)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, result)
}

// GetNetplanStatus handles GET /netplan/status
func (h *NetworkHandler) GetNetplanStatus(c *gin.Context) {
	ctx := c.Request.Context()

	status, err := h.manager.GetNetplanStatus(ctx, "")
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, status)
}

// GetNetplanStatusInterface handles GET /netplan/status/:iface_name
func (h *NetworkHandler) GetNetplanStatusInterface(c *gin.Context) {
	iface := c.Param("iface_name")
	ctx := c.Request.Context()

	status, err := h.manager.GetNetplanStatus(ctx, iface)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, status)
}

// GetNetplanDiff handles GET /netplan/diff
func (h *NetworkHandler) GetNetplanDiff(c *gin.Context) {
	ctx := c.Request.Context()

	diff, err := h.manager.GetNetplanDiff(ctx)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, diff)
}

// ListBackups handles GET /backups
func (h *NetworkHandler) ListBackups(c *gin.Context) {
	ctx := c.Request.Context()

	backups, err := h.manager.ListBackups(ctx)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"backups": backups,
		"count":   len(backups),
	})
}

// CreateBackup handles POST /backups
func (h *NetworkHandler) CreateBackup(c *gin.Context) {
	ctx := c.Request.Context()

	backupID, err := h.manager.BackupNetplanConfig(ctx)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusCreated, map[string]interface{}{
		"message":   "Backup created successfully",
		"backup_id": backupID,
	})
}

// RestoreBackup handles POST /backups/:backup_id/restore
func (h *NetworkHandler) RestoreBackup(c *gin.Context) {
	backupID := c.Param("backup_id")
	ctx := c.Request.Context()

	if err := h.manager.RestoreNetplanConfig(ctx, backupID); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":   "Configuration restored successfully",
		"backup_id": backupID,
	})
}

// GetSystemNetworkInfo handles GET /system
func (h *NetworkHandler) GetSystemNetworkInfo(c *gin.Context) {
	ctx := c.Request.Context()

	info, err := h.manager.GetSystemNetworkInfo(ctx)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, info)
}

// ValidateIPAddress handles POST /validate/ip
func (h *NetworkHandler) ValidateIPAddress(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.ValidateIPAddress(req.Address); err != nil {
		h.sendSuccess(c, http.StatusOK, map[string]interface{}{
			"valid":   false,
			"error":   err.Error(),
			"address": req.Address,
		})
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"valid":   true,
		"address": req.Address,
	})
}

// ValidateInterfaceName handles POST /validate/interface-name
func (h *NetworkHandler) ValidateInterfaceName(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.ValidateInterfaceName(req.Name); err != nil {
		h.sendSuccess(c, http.StatusOK, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
			"name":  req.Name,
		})
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"valid": true,
		"name":  req.Name,
	})
}

// ValidateNetplanConfig handles POST /validate/netplan-config
func (h *NetworkHandler) ValidateNetplanConfig(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.NetplanConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.ValidateNetplanConfig(ctx, req.Config); err != nil {
		h.sendSuccess(c, http.StatusOK, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"valid": true,
	})
}

// GetGlobalDNS handles GET /dns/global
func (h *NetworkHandler) GetGlobalDNS(c *gin.Context) {
	ctx := c.Request.Context()

	dns, err := h.manager.GetGlobalDNS(ctx)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, dns)
}

// SetGlobalDNS handles PUT /dns/global
func (h *NetworkHandler) SetGlobalDNS(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.GlobalDNSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	dns := &types.NameserverConfig{
		Addresses: req.Addresses,
		Search:    req.Search,
	}

	if err := h.manager.SetGlobalDNS(ctx, dns); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Global DNS configuration updated successfully",
		"dns":     dns,
	})
}

// SafeApplyConfig handles POST /netplan/safe-apply
func (h *NetworkHandler) SafeApplyConfig(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.SafeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	result, err := h.manager.SafeApplyConfig(ctx, req.Config, req.Options)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, result)
}
