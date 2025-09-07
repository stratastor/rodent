// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"fmt"
	"time"

	"github.com/stratastor/logger"
)

// TestEventSystem provides a simple way to test the event system without gRPC
func TestEventSystem(l logger.Logger) {
	if !IsInitialized() {
		l.Warn("Event system not initialized, test will be skipped")
		return
	}

	l.Info("Starting event system test...")

	// Test different event types and levels
	testEvents := []struct {
		name     string
		emitFunc func()
	}{
		{
			name: "System Info Event",
			emitFunc: func() {
				EmitSystemEvent("system.test", LevelInfo, 
					map[string]interface{}{
						"message": "Event system test started",
						"test_id": "evt-001",
					}, 
					map[string]string{
						"test": "integration",
					})
			},
		},
		{
			name: "Storage Warning Event", 
			emitFunc: func() {
				EmitStorageEvent("storage.test", LevelWarn, "test-component",
					map[string]interface{}{
						"message": "Test storage warning",
						"test_id": "evt-002",
					},
					map[string]string{
						"test": "integration",
					})
			},
		},
		{
			name: "Security Critical Event",
			emitFunc: func() {
				EmitSecurityEvent("security.test", LevelCritical, "test-security",
					map[string]interface{}{
						"message": "Test critical security event",
						"test_id": "evt-003",
					},
					map[string]string{
						"test": "integration",
					})
			},
		},
		{
			name: "Service Error Event",
			emitFunc: func() {
				EmitServiceEvent("service.test", LevelError, "test-service",
					map[string]interface{}{
						"message": "Test service error",
						"test_id": "evt-004",
					},
					map[string]string{
						"test": "integration",
					})
			},
		},
		{
			name: "Network Info Event",
			emitFunc: func() {
				EmitNetworkEvent("network.test", LevelInfo, "test-network",
					map[string]interface{}{
						"message": "Test network event", 
						"test_id": "evt-005",
					},
					map[string]string{
						"test": "integration",
					})
			},
		},
	}

	// Emit test events
	for _, test := range testEvents {
		l.Info("Emitting test event", "name", test.name)
		test.emitFunc()
		time.Sleep(10 * time.Millisecond) // Small delay between events
	}

	// Get event system statistics
	stats := GetStats()
	l.Info("Event system statistics after test", 
		"buffer_size", stats["buffer_size"],
		"pending_events", stats["pending_events"],
		"initialized", stats["initialized"])

	l.Info("Event system test completed. Check Toggle service for received events.")
}

// TestEventSystemWithContext runs the test and provides more detailed feedback
func TestEventSystemWithContext(ctx context.Context, l logger.Logger) error {
	if !IsInitialized() {
		return fmt.Errorf("event system not initialized")
	}

	l.Info("Starting comprehensive event system test...")

	// Test rapid event emission
	start := time.Now()
	for i := 0; i < 50; i++ {
		EmitSystemEvent("system.load_test", LevelInfo,
			map[string]interface{}{
				"message":    "Load test event",
				"event_num":  i + 1,
				"timestamp":  time.Now().Unix(),
			},
			map[string]string{
				"test":  "load",
				"batch": "1",
			})
	}
	
	emitDuration := time.Since(start)
	l.Info("Rapid event emission completed", 
		"events", 50,
		"duration_ms", emitDuration.Milliseconds(),
		"events_per_second", float64(50)/emitDuration.Seconds())

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Get final statistics
	stats := GetStats()
	l.Info("Final event system statistics",
		"buffer_size", stats["buffer_size"],
		"max_buffer_size", stats["max_buffer_size"],
		"pending_events", stats["pending_events"],
		"max_pending", stats["max_pending"])

	return nil
}