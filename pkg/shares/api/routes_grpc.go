// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/shares/smb"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterSharesGRPCHandlers registers all Shares-related command handlers with Toggle
func RegisterSharesGRPCHandlers(sharesHandler *SharesHandler) {
	// SMB shares operations
	client.RegisterCommandHandler(proto.CmdSharesSMBList, handleSMBList(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBGet, handleSMBGet(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBCreate, handleSMBCreate(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBUpdate, handleSMBUpdate(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBDelete, handleSMBDelete(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBStats, handleSMBStats(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBBulkUpdate, handleSMBBulkUpdate(sharesHandler))

	// SMB global config operations
	client.RegisterCommandHandler(proto.CmdSharesSMBGlobalGet, handleSMBGlobalGet(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBGlobalUpdate, handleSMBGlobalUpdate(sharesHandler))

	// SMB service operations
	client.RegisterCommandHandler(proto.CmdSharesSMBServiceStatus, handleSMBServiceStatus(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBServiceStart, handleSMBServiceStart(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBServiceStop, handleSMBServiceStop(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBServiceRestart, handleSMBServiceRestart(sharesHandler))
	client.RegisterCommandHandler(proto.CmdSharesSMBServiceReload, handleSMBServiceReload(sharesHandler))
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

// SMB HANDLERS

// handleSMBList returns a handler for listing SMB shares
func handleSMBList(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Create a context
		ctx := context.Background()
		
		// Call the manager's ListSharesByType method
		result, err := h.smbManager.ListSharesByType(ctx, "smb")
		if err != nil {
			return nil, errors.Wrap(err, errors.SharesOperationFailed)
		}

		// Return success response with shares list in the same format as REST API
		response := map[string]interface{}{
			"shares": result,
			"count":  len(result),
		}
		return successResponse(req.RequestId, "SMB shares list", response)
	}
}

// handleSMBGet returns a handler for getting an SMB share by name
func handleSMBGet(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if payload.Name == "" {
			return nil, errors.New(errors.SharesInvalidInput, "Share name cannot be empty")
		}

		// Create a context
		ctx := context.Background()

		// Call the manager's GetSMBShare method
		share, err := h.smbManager.GetSMBShare(ctx, payload.Name)
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "SMB share details", share)
	}
}

// handleSMBCreate returns a handler for creating an SMB share
func handleSMBCreate(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var config smb.SMBShareConfig
		if err := parseJSONPayload(cmd, &config); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate share configuration
		if config.Name == "" {
			return nil, errors.New(errors.SharesInvalidInput, "Share name cannot be empty")
		}

		if config.Path == "" {
			return nil, errors.New(errors.SharesInvalidInput, "Share path cannot be empty")
		}

		// Create a context
		ctx := context.Background()

		// Create a new config with defaults if necessary
		defaultConfig := smb.NewSMBShareConfig(config.Name, config.Path)

		// Override with user-provided values where specified
		if config.Description != "" {
			defaultConfig.Description = config.Description
		}

		// Copy non-empty slices
		if len(config.ValidUsers) > 0 {
			defaultConfig.ValidUsers = config.ValidUsers
		}
		if len(config.InvalidUsers) > 0 {
			defaultConfig.InvalidUsers = config.InvalidUsers
		}
		if len(config.ReadList) > 0 {
			defaultConfig.ReadList = config.ReadList
		}
		if len(config.WriteList) > 0 {
			defaultConfig.WriteList = config.WriteList
		}

		// Copy user-provided tags
		if len(config.Tags) > 0 {
			for k, v := range config.Tags {
				defaultConfig.Tags[k] = v
			}
		}

		// Copy user-provided custom parameters
		if len(config.CustomParameters) > 0 {
			for k, v := range config.CustomParameters {
				defaultConfig.CustomParameters[k] = v
			}
		}

		// Use explicitly set booleans
		defaultConfig.ReadOnly = config.ReadOnly
		defaultConfig.Browsable = config.Browsable
		defaultConfig.GuestOk = config.GuestOk
		defaultConfig.Public = config.Public
		defaultConfig.InheritACLs = config.InheritACLs
		defaultConfig.MapACLInherit = config.MapACLInherit
		defaultConfig.FollowSymlinks = config.FollowSymlinks

		// Call the manager's CreateShare method
		if err := h.smbManager.CreateShare(ctx, defaultConfig); err != nil {
			return nil, err
		}

		// Return success response
		response := map[string]interface{}{
			"message": "Share created successfully",
			"name":    config.Name,
		}
		return successResponse(req.RequestId, "Share created successfully", response)
	}
}

// handleSMBUpdate returns a handler for updating an SMB share
func handleSMBUpdate(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var config smb.SMBShareConfig
		if err := parseJSONPayload(cmd, &config); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate share configuration
		if config.Name == "" {
			return nil, errors.New(errors.SharesInvalidInput, "Share name cannot be empty")
		}

		if config.Path == "" {
			return nil, errors.New(errors.SharesInvalidInput, "Share path cannot be empty")
		}

		// Create a context
		ctx := context.Background()

		// Call the manager's UpdateShare method
		if err := h.smbManager.UpdateShare(ctx, config.Name, &config); err != nil {
			return nil, err
		}

		// Return success response
		response := map[string]interface{}{
			"message": "Share updated successfully",
			"name":    config.Name,
		}
		return successResponse(req.RequestId, "Share updated successfully", response)
	}
}

// handleSMBDelete returns a handler for deleting an SMB share
func handleSMBDelete(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if payload.Name == "" {
			return nil, errors.New(errors.SharesInvalidInput, "Share name cannot be empty")
		}

		// Create a context
		ctx := context.Background()

		// Call the manager's DeleteShare method
		if err := h.smbManager.DeleteShare(ctx, payload.Name); err != nil {
			return nil, err
		}

		// Return success response
		response := map[string]interface{}{
			"message": "Share deleted successfully",
			"name":    payload.Name,
		}
		return successResponse(req.RequestId, "Share deleted successfully", response)
	}
}

// handleSMBStats returns a handler for getting statistics for an SMB share
func handleSMBStats(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name     string `json:"name"`
			Detailed bool   `json:"detailed"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if payload.Name == "" {
			return nil, errors.New(errors.SharesInvalidInput, "Share name cannot be empty")
		}

		// Create a context
		ctx := context.Background()

		if payload.Detailed {
			// Get detailed SMB share stats
			stats, err := h.smbManager.GetSMBShareStats(ctx, payload.Name)
			if err != nil {
				return nil, err
			}
			return successResponse(req.RequestId, "SMB share detailed statistics", stats)
		}

		// Get simple share stats
		stats, err := h.smbManager.GetShareStats(ctx, payload.Name)
		if err != nil {
			return nil, err
		}
		return successResponse(req.RequestId, "SMB share statistics", stats)
	}
}

// handleSMBGlobalGet returns a handler for getting the global SMB configuration
func handleSMBGlobalGet(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Create a context
		ctx := context.Background()
		
		// Call the manager's GetGlobalConfig method
		config, err := h.smbManager.GetGlobalConfig(ctx)
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "Global SMB configuration", config)
	}
}

// handleSMBGlobalUpdate returns a handler for updating the global SMB configuration
func handleSMBGlobalUpdate(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var config smb.SMBGlobalConfig
		if err := parseJSONPayload(cmd, &config); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate global configuration
		if config.WorkGroup == "" {
			return nil, errors.New(errors.SharesInvalidInput, "Workgroup cannot be empty")
		}

		if config.SecurityMode == "" {
			return nil, errors.New(errors.SharesInvalidInput, "Security mode cannot be empty")
		}

		// Create a context
		ctx := context.Background()

		// Call the manager's UpdateGlobalConfig method
		if err := h.smbManager.UpdateGlobalConfig(ctx, &config); err != nil {
			return nil, err
		}

		// Return success response
		response := map[string]interface{}{
			"message": "Global SMB configuration updated successfully",
		}
		return successResponse(req.RequestId, "Global SMB configuration updated successfully", response)
	}
}

// handleSMBBulkUpdate returns a handler for bulk updating SMB shares
func handleSMBBulkUpdate(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var config smb.SMBBulkUpdateConfig
		if err := parseJSONPayload(cmd, &config); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate bulk update configuration
		if len(config.Parameters) == 0 {
			return nil, errors.New(errors.SharesInvalidInput, "At least one parameter must be specified for bulk update")
		}

		// Create a context
		ctx := context.Background()

		// Process the bulk update
		results, err := h.smbManager.BulkUpdateShares(ctx, config)
		if err != nil {
			return nil, err
		}

		// Return success response
		response := map[string]interface{}{
			"message": "Bulk update completed",
			"results": results,
		}
		return successResponse(req.RequestId, "Bulk update completed", response)
	}
}

// handleSMBServiceStatus returns a handler for getting the status of the SMB service
func handleSMBServiceStatus(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Create a context
		ctx := context.Background()
		
		// Call the manager's GetSMBServiceStatus method
		status, err := h.smbManager.GetSMBServiceStatus(ctx)
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "SMB service status", status)
	}
}

// handleSMBServiceStart returns a handler for starting the SMB service
func handleSMBServiceStart(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Create a context
		ctx := context.Background()
		
		// Call the service's Start method
		if err := h.smbService.Start(ctx); err != nil {
			return nil, err
		}

		// Return success response
		response := map[string]interface{}{
			"message": "SMB service started successfully",
		}
		return successResponse(req.RequestId, "SMB service started successfully", response)
	}
}

// handleSMBServiceStop returns a handler for stopping the SMB service
func handleSMBServiceStop(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Create a context
		ctx := context.Background()
		
		// Call the service's Stop method
		if err := h.smbService.Stop(ctx); err != nil {
			return nil, err
		}

		// Return success response
		response := map[string]interface{}{
			"message": "SMB service stopped successfully",
		}
		return successResponse(req.RequestId, "SMB service stopped successfully", response)
	}
}

// handleSMBServiceRestart returns a handler for restarting the SMB service
func handleSMBServiceRestart(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Create a context
		ctx := context.Background()
		
		// Call the service's Restart method
		if err := h.smbService.Restart(ctx); err != nil {
			return nil, err
		}

		// Return success response
		response := map[string]interface{}{
			"message": "SMB service restarted successfully",
		}
		return successResponse(req.RequestId, "SMB service restarted successfully", response)
	}
}

// handleSMBServiceReload returns a handler for reloading the SMB service configuration
func handleSMBServiceReload(h *SharesHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Create a context
		ctx := context.Background()
		
		// Call the service's ReloadConfig method
		if err := h.smbService.ReloadConfig(ctx); err != nil {
			return nil, err
		}

		// Return success response
		response := map[string]interface{}{
			"message": "SMB service configuration reloaded successfully",
		}
		return successResponse(req.RequestId, "SMB service configuration reloaded successfully", response)
	}
}