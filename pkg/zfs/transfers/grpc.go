// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package transfers

import (
	"context"
	"encoding/json"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// GRPCHandler handles gRPC requests for transfer policy operations
type GRPCHandler struct {
	manager *Manager
}

// NewGRPCHandlerWithManager creates a new transfer policy gRPC handler with an existing manager
func NewGRPCHandlerWithManager(manager *Manager) *GRPCHandler {
	return &GRPCHandler{
		manager: manager,
	}
}

// StartManager starts the transfer policy manager scheduler
func (h *GRPCHandler) StartManager() error {
	return h.manager.Start()
}

// StopManager stops the transfer policy manager scheduler
func (h *GRPCHandler) StopManager() error {
	return h.manager.Stop()
}

// RegisterTransferPolicyGRPCHandlers registers all transfer policy related command handlers with Toggle
func RegisterTransferPolicyGRPCHandlers(handler *GRPCHandler) {
	// Transfer policy operations
	client.RegisterCommandHandler(proto.CmdPoliciesTransferList, handleListPolicies(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesTransferGet, handleGetPolicy(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesTransferCreate, handleCreatePolicy(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesTransferUpdate, handleUpdatePolicy(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesTransferDelete, handleDeletePolicy(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesTransferRun, handleRunPolicy(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesTransferEnable, handleEnablePolicy(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesTransferDisable, handleDisablePolicy(handler))
}

// Helper for parsing JSON payload from a command request
func parseJSONPayload(cmd *proto.CommandRequest, out interface{}) error {
	if len(cmd.Payload) == 0 {
		return errors.New(errors.ServerRequestValidation, "empty payload")
	}
	return json.Unmarshal(cmd.Payload, out)
}

// successResponse creates a successful response with the provided data
func successResponse(
	requestID string,
	message string,
	data any,
) (*proto.CommandResponse, error) {
	response := APIResponse{
		Success: true,
		Result:  data,
	}

	payload, err := json.Marshal(response)
	if err != nil {
		return nil, errors.Wrap(err, errors.ServerResponseError)
	}

	return &proto.CommandResponse{
		RequestId: requestID,
		Success:   true,
		Message:   message,
		Payload:   payload,
	}, nil
}

// errorResponse creates an error response with the provided error
func errorResponse(_ string, err error) (*proto.CommandResponse, error) {
	return nil, err
}

// handleListPolicies returns a handler for listing all transfer policies
func handleListPolicies(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Call the manager's ListPolicies method
		policies, err := h.manager.ListPolicies()
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		// Return success response with policies list
		response := map[string]interface{}{
			"policies": policies,
			"count":    len(policies),
		}
		return successResponse(req.RequestId, "Transfer policies list", response)
	}
}

// handleGetPolicy returns a handler for getting a transfer policy by ID
func handleGetPolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			PolicyID string `json:"policy_id"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.PolicyID == "" {
			return errorResponse(req.RequestId, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		}

		// Call the manager's GetPolicy method
		policy, err := h.manager.GetPolicy(payload.PolicyID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Transfer policy details", policy)
	}
}

// handleCreatePolicy returns a handler for creating a new transfer policy
func handleCreatePolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var params EditTransferPolicyParams
		if err := parseJSONPayload(cmd, &params); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		// Ensure ID is empty for creation
		params.ID = ""

		// Validate policy
		policy := NewTransferPolicy(params)
		if err := ValidatePolicy(policy); err != nil {
			return errorResponse(req.RequestId, err)
		}

		// Call the manager's AddPolicy method
		ctx := context.Background()
		policyID, err := h.manager.AddPolicy(ctx, params)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		// Get the created policy to return
		createdPolicy, err := h.manager.GetPolicy(policyID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Transfer policy created successfully", createdPolicy)
	}
}

// handleUpdatePolicy returns a handler for updating an existing transfer policy
func handleUpdatePolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var params EditTransferPolicyParams
		if err := parseJSONPayload(cmd, &params); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if params.ID == "" {
			return errorResponse(req.RequestId, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		}

		// Validate policy
		policy := NewTransferPolicy(params)
		if err := ValidatePolicy(policy); err != nil {
			return errorResponse(req.RequestId, err)
		}

		// Call the manager's UpdatePolicy method
		ctx := context.Background()
		if err := h.manager.UpdatePolicy(ctx, params); err != nil {
			return errorResponse(req.RequestId, err)
		}

		// Get the updated policy to return
		updatedPolicy, err := h.manager.GetPolicy(params.ID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Transfer policy updated successfully", updatedPolicy)
	}
}

// handleDeletePolicy returns a handler for deleting a transfer policy
func handleDeletePolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			PolicyID        string `json:"policy_id"`
			RemoveTransfers bool   `json:"remove_transfers"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.PolicyID == "" {
			return errorResponse(req.RequestId, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		}

		// Call the manager's RemovePolicy method
		ctx := context.Background()
		if err := h.manager.RemovePolicy(ctx, payload.PolicyID, payload.RemoveTransfers); err != nil {
			return errorResponse(req.RequestId, err)
		}

		message := "Policy deleted successfully"
		if payload.RemoveTransfers {
			message = "Policy and its transfers deleted successfully"
		}

		response := map[string]interface{}{
			"message":   message,
			"policy_id": payload.PolicyID,
		}

		return successResponse(req.RequestId, message, response)
	}
}

// handleRunPolicy returns a handler for running a transfer policy immediately
func handleRunPolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var params RunTransferPolicyParams
		if err := parseJSONPayload(cmd, &params); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if params.PolicyID == "" {
			return errorResponse(req.RequestId, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		}

		// Call the manager's RunPolicy method with background context
		ctx := context.Background()
		result, err := h.manager.RunPolicy(ctx, params)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		response := map[string]interface{}{
			"policy_id":   result.PolicyID,
			"transfer_id": result.TransferID,
			"status":      result.Status,
		}

		return successResponse(req.RequestId, "Transfer policy executed successfully", response)
	}
}

// handleEnablePolicy returns a handler for enabling a transfer policy
func handleEnablePolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			PolicyID string `json:"policy_id"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.PolicyID == "" {
			return errorResponse(req.RequestId, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		}

		// Call the manager's EnablePolicy method
		ctx := context.Background()
		if err := h.manager.EnablePolicy(ctx, payload.PolicyID); err != nil {
			return errorResponse(req.RequestId, err)
		}

		response := map[string]interface{}{
			"message":   "Policy enabled successfully",
			"policy_id": payload.PolicyID,
		}

		return successResponse(req.RequestId, "Policy enabled", response)
	}
}

// handleDisablePolicy returns a handler for disabling a transfer policy
func handleDisablePolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			PolicyID string `json:"policy_id"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.PolicyID == "" {
			return errorResponse(req.RequestId, errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"))
		}

		// Call the manager's DisablePolicy method
		ctx := context.Background()
		if err := h.manager.DisablePolicy(ctx, payload.PolicyID); err != nil {
			return errorResponse(req.RequestId, err)
		}

		response := map[string]interface{}{
			"message":   "Policy disabled successfully",
			"policy_id": payload.PolicyID,
		}

		return successResponse(req.RequestId, "Policy disabled", response)
	}
}
