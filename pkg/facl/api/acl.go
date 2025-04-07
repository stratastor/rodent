// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/facl"
)

var APIError = common.APIError

// ACLHandler handles HTTP requests for filesystem ACLs
type ACLHandler struct {
	manager *facl.ACLManager
	logger  logger.Logger
}

func (h *ACLHandler) Close() {
	h.manager.Close()
}

// NewACLHandler creates a new ACL handler
func NewACLHandler(manager *facl.ACLManager, logger logger.Logger) *ACLHandler {
	return &ACLHandler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes registers ACL API routes
func (h *ACLHandler) RegisterRoutes(router *gin.RouterGroup) {
	aclGroup := router.Group("")

	// Apply common middleware
	aclGroup.Use(ValidatePathParam())

	// ACL endpoints (RESTful)
	aclGroup.GET("/*path", h.getACL)       // Get ACLs for a path
	aclGroup.PUT("/*path", h.setACL)       // Set/replace ACLs for a path
	aclGroup.PATCH("/*path", h.modifyACL)  // Modify ACLs for a path
	aclGroup.DELETE("/*path", h.removeACL) // Remove ACLs for a path
}

// getACL handles GET requests to retrieve ACLs
func (h *ACLHandler) getACL(c *gin.Context) {
	fsPath := getDecodedPath(c)
	if fsPath == "" {
		APIError(c, errors.New(errors.FACLInvalidInput, "Path cannot be empty"))
		return
	}

	// Parse query parameters
	recursive := c.Query("recursive") == "true"

	config := facl.ACLListConfig{
		Path:      fsPath,
		Recursive: recursive,
	}

	result, err := h.manager.GetACL(c.Request.Context(), config)
	if err != nil {
		APIError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": result})
}

// setACL handles PUT requests to set/replace ACLs
func (h *ACLHandler) setACL(c *gin.Context) {
	fsPath := getDecodedPath(c)
	if fsPath == "" {
		APIError(c, errors.New(errors.FACLInvalidInput, "Path cannot be empty"))
		return
	}

	var req struct {
		Type      facl.ACLType    `json:"type" binding:"required"`
		Entries   []facl.ACLEntry `json:"entries" binding:"required"`
		Recursive bool            `json:"recursive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	// Validate and resolve AD users/groups
	entries, err := h.manager.ResolveADUsers(c.Request.Context(), req.Entries)
	if err != nil {
		APIError(c, err)
		return
	}

	config := facl.ACLConfig{
		Path:      fsPath,
		Type:      req.Type,
		Entries:   entries,
		Recursive: req.Recursive,
	}

	err = h.manager.SetACL(c.Request.Context(), config)
	if err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

// modifyACL handles PATCH requests to modify ACLs
func (h *ACLHandler) modifyACL(c *gin.Context) {
	fsPath := getDecodedPath(c)
	if fsPath == "" {
		APIError(c, errors.New(errors.FACLInvalidInput, "Path cannot be empty"))
		return
	}

	var req struct {
		Type      facl.ACLType    `json:"type" binding:"required"`
		Entries   []facl.ACLEntry `json:"entries" binding:"required"`
		Recursive bool            `json:"recursive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	// Validate and resolve AD users/groups
	entries, err := h.manager.ResolveADUsers(c.Request.Context(), req.Entries)
	if err != nil {
		APIError(c, err)
		return
	}

	config := facl.ACLConfig{
		Path:      fsPath,
		Type:      req.Type,
		Entries:   entries,
		Recursive: req.Recursive,
	}

	err = h.manager.ModifyACL(c.Request.Context(), config)
	if err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

// removeACL handles DELETE requests to remove ACLs
func (h *ACLHandler) removeACL(c *gin.Context) {
	fsPath := getDecodedPath(c)
	if fsPath == "" {
		APIError(c, errors.New(errors.FACLInvalidInput, "Path cannot be empty"))
		return
	}

	var req struct {
		Type      facl.ACLType    `json:"type" binding:"required"`
		Entries   []facl.ACLEntry `json:"entries" binding:"required"`
		Recursive bool            `json:"recursive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	// Validate and resolve AD users/groups
	entries, err := h.manager.ResolveADUsers(c.Request.Context(), req.Entries)
	if err != nil {
		APIError(c, err)
		return
	}

	config := facl.ACLConfig{
		Path:      fsPath,
		Type:      req.Type,
		Entries:   entries,
		Recursive: req.Recursive,
	}

	err = h.manager.RemoveACL(c.Request.Context(), config)
	if err != nil {
		APIError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

// ValidatePathParam validates the path parameter format
func ValidatePathParam() gin.HandlerFunc {
	validPath := regexp.MustCompile(`^/(?:[^<>:"|?*]*/)*(?:[^<>:"|?*]*)$`)

	return func(c *gin.Context) {
		path := c.Param("path")
		if path == "" || path == "/" {
			APIError(c, errors.New(errors.FACLInvalidInput, "Path cannot be empty"))
			c.Abort()
			return
		}

		// URL-decode the path
		decodedPath, err := url.PathUnescape(path)
		if err != nil {
			APIError(c, errors.New(errors.FACLInvalidInput, "Invalid path encoding"))
			c.Abort()
			return
		}

		// Validate path format
		if !validPath.MatchString(decodedPath) {
			APIError(c, errors.New(errors.FACLInvalidInput, "Invalid path format"))
			c.Abort()
			return
		}

		// Store the decoded path for handlers
		c.Set("decodedPath", decodedPath)
		c.Next()
	}
}

// getDecodedPath retrieves the URL-decoded path from the context
func getDecodedPath(c *gin.Context) string {
	path, exists := c.Get("decodedPath")
	if !exists {
		return ""
	}

	// Convert to absolute path, removing any ".." components
	absPath, err := filepath.Abs(path.(string))
	if err != nil {
		return ""
	}

	return absPath
}
