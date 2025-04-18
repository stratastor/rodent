// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"math"
	"math/rand"
	"time"
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

// newBackoff creates a new backoff strategy with improved defaults
// These settings provide longer delays to avoid rapid reconnection cycles
func newBackoff() backoff {
	return backoff{
		attempts:    0,
		baseDelay:   30 * time.Second,  // 30 seconds initial delay (increased from 5s)
		maxDelay:    15 * time.Minute,  // 15 minutes maximum delay (increased from 2m)
		multiplier:  1.5,
		jitter:      0.2,
		maxAttempts: 20,                // Reset after this many attempts
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