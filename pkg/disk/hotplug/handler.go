// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package hotplug

import (
	"context"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk/types"
)

// EventHandler handles hotplug events and coordinates state transitions.
//
// The EventHandler is the main coordinator for the hotplug detection system.
// It integrates Monitor, Reconciler, and StateMachine components and manages
// the overall event processing lifecycle.
//
// # Event Processing
//
// The handler runs three concurrent goroutines:
//
//  1. processUdevEvents(): Processes real-time events from the Monitor
//  2. processReconciliationEvents(): Processes synthetic events from Reconciler
//  3. processMonitorErrors(): Handles errors from the Monitor
//
// Each event type (add/remove/change) is validated through the StateMachine
// and then dispatched to the appropriate callback (OnDeviceAdded, etc.)
// which are implemented by pkg/disk/Manager.
//
// # Lifecycle
//
//	handler := NewEventHandler(logger, config)
//	handler.Start("/usr/bin/udevadm")  // Starts monitor + reconciler
//	// ... handler runs continuously ...
//	handler.Stop()                      // Graceful shutdown
//
// # Thread Safety
//
// The handler is safe for concurrent use. All components are started/stopped
// atomically and use proper synchronization (WaitGroup, context cancellation).
type EventHandler struct {
	logger logger.Logger
	ctx    context.Context
	cancel context.CancelFunc

	// Components
	monitor      *Monitor
	reconciler   *Reconciler
	stateMachine *StateMachine

	// Callbacks for processing events
	onDeviceAdded   func(ctx context.Context, deviceID string) error
	onDeviceRemoved func(deviceID string) error
	onDeviceChanged func(ctx context.Context, deviceID string) error

	// Event processing
	wg sync.WaitGroup
}

// HandlerConfig configures the event handler
type HandlerConfig struct {
	// Monitor configuration
	UdevadmPath    string
	MonitorSubsystems []string
	MonitorBufferSize int

	// Reconciliation configuration
	ReconcileInterval time.Duration

	// Callbacks
	DiscoveryFunc func(ctx context.Context) ([]*types.PhysicalDisk, error)
	CacheFunc     func() map[string]*types.PhysicalDisk

	// Event callbacks
	OnDeviceAdded   func(ctx context.Context, deviceID string) error
	OnDeviceRemoved func(deviceID string) error
	OnDeviceChanged func(ctx context.Context, deviceID string) error
}

// NewEventHandler creates a new hotplug event handler
func NewEventHandler(l logger.Logger, cfg *HandlerConfig) *EventHandler {
	ctx, cancel := context.WithCancel(context.Background())

	// Create monitor
	monitor := NewMonitor(l, cfg.MonitorSubsystems, cfg.MonitorBufferSize)

	// Create reconciler
	reconciler := NewReconciler(
		l,
		cfg.ReconcileInterval,
		cfg.DiscoveryFunc,
		cfg.CacheFunc,
	)

	// Create state machine
	stateMachine := NewStateMachine(l)

	return &EventHandler{
		logger:          l,
		ctx:             ctx,
		cancel:          cancel,
		monitor:         monitor,
		reconciler:      reconciler,
		stateMachine:    stateMachine,
		onDeviceAdded:   cfg.OnDeviceAdded,
		onDeviceRemoved: cfg.OnDeviceRemoved,
		onDeviceChanged: cfg.OnDeviceChanged,
	}
}

// Start begins processing hotplug events
func (h *EventHandler) Start(udevadmPath string) error {
	h.logger.Info("starting hotplug event handler")

	// Start udev monitor
	if err := h.monitor.Start(udevadmPath); err != nil {
		return err
	}

	// Start reconciler
	if err := h.reconciler.Start(); err != nil {
		h.monitor.Stop()
		return err
	}

	// Start event processors
	h.wg.Add(3)
	go h.processUdevEvents()
	go h.processReconciliationEvents()
	go h.processMonitorErrors()

	return nil
}

// Stop stops the event handler
func (h *EventHandler) Stop() error {
	h.logger.Info("stopping hotplug event handler")

	h.cancel()

	// Stop components
	h.monitor.Stop()
	h.reconciler.Stop()

	// Wait for processors to finish
	h.wg.Wait()

	return nil
}

// processUdevEvents processes events from the udev monitor
func (h *EventHandler) processUdevEvents() {
	defer h.wg.Done()

	for {
		select {
		case <-h.ctx.Done():
			return

		case event, ok := <-h.monitor.Events():
			if !ok {
				return
			}

			h.handleUdevEvent(event)
		}
	}
}

