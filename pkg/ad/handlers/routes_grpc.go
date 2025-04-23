// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// Command operation constants for AD gRPC handlers
const (
	// User operations
	CmdUserList   = "ad.user.list"
	CmdUserGet    = "ad.user.get"
	CmdUserCreate = "ad.user.create"
	CmdUserUpdate = "ad.user.update"
	CmdUserDelete = "ad.user.delete"
	CmdUserGroups = "ad.user.groups"

	// Group operations
	CmdGroupList          = "ad.group.list"
	CmdGroupGet           = "ad.group.get"
	CmdGroupCreate        = "ad.group.create"
	CmdGroupUpdate        = "ad.group.update"
	CmdGroupDelete        = "ad.group.delete"
	CmdGroupMembers       = "ad.group.members"
	CmdGroupAddMembers    = "ad.group.members.add"
	CmdGroupRemoveMembers = "ad.group.members.remove"

	// Computer operations
	CmdComputerList   = "ad.computer.list"
	CmdComputerGet    = "ad.computer.get"
	CmdComputerCreate = "ad.computer.create"
	CmdComputerUpdate = "ad.computer.update"
	CmdComputerDelete = "ad.computer.delete"
)

// RegisterADGRPCHandlers registers all AD-related command handlers with Toggle
func RegisterADGRPCHandlers(adHandler *ADHandler) {
	// User operations
	client.RegisterCommandHandler(CmdUserList, handleUserList(adHandler))
	client.RegisterCommandHandler(CmdUserGet, handleUserGet(adHandler))
	client.RegisterCommandHandler(CmdUserCreate, handleUserCreate(adHandler))
	client.RegisterCommandHandler(CmdUserUpdate, handleUserUpdate(adHandler))
	client.RegisterCommandHandler(CmdUserDelete, handleUserDelete(adHandler))
	client.RegisterCommandHandler(CmdUserGroups, handleUserGroups(adHandler))

	// Group operations
	client.RegisterCommandHandler(CmdGroupList, handleGroupList(adHandler))
	client.RegisterCommandHandler(CmdGroupGet, handleGroupGet(adHandler))
	client.RegisterCommandHandler(CmdGroupCreate, handleGroupCreate(adHandler))
	client.RegisterCommandHandler(CmdGroupUpdate, handleGroupUpdate(adHandler))
	client.RegisterCommandHandler(CmdGroupDelete, handleGroupDelete(adHandler))
	client.RegisterCommandHandler(CmdGroupMembers, handleGroupMembers(adHandler))
	client.RegisterCommandHandler(CmdGroupAddMembers, handleGroupAddMembers(adHandler))
	client.RegisterCommandHandler(CmdGroupRemoveMembers, handleGroupRemoveMembers(adHandler))

	// Computer operations
	client.RegisterCommandHandler(CmdComputerList, handleComputerList(adHandler))
	client.RegisterCommandHandler(CmdComputerGet, handleComputerGet(adHandler))
	client.RegisterCommandHandler(CmdComputerCreate, handleComputerCreate(adHandler))
	client.RegisterCommandHandler(CmdComputerUpdate, handleComputerUpdate(adHandler))
	client.RegisterCommandHandler(CmdComputerDelete, handleComputerDelete(adHandler))
}

// Helper for parsing JSON payload from a command request
func parseJSONPayload(cmd *proto.CommandRequest, out interface{}) error {
	if len(cmd.Payload) == 0 {
		return fmt.Errorf("empty payload")
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

// errorResponse creates an error response with the provided message
func errorResponse(requestID string, err error) (*proto.CommandResponse, error) {
	return &proto.CommandResponse{
		RequestId: requestID,
		Success:   false,
		Message:   err.Error(),
	}, nil
}

// USER HANDLERS

// handleUserList returns a handler for listing AD users
func handleUserList(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Call the client's List method
		entries, err := h.client.ListUsers()
		if err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADSearchFailed))
		}

		// Return success response with the result
		return successResponse(req.RequestId, "AD users list", entries)
	}
}

