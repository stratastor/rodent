// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// backoff implements exponential backoff with jitter for connection retries
type backoff struct {
	attempts    int
	baseDelay   time.Duration
	maxDelay    time.Duration
	multiplier  float64
	jitter      float64
	maxAttempts int
}

// newBackoff creates a new backoff strategy with reasonable defaults
func newBackoff() backoff {
	return backoff{
		attempts:    0,
		baseDelay:   500 * time.Millisecond,
		maxDelay:    2 * time.Minute,
		multiplier:  1.5,
		jitter:      0.2,
		maxAttempts: 10, // Reset after this many attempts
	}
}

// nextDelay returns the next delay to wait before retrying
func (b *backoff) nextDelay() time.Duration {
	// Increment attempt counter
	b.attempts++
	
	// Reset attempts after max to prevent potential overflow in calculations
	if b.attempts > b.maxAttempts {
		b.attempts = 1
	}
	
	// Calculate delay with exponential backoff
	backoffTime := float64(b.baseDelay) * math.Pow(b.multiplier, float64(b.attempts-1))
	if backoffTime > float64(b.maxDelay) {
		backoffTime = float64(b.maxDelay)
	}
	
	// Add jitter to prevent reconnection storms
	jitterRange := backoffTime * b.jitter
	backoffWithJitter := backoffTime - (jitterRange/2) + (rand.Float64() * jitterRange)
	
	return time.Duration(backoffWithJitter)
}

// reset resets the backoff attempts
func (b *backoff) reset() {
	b.attempts = 0
}

// StreamConnection represents a bidirectional streaming connection to Toggle
type StreamConnection struct {
	client          *GRPCClient
	stream          proto.RodentService_ConnectClient
	sessionID       string
	streamCtx       context.Context
	stopChan        chan struct{}
	outboundChan    chan *proto.RodentRequest
	inboundChan     chan *proto.ToggleRequest
	wg              sync.WaitGroup
	reconnectMu     sync.Mutex
	isReconnecting  bool
	backoffStrategy backoff
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

	// Generate a unique session ID for this connection
	sessionID := uuid.New().String()

	// Create a new StreamConnection to manage the bidirectional stream
	conn := &StreamConnection{
		client:          c,
		stream:          stream,
		sessionID:       sessionID,
		streamCtx:       streamCtx,
		stopChan:        make(chan struct{}),            // Channel to signal goroutines to stop
		outboundChan:    make(chan *proto.RodentRequest, 100), // Buffer for outbound messages
		inboundChan:     make(chan *proto.ToggleRequest, 100), // Buffer for inbound messages
		backoffStrategy: newBackoff(),                  // Initialize backoff strategy
		isReconnecting:  false,
	}

	// Start the send and receive loops in separate goroutines
	conn.wg.Add(2)
	go conn.sendLoop()    // Handles sending messages to Toggle
	go conn.receiveLoop() // Handles receiving messages from Toggle
	
	// Start the request handler to process incoming Toggle requests
	// This will handle system.status requests from Toggle as heartbeats
	conn.HandleToggleRequests()

	c.Logger.Info("Connected to Toggle via streaming gRPC", "sessionID", sessionID)

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
			// Add session ID if not already set
			if req.SessionId == "" {
				req.SessionId = c.sessionID
			}

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

// tryReconnect attempts to reestablish the connection with exponential backoff
func (c *StreamConnection) tryReconnect() {
	c.reconnectMu.Lock()
	if c.isReconnecting {
		c.reconnectMu.Unlock()
		return // Already reconnecting
	}
	c.isReconnecting = true
	c.reconnectMu.Unlock()

	defer func() {
		c.reconnectMu.Lock()
		c.isReconnecting = false
		c.reconnectMu.Unlock()
	}()

	// Create a new context that can be used for reconnecting
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Keep trying until we succeed or get a permanent error
	for {
		select {
		case <-c.stopChan:
			c.client.Logger.Info("Reconnection canceled - connection is closing")
			return
		default:
			// Calculate delay based on backoff strategy
			delay := c.backoffStrategy.nextDelay()
			c.client.Logger.Info("Reconnecting to Toggle service", "attempt", c.backoffStrategy.attempts, "delay", delay)
			
			// Wait based on backoff
			select {
			case <-time.After(delay):
				// Continue with reconnection
			case <-c.stopChan:
				return // Connection is closing
			}

			// Try to establish a new connection
			if err := c.reestablishConnection(ctx); err != nil {
				c.client.Logger.Error("Failed to reconnect", "error", err, "attempt", c.backoffStrategy.attempts)
				
				// Check if this is a permanent error
				if !shouldReconnect(err) {
					c.client.Logger.Error("Permanent error during reconnection, giving up", "error", err)
					close(c.stopChan)
					return
				}
				
				// Transient error, continue trying
				continue
			}
			
			// Successfully reconnected
			c.client.Logger.Info("Successfully reconnected to Toggle service", "sessionID", c.sessionID)
			c.backoffStrategy.reset() // Reset backoff for next time
			return
		}
	}
}

// reestablishConnection creates a new gRPC stream
func (c *StreamConnection) reestablishConnection(ctx context.Context) error {
	// Create metadata with JWT token for authentication
	md := metadata.Pairs("authorization", "Bearer "+c.client.jwt)
	streamCtx := metadata.NewOutgoingContext(ctx, md)

	// Establish a new bidirectional stream with the Toggle service
	stream, err := c.client.client.Connect(streamCtx)
	if err != nil {
		return fmt.Errorf("failed to reconnect: %w", err)
	}

	// Replace the old stream with the new one
	c.stream = stream
	c.streamCtx = streamCtx

	// Restart the send and receive loops
	c.wg.Add(2)
	go c.sendLoop()
	go c.receiveLoop()
	
	// Restart the request handler
	c.HandleToggleRequests()

	return nil
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

			// Send to inbound channel
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
		codes.DeadlineExceeded, // Deadline exceeded
		codes.ResourceExhausted, // Resource exhausted
		codes.Canceled: // Canceled
		return true
	case codes.PermissionDenied, // Permission denied
		codes.Unauthenticated, // Unauthenticated 
		codes.FailedPrecondition, // Failed precondition
		codes.InvalidArgument: // Invalid argument
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

// HandleToggleRequests starts a goroutine that processes incoming Toggle requests
// and responds appropriately to status/heartbeat requests
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
					
					// Check if this is a status/heartbeat request
					if cmd.CommandType == "system.status" {
						c.client.Logger.Debug("Received heartbeat request from Toggle")
						
						// Respond with system status (acting as heartbeat)
						resp := &proto.RodentRequest{
							SessionId: c.sessionID,
							RequestId: uuid.New().String(),
							Payload: &proto.RodentRequest_CommandResponse{
								CommandResponse: &proto.CommandResponse{
									RequestId: req.RequestId,
									Success:   true,
									Message:   "System status",
									Payload:   []byte(`{"status":"healthy"}`), // Could include real metrics
								},
							},
						}
						
						if err := c.Send(resp); err != nil {
							c.client.Logger.Warn("Failed to send status response", "error", err)
						}
					} else {
						// Handle other command types...
						c.client.Logger.Debug("Received command from Toggle", 
							"type", cmd.CommandType, 
							"target", cmd.Target)
							
						// Process command and send response
						// This would typically pass the command to appropriate handlers
					}
					
				case *proto.ToggleRequest_Config:
					// Handle config updates
					c.client.Logger.Debug("Received config update from Toggle", 
						"type", payload.Config.ConfigType)
					
				case *proto.ToggleRequest_Ack:
					// Handle acknowledgments
					c.client.Logger.Debug("Received acknowledgment from Toggle", 
						"request_id", payload.Ack.RequestId)
				}
			}
		}
	}()
}

