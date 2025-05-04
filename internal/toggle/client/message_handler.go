// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

// Package client provides GRPC client implementation for Toggle communication
package client

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// CommandHandler is a function that processes a specific command type
// It receives the original Toggle request and the command payload
// This signature gives handlers access to the original request context including request ID
type CommandHandler func(toggleReq *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error)

// Domain-specific handler registries
var (
	// Map of command type to handler function
	commandHandlers = make(map[string]CommandHandler)
)

// GetRegisteredCommands returns a list of registered command types for debugging
func GetRegisteredCommands() []string {
	types := make([]string, 0, len(commandHandlers))
	for cmdType := range commandHandlers {
		types = append(types, cmdType)
	}
	return types
}

// RegisterCommandHandler registers a handler for a specific command type
func RegisterCommandHandler(commandType string, handler CommandHandler) {
	commandHandlers[commandType] = handler
}

// StartMessageHandler initializes and starts the message handling service
// It processes messages from the inbound channel and dispatches them to appropriate handlers
func (c *StreamConnection) StartMessageHandler() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		for {
			select {
			case <-c.stopChan:
				return
			case req, ok := <-c.inboundChan:
				if !ok {
					c.client.Logger.Warn("Inbound channel closed unexpectedly")
					return
				}

				// Process each request in its own goroutine to prevent blocking
				go c.processToggleRequest(req)
			}
		}
	}()
}

// processToggleRequest handles a single request from Toggle
func (c *StreamConnection) processToggleRequest(req *proto.ToggleRequest) {
	// Process the request based on payload type
	switch payload := req.Payload.(type) {
	case *proto.ToggleRequest_Command:
		c.handleCommandRequest(req, payload.Command)
	case *proto.ToggleRequest_Config:
		c.handleConfigRequest(req, payload.Config)
	case *proto.ToggleRequest_Ack:
		c.handleAcknowledgment(req, payload.Ack)
	default:
		c.client.Logger.Warn("Received unknown payload type",
			"request_id", req.RequestId)
	}
}

// handleCommandRequest processes command requests from Toggle
func (c *StreamConnection) handleCommandRequest(
	req *proto.ToggleRequest,
	cmd *proto.CommandRequest,
) {
	cmdType := cmd.CommandType
	target := cmd.Target

	c.client.Logger.Debug("Processing command request from Toggle",
		"request_id", req.RequestId,
		"command_type", cmdType,
		"target", target)

	// Find registered handler
	handler, exists := commandHandlers[cmdType]

	var response *proto.CommandResponse

	if exists {
		// Execute the registered handler with the full Toggle request context
		var err error
		response, err = handler(req, cmd)
		if err != nil {
			c.client.Logger.Error("Command handler failed",
				"request_id", req.RequestId,
				"command_type", cmdType,
				"error", err)

			// Create structured error response
			response = errors.ErrorResponse(req.RequestId, err)
		}
	} else {
		// No handler found
		c.client.Logger.Warn("No handler for command type",
			"request_id", req.RequestId,
			"command_type", cmdType,
			"registered_handlers", fmt.Sprintf("%v", GetRegisteredCommands()))

		// Create error with unsupported command information
		errMsg := fmt.Sprintf("Unsupported command type: %s", cmdType)
		rodentErr := errors.New(errors.ServerRequestValidation, errMsg)
		response = errors.ErrorResponse(req.RequestId, rodentErr)
	}

	// Ensure response has the request ID
	if response.RequestId == "" {
		response.RequestId = req.RequestId
	}

	// Send response using the same original request ID
	// This is CRITICAL: Toggle needs matching request IDs to correlate requests and responses
	rodentReq := &proto.RodentRequest{
		RequestId: req.RequestId, // Always use the original request ID
		Payload: &proto.RodentRequest_CommandResponse{
			CommandResponse: response,
		},
	}

	if err := c.Send(rodentReq); err != nil {
		c.client.Logger.Error("Failed to send command response",
			"request_id", req.RequestId,
			"error", err)
	} else {
		c.client.Logger.Debug("Successfully sent command response",
			"request_id", req.RequestId,
			"command_type", cmdType,
			"success", response.Success)
	}
}

// handleConfigRequest processes configuration updates from Toggle
func (c *StreamConnection) handleConfigRequest(
	req *proto.ToggleRequest,
	config *proto.ConfigUpdate,
) {
	c.client.Logger.Debug("Received config update from Toggle",
		"request_id", req.RequestId,
		"config_type", config.ConfigType)

	// Send acknowledgment with the same request ID for correlation
	rodentReq := &proto.RodentRequest{
		RequestId: req.RequestId, // Use the original request ID
		Payload: &proto.RodentRequest_Ack{
			Ack: &proto.Acknowledgement{
				RequestId: req.RequestId, // Match the original request ID here too
				Success:   true,
				Message:   fmt.Sprintf("Received config update type: %s", config.ConfigType),
			},
		},
	}

	if err := c.Send(rodentReq); err != nil {
		c.client.Logger.Error("Failed to send config update acknowledgment",
			"request_id", req.RequestId,
			"error", err)
	}

	// TODO: Apply the config update based on the type
}

// handleAcknowledgment processes acknowledgments from Toggle
func (c *StreamConnection) handleAcknowledgment(
	req *proto.ToggleRequest,
	ack *proto.Acknowledgement,
) {
	c.client.Logger.Debug("Received acknowledgment from Toggle",
		"request_id", req.RequestId,
		"ack_request_id", ack.RequestId,
		"success", ack.Success,
		"message", ack.Message)

	// TODO: Update any pending request status based on the acknowledgment
}

// Register built-in system handlers
func init() {
	// TODO: Move this to RegisterAllHandlers() in internal/toggle/handlers.go
	// Register the system.status handler
	RegisterCommandHandler("system.status", handleSystemStatus)

	// Also register "status" with system target
	RegisterCommandHandler(
		"status",
		func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
			// Check if this is a system target
			if cmd.Target != "system" {
				err := errors.New(errors.ServerRequestValidation,
					fmt.Sprintf("Unsupported target for status command: %s", cmd.Target))
				return errors.ErrorResponse(req.RequestId, err), nil
			}

			// Delegate to the system.status handler
			return handleSystemStatus(req, cmd)
		},
	)
}

// handleSystemStatus is the default handler for system status requests
func handleSystemStatus(
	req *proto.ToggleRequest,
	cmd *proto.CommandRequest,
) (*proto.CommandResponse, error) {
	// Collect basic system metrics
	metrics := map[string]interface{}{
		"status":       "healthy",
		"timestamp":    time.Now().Unix(),
		"cpu_usage":    0.0,
		"memory_usage": 0.0,
		"disk_usage":   0.0,
	}

	// Marshal metrics to JSON
	payload, err := json.Marshal(metrics)
	if err != nil {
		return nil, errors.Wrap(err, errors.ServerResponseError)
	}

	// Return response with the same request ID for correlation
	return errors.SuccessResponse(
		req.RequestId,
		"System status response",
		payload,
	), nil
}