// handleUserGet returns a handler for getting a specific AD user
func handleUserGet(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Username string `json:"username"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.Username == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "username is required"),
			)
		}

		// Call the client's Search method
		entries, err := h.client.SearchUser(payload.Username)
		if err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADSearchFailed))
		}

		if len(entries) == 0 {
			return errorResponse(req.RequestId, errors.New(errors.ADUserNotFound, "User not found"))
		}

		// Return success response with the result
		return successResponse(req.RequestId, "AD user details", entries[0])
	}
}

// handleUserCreate returns a handler for creating a new AD user
func handleUserCreate(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var userReq UserRequest
		if err := parseJSONPayload(cmd, &userReq); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		// Validate required fields
		if userReq.CN == "" || userReq.SAMAccountName == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN and SAM account name are required"),
			)
		}

		// Convert API request to AD User model
		user := userReq.toADUser()

		// Call the client's Create method
		if err := h.client.CreateUser(user); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADCreateUserFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "User created successfully", nil)
	}
}

// handleUserUpdate returns a handler for updating an existing AD user
func handleUserUpdate(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var userReq UserRequest
		if err := parseJSONPayload(cmd, &userReq); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		// Validate required fields
		if userReq.CN == "" || userReq.SAMAccountName == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN and SAM account name are required"),
			)
		}

		// Convert API request to AD User model
		user := userReq.toADUser()

		// Call the client's Update method
		if err := h.client.UpdateUser(user); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADUpdateUserFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "User updated successfully", nil)
	}
}

// handleUserDelete returns a handler for deleting an AD user
func handleUserDelete(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			CN string `json:"cn"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.CN == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN is required"),
			)
		}

		// Call the client's Delete method
		if err := h.client.DeleteUser(payload.CN); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADDeleteUserFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "User deleted successfully", nil)
	}
}

// handleUserGroups returns a handler for getting AD user's group memberships
func handleUserGroups(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Username string `json:"username"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.Username == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "username is required"),
			)
		}

		// Call the client's GetUserGroups method
		groups, err := h.client.GetUserGroups(payload.Username)
		if err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADSearchFailed))
		}

		// Return success response with the result
		return successResponse(req.RequestId, "User group memberships", map[string][]string{
			"groups": groups,
		})
	}
}

// GROUP HANDLERS

// handleGroupList returns a handler for listing AD groups
func handleGroupList(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Call the client's List method
		entries, err := h.client.ListGroups()
		if err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADSearchFailed))
		}

		// Return success response with the result
		return successResponse(req.RequestId, "AD groups list", entries)
	}
}

// handleGroupGet returns a handler for getting a specific AD group
func handleGroupGet(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Groupname string `json:"groupname"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.Groupname == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "groupname is required"),
			)
		}

		// Call the client's Search method
		entries, err := h.client.SearchGroup(payload.Groupname)
		if err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADSearchFailed))
		}

		if len(entries) == 0 {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ADGroupNotFound, "Group not found"),
			)
		}

		// Return success response with the result
		return successResponse(req.RequestId, "AD group details", entries[0])
	}
}

// handleGroupCreate returns a handler for creating a new AD group
func handleGroupCreate(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var groupReq GroupRequest
		if err := parseJSONPayload(cmd, &groupReq); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		// Validate required fields
		if groupReq.CN == "" || groupReq.SAMAccountName == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN and SAM account name are required"),
			)
		}

		// Convert API request to AD Group model
		group := groupReq.toADGroup()

		// Call the client's Create method
		if err := h.client.CreateGroup(group); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADCreateGroupFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "Group created successfully", nil)
	}
}

// handleGroupUpdate returns a handler for updating an existing AD group
func handleGroupUpdate(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var groupReq GroupRequest
		if err := parseJSONPayload(cmd, &groupReq); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		// Validate required fields
		if groupReq.CN == "" || groupReq.SAMAccountName == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN and SAM account name are required"),
			)
		}

		// Convert API request to AD Group model
		group := groupReq.toADGroup()

		// Call the client's Update method
		if err := h.client.UpdateGroup(group); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADUpdateGroupFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "Group updated successfully", nil)
	}
}

