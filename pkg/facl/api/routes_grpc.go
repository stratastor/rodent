// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/facl"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterFACLGRPCHandlers registers all FACL-related command handlers with Toggle
func RegisterFACLGRPCHandlers(aclHandler *ACLHandler) {
	// Standard ACL operations
	client.RegisterCommandHandler(proto.CmdFACLGet, handleGetACL(aclHandler, false))
	client.RegisterCommandHandler(proto.CmdFACLSet, handleSetACL(aclHandler, false))
	client.RegisterCommandHandler(proto.CmdFACLModify, handleModifyACL(aclHandler, false))
	client.RegisterCommandHandler(proto.CmdFACLRemove, handleRemoveACL(aclHandler, false))

	// Recursive ACL operations
	client.RegisterCommandHandler(proto.CmdFACLGetRecursive, handleGetACL(aclHandler, true))
	client.RegisterCommandHandler(proto.CmdFACLSetRecursive, handleSetACL(aclHandler, true))
	client.RegisterCommandHandler(proto.CmdFACLModifyRecursive, handleModifyACL(aclHandler, true))
	client.RegisterCommandHandler(proto.CmdFACLRemoveRecursive, handleRemoveACL(aclHandler, true))
}

// Helper for parsing JSON payload from a command request
func parseJSONPayload(cmd *proto.CommandRequest, out interface{}) error {
	if len(cmd.Payload) == 0 {
		return errors.New(errors.ServerRequestValidation, "empty payload")
	}
	return json.Unmarshal(cmd.Payload, out)
}

// validatePath performs path validation similar to ValidatePathParam middleware
// used in REST API handlers
func validatePath(path string) (string, error) {
	if path == "" || path == "/" {
		return "", errors.New(errors.FACLInvalidInput, "Path cannot be empty")
	}

	// Use the same regex pattern as in the REST API
	validPath := regexp.MustCompile(`^/(?:[^<>:"|?*]*/)*(?:[^<>:"|?*]*)$`)
	if !validPath.MatchString(path) {
		return "", errors.New(errors.FACLInvalidInput, "Invalid path format")
	}

	// Convert to absolute path, removing any ".." components
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", errors.New(errors.FACLInvalidInput, "Failed to resolve path")
	}

	return absPath, nil
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

// handleGetACL returns a handler for getting ACLs for a path
func handleGetACL(h *ACLHandler, recursive bool) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Path string `json:"path"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate and normalize the path
		validatedPath, err := validatePath(payload.Path)
		if err != nil {
			return nil, err
		}

		// Create a context
		ctx := context.Background()

		config := facl.ACLListConfig{
			Path:      validatedPath,
			Recursive: recursive,
		}

		// Call the manager's GetACL method
		result, err := h.manager.GetACL(ctx, config)
		if err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"result": result,
		}

		return successResponse(req.RequestId, "ACL retrieved successfully", response)
	}
}

// handleSetACL returns a handler for setting ACLs for a path
func handleSetACL(h *ACLHandler, recursive bool) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Path    string          `json:"path"`
			Type    facl.ACLType    `json:"type"`
			Entries []facl.ACLEntry `json:"entries"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate and normalize the path
		validatedPath, err := validatePath(payload.Path)
		if err != nil {
			return nil, err
		}

		if len(payload.Entries) == 0 {
			return nil, errors.New(errors.FACLInvalidInput, "No ACL entries provided")
		}

		// Create a context
		ctx := context.Background()

		// Validate and resolve AD users/groups
		entries, err := h.manager.ResolveADUsers(ctx, payload.Entries)
		if err != nil {
			return nil, err
		}

		config := facl.ACLConfig{
			Path:      validatedPath,
			Type:      payload.Type,
			Entries:   entries,
			Recursive: recursive,
		}

		// Call the manager's SetACL method
		if err := h.manager.SetACL(ctx, config); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"message": "ACL set successfully",
			"path":    validatedPath,
		}

		return successResponse(req.RequestId, "ACL set successfully", response)
	}
}

// handleModifyACL returns a handler for modifying ACLs for a path
func handleModifyACL(h *ACLHandler, recursive bool) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Path    string          `json:"path"`
			Type    facl.ACLType    `json:"type"`
			Entries []facl.ACLEntry `json:"entries"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate and normalize the path
		validatedPath, err := validatePath(payload.Path)
		if err != nil {
			return nil, err
		}

		if len(payload.Entries) == 0 {
			return nil, errors.New(errors.FACLInvalidInput, "No ACL entries provided")
		}

		// Create a context
		ctx := context.Background()

		// Validate and resolve AD users/groups
		entries, err := h.manager.ResolveADUsers(ctx, payload.Entries)
		if err != nil {
			return nil, err
		}

		config := facl.ACLConfig{
			Path:      validatedPath,
			Type:      payload.Type,
			Entries:   entries,
			Recursive: recursive,
		}

		// Call the manager's ModifyACL method
		if err := h.manager.ModifyACL(ctx, config); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"message": "ACL modified successfully",
			"path":    validatedPath,
		}

		return successResponse(req.RequestId, "ACL modified successfully", response)
	}
}

// handleRemoveACL returns a handler for removing ACLs for a path
func handleRemoveACL(h *ACLHandler, recursive bool) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Path           string          `json:"path"`
			Type           facl.ACLType    `json:"type"`
			Entries        []facl.ACLEntry `json:"entries"`
			RemoveAllXattr bool            `json:"remove_all_xattr"`
			RemoveDefault  bool            `json:"remove_default"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate and normalize the path
		validatedPath, err := validatePath(payload.Path)
		if err != nil {
			return nil, err
		}

		// Check if we have entries or are removing defaults/all
		if !payload.RemoveAllXattr && !payload.RemoveDefault && len(payload.Entries) == 0 {
			return nil, errors.New(errors.FACLInvalidInput, 
				"Must specify entries to remove or set RemoveAllXattr or RemoveDefault to true")
		}

		// Create a context
		ctx := context.Background()

		config := facl.ACLRemoveConfig{
			ACLConfig: facl.ACLConfig{
				Path:      validatedPath,
				Type:      payload.Type,
				Entries:   payload.Entries,
				Recursive: recursive,
			},
			RemoveAllXattr: payload.RemoveAllXattr,
			RemoveDefault:  payload.RemoveDefault,
		}

		// Call the manager's RemoveACL method
		if err := h.manager.RemoveACL(ctx, config); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"message": "ACLs removed successfully",
			"path":    validatedPath,
		}

		return successResponse(req.RequestId, "ACLs removed successfully", response)
	}
}