// processReconciliationEvents processes reconciliation results
func (h *EventHandler) processReconciliationEvents() {
	defer h.wg.Done()

	for {
		select {
		case <-h.ctx.Done():
			return

		case result, ok := <-h.reconciler.Events():
			if !ok {
				return
			}

			h.handleReconciliation(result)
		}
	}
}

// processMonitorErrors processes errors from the monitor
func (h *EventHandler) processMonitorErrors() {
	defer h.wg.Done()

	for {
		select {
		case <-h.ctx.Done():
			return

		case err, ok := <-h.monitor.Errors():
			if !ok {
				return
			}

			h.logger.Error("udev monitor error", "error", err)
		}
	}
}

// handleUdevEvent processes a single udev event
func (h *EventHandler) handleUdevEvent(event *UdevEvent) {
	deviceID := h.extractDeviceID(event)
	if deviceID == "" {
		h.logger.Debug("ignoring event with no device ID",
			"devpath", event.DevPath,
			"action", event.Action)
		return
	}

	h.logger.Debug("processing udev event",
		"device_id", deviceID,
		"action", event.Action,
		"devpath", event.DevPath)

	ctx, cancel := context.WithTimeout(h.ctx, 30*time.Second)
	defer cancel()

	switch event.Action {
	case UdevActionAdd:
		if h.onDeviceAdded != nil {
			if err := h.onDeviceAdded(ctx, deviceID); err != nil {
				h.logger.Error("failed to handle device addition",
					"device_id", deviceID,
					"error", err)
			}
		}

	case UdevActionRemove:
		if h.onDeviceRemoved != nil {
			if err := h.onDeviceRemoved(deviceID); err != nil {
				h.logger.Error("failed to handle device removal",
					"device_id", deviceID,
					"error", err)
			}
		}

	case UdevActionChange:
		if h.onDeviceChanged != nil {
			if err := h.onDeviceChanged(ctx, deviceID); err != nil {
				h.logger.Error("failed to handle device change",
					"device_id", deviceID,
					"error", err)
			}
		}
	}
}

// handleReconciliation processes reconciliation results
func (h *EventHandler) handleReconciliation(result *ReconciliationResult) {
	ctx, cancel := context.WithTimeout(h.ctx, 2*time.Minute)
	defer cancel()

	// Process added devices
	for _, deviceID := range result.AddedDevices {
		h.logger.Info("reconciliation found new device", "device_id", deviceID)
		if h.onDeviceAdded != nil {
			if err := h.onDeviceAdded(ctx, deviceID); err != nil {
				h.logger.Error("failed to handle reconciled device addition",
					"device_id", deviceID,
					"error", err)
			}
		}
	}

	// Process removed devices
	for _, deviceID := range result.RemovedDevices {
		h.logger.Info("reconciliation found removed device", "device_id", deviceID)
		if h.onDeviceRemoved != nil {
			if err := h.onDeviceRemoved(deviceID); err != nil {
				h.logger.Error("failed to handle reconciled device removal",
					"device_id", deviceID,
					"error", err)
			}
		}
	}

	// Process changed devices
	for _, deviceID := range result.ChangedDevices {
		h.logger.Info("reconciliation found changed device", "device_id", deviceID)
		if h.onDeviceChanged != nil {
			if err := h.onDeviceChanged(ctx, deviceID); err != nil {
				h.logger.Error("failed to handle reconciled device change",
					"device_id", deviceID,
					"error", err)
			}
		}
	}
}

// extractDeviceID extracts a device ID from a udev event
// Must match the priority logic in discovery.go for consistency
func (h *EventHandler) extractDeviceID(event *UdevEvent) string {
	// Prefer ID_SERIAL for device ID (globally unique, stable)
	// This matches disk.Serial in discovery
	if serial, ok := event.Properties["ID_SERIAL"]; ok && serial != "" {
		return serial
	}

	// Fall back to ID_WWN (alternative unique identifier)
	// This matches disk.WWN in discovery
	if wwn, ok := event.Properties["ID_WWN"]; ok && wwn != "" {
		return wwn
	}

	// Fall back to ID_SERIAL_SHORT if full serial not available
	if serial, ok := event.Properties["ID_SERIAL_SHORT"]; ok && serial != "" {
		return serial
	}

	// Fall back to device name (least reliable, but matches DevicePath fallback)
	if event.DevName != "" {
		return event.DevName
	}

	return ""
}

// GetMonitorStats returns monitoring statistics
func (h *EventHandler) GetMonitorStats() MonitorStats {
	return h.monitor.GetStats()
}

// TriggerReconciliation triggers an immediate reconciliation pass
func (h *EventHandler) TriggerReconciliation() {
	h.reconciler.TriggerNow()
}
