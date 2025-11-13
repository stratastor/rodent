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
	"github.com/stratastor/rodent/pkg/system"
)

// SystemHandler handles system management REST API requests
type SystemHandler struct {
	manager *system.Manager
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

// NewSystemHandler creates a new system API handler
func NewSystemHandler(manager *system.Manager, logger logger.Logger) *SystemHandler {
	return &SystemHandler{
		manager: manager,
		logger:  logger,
	}
}

// sendSuccess sends a successful response with the standardized format
func (h *SystemHandler) sendSuccess(c *gin.Context, statusCode int, result interface{}) {
	response := APIResponse{
		Success: true,
		Result:  result,
	}
	c.JSON(statusCode, response)
}

// sendError sends an error response with the standardized format
func (h *SystemHandler) sendError(c *gin.Context, err error) {
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
			Domain:  "SYSTEM",
			Message: "Internal server error",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, response)
	}
}

// RegisterRoutes registers all system API routes
func (h *SystemHandler) RegisterRoutes(router *gin.RouterGroup) {
	// System information routes
	router.GET("/info", h.GetSystemInfo)
	router.GET("/info/os", h.GetOSInfo)
	router.GET("/info/hardware", h.GetHardwareInfo)
	router.GET("/info/performance", h.GetPerformanceInfo)
	router.GET("/health", h.GetSystemHealth)

	// Hostname management routes
	router.GET("/hostname", h.GetHostname)
	router.PUT("/hostname", h.SetHostname)

	// User management routes
	users := router.Group("/users")
	{
		users.GET("", h.GetUsers)
		users.GET("/:username", h.GetUser)
		users.GET("/:username/groups", h.GetUserGroups)
		users.POST("", h.CreateUser)
		users.PUT("/:username", h.UpdateUser)
		users.PUT("/:username/password", h.SetPassword)
		users.PUT("/:username/lock", h.LockUser)
		users.PUT("/:username/unlock", h.UnlockUser)
		users.PUT("/:username/groups/:groupname", h.AddUserToGroup)
		users.DELETE("/:username/groups/:groupname", h.RemoveUserFromGroup)
		users.PUT("/:username/primary-group", h.SetPrimaryGroup)
		users.DELETE("/:username", h.DeleteUser)
	}

	// Group management routes
	groups := router.Group("/groups")
	{
		groups.GET("", h.GetGroups)
		groups.GET("/:groupname", h.GetGroup)
		groups.POST("", h.CreateGroup)
		groups.DELETE("/:groupname", h.DeleteGroup)
	}

	// Power management routes
	power := router.Group("/power")
	{
		power.GET("/status", h.GetPowerStatus)
		power.GET("/scheduled", h.GetScheduledShutdown)
		power.POST("/shutdown", h.Shutdown)
		power.POST("/reboot", h.Reboot)
		power.POST("/schedule-shutdown", h.ScheduleShutdown)
		power.DELETE("/scheduled", h.CancelScheduledShutdown)
	}

	// System configuration routes
	config := router.Group("/config")
	{
		config.GET("/timezone", h.GetTimezone)
		config.PUT("/timezone", h.SetTimezone)
		config.GET("/locale", h.GetLocale)
		config.PUT("/locale", h.SetLocale)
	}
}

// System Information Handlers

func (h *SystemHandler) GetSystemInfo(c *gin.Context) {
	info, err := h.manager.GetSystemInfo(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, info)
}

func (h *SystemHandler) GetOSInfo(c *gin.Context) {
	info, err := h.manager.GetOSInfo(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, info)
}

func (h *SystemHandler) GetHardwareInfo(c *gin.Context) {
	info, err := h.manager.GetHardwareInfo(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, info)
}

func (h *SystemHandler) GetPerformanceInfo(c *gin.Context) {
	info, err := h.manager.GetPerformanceInfo(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, info)
}

func (h *SystemHandler) GetSystemHealth(c *gin.Context) {
	health, err := h.manager.GetSystemHealth(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, health)
}

// Hostname Management Handlers

func (h *SystemHandler) GetHostname(c *gin.Context) {
	hostname, err := h.manager.GetHostname(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"hostname": hostname,
	})
}

