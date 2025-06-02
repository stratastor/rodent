// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/netmage/types"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterNetworkGRPCHandlers registers all network-related command handlers with Toggle
func RegisterNetworkGRPCHandlers(networkHandler *NetworkHandler) {
	// Interface management operations
	client.RegisterCommandHandler(proto.CmdNetworkInterfacesList, handleInterfacesList(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkInterfacesGet, handleInterfacesGet(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkInterfacesSetState, handleInterfacesSetState(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkInterfacesGetStats, handleInterfacesGetStats(networkHandler))

	// IP address management operations
	client.RegisterCommandHandler(proto.CmdNetworkAddressesAdd, handleAddressesAdd(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkAddressesRemove, handleAddressesRemove(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkAddressesGet, handleAddressesGet(networkHandler))

	// Route management operations
	client.RegisterCommandHandler(proto.CmdNetworkRoutesList, handleRoutesList(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkRoutesAdd, handleRoutesAdd(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkRoutesRemove, handleRoutesRemove(networkHandler))

	// Netplan configuration operations
	client.RegisterCommandHandler(proto.CmdNetworkNetplanGetConfig, handleNetplanGetConfig(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkNetplanSetConfig, handleNetplanSetConfig(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkNetplanApply, handleNetplanApply(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkNetplanTry, handleNetplanTry(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkNetplanStatus, handleNetplanStatus(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkNetplanDiff, handleNetplanDiff(networkHandler))

	// Backup and restore operations
	client.RegisterCommandHandler(proto.CmdNetworkBackupsList, handleBackupsList(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkBackupsCreate, handleBackupsCreate(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkBackupsRestore, handleBackupsRestore(networkHandler))

	// System information operations
	client.RegisterCommandHandler(proto.CmdNetworkSystemInfo, handleSystemInfo(networkHandler))

	// Global DNS management operations
	client.RegisterCommandHandler(proto.CmdNetworkDNSGetGlobal, handleDNSGetGlobal(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkDNSSetGlobal, handleDNSSetGlobal(networkHandler))

	// Validation operations
	client.RegisterCommandHandler(proto.CmdNetworkValidateIP, handleValidateIP(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkValidateInterfaceName, handleValidateInterfaceName(networkHandler))
	client.RegisterCommandHandler(proto.CmdNetworkValidateNetplanConfig, handleValidateNetplanConfig(networkHandler))
}

// Helper for parsing JSON payload from a command request
func parseJSONPayload(cmd *proto.CommandRequest, out interface{}) error {
	if len(cmd.Payload) == 0 {
		return errors.New(errors.ServerRequestValidation, "empty payload")
	}
	return json.Unmarshal(cmd.Payload, out)
}

// successResponse creates a successful response with the provided data
func successResponse(requestID string, message string, data interface{}) (*proto.CommandResponse, error) {
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
func errorResponse(requestID string, err error) (*proto.CommandResponse, error) {
	response := APIResponse{
		Success: false,
	}

	if rodentErr, ok := err.(*errors.RodentError); ok {
		response.Error = &APIError{
			Code:    int(rodentErr.Code),
			Domain:  string(rodentErr.Domain),
			Message: rodentErr.Message,
			Details: rodentErr.Details,
		}

		// Add metadata if available
		if rodentErr.Metadata != nil && len(rodentErr.Metadata) > 0 {
			response.Error.Meta = make(map[string]interface{})
			for k, v := range rodentErr.Metadata {
				response.Error.Meta[k] = v
			}
		}
	} else {
		response.Error = &APIError{
			Code:    500,
			Domain:  "NETWORK",
			Message: "Internal server error",
			Details: err.Error(),
		}
	}

	payload, err := json.Marshal(response)
	if err != nil {
		return nil, errors.Wrap(err, errors.ServerResponseError)
	}

	return &proto.CommandResponse{
		RequestId: requestID,
		Success:   false,
		Message:   "Network operation failed",
		Payload:   payload,
	}, nil
}

// INTERFACE HANDLERS

// handleInterfacesList returns a handler for listing network interfaces
func handleInterfacesList(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		interfaces, err := h.manager.ListInterfaces(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"interfaces": interfaces,
			"count":      len(interfaces),
		}

		return successResponse(req.RequestId, "Network interfaces list", result)
	}
}

// handleInterfacesGet returns a handler for getting a specific network interface
func handleInterfacesGet(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if payload.Name == "" {
			return errorResponse(req.RequestId, errors.New(errors.NetworkInterfaceNotFound, "Interface name cannot be empty"))
		}

		ctx := context.Background()
		iface, err := h.manager.GetInterface(ctx, payload.Name)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Network interface details", iface)
	}
}

// handleInterfacesSetState returns a handler for setting interface state
func handleInterfacesSetState(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name  string `json:"name"`
			State string `json:"state"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if payload.Name == "" {
			return errorResponse(req.RequestId, errors.New(errors.NetworkInterfaceNotFound, "Interface name cannot be empty"))
		}

		var state types.InterfaceState
		switch payload.State {
		case "up", "UP":
			state = types.InterfaceStateUp
		case "down", "DOWN":
			state = types.InterfaceStateDown
		default:
			return errorResponse(req.RequestId, errors.New(errors.NetworkOperationFailed, "Invalid interface state: must be 'up' or 'down'"))
		}

		ctx := context.Background()
		if err := h.manager.SetInterfaceState(ctx, payload.Name, state); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message":   "Interface state updated successfully",
			"interface": payload.Name,
			"state":     state,
		}

		return successResponse(req.RequestId, "Interface state updated", result)
	}
}

// handleInterfacesGetStats returns a handler for getting interface statistics
func handleInterfacesGetStats(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if payload.Name == "" {
			return errorResponse(req.RequestId, errors.New(errors.NetworkInterfaceNotFound, "Interface name cannot be empty"))
		}

		ctx := context.Background()
		stats, err := h.manager.GetInterfaceStatistics(ctx, payload.Name)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"interface":  payload.Name,
			"statistics": stats,
		}

		return successResponse(req.RequestId, "Interface statistics", result)
	}
}

// ADDRESS HANDLERS

// handleAddressesAdd returns a handler for adding IP addresses
func handleAddressesAdd(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload types.AddressRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()
		if err := h.manager.AddIPAddress(ctx, payload.Interface, payload.Address); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message":   "IP address added successfully",
			"interface": payload.Interface,
			"address":   payload.Address,
		}

		return successResponse(req.RequestId, "IP address added", result)
	}
}

// handleAddressesRemove returns a handler for removing IP addresses
func handleAddressesRemove(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload types.AddressRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()
		if err := h.manager.RemoveIPAddress(ctx, payload.Interface, payload.Address); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message":   "IP address removed successfully",
			"interface": payload.Interface,
			"address":   payload.Address,
		}

		return successResponse(req.RequestId, "IP address removed", result)
	}
}

// handleAddressesGet returns a handler for getting IP addresses
func handleAddressesGet(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Interface string `json:"interface"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if payload.Interface == "" {
			return errorResponse(req.RequestId, errors.New(errors.NetworkInterfaceNotFound, "Interface name cannot be empty"))
		}

		ctx := context.Background()
		addresses, err := h.manager.GetIPAddresses(ctx, payload.Interface)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"interface": payload.Interface,
			"addresses": addresses,
			"count":     len(addresses),
		}

		return successResponse(req.RequestId, "IP addresses", result)
	}
}

// ROUTE HANDLERS

// handleRoutesList returns a handler for listing routes
func handleRoutesList(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Table string `json:"table"`
		}

		// Table is optional
		parseJSONPayload(cmd, &payload)

		ctx := context.Background()
		routes, err := h.manager.GetRoutes(ctx, payload.Table)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"routes": routes,
			"count":  len(routes),
		}
		if payload.Table != "" {
			result["table"] = payload.Table
		}

		return successResponse(req.RequestId, "Network routes", result)
	}
}

// handleRoutesAdd returns a handler for adding routes
func handleRoutesAdd(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload types.RouteRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		route := &types.Route{
			To:     payload.To,
			Via:    payload.Via,
			From:   payload.From,
			Device: payload.Device,
			Table:  payload.Table,
			Metric: payload.Metric,
		}

		ctx := context.Background()
		if err := h.manager.AddRoute(ctx, route); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message": "Route added successfully",
			"route":   route,
		}

		return successResponse(req.RequestId, "Route added", result)
	}
}

// handleRoutesRemove returns a handler for removing routes
func handleRoutesRemove(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload types.RouteRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		route := &types.Route{
			To:     payload.To,
			Via:    payload.Via,
			From:   payload.From,
			Device: payload.Device,
			Table:  payload.Table,
			Metric: payload.Metric,
		}

		ctx := context.Background()
		if err := h.manager.RemoveRoute(ctx, route); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message": "Route removed successfully",
			"route":   route,
		}

		return successResponse(req.RequestId, "Route removed", result)
	}
}

// NETPLAN HANDLERS

// handleNetplanGetConfig returns a handler for getting netplan config
func handleNetplanGetConfig(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		config, err := h.manager.GetNetplanConfig(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Netplan configuration", config)
	}
}

// handleNetplanSetConfig returns a handler for setting netplan config
func handleNetplanSetConfig(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload types.NetplanConfigRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()

		// Create backup if description provided
		var backupID string
		if payload.BackupDesc != "" {
			id, err := h.manager.BackupNetplanConfig(ctx)
			if err != nil {
				h.logger.Warn("Failed to create backup before config update", "error", err)
			} else {
				backupID = id
			}
		}

		if err := h.manager.SetNetplanConfig(ctx, payload.Config); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message": "Netplan configuration updated successfully",
		}
		if backupID != "" {
			result["backup_id"] = backupID
		}

		return successResponse(req.RequestId, "Netplan config updated", result)
	}
}

// handleNetplanApply returns a handler for applying netplan config
func handleNetplanApply(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		if err := h.manager.ApplyNetplanConfig(ctx); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message": "Netplan configuration applied successfully",
		}

		return successResponse(req.RequestId, "Netplan config applied", result)
	}
}

// handleNetplanTry returns a handler for trying netplan config
func handleNetplanTry(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload types.NetplanTryRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()

		// If config is provided, set it first
		if payload.Config != nil {
			if err := h.manager.SetNetplanConfig(ctx, payload.Config); err != nil {
				return errorResponse(req.RequestId, err)
			}
		}

		timeout := 120 // Default timeout in seconds
		if payload.Timeout > 0 {
			timeout = payload.Timeout
		}

		result, err := h.manager.TryNetplanConfig(ctx, time.Duration(timeout)*time.Second)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Netplan try completed", result)
	}
}

// handleNetplanStatus returns a handler for getting netplan status
func handleNetplanStatus(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Interface string `json:"interface"`
		}

		// Interface is optional
		parseJSONPayload(cmd, &payload)

		ctx := context.Background()
		status, err := h.manager.GetNetplanStatus(ctx, payload.Interface)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Netplan status", status)
	}
}

