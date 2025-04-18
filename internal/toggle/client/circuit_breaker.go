// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"sync"
	"time"
)

// CircuitBreaker implements the circuit breaker pattern for connection attempts
type CircuitBreaker struct {
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	lastFailureTime  time.Time
	state            string // "closed", "open", "half-open"
	mu               sync.Mutex
}

// newCircuitBreaker creates a new circuit breaker with default settings
func newCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		failureCount:     0,
		failureThreshold: 5,                // Open after 5 consecutive failures
		resetTimeout:     5 * time.Minute,  // Wait 5 minutes before transitioning to half-open
		state:            "closed",         // Start in closed state (allowing connections)
	}
}

// recordSuccess records a successful operation and resets the circuit breaker
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.failureCount = 0
	cb.state = "closed"
}

// recordFailure records a failed operation and potentially opens the circuit
func (cb *CircuitBreaker) recordFailure() {
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
func (cb *CircuitBreaker) allowRequest() bool {
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
func (cb *CircuitBreaker) getState() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}