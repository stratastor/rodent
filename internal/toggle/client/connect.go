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
	"google.golang.org/grpc/metadata"
)

// StreamConnection represents a bidirectional streaming connection to Toggle
type StreamConnection struct {
	client       *GRPCClient
	stream       proto.RodentService_ConnectClient
	sessionID    string
	streamCtx    context.Context
	stopChan     chan struct{}
	outboundChan chan *proto.RodentRequest
	inboundChan  chan *proto.ToggleRequest
	wg           sync.WaitGroup
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
		client:       c,
		stream:       stream,
		sessionID:    sessionID,
		streamCtx:    streamCtx,
		stopChan:     make(chan struct{}),           // Channel to signal goroutines to stop
		outboundChan: make(chan *proto.RodentRequest, 100), // Buffer for outbound messages
		inboundChan:  make(chan *proto.ToggleRequest, 100), // Buffer for inbound messages
	}

	// Start the send and receive loops in separate goroutines
	conn.wg.Add(2)
	go conn.sendLoop()  // Handles sending messages to Toggle
	go conn.receiveLoop() // Handles receiving messages from Toggle

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
				// Handle disconnection - attempt to reconnect
				return
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
				c.client.Logger.Error("Failed to receive message from Toggle", "error", err)
				// Signal to stop the connection on error
				close(c.stopChan)
				return
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

// Send sends a message to the Toggle server
func (c *StreamConnection) Send(req *proto.RodentRequest) error {
	select {
	case c.outboundChan <- req:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout: outbound channel is full")
	}
}

// Receive returns a channel to receive messages from the Toggle server
func (c *StreamConnection) Receive() <-chan *proto.ToggleRequest {
	return c.inboundChan
}

// Close closes the streaming connection
func (c *StreamConnection) Close() error {
	// Signal both loops to stop
	close(c.stopChan)

	// Wait for goroutines to finish
	c.wg.Wait()

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