// handleNetplanDiff returns a handler for getting netplan diff
func handleNetplanDiff(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		diff, err := h.manager.GetNetplanDiff(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Netplan diff", diff)
	}
}

// BACKUP HANDLERS

// handleBackupsList returns a handler for listing backups
func handleBackupsList(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		backups, err := h.manager.ListBackups(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"backups": backups,
			"count":   len(backups),
		}

		return successResponse(req.RequestId, "Configuration backups", result)
	}
}

// handleBackupsCreate returns a handler for creating backups
func handleBackupsCreate(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		backupID, err := h.manager.BackupNetplanConfig(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message":   "Backup created successfully",
			"backup_id": backupID,
		}

		return successResponse(req.RequestId, "Backup created", result)
	}
}

// handleBackupsRestore returns a handler for restoring backups
func handleBackupsRestore(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload types.RestoreRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()
		if err := h.manager.RestoreNetplanConfig(ctx, payload.BackupID); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message":   "Configuration restored successfully",
			"backup_id": payload.BackupID,
		}

		return successResponse(req.RequestId, "Configuration restored", result)
	}
}

// SYSTEM HANDLERS

// handleSystemInfo returns a handler for getting system network info
func handleSystemInfo(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		info, err := h.manager.GetSystemNetworkInfo(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "System network information", info)
	}
}