// handleGroupDelete returns a handler for deleting an AD group
func handleGroupDelete(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			CN string `json:"cn"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.CN == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN is required"),
			)
		}
		// Call the client's Delete method
		if err := h.client.DeleteGroup(payload.CN); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADDeleteGroupFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "Group deleted successfully", nil)
	}
}

// handleGroupMembers returns a handler for getting AD group's members
func handleGroupMembers(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Groupname string `json:"groupname"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.Groupname == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "groupname is required"),
			)
		}

		// Call the client's GetGroupMembers method
		members, err := h.client.GetGroupMembers(payload.Groupname)
		if err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADSearchFailed))
		}

		// Return success response with the result
		return successResponse(req.RequestId, "Group members", map[string][]string{
			"members": members,
		})
	}
}

// handleGroupAddMembers returns a handler for adding members to an AD group
func handleGroupAddMembers(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			CN      string   `json:"cn"`
			Members []string `json:"members"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.CN == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN is required"),
			)
		}

		if len(payload.Members) == 0 {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "at least one member is required"),
			)
		}

		// Call the client's AddMembersToGroup method
		if err := h.client.AddMembersToGroup(payload.Members, payload.CN); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADUpdateGroupFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "Members added to group successfully", nil)
	}
}

// handleGroupRemoveMembers returns a handler for removing members from an AD group
func handleGroupRemoveMembers(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			CN      string   `json:"cn"`
			Members []string `json:"members"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.CN == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN is required"),
			)
		}

		if len(payload.Members) == 0 {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "at least one member is required"),
			)
		}

		// Call the client's RemoveMembersFromGroup method
		if err := h.client.RemoveMembersFromGroup(payload.Members, payload.CN); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADUpdateGroupFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "Members removed from group successfully", nil)
	}
}

// COMPUTER HANDLERS

// handleComputerList returns a handler for listing AD computers
func handleComputerList(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Call the client's List method
		entries, err := h.client.ListComputers()
		if err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADSearchFailed))
		}

		// Return success response with the result
		return successResponse(req.RequestId, "AD computers list", entries)
	}
}

// handleComputerGet returns a handler for getting a specific AD computer
func handleComputerGet(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Computername string `json:"computername"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.Computername == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "computername is required"),
			)
		}

		// Call the client's Search method
		entries, err := h.client.SearchComputer(payload.Computername)
		if err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADSearchFailed))
		}

		// Return success response with the result
		// Note: Matching the HTTP API which returns empty array if no computers found
		if len(entries) == 0 {
			return successResponse(req.RequestId, "AD computer details", []interface{}{})
		}

		return successResponse(req.RequestId, "AD computer details", entries)
	}
}

// handleComputerCreate returns a handler for creating a new AD computer
func handleComputerCreate(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var computerReq ComputerRequest
		if err := parseJSONPayload(cmd, &computerReq); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		// Validate required fields
		if computerReq.CN == "" || computerReq.SAMAccountName == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN and SAM account name are required"),
			)
		}

		// Convert API request to AD Computer model
		computer := computerReq.toADComputer()

		// Call the client's Create method
		if err := h.client.CreateComputer(computer); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADCreateComputerFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "Computer created successfully", nil)
	}
}

// handleComputerUpdate returns a handler for updating an existing AD computer
func handleComputerUpdate(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var computerReq ComputerRequest
		if err := parseJSONPayload(cmd, &computerReq); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		// Validate required fields
		if computerReq.CN == "" || computerReq.SAMAccountName == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN and SAM account name are required"),
			)
		}

		// Convert API request to AD Computer model
		computer := computerReq.toADComputer()

		// Call the client's Update method
		if err := h.client.UpdateComputer(computer); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADUpdateComputerFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "Computer updated successfully", nil)
	}
}

// handleComputerDelete returns a handler for deleting an AD computer
func handleComputerDelete(h *ADHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			CN string `json:"cn"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerRequestValidation))
		}

		if payload.CN == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "CN is required"),
			)
		}

		// Call the client's Delete method
		if err := h.client.DeleteComputer(payload.CN); err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ADDeleteComputerFailed))
		}

		// Return success response
		return successResponse(req.RequestId, "Computer deleted successfully", nil)
	}
}
