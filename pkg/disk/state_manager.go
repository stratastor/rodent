// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// GetState returns the current disk manager state
func (m *Manager) GetState() *types.DiskManagerState {
	return m.stateManager.Get()
}

// GetStatistics returns calculated statistics on-demand
func (m *Manager) GetStatistics() *types.GlobalStatistics {
	return m.stateManager.CalculateStatistics()
}

// SetDiskState sets the state for a specific disk
func (m *Manager) SetDiskState(deviceID string, state types.DiskState, reason string) error {
	// Verify disk exists
	m.cacheMu.RLock()
	disk, exists := m.deviceCache[deviceID]
	m.cacheMu.RUnlock()

	if !exists {
		return errors.New(errors.DiskNotFound, "device not found").
			WithMetadata("device_id", deviceID)
	}

	// Update state
	m.stateManager.UpdateDeviceState(deviceID, state, disk.Health)

	m.logger.Info("disk state updated",
		"device_id", deviceID,
		"state", state,
		"reason", reason)

	return nil
}

// GetDeviceState retrieves the state for a specific device
func (m *Manager) GetDeviceState(deviceID string) (*types.DeviceState, error) {
	return m.stateManager.GetDeviceState(deviceID)
}

// QuarantineDisk quarantines a disk (convenience method)
func (m *Manager) QuarantineDisk(deviceID string, reason string) error {
	return m.SetDiskState(deviceID, types.DiskStateQuarantined, reason)
}
