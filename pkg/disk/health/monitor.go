// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk/parsers"
	"github.com/stratastor/rodent/pkg/disk/tools"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// Monitor handles SMART health monitoring
type Monitor struct {
	logger      logger.Logger
	smartctl    *tools.SmartctlExecutor
	thresholds  *types.SMARTThresholds
	mu          sync.RWMutex
	healthCache map[string]*HealthStatus // Keyed by device ID
}

// HealthStatus represents the health status of a device
type HealthStatus struct {
	DeviceID     string
	Health       types.HealthStatus
	HealthReason string
	SMARTInfo    *types.SMARTInfo
	CheckedAt    time.Time
}

// NewMonitor creates a new SMART health monitor
func NewMonitor(
	l logger.Logger,
	smartctl *tools.SmartctlExecutor,
	thresholds *types.SMARTThresholds,
) *Monitor {
	if thresholds == nil {
		thresholds = types.DefaultSMARTThresholds()
	}

	return &Monitor{
		logger:      l,
		smartctl:    smartctl,
		thresholds:  thresholds,
		healthCache: make(map[string]*HealthStatus),
	}
}

// CheckHealth checks the health of a single disk
func (m *Monitor) CheckHealth(ctx context.Context, disk *types.PhysicalDisk) (*HealthStatus, error) {
	if !disk.SMARTAvailable {
		return &HealthStatus{
			DeviceID:     disk.DeviceID,
			Health:       types.HealthUnknown,
			HealthReason: "SMART not available",
			CheckedAt:    time.Now(),
		}, nil
	}

	// Get SMART data
	output, err := m.smartctl.GetAll(ctx, disk.DevicePath)
	if err != nil {
		return nil, errors.Wrap(err, errors.DiskHealthCheckFailed).
			WithMetadata("device_id", disk.DeviceID).
			WithMetadata("device_path", disk.DevicePath)
	}

	// Parse SMART data
	smartInfo, err := parsers.ParseSmartctlJSON(output, disk.DeviceID)
	if err != nil {
		return nil, errors.Wrap(err, errors.DiskSMARTParseFailed).
			WithMetadata("device_id", disk.DeviceID)
	}

	// Evaluate health
	health, reason := smartInfo.EvaluateHealth(m.thresholds)

	status := &HealthStatus{
		DeviceID:     disk.DeviceID,
		Health:       health,
		HealthReason: reason,
		SMARTInfo:    smartInfo,
		CheckedAt:    time.Now(),
	}

	// Update cache
	m.mu.Lock()
	m.healthCache[disk.DeviceID] = status
	m.mu.Unlock()

	return status, nil
}

// CheckAllHealth checks health of all provided disks
func (m *Monitor) CheckAllHealth(ctx context.Context, disks []*types.PhysicalDisk) ([]*HealthStatus, error) {
	var statuses []*HealthStatus
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Check health concurrently (with limit)
	semaphore := make(chan struct{}, 4) // Max 4 concurrent checks

	for _, disk := range disks {
		if !disk.SMARTAvailable {
			continue
		}

		wg.Add(1)
		go func(d *types.PhysicalDisk) {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			status, err := m.CheckHealth(ctx, d)
			if err != nil {
				m.logger.Warn("failed to check health",
					"device_id", d.DeviceID,
					"error", err)
				return
			}

			mu.Lock()
			statuses = append(statuses, status)
			mu.Unlock()
		}(disk)
	}

	wg.Wait()

	return statuses, nil
}

// RefreshSMART refreshes SMART data for a disk without evaluating health
func (m *Monitor) RefreshSMART(ctx context.Context, disk *types.PhysicalDisk) (*types.SMARTInfo, error) {
	if !disk.SMARTAvailable {
		return nil, errors.New(errors.DiskSMARTNotAvailable, "SMART not available").
			WithMetadata("device_id", disk.DeviceID)
	}

	// Get SMART data
	output, err := m.smartctl.GetAll(ctx, disk.DevicePath)
	if err != nil {
		return nil, errors.Wrap(err, errors.DiskSMARTRefreshFailed).
			WithMetadata("device_id", disk.DeviceID).
			WithMetadata("device_path", disk.DevicePath)
	}

	// Parse SMART data
	smartInfo, err := parsers.ParseSmartctlJSON(output, disk.DeviceID)
	if err != nil {
		return nil, errors.Wrap(err, errors.DiskSMARTParseFailed).
			WithMetadata("device_id", disk.DeviceID)
	}

	return smartInfo, nil
}

// GetCachedHealth returns cached health status
func (m *Monitor) GetCachedHealth(deviceID string) (*HealthStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.healthCache[deviceID]
	if !exists {
		return nil, false
	}

	// Return a copy
	statusCopy := *status
	return &statusCopy, true
}

// GetAllCachedHealth returns all cached health statuses
func (m *Monitor) GetAllCachedHealth() map[string]*HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cache := make(map[string]*HealthStatus, len(m.healthCache))
	for k, v := range m.healthCache {
		statusCopy := *v
		cache[k] = &statusCopy
	}

	return cache
}

// UpdateThresholds updates the SMART thresholds
func (m *Monitor) UpdateThresholds(thresholds *types.SMARTThresholds) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.thresholds = thresholds
}

// GetThresholds returns current SMART thresholds
func (m *Monitor) GetThresholds() *types.SMARTThresholds {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy
	thresholds := *m.thresholds
	return &thresholds
}

// IdentifyDegradedDisks returns disks with warning or critical health status
func (m *Monitor) IdentifyDegradedDisks() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var degraded []string
	for deviceID, status := range m.healthCache {
		if status.Health == types.HealthWarning || status.Health == types.HealthCritical {
			degraded = append(degraded, deviceID)
		}
	}

	return degraded
}

// IdentifyFailedDisks returns disks with failed health status
func (m *Monitor) IdentifyFailedDisks() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var failed []string
	for deviceID, status := range m.healthCache {
		if status.Health == types.HealthFailed {
			failed = append(failed, deviceID)
		}
	}

	return failed
}

// GetHealthSummary returns a summary of health statuses
func (m *Monitor) GetHealthSummary() map[types.HealthStatus]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := map[types.HealthStatus]int{
		types.HealthUnknown:  0,
		types.HealthHealthy:  0,
		types.HealthWarning:  0,
		types.HealthCritical: 0,
		types.HealthFailed:   0,
	}

	for _, status := range m.healthCache {
		summary[status.Health]++
	}

	return summary
}
