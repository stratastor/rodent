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

	// If we received a message without certificate data, we're either already registered
	// or we're in a private network
	if result.Certificate == "" {
		logger.Info("Node already registered with Toggle service")
		
		// For private network nodes, establish bidirectional stream
		if isPrivate {
			go establishStreamConnection(ctx, toggleClient, logger)
		}
		
		return nil
	}

	logger.Info("Registration successful with Toggle service",
		"orgID", orgID, "domain", result.Domain,
		"expiresOn", result.ExpiresOn)

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

// establishStreamConnection establishes and maintains a bidirectional stream connection with Toggle
// It's designed to be run as a goroutine and will keep the connection alive indefinitely,
// handling reconnections as needed
func establishStreamConnection(
	ctx context.Context,
	toggleClient client.ToggleClient,
	logger logger.Logger,
) {
	// We can only use Connect with a gRPC client
	grpcClient, ok := toggleClient.(*client.GRPCClient)
	if !ok {
		logger.Error("Cannot establish stream connection with non-gRPC client")
		return
	}

	// Create a context that can be canceled independently of the parent
	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run connection loop indefinitely with backoff
	initialRetryDelay := 5 * time.Second
	maxRetryDelay := 5 * time.Minute
	retryDelay := initialRetryDelay
	
	for {
		select {
		case <-ctx.Done():
			// Parent context was canceled, exit the goroutine
			logger.Info("Stream connection loop terminated due to context cancellation")
			return
		default:
			// Attempt to establish the connection
			logger.Info("Establishing bidirectional stream connection with Toggle")
			conn, err := grpcClient.Connect(streamCtx)
			
			if err != nil {
				logger.Error("Failed to establish stream connection", "error", err)
				
				// Sleep with backoff before retrying
				logger.Info("Retrying connection in " + retryDelay.String())
				time.Sleep(retryDelay)
				
				// Increase retry delay with exponential backoff (up to max)
				retryDelay = time.Duration(float64(retryDelay) * 1.5)
				if retryDelay > maxRetryDelay {
					retryDelay = maxRetryDelay
				}
				
				continue
			}
			
			// Connection established, reset retry delay
			retryDelay = initialRetryDelay
			logger.Info("Bidirectional stream established with Toggle")
			
			// Wait for the connection to be closed or parent context to be canceled
			connClosed := make(chan struct{})
			
			// Start a goroutine to monitor the connection
			go func() {
				// This channel will receive all messages from Toggle
				msgChan := conn.Receive()
				
				// Keep receiving until channel closes
				for {
					_, ok := <-msgChan
					if !ok {
						// Channel closed, connection is down
						close(connClosed)
						return
					}
					// Message handling is done in the HandleToggleRequests method
				}
			}()
			
			// Wait for either connection closure or context cancellation
			select {
			case <-connClosed:
				logger.Warn("Stream connection closed, will reconnect")
				// Let the loop handle reconnection after a short delay
				time.Sleep(initialRetryDelay)
			case <-ctx.Done():
				// Parent context canceled, close connection and exit
				logger.Info("Closing stream connection due to context cancellation")
				conn.Close()
				return
			}
			
			// Always ensure the connection is closed before retrying
			conn.Close()
		}
	}
}

// StartRegistrationProcess begins the async process of registering with Toggle
func StartRegistrationProcess(ctx context.Context, l logger.Logger) {
	cfg := config.GetConfig()

	if !cfg.StrataSecure {
		if l != nil {
			l.Info("StrataSecure is disabled, skipping registration")
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
	toggleClient, err := client.NewToggleClient(l, cfg.Toggle.JWT, cfg.Toggle.BaseURL, cfg.Toggle.RPCAddr)
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