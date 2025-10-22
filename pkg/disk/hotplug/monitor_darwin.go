// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package hotplug

// Suppress unused warnings for methods used in Linux-specific code
var (
	_ = (*Monitor).isRelevantEvent
	_ = (*Monitor).emitEvent
)

// Start begins monitoring udev events
// Darwin stub: No netlink support on macOS, relies on reconciliation loop only
func (m *Monitor) Start(udevadmPath string) error {
	m.logger.Warn("udev netlink monitoring not available on macOS - relying on reconciliation loop only")

	// Start the correlation cleanup goroutine
	go m.correlationCleanup()

	// No actual monitoring on Darwin - reconciliation loop will handle everything
	return nil
}

// Stop stops the udev monitor
func (m *Monitor) Stop() error {
	m.logger.Info("stopping udev monitor")
	m.cancel()
	close(m.events)
	close(m.errors)
	return nil
}