// VALIDATION HANDLERS

// handleValidateIP returns a handler for validating IP addresses
func handleValidateIP(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Address string `json:"address"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if payload.Address == "" {
			return errorResponse(req.RequestId, errors.New(errors.NetworkIPAddressInvalid, "IP address cannot be empty"))
		}

		var result map[string]interface{}
		if err := h.manager.ValidateIPAddress(payload.Address); err != nil {
			result = map[string]interface{}{
				"valid":   false,
				"error":   err.Error(),
				"address": payload.Address,
			}
		} else {
			result = map[string]interface{}{
				"valid":   true,
				"address": payload.Address,
			}
		}

		return successResponse(req.RequestId, "IP address validation", result)
	}
}

// handleValidateInterfaceName returns a handler for validating interface names
func handleValidateInterfaceName(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if payload.Name == "" {
			return errorResponse(req.RequestId, errors.New(errors.NetworkInterfaceNameInvalid, "Interface name cannot be empty"))
		}

		var result map[string]interface{}
		if err := h.manager.ValidateInterfaceName(payload.Name); err != nil {
			result = map[string]interface{}{
				"valid": false,
				"error": err.Error(),
				"name":  payload.Name,
			}
		} else {
			result = map[string]interface{}{
				"valid": true,
				"name":  payload.Name,
			}
		}

		return successResponse(req.RequestId, "Interface name validation", result)
	}
}

// handleValidateNetplanConfig returns a handler for validating netplan config
func handleValidateNetplanConfig(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload types.NetplanConfigRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()
		var result map[string]interface{}
		if err := h.manager.ValidateNetplanConfig(ctx, payload.Config); err != nil {
			result = map[string]interface{}{
				"valid": false,
				"error": err.Error(),
			}
		} else {
			result = map[string]interface{}{
				"valid": true,
			}
		}

		return successResponse(req.RequestId, "Netplan config validation", result)
	}
}

// DNS HANDLERS

// handleDNSGetGlobal returns a handler for getting global DNS configuration
func handleDNSGetGlobal(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		dns, err := h.manager.GetGlobalDNS(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Global DNS configuration", dns)
	}
}

// handleDNSSetGlobal returns a handler for setting global DNS configuration
func handleDNSSetGlobal(h *NetworkHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload types.GlobalDNSRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		dns := &types.NameserverConfig{
			Addresses: payload.Addresses,
			Search:    payload.Search,
		}

		ctx := context.Background()
		if err := h.manager.SetGlobalDNS(ctx, dns); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message": "Global DNS configuration updated successfully",
			"dns":     dns,
		}

		return successResponse(req.RequestId, "Global DNS updated", result)
	}
}