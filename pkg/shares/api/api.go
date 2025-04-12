// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/shares"
	"github.com/stratastor/rodent/pkg/shares/smb"
)

// SharesHandler handles HTTP requests for shares
type SharesHandler struct {
	logger     logger.Logger
	smbManager *smb.Manager
	smbService *smb.ServiceManager
}

// NewSharesHandler creates a new shares handler
func NewSharesHandler(
	logger logger.Logger,
	smbManager *smb.Manager,
	smbService *smb.ServiceManager,
) *SharesHandler {
	return &SharesHandler{
		logger:     logger,
		smbManager: smbManager,
		smbService: smbService,
	}
}

// RegisterRoutes registers routes for the shares API
func (h *SharesHandler) RegisterRoutes(router *gin.RouterGroup) {
	sharesAPI := router.Group("")
	{
		// SMB specific operations
		smb := sharesAPI.Group("/smb")
		{
			smb.GET("", h.listSMBShares)
			smb.GET("/:name", ValidateShareName(), h.getSMBShare)
			smb.POST("", ValidateSMBShareConfig(), h.createSMBShare)
			smb.PUT("/:name", ValidateShareName(), ValidateSMBShareConfig(), h.updateSMBShare)
			smb.DELETE("/:name", ValidateShareName(), h.deleteSMBShare)
			smb.GET("/:name/stats", ValidateShareName(), h.getSMBStats)

			// Global SMB config
			smb.GET("/global", h.getSMBGlobalConfig)
			smb.PUT("/global", ValidateSMBGlobalConfig(), h.updateSMBGlobalConfig)

			// Bulk operations
			smb.PUT("/bulk-update", ValidateSMBBulkUpdateConfig(), h.bulkUpdateSMBShares)

			// Service operations
			smb.GET("/service/status", h.getSMBServiceStatus)
			smb.POST("/service/start", h.startSMBService)
			smb.POST("/service/stop", h.stopSMBService)
			smb.POST("/service/restart", h.restartSMBService)
			smb.POST("/service/reload", h.reloadSMBService)
		}

		// NFS and iSCSI can be added similarly when implementing them
	}
}

var APIError = common.APIError

// ValidateShareName validates share name format
func ValidateShareName() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			APIError(c, errors.New(errors.SharesInvalidInput, "Share name cannot be empty"))
			return
		}

		// Use the same regex as in the manager for consistency
		if !smb.GetShareNameRegex().MatchString(name) {
			APIError(c, errors.New(errors.SharesInvalidInput, "Invalid share name format").
				WithMetadata("name", name))
			return
		}

		c.Next()
	}
}

// ValidateSMBShareConfig validates SMB share configuration
func ValidateSMBShareConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var config smb.SMBShareConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			APIError(
				c,
				errors.New(
					errors.ServerRequestValidation,
					"Invalid SMB share configuration: "+err.Error(),
				),
			)
			return
		}

		// Additional validation if needed
		if config.Name == "" {
			APIError(c, errors.New(errors.SharesInvalidInput, "Share name cannot be empty"))
			return
		}

		// Path must be absolute
		if !strings.HasPrefix(config.Path, "/") {
			APIError(c, errors.New(errors.SharesInvalidInput, "Path must be absolute").
				WithMetadata("path", config.Path))
			return
		}

		// Path should be clean and normalized
		cleanPath := filepath.Clean(config.Path)
		if config.Path != cleanPath {
			APIError(c, errors.New(errors.SharesInvalidInput, "Path must be clean and normalized").
				WithMetadata("path", config.Path).
				WithMetadata("clean_path", cleanPath))
			return
		}

		c.Set("smbConfig", config)
		c.Next()
	}
}

