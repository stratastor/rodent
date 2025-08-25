// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/system"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterSystemGRPCHandlers registers all system-related command handlers with Toggle
func RegisterSystemGRPCHandlers(systemHandler *SystemHandler) {
	// System information operations
	client.RegisterCommandHandler(proto.CmdSystemInfoGet, handleSystemInfoGet(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemInfoCPUGet, handleSystemInfoCPU(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemInfoMemoryGet, handleSystemInfoMemory(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemInfoOSGet, handleSystemInfoOS(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemInfoPerformanceGet, handleSystemInfoPerformance(systemHandler))

	// Hostname management operations
	client.RegisterCommandHandler(proto.CmdSystemHostnameGet, handleHostnameGet(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemHostnameSet, handleHostnameSet(systemHandler))

	// User management operations
	client.RegisterCommandHandler(proto.CmdSystemUsersList, handleUsersList(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemUsersCreate, handleUsersCreate(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemUsersDelete, handleUsersDelete(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemUsersGet, handleUsersGet(systemHandler))

	// Group management operations
	client.RegisterCommandHandler(proto.CmdSystemGroupsList, handleGroupsList(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemGroupsCreate, handleGroupsCreate(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemGroupsDelete, handleGroupsDelete(systemHandler))

	// Power management operations
	client.RegisterCommandHandler(proto.CmdSystemPowerShutdown, handlePowerShutdown(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemPowerReboot, handlePowerReboot(systemHandler))

	// System configuration operations
	client.RegisterCommandHandler(proto.CmdSystemTimezoneGet, handleTimezoneGet(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemTimezoneSet, handleTimezoneSet(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemLocaleGet, handleLocaleGet(systemHandler))
	client.RegisterCommandHandler(proto.CmdSystemLocaleSet, handleLocaleSet(systemHandler))
}

// Helper function to parse JSON payload from a command request
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

// SYSTEM INFORMATION HANDLERS

// handleSystemInfoGet returns a handler for getting complete system information
func handleSystemInfoGet(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		info, err := h.manager.GetSystemInfo(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "System information retrieved", info)
	}
}

// handleSystemInfoCPU returns a handler for getting CPU information
func handleSystemInfoCPU(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		hwInfo, err := h.manager.GetHardwareInfo(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "CPU information retrieved", hwInfo.CPU)
	}
}

// handleSystemInfoMemory returns a handler for getting memory information
func handleSystemInfoMemory(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		hwInfo, err := h.manager.GetHardwareInfo(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Memory information retrieved", hwInfo.Memory)
	}
}

// handleSystemInfoOS returns a handler for getting OS information
func handleSystemInfoOS(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		osInfo, err := h.manager.GetOSInfo(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "OS information retrieved", osInfo)
	}
}

// handleSystemInfoPerformance returns a handler for getting performance information
func handleSystemInfoPerformance(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		perfInfo, err := h.manager.GetPerformanceInfo(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Performance information retrieved", perfInfo)
	}
}

// HOSTNAME MANAGEMENT HANDLERS

// handleHostnameGet returns a handler for getting the system hostname
func handleHostnameGet(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		hostname, err := h.manager.GetHostname(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"hostname": hostname,
		}

		return successResponse(req.RequestId, "Hostname retrieved", result)
	}
}

// handleHostnameSet returns a handler for setting the system hostname
func handleHostnameSet(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload system.SetHostnameRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()
		if err := h.manager.SetHostname(ctx, payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message":  "Hostname set successfully",
			"hostname": payload.Hostname,
		}

		return successResponse(req.RequestId, "Hostname set", result)
	}
}

// USER MANAGEMENT HANDLERS

// handleUsersList returns a handler for listing system users
func handleUsersList(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		users, err := h.manager.GetUsers(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"users": users,
			"count": len(users),
		}

		return successResponse(req.RequestId, "Users list retrieved", result)
	}
}

// handleUsersGet returns a handler for getting a specific user
func handleUsersGet(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Username string `json:"username"`
		}
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if payload.Username == "" {
			return errorResponse(req.RequestId, errors.New(errors.ServerRequestValidation, "Username cannot be empty"))
		}

		ctx := context.Background()
		user, err := h.manager.GetUser(ctx, payload.Username)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "User information retrieved", user)
	}
}

// handleUsersCreate returns a handler for creating a system user
func handleUsersCreate(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload system.CreateUserRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()
		if err := h.manager.CreateUser(ctx, payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message":  "User created successfully",
			"username": payload.Username,
		}

		return successResponse(req.RequestId, "User created", result)
	}
}

// handleUsersDelete returns a handler for deleting a system user
func handleUsersDelete(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Username string `json:"username"`
		}
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if payload.Username == "" {
			return errorResponse(req.RequestId, errors.New(errors.ServerRequestValidation, "Username cannot be empty"))
		}

		ctx := context.Background()
		if err := h.manager.DeleteUser(ctx, payload.Username); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message":  "User deleted successfully",
			"username": payload.Username,
		}

		return successResponse(req.RequestId, "User deleted", result)
	}
}