// Receive returns a channel to receive messages from the Toggle server
func (c *StreamConnection) Receive() <-chan *proto.ToggleRequest {
	return c.inboundChan
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

	c.client.Logger.Info("Closed Toggle connection", "sessionID", c.sessionID)
	return nil
}

// SendEvent sends an event notification to Toggle
func (c *StreamConnection) SendEvent(eventType, source string, payload []byte) error {
	req := &proto.RodentRequest{
		SessionId: c.sessionID,
		RequestId: uuid.New().String(),
		Payload: &proto.RodentRequest_Event{
			Event: &proto.EventNotification{
				EventType:  eventType,
				Source:     source,
				Timestamp:  time.Now().UnixMilli(),
				Payload:    payload,
			},
		},
	}

	return c.Send(req)
}

// SendCommandResponse sends a response to a command request
func (c *StreamConnection) SendCommandResponse(requestID string, success bool, message string, payload []byte) error {
	req := &proto.RodentRequest{
		SessionId: c.sessionID,
		RequestId: uuid.New().String(),
		Payload: &proto.RodentRequest_CommandResponse{
			CommandResponse: &proto.CommandResponse{
				RequestId: requestID,
				Success:   success,
				Message:   message,
				Payload:   payload,
			},
		},
	}

	return c.Send(req)
}

// SendAcknowledgement sends an acknowledgement message
func (c *StreamConnection) SendAcknowledgement(requestID string, success bool, message string) error {
	req := &proto.RodentRequest{
		SessionId: c.sessionID,
		RequestId: uuid.New().String(),
		Payload: &proto.RodentRequest_Ack{
			Ack: &proto.Acknowledgement{
				RequestId: requestID,
				Success:   success,
				Message:   message,
			},
		},
	}

	return c.Send(req)
}