// ValidateSMBGlobalConfig validates global SMB configuration
func ValidateSMBGlobalConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var config smb.SMBGlobalConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			APIError(
				c,
				errors.New(
					errors.ServerRequestValidation,
					"Invalid SMB global configuration: "+err.Error(),
				),
			)
			return
		}

		// Basic validation
		if config.WorkGroup == "" {
			APIError(c, errors.New(errors.SharesInvalidInput, "Workgroup cannot be empty"))
			return
		}

		if config.SecurityMode == "" {
			APIError(c, errors.New(errors.SharesInvalidInput, "Security mode cannot be empty"))
			return
		}

		c.Set("smbGlobalConfig", config)
		c.Next()
	}
}

// ValidateSMBBulkUpdateConfig validates bulk update configuration
func ValidateSMBBulkUpdateConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var config smb.SMBBulkUpdateConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			APIError(
				c,
				errors.New(
					errors.ServerRequestValidation,
					"Invalid SMB bulk update configuration: "+err.Error(),
				),
			)
			return
		}

		// At least one parameter must be specified
		if len(config.Parameters) == 0 {
			APIError(
				c,
				errors.New(
					errors.SharesInvalidInput,
					"At least one parameter must be specified for bulk update",
				),
			)
			return
		}

		c.Set("smbBulkConfig", config)
		c.Next()
	}
}

// deleteSMBShare deletes a share
func (h *SharesHandler) deleteSMBShare(c *gin.Context) {
	name := c.Param("name")

	if err := h.smbManager.DeleteShare(c.Request.Context(), name); err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// listSMBShares lists all SMB shares
func (h *SharesHandler) listSMBShares(c *gin.Context) {
	result, err := h.smbManager.ListSharesByType(c.Request.Context(), shares.ShareTypeSMB)
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"shares": result,
		"count":  len(result),
	})
}

// getSMBShare gets an SMB share by name
func (h *SharesHandler) getSMBShare(c *gin.Context) {
	name := c.Param("name")

	share, err := h.smbManager.GetSMBShare(c.Request.Context(), name)
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, share)
}

func (h *SharesHandler) createSMBShare(c *gin.Context) {
	config, exists := c.Get("smbConfig")
	if !exists {
		APIError(
			c,
			errors.New(errors.ServerInternalError, "SMB configuration not found in context"),
		)
		return
	}

	var smbConfig smb.SMBShareConfig
	if rawConfig, ok := config.(smb.SMBShareConfig); ok {
		// Create a new config with defaults
		defaultConfig := smb.NewSMBShareConfig(rawConfig.Name, rawConfig.Path)

		// Override with user-provided values where specified
		if rawConfig.Description != "" {
			defaultConfig.Description = rawConfig.Description
		}

		// Copy non-empty slices
		if len(rawConfig.ValidUsers) > 0 {
			defaultConfig.ValidUsers = rawConfig.ValidUsers
		}
		if len(rawConfig.InvalidUsers) > 0 {
			defaultConfig.InvalidUsers = rawConfig.InvalidUsers
		}
		if len(rawConfig.ReadList) > 0 {
			defaultConfig.ReadList = rawConfig.ReadList
		}
		if len(rawConfig.WriteList) > 0 {
			defaultConfig.WriteList = rawConfig.WriteList
		}

		// Copy user-provided tags
		if len(rawConfig.Tags) > 0 {
			for k, v := range rawConfig.Tags {
				defaultConfig.Tags[k] = v
			}
		}

		// Copy user-provided custom parameters
		if len(rawConfig.CustomParameters) > 0 {
			for k, v := range rawConfig.CustomParameters {
				defaultConfig.CustomParameters[k] = v
			}
		}

		// Use explicitly set booleans
		defaultConfig.ReadOnly = rawConfig.ReadOnly
		defaultConfig.Browsable = rawConfig.Browsable
		defaultConfig.GuestOk = rawConfig.GuestOk
		defaultConfig.Public = rawConfig.Public
		defaultConfig.InheritACLs = rawConfig.InheritACLs
		defaultConfig.MapACLInherit = rawConfig.MapACLInherit
		defaultConfig.FollowSymlinks = rawConfig.FollowSymlinks

		smbConfig = *defaultConfig
	} else {
		smbConfig = config.(smb.SMBShareConfig)
	}

	if err := h.smbManager.CreateShare(c.Request.Context(), &smbConfig); err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Share created successfully",
		"name":    smbConfig.Name,
	})
}