// GROUP MANAGEMENT HANDLERS

// handleGroupsList returns a handler for listing system groups
func handleGroupsList(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		groups, err := h.manager.GetGroups(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"groups": groups,
			"count":  len(groups),
		}

		return successResponse(req.RequestId, "Groups list retrieved", result)
	}
}

// handleGroupsCreate returns a handler for creating a system group
func handleGroupsCreate(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload system.CreateGroupRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()
		if err := h.manager.CreateGroup(ctx, payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message": "Group created successfully",
			"name":    payload.Name,
		}

		return successResponse(req.RequestId, "Group created", result)
	}
}

// handleGroupsDelete returns a handler for deleting a system group
func handleGroupsDelete(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if payload.Name == "" {
			return errorResponse(req.RequestId, errors.New(errors.ServerRequestValidation, "Group name cannot be empty"))
		}

		ctx := context.Background()
		if err := h.manager.DeleteGroup(ctx, payload.Name); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message": "Group deleted successfully",
			"name":    payload.Name,
		}

		return successResponse(req.RequestId, "Group deleted", result)
	}
}

// POWER MANAGEMENT HANDLERS

// handlePowerShutdown returns a handler for shutting down the system
func handlePowerShutdown(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload system.PowerOperationRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			// Allow empty payload for simple shutdown
			payload = system.PowerOperationRequest{}
		}

		h.logger.Warn("System shutdown requested via gRPC", "request_id", req.RequestId)

		ctx := context.Background()
		if err := h.manager.Shutdown(ctx, payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message": "System shutdown initiated",
		}

		return successResponse(req.RequestId, "Shutdown initiated", result)
	}
}

// handlePowerReboot returns a handler for rebooting the system
func handlePowerReboot(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload system.PowerOperationRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			// Allow empty payload for simple reboot
			payload = system.PowerOperationRequest{}
		}

		h.logger.Warn("System reboot requested via gRPC", "request_id", req.RequestId)

		ctx := context.Background()
		if err := h.manager.Reboot(ctx, payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message": "System reboot initiated",
		}

		return successResponse(req.RequestId, "Reboot initiated", result)
	}
}

// SYSTEM CONFIGURATION HANDLERS

// handleTimezoneGet returns a handler for getting the system timezone
func handleTimezoneGet(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		timezone, err := h.manager.GetTimezone(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"timezone": timezone,
		}

		return successResponse(req.RequestId, "Timezone retrieved", result)
	}
}

// handleTimezoneSet returns a handler for setting the system timezone
func handleTimezoneSet(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload system.SetTimezoneRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()
		if err := h.manager.SetTimezone(ctx, payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message":  "Timezone set successfully",
			"timezone": payload.Timezone,
		}

		return successResponse(req.RequestId, "Timezone set", result)
	}
}

// handleLocaleGet returns a handler for getting the system locale
func handleLocaleGet(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		locale, err := h.manager.GetLocale(ctx)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"locale": locale,
		}

		return successResponse(req.RequestId, "Locale retrieved", result)
	}
}

// handleLocaleSet returns a handler for setting the system locale
func handleLocaleSet(h *SystemHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload system.SetLocaleRequest
		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		ctx := context.Background()
		if err := h.manager.SetLocale(ctx, payload); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result := map[string]interface{}{
			"message": "Locale set successfully",
			"locale":  payload.Locale,
		}

		return successResponse(req.RequestId, "Locale set", result)
	}
}