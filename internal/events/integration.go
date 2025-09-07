// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/lifecycle"
)

var (
	globalEventBus *EventBus
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

	// Create event bus with default config
	config := DefaultEventConfig()
	
	// Get the underlying proto client
	protoClient := grpcClient.GetProtoClient()
	if protoClient == nil {
		return fmt.Errorf("failed to get proto client from gRPC client")
	}

	globalEventBus = NewEventBus(protoClient, grpcClient.GetJWT(), config, l)

	// Start the event bus
	if err := globalEventBus.Start(ctx); err != nil {
		return fmt.Errorf("failed to start event bus: %w", err)
	}

	// Register shutdown hook (lifecycle expects func(), not func() error)
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

	if !initialized || globalEventBus == nil {
		return nil
	}

	err := globalEventBus.Shutdown(ctx)
	initialized = false
	return err
}

// EmitSystemEvent emits a system-level event
func EmitSystemEvent(eventType string, level EventLevel, payload interface{}, metadata map[string]string) {
	emitEvent(eventType, level, CategorySystem, "system", payload, metadata)
}

// EmitStorageEvent emits a storage-related event
func EmitStorageEvent(eventType string, level EventLevel, source string, payload interface{}, metadata map[string]string) {
	emitEvent(eventType, level, CategoryStorage, source, payload, metadata)
}

// EmitNetworkEvent emits a network-related event
func EmitNetworkEvent(eventType string, level EventLevel, source string, payload interface{}, metadata map[string]string) {
	emitEvent(eventType, level, CategoryNetwork, source, payload, metadata)
}

// EmitSecurityEvent emits a security-related event
func EmitSecurityEvent(eventType string, level EventLevel, source string, payload interface{}, metadata map[string]string) {
	emitEvent(eventType, level, CategorySecurity, source, payload, metadata)
}

// EmitServiceEvent emits a service-related event
func EmitServiceEvent(eventType string, level EventLevel, source string, payload interface{}, metadata map[string]string) {
	emitEvent(eventType, level, CategoryService, source, payload, metadata)
}

// Emit emits a generic event
func Emit(eventType string, level EventLevel, category EventCategory, source string, payload interface{}, metadata map[string]string) {
	emitEvent(eventType, level, category, source, payload, metadata)
}

// emitEvent is the internal implementation for emitting events
func emitEvent(eventType string, level EventLevel, category EventCategory, source string, payload interface{}, metadata map[string]string) {
	globalMu.RLock()
	bus := globalEventBus
	globalMu.RUnlock()

	if bus == nil {
		// Events not initialized - silently ignore
		return
	}

	// Marshal payload to JSON
	var payloadBytes []byte
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			// Log error but don't fail event emission
			// We could emit a meta-event about this failure, but that might cause loops
			return
		}
	}

	// Ensure metadata is not nil
	if metadata == nil {
		metadata = make(map[string]string)
	}

	bus.Emit(eventType, level, category, source, payloadBytes, metadata)
}

// GetStats returns event system statistics
func GetStats() map[string]interface{} {
	globalMu.RLock()
	bus := globalEventBus
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
	return initialized && globalEventBus != nil
}