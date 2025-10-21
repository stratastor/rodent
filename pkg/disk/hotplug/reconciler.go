// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package hotplug

import (
	"context"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk/types"
)

// Reconciler periodically reconciles actual disk state with cached state.
//
// The reconciler provides a safety net for the udev monitor by ensuring that
// no events are missed. It runs on a configurable interval (default: 30s) and:
//
//  1. Calls DiscoveryFunc to get actual devices on the system
//  2. Calls CacheFunc to get Manager's current device cache
//  3. Computes the difference:
//     - AddedDevices: In actual but not in cache (missed 'add' event)
//     - RemovedDevices: In cache but not actual (missed 'remove' event)
//     - ChangedDevices: In both but properties differ (missed 'change' event)
//  4. Emits synthetic ReconciliationResult events for processing
//
// This ensures eventual consistency even if:
//   - udevadm monitor misses events due to buffer overflow
//   - The monitor wasn't running when a device was added/removed
//   - Race conditions cause events to be dropped
//
// The reconciler runs continuously until Stop() is called. It uses non-blocking
// channel sends to avoid blocking on event processing.
type Reconciler struct {
	logger logger.Logger
	ctx    context.Context
	cancel context.CancelFunc

	// Reconciliation configuration
	interval time.Duration

	// Callbacks
	discoveryFunc func(ctx context.Context) ([]*types.PhysicalDisk, error)
	cacheFunc     func() map[string]*types.PhysicalDisk

	// Event notification
	events chan *ReconciliationResult
}

// NewReconciler creates a new reconciliation loop
func NewReconciler(
	l logger.Logger,
	interval time.Duration,
	discoveryFunc func(ctx context.Context) ([]*types.PhysicalDisk, error),
	cacheFunc func() map[string]*types.PhysicalDisk,
) *Reconciler {
	ctx, cancel := context.WithCancel(context.Background())

	return &Reconciler{
		logger:        l,
		ctx:           ctx,
		cancel:        cancel,
		interval:      interval,
		discoveryFunc: discoveryFunc,
		cacheFunc:     cacheFunc,
		events:        make(chan *ReconciliationResult, 10),
	}
}

// Start begins the reconciliation loop
func (r *Reconciler) Start() error {
	r.logger.Info("starting reconciliation loop", "interval", r.interval)

	go r.run()

	return nil
}

// Stop stops the reconciliation loop
func (r *Reconciler) Stop() error {
	r.logger.Info("stopping reconciliation loop")
	r.cancel()
	close(r.events)
	return nil
}

// Events returns the reconciliation events channel
func (r *Reconciler) Events() <-chan *ReconciliationResult {
	return r.events
}

// run executes the reconciliation loop
func (r *Reconciler) run() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run immediately on start
	r.reconcile()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.reconcile()
		}
	}
}

// reconcile performs a single reconciliation pass
func (r *Reconciler) reconcile() {
	start := time.Now()
	r.logger.Debug("starting reconciliation pass")

	// Get current cache state
	cache := r.cacheFunc()

	// Discover actual devices
	ctx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
	defer cancel()

	discovered, err := r.discoveryFunc(ctx)
	if err != nil {
		r.logger.Error("reconciliation discovery failed", "error", err)
		return
	}

	// Build result
	result := &ReconciliationResult{
		Timestamp: start,
		Duration:  time.Since(start),
	}

	// Build maps for comparison
	discoveredMap := make(map[string]*types.PhysicalDisk)
	for _, disk := range discovered {
		discoveredMap[disk.DeviceID] = disk
	}

	// Find added devices (in discovered but not in cache)
	for deviceID := range discoveredMap {
		if _, exists := cache[deviceID]; !exists {
			result.AddedDevices = append(result.AddedDevices, deviceID)
		}
	}

	// Find removed devices (in cache but not in discovered)
	for deviceID := range cache {
		if _, exists := discoveredMap[deviceID]; !exists {
			result.RemovedDevices = append(result.RemovedDevices, deviceID)
		}
	}

	// Find changed devices (basic comparison - check if properties differ)
	for deviceID, discoveredDisk := range discoveredMap {
		cachedDisk, exists := cache[deviceID]
		if !exists {
			continue
		}

		if r.hasChanged(cachedDisk, discoveredDisk) {
			result.ChangedDevices = append(result.ChangedDevices, deviceID)
		}
	}

	result.Duration = time.Since(start)

	// Log results
	if len(result.AddedDevices) > 0 || len(result.RemovedDevices) > 0 || len(result.ChangedDevices) > 0 {
		r.logger.Info("reconciliation completed",
			"added", len(result.AddedDevices),
			"removed", len(result.RemovedDevices),
			"changed", len(result.ChangedDevices),
			"duration", result.Duration)
	} else {
		r.logger.Debug("reconciliation completed - no changes",
			"duration", result.Duration)
	}

	// Send result (non-blocking)
	select {
	case r.events <- result:
	default:
		r.logger.Warn("reconciliation event buffer full")
	}
}

// hasChanged checks if a disk has changed between cache and discovered state
func (r *Reconciler) hasChanged(cached, discovered *types.PhysicalDisk) bool {
	// Compare key properties
	if cached.Serial != discovered.Serial {
		return true
	}

	if cached.Model != discovered.Model {
		return true
	}

	if cached.SizeBytes != discovered.SizeBytes {
		return true
	}

	if cached.DevicePath != discovered.DevicePath {
		return true
	}

	// Compare health status
	if cached.Health != discovered.Health {
		return true
	}

	return false
}

// TriggerNow triggers an immediate reconciliation pass
func (r *Reconciler) TriggerNow() {
	go r.reconcile()
}
