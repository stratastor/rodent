// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// The backoff implementation has been moved to backoff.go for better organization
// The circuit breaker implementation is in circuit_breaker.go

// StreamConnection represents a bidirectional streaming connection to Toggle
type StreamConnection struct {
	client          *GRPCClient
	stream          proto.RodentService_ConnectClient
	streamCtx       context.Context
	stopChan        chan struct{}
	outboundChan    chan *proto.RodentRequest
	inboundChan     chan *proto.ToggleRequest
	wg              sync.WaitGroup
	reconnectMu     sync.Mutex
	isReconnecting  bool
	backoffStrategy backoff
	circuitBreaker  *CircuitBreaker // Circuit breaker to prevent excessive reconnection attempts
}

// Connect establishes a bidirectional streaming connection with Toggle
// This long-lived connection enables Toggle to send commands to Rodent nodes
// that are behind firewalls or NATs, where Toggle cannot initiate connections
// to the Rodent node directly.
func (c *GRPCClient) Connect(ctx context.Context) (*StreamConnection, error) {
	// Only private networks use gRPC streaming
	isPrivate, err := IsPrivateNetwork(c.jwt)
	if err != nil {
		return nil, fmt.Errorf("failed to determine network type: %w", err)
	}

	if !isPrivate {
		return nil, fmt.Errorf("Connect is only available for private networks")
	}

	// Create metadata with JWT token for authentication
	// The token is sent as a Bearer token in the request metadata
	md := metadata.Pairs("authorization", "Bearer "+c.jwt)
	streamCtx := metadata.NewOutgoingContext(ctx, md)

	// Establish the bidirectional stream with the Toggle service
	// This creates a long-lived connection that will remain open
	stream, err := c.client.Connect(streamCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Toggle streaming service: %w", err)
	}

	// Create a new StreamConnection to manage the bidirectional stream
	conn := &StreamConnection{
		client:    c,
		stream:    stream,
		streamCtx: streamCtx,
		stopChan: make(
			chan struct{},
		), // Channel to signal goroutines to stop
		outboundChan: make(
			chan *proto.RodentRequest,
			200,
		), // Increased buffer for outbound messages
		inboundChan: make(
			chan *proto.ToggleRequest,
			500,
		), // Significantly increased buffer for inbound messages
		backoffStrategy: newBackoff(),        // Initialize backoff strategy
		circuitBreaker:  newCircuitBreaker(), // Initialize circuit breaker for connection stability
		isReconnecting:  false,
	}

	// Start the send and receive loops in separate goroutines
	conn.wg.Add(2)
	go conn.sendLoop()    // Handles sending messages to Toggle
	go conn.receiveLoop() // Handles receiving messages from Toggle

	// Start the message handler to process incoming Toggle requests
	// This handles all types of messages from Toggle in a consistent way
	conn.StartMessageHandler()

	c.Logger.Info("Connected to Toggle via streaming gRPC")

	return conn, nil
}

// sendLoop continuously sends messages from the outbound channel to the stream
func (c *StreamConnection) sendLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		case req := <-c.outboundChan:
			// Send the message
			if err := c.stream.Send(req); err != nil {
				c.client.Logger.Error("Failed to send message to Toggle", "error", err)

				// Handle different error types appropriately
				if shouldReconnect(err) {
					c.client.Logger.Warn("Send connection disruption detected", "error", err)
					go c.tryReconnect()
					return
				} else {
					// Permanent error - stop the connection
					c.client.Logger.Error("Permanent error in send stream", "error", err)
					close(c.stopChan)
					return
				}
			}
		}
	}
}

// receiveLoop continuously receives messages from the stream and puts them in the inbound channel
func (c *StreamConnection) receiveLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		default:
			// Receive messages from server
			resp, err := c.stream.Recv()
			if err != nil {
				// Handle different error types appropriately
				if shouldReconnect(err) {
					c.client.Logger.Warn("Connection disruption detected", "error", err)
					go c.tryReconnect()
					return
				} else {
					// Permanent error - stop the connection
					c.client.Logger.Error("Permanent error in stream", "error", err)
					close(c.stopChan)
					return
				}
			}

			// Add debug logging for tracking message flow
			if cmd, ok := resp.Payload.(*proto.ToggleRequest_Command); ok {
				command := cmd.Command
				if command.CommandType == "system.status" ||
					(command.CommandType == "status" && command.Target == "system") {
					c.client.Logger.Debug("Received system.status request",
						"request_id", resp.RequestId,
						"timestamp", time.Now().Unix())
				}
			}

			// Send all messages to inbound channel
			select {
			case c.inboundChan <- resp:
				// Successfully sent to channel
			default:
				// Channel is full, log and continue
				c.client.Logger.Warn("Inbound channel is full, dropping message")
			}
		}
	}
}

