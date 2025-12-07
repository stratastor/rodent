// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package autotransfers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
)

// Handler handles HTTP requests for transfer policy operations
type Handler struct {
	manager *Manager
}

// APIResponse represents a standardized API response format
type APIResponse struct {
	Success bool              `json:"success"`
	Result  interface{}       `json:"result,omitempty"`
	Error   *APIErrorResponse `json:"error,omitempty"`
}

// APIErrorResponse represents error information in API responses
type APIErrorResponse struct {
	Code    int                    `json:"code"`
	Domain  string                 `json:"domain"`
	Message string                 `json:"message"`
	Details string                 `json:"details,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// NewHandlerWithManager creates a new transfer policy handler with an existing manager
func NewHandlerWithManager(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// Manager returns the transfer policy manager for use by other subsystems
func (h *Handler) Manager() *Manager {
	return h.manager
}

// RegisterRoutes registers HTTP routes for transfer policy operations
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	transfers := router.Group("/transfers")
	{
		// Policy management
		policies := transfers.Group("/policies")
		{
			policies.GET("", h.listPolicies)
			policies.POST("",
				ValidateTransferPolicyConfig(),
				h.createPolicy)
			policies.GET("/:policy_id", h.getPolicy)
			policies.PUT("/:policy_id",
				ValidateTransferPolicyConfig(),
				h.updatePolicy)
			policies.DELETE("/:policy_id", h.deletePolicy)
			policies.POST("/:policy_id/run",
				ValidateRunPolicyParams(),
				h.runPolicy)
			policies.POST("/:policy_id/enable",
				ValidateEnableDisableParams(),
				h.enablePolicy)
			policies.POST("/:policy_id/disable",
				ValidateEnableDisableParams(),
				h.disablePolicy)
		}
	}
}

// StartManager starts the transfer policy manager scheduler
func (h *Handler) StartManager() error {
	return h.manager.Start()
}

// StopManager stops the transfer policy manager scheduler
func (h *Handler) StopManager() error {
	return h.manager.Stop()
}

// sendSuccess sends a successful response with the standardized format
func (h *Handler) sendSuccess(c *gin.Context, statusCode int, result interface{}) {
	response := APIResponse{
		Success: true,
		Result:  result,
	}
	c.JSON(statusCode, response)
}

// sendError sends an error response with the standardized format
func (h *Handler) sendError(c *gin.Context, err error) {
	response := APIResponse{
		Success: false,
	}

	if rodentErr, ok := err.(*errors.RodentError); ok {
		response.Error = &APIErrorResponse{
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
		response.Error = &APIErrorResponse{
			Code:    http.StatusInternalServerError,
			Domain:  "TRANSFER_POLICY",
			Message: "Internal server error",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, response)
	}
}

// createPolicy creates a new transfer policy
func (h *Handler) createPolicy(c *gin.Context) {
	var params EditTransferPolicyParams
	if err := c.ShouldBindJSON(&params); err != nil {
		h.sendError(c, errors.Wrap(err, errors.TransferPolicyInvalidConfig))
		return
	}

	// Ensure ID is empty for creation
	params.ID = ""

	ctx := c.Request.Context()
	policyID, err := h.manager.AddPolicy(ctx, params)
	if err != nil {
		h.sendError(c, err)
		return
	}

	// Get the created policy to return
	policy, err := h.manager.GetPolicy(policyID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusCreated, policy)
}

// listPolicies lists all transfer policies
func (h *Handler) listPolicies(c *gin.Context) {
	policies, err := h.manager.ListPolicies()
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"policies": policies,
		"count":    len(policies),
	})
}

// getPolicy gets a transfer policy by ID
func (h *Handler) getPolicy(c *gin.Context) {
	policyID := c.Param("policy_id")
	if policyID == "" {
		h.sendError(c, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		return
	}

	policy, err := h.manager.GetPolicy(policyID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, policy)
}

// updatePolicy updates a transfer policy
func (h *Handler) updatePolicy(c *gin.Context) {
	policyID := c.Param("policy_id")
	if policyID == "" {
		h.sendError(c, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		return
	}

	var params EditTransferPolicyParams
	if err := c.ShouldBindJSON(&params); err != nil {
		h.sendError(c, errors.Wrap(err, errors.TransferPolicyInvalidConfig))
		return
	}

	// Set the ID from path parameter
	params.ID = policyID

	ctx := c.Request.Context()
	err := h.manager.UpdatePolicy(ctx, params)
	if err != nil {
		h.sendError(c, err)
		return
	}

	// Get the updated policy to return
	policy, err := h.manager.GetPolicy(policyID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, policy)
}

// deletePolicy deletes a transfer policy
func (h *Handler) deletePolicy(c *gin.Context) {
	policyID := c.Param("policy_id")
	if policyID == "" {
		h.sendError(c, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		return
	}

	// Check if we should remove all transfers associated with the policy
	removeTransfersStr := c.DefaultQuery("remove_transfers", "false")
	removeTransfers, err := strconv.ParseBool(removeTransfersStr)
	if err != nil {
		h.sendError(
			c,
			errors.New(errors.TransferPolicyInvalidConfig, "invalid remove_transfers value"),
		)
		return
	}

	ctx := c.Request.Context()
	err = h.manager.RemovePolicy(ctx, policyID, removeTransfers)
	if err != nil {
		h.sendError(c, err)
		return
	}

	message := "Policy deleted successfully"
	if removeTransfers {
		message = "Policy and its transfers deleted successfully"
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":   message,
		"policy_id": policyID,
	})
}

// runPolicy runs a transfer policy immediately
func (h *Handler) runPolicy(c *gin.Context) {
	policyID := c.Param("policy_id")
	if policyID == "" {
		h.sendError(c, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		return
	}

	var params RunTransferPolicyParams
	if err := c.ShouldBindJSON(&params); err != nil {
		// Allow empty body
		params = RunTransferPolicyParams{}
	}

	// Set the PolicyID from path parameter (this takes precedence over any ID in the body)
	params.PolicyID = policyID

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	// Replace the context in the gin context
	c.Request = c.Request.WithContext(ctx)

	result, err := h.manager.RunPolicy(ctx, params)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"policy_id":   result.PolicyID,
		"transfer_id": result.TransferID,
		"status":      result.Status,
	})
}

// enablePolicy enables a transfer policy
func (h *Handler) enablePolicy(c *gin.Context) {
	policyID := c.Param("policy_id")
	if policyID == "" {
		h.sendError(c, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		return
	}

	ctx := c.Request.Context()
	err := h.manager.EnablePolicy(ctx, policyID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":   "Policy enabled successfully",
		"policy_id": policyID,
	})
}

// disablePolicy disables a transfer policy
func (h *Handler) disablePolicy(c *gin.Context) {
	policyID := c.Param("policy_id")
	if policyID == "" {
		h.sendError(c, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		return
	}

	ctx := c.Request.Context()
	err := h.manager.DisablePolicy(ctx, policyID)
	if err != nil {
		h.sendError(c, err)
		return
	}

	h.sendSuccess(c, http.StatusOK, map[string]interface{}{
		"message":   "Policy disabled successfully",
		"policy_id": policyID,
	})
}
