// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// CommandHandler is a function type for handling specific command types
type CommandHandler func(cmd *proto.CommandRequest) (*proto.CommandResponse, error)

// commandHandlers stores command handlers by command type
var commandHandlers = map[string]CommandHandler{}

// RegisterCommandHandler registers a handler for a specific command type
func RegisterCommandHandler(commandType string, handler CommandHandler) {
	commandHandlers[commandType] = handler
}

// getRegisteredCommandTypes returns a list of registered command types for debugging
func getRegisteredCommandTypes() []string {
	types := make([]string, 0, len(commandHandlers))
	for cmdType := range commandHandlers {
		types = append(types, cmdType)
	}
	return types
}

// init registers default command handlers
func init() {
	// The Toggle server sends "system.status" commands to check on node health
	RegisterCommandHandler("system.status", func(cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Collect basic system metrics
		// In a production environment, this would collect real system metrics
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
			return nil, fmt.Errorf("failed to marshal metrics: %w", err)
		}
		
		return &proto.CommandResponse{
			Success: true,
			Message: "System status",
			Payload: payload,
		}, nil
	})
	
	// Also register "status" since the server might use this format with "system" target
	RegisterCommandHandler("status", func(cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// If not the system target, return unsupported
		if cmd.Target != "system" {
			return nil, fmt.Errorf("unsupported target for status command: %s", cmd.Target)
		}
		
		// Collect basic system metrics (same as system.status)
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
			return nil, fmt.Errorf("failed to marshal metrics: %w", err)
		}
		
		return &proto.CommandResponse{
			Success: true,
			Message: "System status",
			Payload: payload,
		}, nil
	})
}

// HandleToggleRequests starts a goroutine that processes incoming Toggle requests
// and responds appropriately to commands including status/heartbeat requests
func (c *StreamConnection) HandleToggleRequests() {
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
				
				// Process the request based on payload type
				switch payload := req.Payload.(type) {
				case *proto.ToggleRequest_Command:
					// Handle command requests
					cmd := payload.Command
					
					c.client.Logger.Debug("Received command from Toggle", 
						"type", cmd.CommandType, 
						"target", cmd.Target,
						"request_id", req.RequestId,
						"payload_size", len(cmd.Payload),
						"registered_types", fmt.Sprintf("%v", getRegisteredCommandTypes()))
					
					var response *proto.CommandResponse
					var err error
					
					// Special handling for system status requests
					if (cmd.CommandType == "system.status") || 
					   (cmd.CommandType == "status" && cmd.Target == "system") {
						// This is a heartbeat/status request - log explicitly
						c.client.Logger.Info("Responding to heartbeat/status request from Toggle", 
							"request_id", req.RequestId,
							"command_type", cmd.CommandType,
							"target", cmd.Target,
							"session_id", c.sessionID)
						
						// Create system metrics for heartbeat response
						metrics := map[string]interface{}{
							"status":       "healthy",
							"timestamp":    time.Now().Unix(),
							"cpu_usage":    0.0,
							"memory_usage": 0.0,
							"disk_usage":   0.0,
						}
						
						payload, err := json.Marshal(metrics)
						if err != nil {
							c.client.Logger.Error("Failed to marshal status metrics", "error", err)
							payload = []byte(`{"status":"healthy"}`)
						}
						
						response = &proto.CommandResponse{
							RequestId: req.RequestId, // CRITICAL: Use the original request ID here
							Success:   true,
							Message:   "System status response",
							Payload:   payload,
						}
					} else {
						// Look for a registered handler for this command type
						handler, exists := commandHandlers[cmd.CommandType]
						
						if exists {
							// Execute the handler
							response, err = handler(cmd)
							if err != nil {
								c.client.Logger.Error("Command handler failed", 
									"type", cmd.CommandType, 
									"error", err)
								
								// Create error response
								response = &proto.CommandResponse{
									RequestId: req.RequestId, // Use original request ID
									Success:   false,
									Message:   fmt.Sprintf("Command failed: %v", err),
								}
							}
						} else {
							// No handler found
							c.client.Logger.Warn("No handler for command type", 
								"type", cmd.CommandType,
								"target", cmd.Target,
								"registered_handlers", fmt.Sprintf("%v", getRegisteredCommandTypes()))
							
							response = &proto.CommandResponse{
								RequestId: req.RequestId, // Use original request ID
								Success:   false,
								Message:   fmt.Sprintf("Unsupported command type: %s", cmd.CommandType),
							}
						}
					}
					
					// Add request ID from Toggle request if missing
					if response.RequestId == "" {
						response.RequestId = req.RequestId
					}
					
					// CRITICAL DEBUG LOGGING FOR STATUS REQUESTS
					if cmd.CommandType == "system.status" {
						c.client.Logger.Info("Sending system status response", 
							"original_request_id", req.RequestId,
							"response_request_id", response.RequestId)
					}
					
					// Create and send the response
					// CRITICAL: For system status requests, we use the SAME request ID for both the outer
					// message and the inner CommandResponse to ensure Toggle can correlate them properly
					requestID := uuid.New().String()
					if cmd.CommandType == "system.status" {
						// For system.status, use the original request ID for both
						requestID = req.RequestId
					}
					
					rodentReq := &proto.RodentRequest{
						SessionId: c.sessionID,
						RequestId: requestID, // Use original request ID for system.status, new ID for others
						Payload: &proto.RodentRequest_CommandResponse{
							CommandResponse: response, // This has the original request ID
						},
					}
					
					if err := c.Send(rodentReq); err != nil {
						c.client.Logger.Warn("Failed to send command response", "error", err)
					} else {
						c.client.Logger.Debug("Successfully sent command response", 
							"original_request_id", req.RequestId,
							"success", response.Success)
					}
					
				case *proto.ToggleRequest_Config:
					// Handle config updates
					configUpdate := payload.Config
					c.client.Logger.Debug("Received config update from Toggle", 
						"type", configUpdate.ConfigType)
					
					// Send acknowledgment
					ack := &proto.RodentRequest{
						SessionId: c.sessionID,
						RequestId: uuid.New().String(),
						Payload: &proto.RodentRequest_Ack{
							Ack: &proto.Acknowledgement{
								RequestId: req.RequestId, // Use original request ID
								Success:   true,
								Message:   fmt.Sprintf("Received config update type: %s", configUpdate.ConfigType),
							},
						},
					}
					
					if err := c.Send(ack); err != nil {
						c.client.Logger.Warn("Failed to send config update acknowledgment", "error", err)
					}
					
					// TODO: Apply the config update based on the type
					// This would likely involve unmarshaling the payload and applying changes
					
				case *proto.ToggleRequest_Ack:
					// Handle acknowledgments
					ack := payload.Ack
					c.client.Logger.Debug("Received acknowledgment from Toggle", 
						"request_id", ack.RequestId,
						"success", ack.Success,
						"message", ack.Message)
					
					// TODO: Update any pending request status based on the acknowledgment
				}
			}
		}
	}()
}