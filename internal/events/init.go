// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/lifecycle"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// InitializeWithClient initializes the event system with a Toggle client
func InitializeWithClient(ctx context.Context, toggleClient client.ToggleClient, l logger.Logger) error {
	cfg := config.GetConfig()

	if !cfg.StrataSecure {
		if l != nil {
			l.Info("StrataSecure is disabled, skipping event system initialization")
		}
		return nil
	}

	// Skip if JWT is not configured
	if cfg.Toggle.JWT == "" {
		if l != nil {
			l.Info("Toggle JWT not configured, skipping event system initialization")
		}
		return nil
	}

	if toggleClient == nil {
		l.Info("Toggle client not available, events will be disabled")
		return nil
	}

	// Try to get gRPC client with proto client access
	grpcClient, ok := toggleClient.(interface {
		GetProtoClient() proto.RodentServiceClient
	})
	if !ok {
		l.Warn("Toggle client does not support events (not gRPC), events will be disabled")
		return nil
	}

	// Get the proto client
	protoClient := grpcClient.GetProtoClient()
	if protoClient == nil {
		l.Warn("Failed to get proto client, events will be disabled")
		return nil
	}

	// Initialize the event system with JWT from config
	return initializeWithProtoClient(ctx, protoClient, cfg.Toggle.JWT, l)
}

// initializeWithProtoClient initializes with a proto client directly
func initializeWithProtoClient(ctx context.Context, protoClient proto.RodentServiceClient, jwt string, l logger.Logger) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if initialized {
		return nil
	}

	// Create event bus with default config
	config := DefaultEventConfig()
	globalEventBus = NewEventBus(protoClient, jwt, config, l)

	// Start the event bus
	if err := globalEventBus.Start(ctx); err != nil {
		return err
	}

	// Register shutdown hook
	// Note: We capture the current context here, but in a real shutdown scenario,
	// the server should provide a proper shutdown context with timeout
	lifecycle.RegisterShutdownHook(func() {
		shutdownCtx := context.Background() // Use background context for shutdown
		if err := globalEventBus.Shutdown(shutdownCtx); err != nil {
			l.Error("Failed to shutdown event system", "error", err)
		}
	})

	initialized = true
	l.Info("Event system initialized successfully")
	return nil
}