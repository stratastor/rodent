// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

// Package hotplug provides real-time disk hotplug detection using udev events.
//
// # Overview
//
// This package implements a hybrid hotplug detection system that combines:
//   - Real-time udev event monitoring via netlink (sub-second latency)
//   - Periodic reconciliation to catch missed events (guaranteed consistency)
//
// # Architecture
//
// The hotplug system consists of four main components:
//
// 1. Monitor: Streams udev events from 'udevadm monitor' and parses them
// 2. StateMachine: Validates disk state transitions before applying them
// 3. Reconciler: Periodically compares cache vs actual devices to catch missed events
// 4. EventHandler: Coordinates all components and dispatches events to Manager
//
// # Event Flow
//
//	udev event → Monitor → EventHandler → extractDeviceID → StateMachine
//	    ↓
//	OnDeviceAdded/Removed/Changed callbacks → Manager
//
// # Reconciliation
//
// Runs every 30 seconds (configurable) to ensure no events were missed:
//   - Compares deviceCache (Manager's view) with DiscoverAll (actual devices)
//   - Emits synthetic add/remove/change events for discrepancies
//   - Provides eventual consistency guarantee
//
// # DeviceID Extraction
//
// Critical: DeviceID must match discovery.go's logic exactly:
//   Priority: Serial > WWN > ByIDPath > DevicePath
//
// This ensures udev events correctly match devices in the cache.
//
// # Usage
//
// The hotplug handler is created and managed by pkg/disk/Manager:
//
//	handler := hotplug.NewEventHandler(logger, &hotplug.HandlerConfig{
//	    UdevadmPath:       "/usr/bin/udevadm",
//	    ReconcileInterval: 30 * time.Second,
//	    DiscoveryFunc:     manager.discoverDevices,
//	    CacheFunc:         manager.getDeviceCache,
//	    OnDeviceAdded:     manager.handleDeviceAdded,
//	    OnDeviceRemoved:   manager.handleDeviceRemoved,
//	    OnDeviceChanged:   manager.handleDeviceChanged,
//	})
//	handler.Start("/usr/bin/udevadm")
//
// # Security
//
// - udevadm path is validated via configuration
// - All command arguments are hardcoded (no user input)
// - Uses exec.CommandContext for proper cancellation
// - Context cancellation stops all goroutines cleanly
package hotplug

import (
	"time"

	"github.com/stratastor/rodent/pkg/disk/types"
)

// UdevAction represents a udev event action
type UdevAction string

const (
	UdevActionAdd    UdevAction = "add"
	UdevActionRemove UdevAction = "remove"
	UdevActionChange UdevAction = "change"
	UdevActionMove   UdevAction = "move"
	UdevActionOnline UdevAction = "online"
	UdevActionOffline UdevAction = "offline"
)

// UdevEvent represents a udev device event
type UdevEvent struct {
	Action    UdevAction        // Event action (add, remove, change)
	DevPath   string            // Device path (e.g., /dev/sda)
	SysPath   string            // Sysfs path
	DevName   string            // Device name (e.g., sda)
	DevType   string            // Device type (disk, partition)
	Subsystem string            // Subsystem (block)
	SeqNum    uint64            // Event sequence number
	Timestamp time.Time         // Event timestamp
	Properties map[string]string // udev properties (ID_SERIAL, ID_WWN, etc.)
}

// DeviceTransition represents a state transition for a device
type DeviceTransition struct {
	DeviceID  string           // Device identifier
	OldState  types.DiskState  // Previous state
	NewState  types.DiskState  // New state
	Reason    string           // Reason for transition
	Event     *UdevEvent       // Triggering event (if any)
	Timestamp time.Time        // Transition timestamp
}

// ReconciliationResult represents the result of a reconciliation check
type ReconciliationResult struct {
	AddedDevices   []string  // Devices added since last check
	RemovedDevices []string  // Devices removed since last check
	ChangedDevices []string  // Devices with state changes
	Timestamp      time.Time // Reconciliation timestamp
	Duration       time.Duration // Time taken
}

// EventCorrelationKey represents a unique key for event correlation
type EventCorrelationKey struct {
	DevPath string
	Action  UdevAction
}

// CorrelatedEvent tracks recent events for deduplication
type CorrelatedEvent struct {
	Event     *UdevEvent
	Timestamp time.Time
	Count     int // Number of times this event was seen
}