func (h *SystemHandler) SetHostname(c *gin.Context) {
	var request system.SetHostnameRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.SetHostname(c.Request.Context(), request); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":  "Hostname set successfully",
		"hostname": request.Hostname,
	})
}

// User Management Handlers

func (h *SystemHandler) GetUsers(c *gin.Context) {
	users, err := h.manager.GetUsers(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"users": users,
		"count": len(users),
	})
}

func (h *SystemHandler) GetUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		h.sendError(c, errors.New(errors.SystemUserInvalidName, "Username parameter is required"))
		return
	}

	user, err := h.manager.GetUser(c.Request.Context(), username)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, user)
}

func (h *SystemHandler) CreateUser(c *gin.Context) {
	var request system.CreateUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.CreateUser(c.Request.Context(), request); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusCreated, map[string]interface{}{
		"message":  "User created successfully",
		"username": request.Username,
	})
}

func (h *SystemHandler) DeleteUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		h.sendError(c, errors.New(errors.SystemUserInvalidName, "Username parameter is required"))
		return
	}

	if err := h.manager.DeleteUser(c.Request.Context(), username); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":  "User deleted successfully",
		"username": username,
	})
}

func (h *SystemHandler) UpdateUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		h.sendError(c, errors.New(errors.SystemUserInvalidName, "Username parameter is required"))
		return
	}

	var request system.UpdateUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	// Override username from URL parameter
	request.Username = username

	if err := h.manager.UpdateUser(c.Request.Context(), request); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":  "User updated successfully",
		"username": username,
	})
}

func (h *SystemHandler) SetPassword(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		h.sendError(c, errors.New(errors.SystemUserInvalidName, "Username parameter is required"))
		return
	}

	var request system.SetPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.SetPassword(c.Request.Context(), username, request.Password); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":  "Password set successfully",
		"username": username,
	})
}

func (h *SystemHandler) LockUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		h.sendError(c, errors.New(errors.SystemUserInvalidName, "Username parameter is required"))
		return
	}

	if err := h.manager.LockUser(c.Request.Context(), username); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":  "User account locked successfully",
		"username": username,
	})
}

func (h *SystemHandler) UnlockUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		h.sendError(c, errors.New(errors.SystemUserInvalidName, "Username parameter is required"))
		return
	}

	if err := h.manager.UnlockUser(c.Request.Context(), username); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":  "User account unlocked successfully",
		"username": username,
	})
}

func (h *SystemHandler) AddUserToGroup(c *gin.Context) {
	username := c.Param("username")
	groupname := c.Param("groupname")

	if username == "" {
		h.sendError(c, errors.New(errors.SystemUserInvalidName, "Username parameter is required"))
		return
	}

	if groupname == "" {
		h.sendError(c, errors.New(errors.SystemGroupInvalidName, "Group name parameter is required"))
		return
	}

	if err := h.manager.AddUserToGroup(c.Request.Context(), username, groupname); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "User added to group successfully",
		"username": username,
		"group":   groupname,
	})
}

func (h *SystemHandler) RemoveUserFromGroup(c *gin.Context) {
	username := c.Param("username")
	groupname := c.Param("groupname")

	if username == "" {
		h.sendError(c, errors.New(errors.SystemUserInvalidName, "Username parameter is required"))
		return
	}

	if groupname == "" {
		h.sendError(c, errors.New(errors.SystemGroupInvalidName, "Group name parameter is required"))
		return
	}

	if err := h.manager.RemoveUserFromGroup(c.Request.Context(), username, groupname); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":  "User removed from group successfully",
		"username": username,
		"group":    groupname,
	})
}

func (h *SystemHandler) SetPrimaryGroup(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		h.sendError(c, errors.New(errors.SystemUserInvalidName, "Username parameter is required"))
		return
	}

	var request system.GroupMembershipRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.SetPrimaryGroup(c.Request.Context(), username, request.GroupName); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":       "Primary group set successfully",
		"username":      username,
		"primary_group": request.GroupName,
	})
}

func (h *SystemHandler) GetUserGroups(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		h.sendError(c, errors.New(errors.SystemUserInvalidName, "Username parameter is required"))
		return
	}

	groups, err := h.manager.GetUserGroups(c.Request.Context(), username)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"username": username,
		"groups":   groups,
		"count":    len(groups),
	})
}

