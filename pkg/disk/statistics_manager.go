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

// GlobalStatistics represents aggregated statistics
type GlobalStatistics struct {
	TotalDisks    int       `json:"total_disks"`
	HealthyDisks  int       `json:"healthy_disks"`
	WarningDisks  int       `json:"warning_disks"`
	FailingDisks  int       `json:"failing_disks"`
	UnknownDisks  int       `json:"unknown_disks"`
	TotalProbes   int       `json:"total_probes"`
	ActiveProbes  int       `json:"active_probes"`
	LastDiscovery time.Time `json:"last_discovery,omitempty"`
	LastHealthCheck time.Time `json:"last_health_check,omitempty"`
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

// GetGlobalStatistics returns aggregated statistics for all disks
func (m *Manager) GetGlobalStatistics() *GlobalStatistics {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	stats := &GlobalStatistics{
		TotalDisks: len(m.deviceCache),
	}

	// Count by health status
	for _, disk := range m.deviceCache {
		switch disk.Health {
		case types.HealthHealthy:
			stats.HealthyDisks++
		case types.HealthWarning:
			stats.WarningDisks++
		case types.HealthCritical, types.HealthFailed:
			stats.FailingDisks++
		default:
			stats.UnknownDisks++
		}
	}

	// Get active probes count
	activeProbes := m.stateManager.GetActiveProbes()
	stats.ActiveProbes = len(activeProbes)

	// Get total probe count
	state := m.stateManager.Get()
	stats.TotalProbes = len(state.ProbeExecutions)

	return stats
}
