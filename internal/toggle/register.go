// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"context"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
)

// StartRegistrationProcess begins the async process of registering with Toggle
func StartRegistrationProcess(ctx context.Context, l logger.Logger) {
	cfg := config.GetConfig()

	// Skip if JWT is not configured
	if cfg.Toggle.JWT == "" {
		if l != nil {
			l.Info("Toggle JWT not configured, skipping registration")
		}
		return
	}

	// Create a Toggle client
	client := NewClient(cfg.Toggle.JWT, l)

	// Start registration process in background
	go runRegistrationProcess(ctx, client, l)
}

// runRegistrationProcess handles the registration process with retries
func runRegistrationProcess(ctx context.Context, client *Client, l logger.Logger) {
	retryInterval := DefaultRetryInterval

	for {
		err := client.RegisterNode(ctx)
		if err == nil {
			// Registration successful
			return
		}

		// Log failure and retry
		if l != nil {
			l.Error("Failed to register with Toggle service", "error", err)
			l.Info("Will retry registration in 1 minute")
		}

		// Wait for retry interval or context cancellation
		select {
		case <-time.After(retryInterval):
			// Continue to retry
		case <-ctx.Done():
			// Context cancelled, stop retrying
			if l != nil {
				l.Info("Registration process stopped due to context cancellation")
			}
			return
		}
	}
}