// Group Management Handlers

func (h *SystemHandler) GetGroups(c *gin.Context) {
	groups, err := h.manager.GetGroups(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"groups": groups,
		"count":  len(groups),
	})
}

func (h *SystemHandler) GetGroup(c *gin.Context) {
	groupName := c.Param("groupname")
	if groupName == "" {
		h.sendError(c, errors.New(errors.SystemGroupInvalidName, "Group name parameter is required"))
		return
	}

	group, err := h.manager.GetGroup(c.Request.Context(), groupName)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, group)
}

func (h *SystemHandler) CreateGroup(c *gin.Context) {
	var request system.CreateGroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.CreateGroup(c.Request.Context(), request); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusCreated, map[string]interface{}{
		"message": "Group created successfully",
		"name":    request.Name,
	})
}

func (h *SystemHandler) DeleteGroup(c *gin.Context) {
	groupName := c.Param("groupname")
	if groupName == "" {
		h.sendError(c, errors.New(errors.SystemGroupInvalidName, "Group name parameter is required"))
		return
	}

	if err := h.manager.DeleteGroup(c.Request.Context(), groupName); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Group deleted successfully",
		"name":    groupName,
	})
}

// Power Management Handlers

func (h *SystemHandler) GetPowerStatus(c *gin.Context) {
	status, err := h.manager.GetPowerStatus(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, status)
}

func (h *SystemHandler) GetScheduledShutdown(c *gin.Context) {
	info, err := h.manager.GetScheduledShutdown(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, info)
}

func (h *SystemHandler) Shutdown(c *gin.Context) {
	var request system.PowerOperationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		// Allow empty body for simple shutdown
		request = system.PowerOperationRequest{}
	}

	h.logger.Warn("System shutdown requested via API", "client_ip", c.ClientIP(), "user_agent", c.GetHeader("User-Agent"))

	if err := h.manager.Shutdown(c.Request.Context(), request); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "System shutdown initiated",
	})
}

func (h *SystemHandler) Reboot(c *gin.Context) {
	var request system.PowerOperationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		// Allow empty body for simple reboot
		request = system.PowerOperationRequest{}
	}

	h.logger.Warn("System reboot requested via API", "client_ip", c.ClientIP(), "user_agent", c.GetHeader("User-Agent"))

	if err := h.manager.Reboot(c.Request.Context(), request); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "System reboot initiated",
	})
}

func (h *SystemHandler) ScheduleShutdown(c *gin.Context) {
	var request struct {
		DelayMinutes int    `json:"delay_minutes" binding:"required,min=1"`
		Message      string `json:"message"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	delay := time.Duration(request.DelayMinutes) * time.Minute

	h.logger.Warn("Scheduled shutdown requested via API", "delay_minutes", request.DelayMinutes, "client_ip", c.ClientIP())

	if err := h.manager.ScheduleShutdown(c.Request.Context(), delay, request.Message); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":       "System shutdown scheduled",
		"delay_minutes": request.DelayMinutes,
		"schedule_message": request.Message,
	})
}

func (h *SystemHandler) CancelScheduledShutdown(c *gin.Context) {
	h.logger.Info("Cancelling scheduled shutdown via API", "client_ip", c.ClientIP())

	if err := h.manager.CancelScheduledShutdown(c.Request.Context()); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Scheduled shutdown cancelled",
	})
}

// System Configuration Handlers

func (h *SystemHandler) GetTimezone(c *gin.Context) {
	timezone, err := h.manager.GetTimezone(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"timezone": timezone,
	})
}

func (h *SystemHandler) SetTimezone(c *gin.Context) {
	var request system.SetTimezoneRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.SetTimezone(c.Request.Context(), request); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":  "Timezone set successfully",
		"timezone": request.Timezone,
	})
}

func (h *SystemHandler) GetLocale(c *gin.Context) {
	locale, err := h.manager.GetLocale(c.Request.Context())
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"locale": locale,
	})
}

func (h *SystemHandler) SetLocale(c *gin.Context) {
	var request system.SetLocaleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerRequestValidation))
		return
	}

	if err := h.manager.SetLocale(c.Request.Context(), request); err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message": "Locale set successfully",
		"locale":  request.Locale,
	})
}