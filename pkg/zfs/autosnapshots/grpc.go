// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package autosnapshots

import (
	"encoding/json"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// GRPCHandler handles gRPC requests for auto-snapshot operations
type GRPCHandler struct {
	manager *Manager
}

// NewGRPCHandler creates a new snapshot gRPC handler
func NewGRPCHandler(dsManager *dataset.Manager) (*GRPCHandler, error) {
	manager, err := NewManager(dsManager, "")
	if err != nil {
		return nil, err
	}

	return &GRPCHandler{
		manager: manager,
	}, nil
}

// NewGRPCHandlerWithManager creates a new snapshot gRPC handler with an existing manager
func NewGRPCHandlerWithManager(manager *Manager) *GRPCHandler {
	return &GRPCHandler{
		manager: manager,
	}
}

// StartManager starts the snapshot manager scheduler
func (h *GRPCHandler) StartManager() error {
	return h.manager.Start()
}

// StopManager stops the snapshot manager scheduler
func (h *GRPCHandler) StopManager() error {
	return h.manager.Stop()
}

// RegisterAutosnapshotGRPCHandlers registers all auto-snapshot related command handlers with Toggle
func RegisterAutosnapshotGRPCHandlers(handler *GRPCHandler) {
	// Snapshot policy operations
	client.RegisterCommandHandler(proto.CmdPoliciesAutosnapList, handleListPolicies(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesAutosnapGet, handleGetPolicy(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesAutosnapCreate, handleCreatePolicy(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesAutosnapUpdate, handleUpdatePolicy(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesAutosnapDelete, handleDeletePolicy(handler))
	client.RegisterCommandHandler(proto.CmdPoliciesAutosnapRun, handleRunPolicy(handler))
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
	payload, err := json.Marshal(data)
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

// handleListPolicies returns a handler for listing all snapshot policies
func handleListPolicies(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Call the manager's ListPolicies method
		policies, err := h.manager.ListPolicies()
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotPolicyError)
		}

		// Return success response with policies list in the same format as REST API
		response := map[string]interface{}{
			"policies": policies,
			"count":    len(policies),
		}
		return successResponse(req.RequestId, "Snapshot policies list", response)
	}
}

// handleGetPolicy returns a handler for getting a snapshot policy by ID
func handleGetPolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			ID string `json:"id"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if payload.ID == "" {
			return nil, errors.New(errors.ZFSRequestValidationError, "policy ID is required")
		}

		// Call the manager's GetPolicy method
		policy, err := h.manager.GetPolicy(payload.ID)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotPolicyError)
		}

		return successResponse(req.RequestId, "Snapshot policy details", policy)
	}
}

// handleCreatePolicy returns a handler for creating a new snapshot policy
func handleCreatePolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var params EditPolicyParams
		if err := parseJSONPayload(cmd, &params); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Ensure ID is empty for creation
		params.ID = ""

		// Validate policy
		policy := NewSnapshotPolicy(params)
		if err := ValidatePolicy(policy); err != nil {
			return nil, err
		}

		// Call the manager's AddPolicy method
		policyID, err := h.manager.AddPolicy(params)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotPolicyError)
		}

		// Get the created policy to return
		createdPolicy, err := h.manager.GetPolicy(policyID)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotPolicyError)
		}

		return successResponse(req.RequestId, "Snapshot policy created successfully", createdPolicy)
	}
}

// handleUpdatePolicy returns a handler for updating an existing snapshot policy
func handleUpdatePolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var params EditPolicyParams
		if err := parseJSONPayload(cmd, &params); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if params.ID == "" {
			return nil, errors.New(errors.ZFSRequestValidationError, "policy ID is required")
		}

		// Validate policy
		policy := NewSnapshotPolicy(params)
		if err := ValidatePolicy(policy); err != nil {
			return nil, err
		}

		// Call the manager's UpdatePolicy method
		if err := h.manager.UpdatePolicy(params); err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotPolicyError)
		}

		// Get the updated policy to return
		updatedPolicy, err := h.manager.GetPolicy(params.ID)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotPolicyError)
		}

		return successResponse(req.RequestId, "Snapshot policy updated successfully", updatedPolicy)
	}
}

// handleDeletePolicy returns a handler for deleting a snapshot policy
func handleDeletePolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			ID              string `json:"id"`
			RemoveSnapshots bool   `json:"remove_snapshots"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if payload.ID == "" {
			return nil, errors.New(errors.ZFSRequestValidationError, "policy ID is required")
		}

		// Call the manager's RemovePolicy method
		if err := h.manager.RemovePolicy(payload.ID, payload.RemoveSnapshots); err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotPolicyError)
		}

		message := "Policy deleted successfully"
		if payload.RemoveSnapshots {
			message = "Policy and its snapshots deleted successfully"
		}

		response := map[string]interface{}{
			"message": message,
			"id":      payload.ID,
		}

		return successResponse(req.RequestId, message, response)
	}
}

// handleRunPolicy returns a handler for running a snapshot policy immediately
func handleRunPolicy(h *GRPCHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var params RunPolicyParams
		if err := parseJSONPayload(cmd, &params); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if params.ID == "" {
			return nil, errors.New(errors.ZFSRequestValidationError, "policy ID is required")
		}

		// Call the manager's RunPolicy method
		result, err := h.manager.RunPolicy(params)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotPolicyError)
		}

		response := map[string]interface{}{
			"policy_id":        result.PolicyID,
			"dataset":          result.DatasetName,
			"snapshot":         result.SnapshotName,
			"created_at":       result.CreatedAt,
			"pruned_snapshots": result.PrunedSnapshots,
			"pruned_count":     len(result.PrunedSnapshots),
		}

		return successResponse(req.RequestId, "Snapshot policy executed successfully", response)
	}
}