// updateSMBShare updates an existing SMB share
func (h *SharesHandler) updateSMBShare(c *gin.Context) {
	name := c.Param("name")

	config, exists := c.Get("smbConfig")
	if !exists {
		APIError(
			c,
			errors.New(errors.ServerInternalError, "SMB configuration not found in context"),
		)
		return
	}

	smbConfig := config.(smb.SMBShareConfig)

	// Ensure name consistency
	if name != smbConfig.Name {
		APIError(
			c,
			errors.New(errors.SharesInvalidInput, "Share name in URL does not match name in config").
				WithMetadata("url_name", name).
				WithMetadata("config_name", smbConfig.Name),
		)
		return
	}

	if err := h.smbManager.UpdateShare(c.Request.Context(), name, &smbConfig); err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Share updated successfully",
		"name":    name,
	})
}

// getSMBStats gets statistics for an SMB share
func (h *SharesHandler) getSMBStats(c *gin.Context) {
	name := c.Param("name")

	detailed := c.Query("detailed") == "true"

	if detailed {
		stats, err := h.smbManager.GetSMBShareStats(c.Request.Context(), name)
		if err != nil {
			APIError(c, err)
			return
		}

		c.JSON(http.StatusOK, stats)
		return
	}

	// Get simple stats
	stats, err := h.smbManager.GetShareStats(c.Request.Context(), name)
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// getSMBGlobalConfig gets the global SMB configuration
func (h *SharesHandler) getSMBGlobalConfig(c *gin.Context) {
	config, err := h.smbManager.GetGlobalConfig(c.Request.Context())
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, config)
}

// updateSMBGlobalConfig updates the global SMB configuration
func (h *SharesHandler) updateSMBGlobalConfig(c *gin.Context) {
	config, exists := c.Get("smbGlobalConfig")
	if !exists {
		APIError(
			c,
			errors.New(errors.ServerInternalError, "SMB global configuration not found in context"),
		)
		return
	}

	smbGlobalConfig := config.(smb.SMBGlobalConfig)

	if err := h.smbManager.UpdateGlobalConfig(c.Request.Context(), &smbGlobalConfig); err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Global SMB configuration updated successfully",
	})
}

// bulkUpdateSMBShares updates multiple SMB shares with the same parameters
func (h *SharesHandler) bulkUpdateSMBShares(c *gin.Context) {
	config, exists := c.Get("smbBulkConfig")
	if !exists {
		APIError(
			c,
			errors.New(
				errors.ServerInternalError,
				"SMB bulk update configuration not found in context",
			),
		)
		return
	}

	bulkConfig := config.(smb.SMBBulkUpdateConfig)

	// Process the bulk update
	results, err := h.smbManager.BulkUpdateShares(c.Request.Context(), bulkConfig)
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Bulk update completed",
		"results": results,
	})
}

// getSMBServiceStatus gets the status of the SMB service
func (h *SharesHandler) getSMBServiceStatus(c *gin.Context) {
	status, err := h.smbManager.GetSMBServiceStatus(c.Request.Context())
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, status)
}

// startSMBService starts the SMB service
func (h *SharesHandler) startSMBService(c *gin.Context) {
	if err := h.smbService.Start(c.Request.Context()); err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "SMB service started successfully",
	})
}

// stopSMBService stops the SMB service
func (h *SharesHandler) stopSMBService(c *gin.Context) {
	if err := h.smbService.Stop(c.Request.Context()); err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "SMB service stopped successfully",
	})
}

// restartSMBService restarts the SMB service
func (h *SharesHandler) restartSMBService(c *gin.Context) {
	if err := h.smbService.Restart(c.Request.Context()); err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "SMB service restarted successfully",
	})
}

// reloadSMBService reloads the SMB service configuration
func (h *SharesHandler) reloadSMBService(c *gin.Context) {
	if err := h.smbService.ReloadConfig(c.Request.Context()); err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "SMB service configuration reloaded successfully",
	})
}
