// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package snapshot

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
		}

		policy := autosnapshots.Group("/policy")
		{
			policy.GET("/:id", h.getPolicy)
			policy.PUT("/:id",
				ValidateSnapshotPolicyConfig(),
				h.updatePolicy)
			policy.DELETE("/:id", h.deletePolicy)
			policy.POST("/:id/run",
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

	// Get optional schedule index
	scheduleIndexStr := c.DefaultQuery("schedule_index", "0")
	scheduleIndex, err := strconv.Atoi(scheduleIndexStr)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			errors.New(errors.ZFSRequestValidationError, "invalid schedule_index"),
		)
		return
	}

	// Get optional dry run parameter
	dryRunStr := c.DefaultQuery("dry_run", "false")
	dryRun, err := strconv.ParseBool(dryRunStr)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			errors.New(errors.ZFSRequestValidationError, "invalid dry_run value"),
		)
		return
	}

	params := RunPolicyParams{
		ID:            id,
		ScheduleIndex: scheduleIndex,
		DryRun:        dryRun,
	}

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
