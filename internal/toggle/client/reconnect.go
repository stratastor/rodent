// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/metadata"
)

// tryReconnect attempts to reestablish the connection with exponential backoff
// and circuit breaker to prevent excessive reconnection attempts
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

	// Check if the circuit breaker allows reconnection attempts
	if !c.circuitBreaker.allowRequest() {
		circuitState := c.circuitBreaker.getState()
		c.client.Logger.Warn("Circuit breaker preventing reconnection", 
			"state", circuitState,
			"next_retry_in", c.circuitBreaker.resetTimeout)
		return
	}

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
			
			// Use a longer initial delay to reduce rapid cycling
			if c.backoffStrategy.attempts == 1 {
				delay = 30 * time.Second // Start with 30 seconds minimum delay on first attempt
			}
			
			c.client.Logger.Info("Reconnecting to Toggle service", 
				"attempt", c.backoffStrategy.attempts, 
				"delay", delay,
				"circuit_state", c.circuitBreaker.getState())
			
			// Wait based on backoff
			select {
			case <-time.After(delay):
				// Continue with reconnection
			case <-c.stopChan:
				return // Connection is closing
			}

			// Try to establish a new connection
			if err := c.reestablishConnection(ctx); err != nil {
				c.client.Logger.Error("Failed to reconnect", 
					"error", err, 
					"attempt", c.backoffStrategy.attempts)
				
				// Record the failure in the circuit breaker
				c.circuitBreaker.recordFailure()
				
				// Check if this is a permanent error
				if !shouldReconnect(err) {
					c.client.Logger.Error("Permanent error during reconnection, giving up", "error", err)
					close(c.stopChan)
					return
				}
				
				// Check if the circuit breaker has opened after this failure
				if !c.circuitBreaker.allowRequest() {
					c.client.Logger.Warn("Circuit breaker opened after failed reconnection attempts", 
						"state", c.circuitBreaker.getState(),
						"will_retry_after", c.circuitBreaker.resetTimeout)
					return
				}
				
				// Transient error, continue trying
				continue
			}
			
			// Successfully reconnected
			c.client.Logger.Info("Successfully reconnected to Toggle service", "sessionID", c.sessionID)
			c.backoffStrategy.reset() // Reset backoff for next time
			c.circuitBreaker.recordSuccess() // Record success in circuit breaker
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
	
	// Restart the request handler with the improved version
	c.ModifyHandleToggleRequests()

	return nil
}