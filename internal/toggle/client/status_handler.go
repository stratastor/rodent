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

// HandleSystemStatus is a specialized handler for system.status commands
// It ensures proper correlation between the request and response IDs
func (c *StreamConnection) HandleSystemStatus(req *proto.ToggleRequest, cmd *proto.CommandRequest) error {
	c.client.Logger.Info("Handling system.status request from Toggle", 
		"request_id", req.RequestId,
		"session_id", c.sessionID)
	
	// Create system metrics for heartbeat response
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
		c.client.Logger.Error("Failed to marshal status metrics", "error", err)
		payload = []byte(`{"status":"healthy"}`)
	}
	
	// Create the command response
	// CRITICAL: The CommandResponse must use the EXACT same RequestId as the original request
	commandResponse := &proto.CommandResponse{
		RequestId: req.RequestId,  // Original request ID from Toggle
		Success:   true,
		Message:   "System status response",
		Payload:   payload,
	}
	
	// Create the outer RodentRequest
	// CRITICAL: The RodentRequest must ALSO use the same RequestId for system.status
	rodentReq := &proto.RodentRequest{
		SessionId: c.sessionID,
		RequestId: req.RequestId,  // Use the SAME request ID for correlation
		Payload: &proto.RodentRequest_CommandResponse{
			CommandResponse: commandResponse,
		},
	}
	
	// Add detailed logging
	c.client.Logger.Info("Sending system.status response with matching IDs", 
		"request_id", req.RequestId,
		"response_session_id", c.sessionID)
	
	// Send the response
	if err := c.Send(rodentReq); err != nil {
		c.client.Logger.Error("Failed to send system.status response", 
			"error", err, 
			"request_id", req.RequestId)
		return err
	}
	
	c.client.Logger.Info("Successfully sent system.status response", 
		"request_id", req.RequestId,
		"session_id", c.sessionID)
	return nil
}

// ModifyHandleToggleRequests is a helper function to integrate this specialized handler
// It should be added to the switch statement in HandleToggleRequests method
func (c *StreamConnection) ModifyHandleToggleRequests() {
	// This is a replacement handler function to correctly process system.status requests
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
				// This ensures quick processing of status requests even when other operations are running
				go func(request *proto.ToggleRequest) {
					// Process the request based on payload type
					switch payload := request.Payload.(type) {
					case *proto.ToggleRequest_Command:
						// Handle command requests
						cmd := payload.Command
						
						// Log all command requests
						c.client.Logger.Debug("Received command from Toggle", 
							"type", cmd.CommandType, 
							"target", cmd.Target,
							"request_id", request.RequestId)
						
						// Special case for system.status for proper request/response correlation
						if cmd.CommandType == "system.status" || 
							(cmd.CommandType == "status" && cmd.Target == "system") {
							// Handle system status with specialized function
							if err := c.HandleSystemStatus(request, cmd); err != nil {
								c.client.Logger.Error("Failed to handle system status", "error", err)
							}
							return // Return from goroutine after processing status request
						}
						
						// Handle other command types using registered handlers
						var response *proto.CommandResponse
						var err error
						
						// Look for a registered handler
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
									RequestId: request.RequestId,
									Success:   false,
									Message:   fmt.Sprintf("Command failed: %v", err),
								}
							}
						} else {
							// No handler found
							c.client.Logger.Warn("No handler for command type", 
								"type", cmd.CommandType,
								"registered_handlers", fmt.Sprintf("%v", getRegisteredCommandTypes()))
							
							response = &proto.CommandResponse{
								RequestId: request.RequestId,
								Success:   false,
								Message:   fmt.Sprintf("Unsupported command type: %s", cmd.CommandType),
							}
						}
						
						// Create and send the response
						rodentReq := &proto.RodentRequest{
							SessionId: c.sessionID,
							RequestId: uuid.New().String(), // For non-system.status, use a new ID
							Payload: &proto.RodentRequest_CommandResponse{
								CommandResponse: response,
							},
						}
						
						if err := c.Send(rodentReq); err != nil {
							c.client.Logger.Warn("Failed to send command response", "error", err)
						}
						
					// Other message types handled the same way...
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
									RequestId: request.RequestId,
									Success:   true,
									Message:   fmt.Sprintf("Received config update type: %s", configUpdate.ConfigType),
								},
							},
						}
						
						if err := c.Send(ack); err != nil {
							c.client.Logger.Warn("Failed to send config update acknowledgment", "error", err)
						}
						
					case *proto.ToggleRequest_Ack:
						// Handle acknowledgments
						ack := payload.Ack
						c.client.Logger.Debug("Received acknowledgment from Toggle", 
							"request_id", ack.RequestId,
							"success", ack.Success,
							"message", ack.Message)
					}
				}(req) // Pass req to the goroutine
			}
		}
	}()
}