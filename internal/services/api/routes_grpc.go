// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"

	"github.com/stratastor/rodent/internal/services"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterServiceGRPCHandlers registers all service command handlers with Toggle
func RegisterServiceGRPCHandlers(handler *ServiceHandler) {
	// Service listing and status operations
	client.RegisterCommandHandler(proto.CmdServicesList, handleListServices(handler))
	client.RegisterCommandHandler(proto.CmdServicesStatuses, handleGetAllServiceStatuses(handler))
	client.RegisterCommandHandler(proto.CmdServiceStatus, handleGetServiceStatus(handler))

	// Service lifecycle operations
	client.RegisterCommandHandler(proto.CmdServiceStart, handleStartService(handler))
	client.RegisterCommandHandler(proto.CmdServiceStop, handleStopService(handler))
	client.RegisterCommandHandler(proto.CmdServiceRestart, handleRestartService(handler))

	// Service startup management operations
	client.RegisterCommandHandler(proto.CmdServiceIsEnabled, handleGetStartupStatus(handler))
	client.RegisterCommandHandler(proto.CmdServiceEnable, handleEnableService(handler))
	client.RegisterCommandHandler(proto.CmdServiceDisable, handleDisableService(handler))
}

// Helper function to create a success response
func successResponse(
	requestID string,
	message string,
	data interface{},
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

// Helper function to parse JSON payload
func parseJSONPayload(cmd *proto.CommandRequest, out interface{}) error {
	if len(cmd.Payload) == 0 {
		return errors.New(errors.ServerRequestValidation, "empty payload")
	}
	return json.Unmarshal(cmd.Payload, out)
}

// handleListServices handles requests to list all available services
func handleListServices(handler *ServiceHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		services := handler.manager.ListServices()

		response := map[string]interface{}{
			"services": services,
		}

		return successResponse(req.RequestId, "Services listed successfully", response)
	}
}

// handleGetAllServiceStatuses handles requests to get the status of all services
func handleGetAllServiceStatuses(handler *ServiceHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		statuses := handler.manager.GetAllServiceStatuses(context.Background())

		response := map[string]interface{}{
			"statuses": statuses,
		}

		return successResponse(req.RequestId, "Service statuses retrieved successfully", response)
	}
}

// handleGetServiceStatus handles requests to get the status of a specific service
func handleGetServiceStatus(handler *ServiceHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate service name
		if payload.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "service name is required")
		}

		svc, ok := handler.manager.GetService(payload.Name)
		if !ok {
			return nil, errors.New(errors.ServiceNotFound, "service not found")
		}

		status, err := svc.Status(context.Background())
		if err != nil {
			return nil, err
		}

		// Status could be a simple string or an array of statuses for multi-services like samba
		response := map[string]interface{}{
			"name":   payload.Name,
			"status": status,
		}

		return successResponse(req.RequestId, "Service status retrieved successfully", response)
	}
}

// handleStartService handles requests to start a specific service
func handleStartService(handler *ServiceHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate service name
		if payload.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "service name is required")
		}

		if err := handler.manager.StartService(context.Background(), payload.Name); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"name":    payload.Name,
			"message": "service started successfully",
		}

		return successResponse(req.RequestId, "Service started successfully", response)
	}
}

// handleStopService handles requests to stop a specific service
func handleStopService(handler *ServiceHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate service name
		if payload.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "service name is required")
		}

		if err := handler.manager.StopService(context.Background(), payload.Name); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"name":    payload.Name,
			"message": "service stopped successfully",
		}

		return successResponse(req.RequestId, "Service stopped successfully", response)
	}
}

// handleRestartService handles requests to restart a specific service
func handleRestartService(handler *ServiceHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate service name
		if payload.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "service name is required")
		}

		if err := handler.manager.RestartService(context.Background(), payload.Name); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"name":    payload.Name,
			"message": "service restarted successfully",
		}

		return successResponse(req.RequestId, "Service restarted successfully", response)
	}
}

// handleGetStartupStatus handles requests to get the startup status of a service
func handleGetStartupStatus(handler *ServiceHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate service name
		if payload.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "service name is required")
		}

		svc, ok := handler.manager.GetService(payload.Name)
		if !ok {
			return nil, errors.New(errors.ServiceNotFound, "service not found")
		}

		// Check if service supports startup management
		startupSvc, ok := svc.(services.StartupService)
		if !ok {
			return nil, errors.New(
				errors.ServerBadRequest,
				"service does not support startup management",
			)
		}

		enabled, err := startupSvc.IsEnabledAtStartup(context.Background())
		if err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"name":    payload.Name,
			"enabled": enabled,
		}

		return successResponse(
			req.RequestId,
			"Service startup status retrieved successfully",
			response,
		)
	}
}

// handleEnableService handles requests to enable a service at startup
func handleEnableService(handler *ServiceHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate service name
		if payload.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "service name is required")
		}

		svc, ok := handler.manager.GetService(payload.Name)
		if !ok {
			return nil, errors.New(errors.ServiceNotFound, "service not found")
		}

		// Check if service supports startup management
		startupSvc, ok := svc.(services.StartupService)
		if !ok {
			return nil, errors.New(
				errors.ServerBadRequest,
				"service does not support startup management",
			)
		}

		if err := startupSvc.EnableAtStartup(context.Background()); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"name":    payload.Name,
			"message": "service enabled to start at system boot",
		}

		return successResponse(req.RequestId, "Service enabled successfully", response)
	}
}

// handleDisableService handles requests to disable a service at startup
func handleDisableService(handler *ServiceHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var payload struct {
			Name string `json:"name"`
		}

		if err := parseJSONPayload(cmd, &payload); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate service name
		if payload.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "service name is required")
		}

		svc, ok := handler.manager.GetService(payload.Name)
		if !ok {
			return nil, errors.New(errors.ServiceNotFound, "service not found")
		}

		// Check if service supports startup management
		startupSvc, ok := svc.(services.StartupService)
		if !ok {
			return nil, errors.New(
				errors.ServerBadRequest,
				"service does not support startup management",
			)
		}

		if err := startupSvc.DisableAtStartup(context.Background()); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"name":    payload.Name,
			"message": "service disabled from starting at system boot",
		}

		return successResponse(req.RequestId, "Service disabled successfully", response)
	}
}
