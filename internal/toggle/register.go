// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"context"
	"fmt"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/events"
	"github.com/stratastor/rodent/internal/services/traefik"
	"github.com/stratastor/rodent/internal/toggle/client"
)

const (
	// DefaultRetryInterval is the interval between retry attempts
	DefaultRetryInterval = 1 * time.Minute
)

// RegisterNode registers this Rodent node with the Toggle service
func RegisterNode(
	ctx context.Context,
	toggleClient client.ToggleClient,
	logger logger.Logger,
) error {
	// Call the register method
	result, err := toggleClient.Register(ctx)
	if err != nil {
		return fmt.Errorf("registration request failed: %w", err)
	}

	// Extract org ID for logging
	orgID, err := toggleClient.GetOrgID()
	if err != nil {
		logger.Warn("Failed to extract organization ID from JWT", "error", err)
	}

	// For nodes in private networks, we don't need to install certificates
	// as indicated by the "prv" claim in the JWT
	cfg := config.GetConfig()
	isPrivate, err := client.IsPrivateNetwork(cfg.Toggle.JWT)
	if err != nil {
		logger.Warn("Failed to determine network type from JWT", "error", err)
	}

	// Initialize event system regardless of registration status
	// This ensures events work on both first-time registration and subsequent startups
	if err := events.InitializeWithClient(ctx, toggleClient, logger); err != nil {
		if logger != nil {
			logger.Warn("Failed to initialize event system", "error", err)
		}
		// Continue anyway - events are not critical for core functionality
	}

	// If we received a message without certificate data, we're either already registered
	// or we're in a private network
	if result.Certificate == "" {
		logger.Info("Node already registered with Toggle service")

		// For private network nodes, establish bidirectional stream
		if isPrivate {
			go establishStreamConnection(ctx, toggleClient, logger)
		}

		// Emit system startup event even for already-registered nodes
		events.EmitSystemStartup("restart")

		return nil
	}

	logger.Info("Registration successful with Toggle service",
		"orgID", orgID, "domain", result.Domain,
		"expiresOn", result.ExpiresOn)

	// Emit registration event for new registrations
	events.EmitSystemRegistration("new_registration", result.Domain, result.ExpiresOn, isPrivate)

	// Skip certificate installation if we're in development mode or a private network
	if !cfg.Development.Enabled && !isPrivate {
		traefikSvc, err := traefik.NewClient(logger)
		if err != nil {
			return fmt.Errorf("failed to create Traefik client: %w", err)
		}
		if err := traefikSvc.InstallCertificate(ctx, traefik.CertificateData{
			Domain:      result.Domain,
			Certificate: result.Certificate,
			PrivateKey:  result.PrivateKey,
			ExpiresOn:   result.ExpiresOn,
		}); err != nil {
			return fmt.Errorf("failed to install certificate: %w", err)
		}

		logger.Info("Certificate installed successfully")
	} else if isPrivate {
		logger.Info("Skipping certificate installation for private network node")

		// For private network nodes, establish bidirectional stream after successful registration
		go establishStreamConnection(ctx, toggleClient, logger)
	}

	return nil
}

// Global connection monitor instance
var connectionMonitor *ConnectionMonitor

// establishStreamConnection establishes and maintains a bidirectional stream connection with Toggle
// It's designed to be run as a goroutine and will use the ConnectionMonitor to maintain
// a robust and stable connection
func establishStreamConnection(
	ctx context.Context,
	toggleClient client.ToggleClient,
	logger logger.Logger,
) {
	// Initialize the connection monitor if needed
	if connectionMonitor == nil {
		connectionMonitor = NewConnectionMonitor(ctx, toggleClient, logger)
	}

	// Start the connection monitor
	connectionMonitor.Start()

	// Log that the connection monitor has been started
	logger.Info("Toggle connection monitor started")
}

// StartRegistrationProcess begins the async process of registering with Toggle
func StartRegistrationProcess(ctx context.Context, l logger.Logger) {
	cfg := config.GetConfig()

	if !cfg.Toggle.Enable {
		if l != nil {
			l.Info("Toggle integration is disabled, skipping registration")
		}
		return
	}

	// Skip if JWT is not configured
	if cfg.Toggle.JWT == "" {
		if l != nil {
			l.Info("Toggle JWT not configured, skipping registration")
		}
		return
	}

	// Create a unified Toggle client that will use either REST or gRPC
	// based on the JWT claims
	toggleClient, err := client.NewToggleClient(
		l,
		cfg.Toggle.JWT,
		cfg.Toggle.BaseURL,
		cfg.Toggle.RPCAddr,
	)
	if err != nil {
		if l != nil {
			l.Error("Failed to create Toggle client", "error", err)
		}
		return
	}

	go runRegistrationProcess(ctx, toggleClient, l)
}

func runRegistrationProcess(
	ctx context.Context,
	toggleClient client.ToggleClient,
	l logger.Logger,
) {
	retryInterval := DefaultRetryInterval

	for {
		err := RegisterNode(ctx, toggleClient, l)
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
