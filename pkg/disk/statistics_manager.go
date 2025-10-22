// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"time"

	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// ============================================================================
// Statistics Operations
// ============================================================================

// DeviceStatistics represents statistics for a single device
type DeviceStatistics struct {
	DeviceID        string    `json:"device_id"`
	TotalProbes     int       `json:"total_probes"`
	FailedProbes    int       `json:"failed_probes"`
	LastProbeTime   time.Time `json:"last_probe_time,omitempty"`
	LastProbeStatus string    `json:"last_probe_status,omitempty"`
	HealthChanges   int       `json:"health_changes"`
	ErrorCount      int       `json:"error_count"`
}

// GetDeviceStatistics returns statistics for a specific device
func (m *Manager) GetDeviceStatistics(deviceID string) (*DeviceStatistics, error) {
	m.cacheMu.RLock()
	_, exists := m.deviceCache[deviceID]
	m.cacheMu.RUnlock()

	if !exists {
		return nil, errors.New(errors.DiskNotFound, "device not found").
			WithMetadata("device_id", deviceID)
	}

	// Get probe history for this device
	probeHistory := m.stateManager.GetProbeHistory(deviceID, 0)

	stats := &DeviceStatistics{
		DeviceID:     deviceID,
		TotalProbes:  len(probeHistory),
		FailedProbes: 0,
	}

	// Calculate statistics from probe history
	for _, probe := range probeHistory {
		if probe.Status == types.ProbeStatusFailed {
			stats.FailedProbes++
		}

		// Track most recent probe (StartedAt is a pointer)
		if probe.StartedAt != nil {
			if stats.LastProbeTime.IsZero() || probe.StartedAt.After(stats.LastProbeTime) {
				stats.LastProbeTime = *probe.StartedAt
				stats.LastProbeStatus = string(probe.Status)
			}
		}
	}

	// Get device state for additional stats
	deviceState, _ := m.stateManager.GetDeviceState(deviceID)
	if deviceState != nil {
		stats.HealthChanges = deviceState.HealthChanges
		stats.ErrorCount = deviceState.FailedProbeCount
	}

	return stats, nil
}
