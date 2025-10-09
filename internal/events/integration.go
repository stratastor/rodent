// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/lifecycle"
)

var (
	GlobalEventBus *EventBus
	globalMu       sync.RWMutex
	initialized    bool
)

// Initialize sets up the global event system
func Initialize(ctx context.Context, toggleClient client.ToggleClient, l logger.Logger) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if initialized {
		return nil
	}

	// Get gRPC client for events
	grpcClient, ok := toggleClient.(*client.GRPCClient)
	if !ok {
		l.Warn("Toggle client is not gRPC client, events will be disabled")
		return fmt.Errorf("events require gRPC client, got %T", toggleClient)
	}

	// Create event bus with config from main configuration
	config := GetEventConfig()

	// Get the underlying proto client
	protoClient := grpcClient.GetProtoClient()
	if protoClient == nil {
		return fmt.Errorf("failed to get proto client from gRPC client")
	}

	GlobalEventBus = NewEventBus(protoClient, grpcClient.GetJWT(), config, l)

	// Start the event bus
	if err := GlobalEventBus.Start(ctx); err != nil {
		return fmt.Errorf("failed to start event bus: %w", err)
	}

	// Register shutdown hook
	// TODO: Improve lifecycle package to support context-aware shutdown hooks
	// See detailed explanation in init.go - same limitation applies here
	lifecycle.RegisterShutdownHook(func() {
		if err := Shutdown(ctx); err != nil {
			l.Error("Failed to shutdown event system", "error", err)
		}
	})

	initialized = true
	l.Info("Event system initialized successfully")
	return nil
}

// Shutdown gracefully shuts down the event system
func Shutdown(ctx context.Context) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if !initialized || GlobalEventBus == nil {
		return nil
	}

	err := GlobalEventBus.Shutdown(ctx)
	initialized = false
	return err
}

// Legacy emission functions removed - use type-safe structured emission functions from schema.go:
// - EmitSystemStartup, EmitSystemShutdown, EmitSystemConfigChange, EmitSystemUser
// - EmitServiceStatus
// - EmitStoragePool, EmitStorageDataset, EmitStorageTransfer
// - etc.

// GetStats returns event system statistics
func GetStats() map[string]interface{} {
	globalMu.RLock()
	bus := GlobalEventBus
	globalMu.RUnlock()

	if bus == nil {
		return map[string]interface{}{
			"initialized": false,
		}
	}

	stats := bus.GetStats()
	stats["initialized"] = true
	return stats
}

// IsInitialized returns whether the event system is initialized
func IsInitialized() bool {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return initialized && GlobalEventBus != nil
}
