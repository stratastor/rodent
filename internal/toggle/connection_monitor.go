// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"context"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/toggle/client"
)

// ConnectionMonitor manages a persistent GRPC connection to Toggle
// with robust connection monitoring and automatic reconnection
type ConnectionMonitor struct {
	ctx             context.Context
	cancel          context.CancelFunc
	toggleClient    client.ToggleClient
	logger          logger.Logger
	connectionMutex sync.Mutex
	connection      *client.StreamConnection
	circuitBreaker  *circuitBreaker
	isConnected     bool
	isRunning       bool
}

// circuitBreaker implements the circuit breaker pattern for connection attempts
type circuitBreaker struct {
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	lastFailureTime  time.Time
	state            string // "closed", "open", "half-open"
	mu               sync.Mutex
}

// newCircuitBreaker creates a new circuit breaker with default settings
func newCircuitBreaker() *circuitBreaker {
	return &circuitBreaker{
		failureCount:     0,
		failureThreshold: 8,                // Open after 8 consecutive failures
		resetTimeout:     10 * time.Minute, // Wait 10 minutes before transitioning to half-open
		state:            "closed",         // Start in closed state (allowing connections)
	}
}

// recordSuccess records a successful operation and resets the circuit breaker
func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount = 0
	cb.state = "closed"
}

// recordFailure records a failed operation and potentially opens the circuit
func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()

	if cb.state == "half-open" {
		// Failed in half-open state, immediately open the circuit
		cb.state = "open"
		return
	}

	if cb.state == "closed" {
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = "open"
		}
	}
}

// allowRequest checks if a request should be allowed based on the circuit state
func (cb *circuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case "closed":
		return true
	case "open":
		// Check if reset timeout has elapsed
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			// Transition to half-open to test if the connection can be restored
			cb.state = "half-open"
			return true
		}
		return false
	case "half-open":
		// In half-open state, allow only one request to test the connection
		return true
	default:
		return true
	}
}

// getState returns the current circuit breaker state
func (cb *circuitBreaker) getState() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// NewConnectionMonitor creates a new connection monitor
func NewConnectionMonitor(
	parentCtx context.Context,
	toggleClient client.ToggleClient,
	logger logger.Logger,
) *ConnectionMonitor {
	ctx, cancel := context.WithCancel(parentCtx)

	return &ConnectionMonitor{
		ctx:            ctx,
		cancel:         cancel,
		toggleClient:   toggleClient,
		logger:         logger,
		circuitBreaker: newCircuitBreaker(),
		isConnected:    false,
		isRunning:      false,
	}
}

// Start begins monitoring and maintaining the connection
func (m *ConnectionMonitor) Start() {
	m.connectionMutex.Lock()
	if m.isRunning {
		m.connectionMutex.Unlock()
		return // Already running
	}
	m.isRunning = true
	m.connectionMutex.Unlock()

	// Register all domain-specific handlers
	RegisterAllHandlers()

	go m.monitorConnection()
}

// Stop terminates the connection monitoring
func (m *ConnectionMonitor) Stop() {
	m.connectionMutex.Lock()
	defer m.connectionMutex.Unlock()

	if !m.isRunning {
		return // Not running
	}

	// Cancel our context to stop all running goroutines
	m.cancel()
	m.isRunning = false

	// Close any existing connection
	if m.connection != nil {
		m.connection.Close()
		m.connection = nil
	}

	m.isConnected = false
}

// IsConnected returns whether there is an active connection
func (m *ConnectionMonitor) IsConnected() bool {
	m.connectionMutex.Lock()
	defer m.connectionMutex.Unlock()
	return m.isConnected
}

// monitorConnection is the main connection monitoring loop
func (m *ConnectionMonitor) monitorConnection() {
	// We can only use Connect with a gRPC client
	grpcClient, ok := m.toggleClient.(*client.GRPCClient)
	if !ok {
		m.logger.Error("Cannot establish stream connection with non-gRPC client")
		return
	}

	// Run connection loop indefinitely with improved backoff and circuit breaker
	initialRetryDelay := 30 * time.Second // Increased initial delay
	maxRetryDelay := 15 * time.Minute     // Increased max delay
	retryDelay := initialRetryDelay

	for {
		select {
		case <-m.ctx.Done():
			// Parent context was canceled, exit the goroutine
			m.logger.Info("Stream connection loop terminated due to context cancellation")
			return
		default:
			// Check if the circuit breaker allows connection attempts
			if !m.circuitBreaker.allowRequest() {
				m.logger.Warn("Circuit breaker preventing connection attempt",
					"state", m.circuitBreaker.getState(),
					"will_retry_after", m.circuitBreaker.resetTimeout)

				// Sleep for a while before checking again
				select {
				case <-time.After(1 * time.Minute):
					// Check again after a minute
					continue
				case <-m.ctx.Done():
					return
				}
			}

			// Attempt to establish the connection
			m.logger.Info("Establishing bidirectional stream connection with Toggle")
			conn, err := grpcClient.Connect(m.ctx)

			if err != nil {
				m.logger.Error("Failed to establish stream connection", "error", err)
				m.circuitBreaker.recordFailure()

				// Update connection status
				m.connectionMutex.Lock()
				m.isConnected = false
				m.connectionMutex.Unlock()

				// Sleep with backoff before retrying
				sleepDuration := retryDelay
				m.logger.Info("Retrying connection in " + sleepDuration.String())

				select {
				case <-time.After(sleepDuration):
					// Continue to retry
				case <-m.ctx.Done():
					return
				}

				// Increase retry delay with exponential backoff (up to max)
				retryDelay = time.Duration(float64(retryDelay) * 1.5)
				if retryDelay > maxRetryDelay {
					retryDelay = maxRetryDelay
				}

				continue
			}

			// Connection established, reset retry delay and update status
			retryDelay = initialRetryDelay
			m.circuitBreaker.recordSuccess()

			m.connectionMutex.Lock()
			m.connection = conn
			m.isConnected = true
			m.connectionMutex.Unlock()

			m.logger.Info("Bidirectional stream established with Toggle")

			// Wait for the connection to be closed or context cancellation
			connectionClosed := m.waitForConnectionClose(conn)

			// Handle connection closure
			select {
			case <-connectionClosed:
				m.logger.Warn("Stream connection closed, will reconnect after delay")

				// Update connection status
				m.connectionMutex.Lock()
				m.isConnected = false
				m.connection = nil
				m.connectionMutex.Unlock()

				// Sleep for a bit before trying again to avoid rapid cycling
				time.Sleep(initialRetryDelay * 2)

			case <-m.ctx.Done():
				// Parent context canceled, close connection and exit
				m.logger.Info("Closing stream connection due to context cancellation")

				m.connectionMutex.Lock()
				if m.connection != nil {
					m.connection.Close()
					m.connection = nil
				}
				m.isConnected = false
				m.connectionMutex.Unlock()

				return
			}
		}
	}
}

// waitForConnectionClose monitors the connection and returns a channel that will be closed
// when the connection is closed
func (m *ConnectionMonitor) waitForConnectionClose(conn *client.StreamConnection) <-chan struct{} {
	connClosed := make(chan struct{})

	// Start a goroutine to monitor the connection
	go func() {
		// Monitor the stop channel instead of consuming messages
		// This avoids competing with the message processor for messages
		<-conn.StopChan()

		// Connection is down
		close(connClosed)
		// return
	}()

	return connClosed
}