// shouldReconnect determines if the stream error is transient and we should attempt reconnection
func shouldReconnect(err error) bool {
	// Get the status code from the error
	s, ok := status.FromError(err)
	if !ok {
		// Not a gRPC status error, likely a connection issue - try reconnecting
		return true
	}

	// Check specific codes that indicate temporary issues
	switch s.Code() {
	case codes.Unavailable, // Server is unavailable
		codes.DeadlineExceeded,  // Deadline exceeded
		codes.ResourceExhausted, // Resource exhausted
		codes.Canceled:          // Canceled
		return true
	case codes.PermissionDenied, // Permission denied
		codes.Unauthenticated,    // Unauthenticated
		codes.FailedPrecondition, // Failed precondition
		codes.InvalidArgument:    // Invalid argument
		// These are likely permanent errors, don't reconnect
		return false
	default:
		// For any other code, attempt reconnection
		return true
	}
}

// Send sends a message to the Toggle server
func (c *StreamConnection) Send(req *proto.RodentRequest) error {
	// Check if connection is closing
	select {
	case <-c.stopChan:
		return fmt.Errorf("connection is closing")
	default:
		// Continue with send
	}

	select {
	case c.outboundChan <- req:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout: outbound channel is full")
	case <-c.stopChan:
		return fmt.Errorf("connection closed while waiting to send")
	}
}

// All command handling is now in message_handler.go

// Receive returns a channel to receive messages from the Toggle server
func (c *StreamConnection) Receive() <-chan *proto.ToggleRequest {
	return c.inboundChan
}

// StopChan returns the stop channel to monitor for connection closure
func (c *StreamConnection) StopChan() <-chan struct{} {
	return c.stopChan
}

// Close closes the streaming connection
func (c *StreamConnection) Close() error {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	// Check if already closed
	select {
	case <-c.stopChan:
		// Already closed
		return nil
	default:
		// Close the channel to signal all goroutines to stop
		close(c.stopChan)
	}

	// Wait for all goroutines to finish with a timeout
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished
	case <-time.After(5 * time.Second):
		// Timeout waiting for goroutines to finish
		c.client.Logger.Warn("Timeout waiting for goroutines to finish during Close()")
	}

	// Try to gracefully close the stream if it exists
	if c.stream != nil {
		if err := c.stream.CloseSend(); err != nil {
			c.client.Logger.Warn("Error closing stream send", "error", err)
		}
	}

	c.client.Logger.Info("Closed Toggle connection")
	return nil
}

// SendEvent sends an event notification to Toggle
func (c *StreamConnection) SendEvent(eventType, source string, payload []byte) error {
	req := &proto.RodentRequest{
		RequestId: uuid.New().String(),
		Payload: &proto.RodentRequest_Event{
			Event: &proto.EventNotification{
				EventType: eventType,
				Source:    source,
				Timestamp: time.Now().UnixMilli(),
				Payload:   payload,
			},
		},
	}

	return c.Send(req)
}

// SendCommandResponse sends a response to a command request
// requestID is the original request ID from Toggle which must be preserved
// in both the outer RodentRequest and inner CommandResponse for correlation
func (c *StreamConnection) SendCommandResponse(
	requestID string,
	success bool,
	message string,
	payload []byte,
) error {
	req := &proto.RodentRequest{
		RequestId: requestID, // Use the original request ID for correlation
		Payload: &proto.RodentRequest_CommandResponse{
			CommandResponse: &proto.CommandResponse{
				RequestId: requestID, // Also use original request ID here
				Success:   success,
				Message:   message,
				Payload:   payload,
			},
		},
	}

	return c.Send(req)
}

// SendAcknowledgement sends an acknowledgement message
// requestID is the original request ID from Toggle which must be preserved
// in both the outer RodentRequest and inner Acknowledgement for correlation
func (c *StreamConnection) SendAcknowledgement(
	requestID string,
	success bool,
	message string,
) error {
	req := &proto.RodentRequest{
		RequestId: requestID, // Use the original request ID for correlation
		Payload: &proto.RodentRequest_Ack{
			Ack: &proto.Acknowledgement{
				RequestId: requestID, // Also use original request ID here
				Success:   success,
				Message:   message,
			},
		},
	}

	return c.Send(req)
}
