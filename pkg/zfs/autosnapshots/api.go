// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package autosnapshots

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
)

// Handler handles HTTP requests for auto-snapshot operations
type Handler struct {
	manager *Manager
}

// NewHandler creates a new snapshot handler
func NewHandler(dsManager *dataset.Manager) (*Handler, error) {
	manager, err := NewManager(dsManager, "")
	if err != nil {
		return nil, err
	}

	return &Handler{
		manager: manager,
	}, nil
}

// RegisterRoutes registers HTTP routes for auto-snapshot operations
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	autosnapshots := router.Group("/autosnapshot")
	{
		// Policy management
		policies := autosnapshots.Group("/policies")
		{
			policies.GET("", h.listPolicies)
			policies.POST("",
				ValidateSnapshotPolicyConfig(),
				h.createPolicy)
			policies.GET("/:id", h.getPolicy)
			policies.PUT("/:id",
				ValidateSnapshotPolicyConfig(),
				h.updatePolicy)
			policies.DELETE("/:id", h.deletePolicy)
			policies.POST("/:id/run",
				ValidateRunPolicyParams(),
				h.runPolicy)
		}
	}
}

// StartManager starts the snapshot manager scheduler
func (h *Handler) StartManager() error {
	return h.manager.Start()
}

// StopManager stops the snapshot manager scheduler
func (h *Handler) StopManager() error {
	return h.manager.Stop()
}

// SchedulerInterface implementation - delegate to manager
// These methods allow Handler to be used as a SchedulerInterface

// AddPolicy adds a new snapshot policy
func (h *Handler) AddPolicy(params EditPolicyParams) (string, error) {
	return h.manager.AddPolicy(params)
}

// UpdatePolicy updates an existing snapshot policy
func (h *Handler) UpdatePolicy(params EditPolicyParams) error {
	return h.manager.UpdatePolicy(params)
}

// RemovePolicy removes a snapshot policy
func (h *Handler) RemovePolicy(policyID string, removeSnapshots bool) error {
	return h.manager.RemovePolicy(policyID, removeSnapshots)
}

// GetPolicy gets a snapshot policy by ID
func (h *Handler) GetPolicy(policyID string) (SnapshotPolicy, error) {
	return h.manager.GetPolicy(policyID)
}

// ListPolicies lists all snapshot policies
func (h *Handler) ListPolicies() ([]SnapshotPolicy, error) {
	return h.manager.ListPolicies()
}

// RunPolicy runs a snapshot policy
func (h *Handler) RunPolicy(params RunPolicyParams) (CreateSnapshotResult, error) {
	return h.manager.RunPolicy(params)
}

// Start starts the scheduler
func (h *Handler) Start() error {
	return h.manager.Start()
}

// Stop stops the scheduler
func (h *Handler) Stop() error {
	return h.manager.Stop()
}

// LoadConfig loads the configuration
func (h *Handler) LoadConfig() error {
	return h.manager.LoadConfig()
}

// SaveConfig saves the configuration
func (h *Handler) SaveConfig(skipLock bool) error {
	return h.manager.SaveConfig(skipLock)
}

// createPolicy creates a new snapshot policy
func (h *Handler) createPolicy(c *gin.Context) {
	var params EditPolicyParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ZFSRequestValidationError, err.Error()))
		return
	}

	// Ensure ID is empty for creation
	params.ID = ""

	policyID, err := h.manager.AddPolicy(params)
	if err != nil {
		c.JSON(errors.GetHTTPStatus(err), errors.Wrap(err, errors.ZFSSnapshotPolicyError))
		return
	}

	// Get the created policy to return
	policy, err := h.manager.GetPolicy(policyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.Wrap(err, errors.ZFSSnapshotPolicyError))
		return
	}

	c.JSON(http.StatusCreated, policy)
}

// listPolicies lists all snapshot policies
func (h *Handler) listPolicies(c *gin.Context) {
	policies, err := h.manager.ListPolicies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.Wrap(err, errors.ZFSSnapshotPolicyError))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
		"count":    len(policies),
	})
}

// getPolicy gets a snapshot policy by ID
func (h *Handler) getPolicy(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(
			http.StatusBadRequest,
			errors.New(errors.ZFSRequestValidationError, "policy ID is required"),
		)
		return
	}

	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(errors.GetHTTPStatus(err), errors.Wrap(err, errors.ZFSSnapshotPolicyError))
		return
	}

	c.JSON(http.StatusOK, policy)
}

// updatePolicy updates a snapshot policy
func (h *Handler) updatePolicy(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(
			http.StatusBadRequest,
			errors.New(errors.ZFSRequestValidationError, "policy ID is required"),
		)
		return
	}

	var params EditPolicyParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ZFSRequestValidationError, err.Error()))
		return
	}

	// Set the ID from path parameter
	params.ID = id

	err := h.manager.UpdatePolicy(params)
	if err != nil {
		c.JSON(errors.GetHTTPStatus(err), errors.Wrap(err, errors.ZFSSnapshotPolicyError))
		return
	}

	// Get the updated policy to return
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.Wrap(err, errors.ZFSSnapshotPolicyError))
		return
	}

	c.JSON(http.StatusOK, policy)
}

// deletePolicy deletes a snapshot policy
func (h *Handler) deletePolicy(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(
			http.StatusBadRequest,
			errors.New(errors.ZFSRequestValidationError, "policy ID is required"),
		)
		return
	}

	// Check if we should remove all snapshots associated with the policy
	removeSnapshotsStr := c.DefaultQuery("remove_snapshots", "false")
	removeSnapshots, err := strconv.ParseBool(removeSnapshotsStr)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			errors.New(errors.ZFSRequestValidationError, "invalid remove_snapshots value"),
		)
		return
	}

	err = h.manager.RemovePolicy(id, removeSnapshots)
	if err != nil {
		c.JSON(errors.GetHTTPStatus(err), errors.Wrap(err, errors.ZFSSnapshotPolicyError))
		return
	}

	message := "Policy deleted successfully"
	if removeSnapshots {
		message = "Policy and its snapshots deleted successfully"
	}

	c.JSON(http.StatusOK, gin.H{"message": message})
}

// runPolicy runs a snapshot policy immediately
func (h *Handler) runPolicy(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(
			http.StatusBadRequest,
			errors.New(errors.ZFSRequestValidationError, "policy ID is required"),
		)
		return
	}

	var params RunPolicyParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ZFSRequestValidationError, err.Error()))
		return
	}

	// Set the ID from path parameter (this takes precedence over any ID in the body)
	params.ID = id

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Replace the context in the gin context
	c.Request = c.Request.WithContext(ctx)

	result, err := h.manager.RunPolicy(params)
	if err != nil {
		c.JSON(errors.GetHTTPStatus(err), errors.Wrap(err, errors.ZFSSnapshotPolicyError))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"policy_id":        result.PolicyID,
		"dataset":          result.DatasetName,
		"snapshot":         result.SnapshotName,
		"created_at":       result.CreatedAt,
		"pruned_snapshots": result.PrunedSnapshots,
		"pruned_count":     len(result.PrunedSnapshots),
	})
}
