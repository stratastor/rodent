// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"context"

	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// ============================================================================
// Probe Operations (Priority 1 API)
// ============================================================================

// TriggerProbe manually triggers a SMART probe on a specific device
func (m *Manager) TriggerProbe(ctx context.Context, deviceID string, probeType types.ProbeType) (string, error) {
	// Get device from cache to retrieve device path
	m.cacheMu.RLock()
	disk, exists := m.deviceCache[deviceID]
	m.cacheMu.RUnlock()

	if !exists {
		return "", errors.New(errors.DiskNotFound, "device not found").
			WithMetadata("device_id", deviceID)
	}

	// Trigger probe via scheduler
	probeID, err := m.probeScheduler.TriggerProbe(ctx, deviceID, disk.DevicePath, probeType)
	if err != nil {
		return "", errors.Wrap(err, errors.DiskProbeStartFailed).
			WithMetadata("device_id", deviceID).
			WithMetadata("probe_type", string(probeType))
	}

	m.logger.Info("probe triggered",
		"device_id", deviceID,
		"probe_type", probeType,
		"probe_id", probeID)

	return probeID, nil
}

// CancelProbe cancels a running probe
func (m *Manager) CancelProbe(probeID string) error {
	// Get probe execution from state
	execution, err := m.stateManager.GetProbeExecution(probeID)
	if err != nil {
		return errors.Wrap(err, errors.DiskProbeNotFound).
			WithMetadata("probe_id", probeID)
	}

	// Check if probe is still running
	if execution.Status == types.ProbeStatusCompleted ||
		execution.Status == types.ProbeStatusFailed ||
		execution.Status == types.ProbeStatusCancelled {
		return errors.New(errors.DiskProbeNotRunning, "probe is not running").
			WithMetadata("probe_id", probeID).
			WithMetadata("status", string(execution.Status))
	}

	// Cancel the probe
	execution.Cancel()

	m.logger.Info("probe cancelled", "probe_id", probeID)

	return nil
}

// GetProbeExecution retrieves details of a probe execution
func (m *Manager) GetProbeExecution(probeID string) (*types.ProbeExecution, error) {
	execution, err := m.stateManager.GetProbeExecution(probeID)
	if err != nil {
		return nil, errors.Wrap(err, errors.DiskProbeNotFound).
			WithMetadata("probe_id", probeID)
	}

	return execution, nil
}

// GetActiveProbes returns all currently running probes
func (m *Manager) GetActiveProbes() []*types.ProbeExecution {
	return m.probeScheduler.GetActiveProbes()
}

// GetProbeHistory returns probe history for a specific device
func (m *Manager) GetProbeHistory(deviceID string, limit int) ([]*types.ProbeExecution, error) {
	// Verify device exists
	m.cacheMu.RLock()
	_, exists := m.deviceCache[deviceID]
	m.cacheMu.RUnlock()

	if !exists {
		return nil, errors.New(errors.DiskNotFound, "device not found").
			WithMetadata("device_id", deviceID)
	}

	// Get history from state manager
	history := m.stateManager.GetProbeHistory(deviceID, limit)

	return history, nil
}

// ============================================================================
// Probe Schedule Operations (Priority 2 API)
// ============================================================================

// GetProbeSchedules returns all probe schedules
func (m *Manager) GetProbeSchedules() []*types.ProbeSchedule {
	return m.stateManager.GetProbeSchedules()
}

// GetProbeSchedule retrieves a specific probe schedule
func (m *Manager) GetProbeSchedule(scheduleID string) (*types.ProbeSchedule, error) {
	schedule, err := m.stateManager.GetProbeSchedule(scheduleID)
	if err != nil {
		return nil, errors.Wrap(err, errors.DiskProbeScheduleNotFound).
			WithMetadata("schedule_id", scheduleID)
	}

	return schedule, nil
}

// CreateProbeSchedule creates a new probe schedule
func (m *Manager) CreateProbeSchedule(schedule *types.ProbeSchedule) error {
	// Validate schedule
	if schedule.ID == "" {
		return errors.New(errors.ServerRequestValidation, "schedule_id is required")
	}

	if schedule.CronExpression == "" {
		return errors.New(errors.ServerRequestValidation, "cron_expression is required")
	}

	// Add schedule to scheduler
	if err := m.probeScheduler.AddSchedule(m.ctx, schedule); err != nil {
		return errors.Wrap(err, errors.DiskProbeScheduleCreateFailed).
			WithMetadata("schedule_id", schedule.ID)
	}

	m.logger.Info("probe schedule created",
		"schedule_id", schedule.ID,
		"cron", schedule.CronExpression,
		"probe_type", schedule.Type)

	return nil
}

// UpdateProbeSchedule updates an existing probe schedule
func (m *Manager) UpdateProbeSchedule(scheduleID string, schedule *types.ProbeSchedule) error {
	// Verify schedule exists
	if _, err := m.stateManager.GetProbeSchedule(scheduleID); err != nil {
		return errors.Wrap(err, errors.DiskProbeScheduleNotFound).
			WithMetadata("schedule_id", scheduleID)
	}

	// Ensure schedule ID matches
	schedule.ID = scheduleID

	// Remove old schedule and add new one (update)
	if err := m.probeScheduler.RemoveSchedule(m.ctx, scheduleID); err != nil {
		return errors.Wrap(err, errors.DiskProbeScheduleUpdateFailed).
			WithMetadata("schedule_id", scheduleID).
			WithMetadata("operation", "remove_old")
	}

	if err := m.probeScheduler.AddSchedule(m.ctx, schedule); err != nil {
		return errors.Wrap(err, errors.DiskProbeScheduleUpdateFailed).
			WithMetadata("schedule_id", scheduleID).
			WithMetadata("operation", "add_new")
	}

	m.logger.Info("probe schedule updated", "schedule_id", scheduleID)

	return nil
}

// DeleteProbeSchedule deletes a probe schedule
func (m *Manager) DeleteProbeSchedule(scheduleID string) error {
	// Remove schedule from scheduler
	if err := m.probeScheduler.RemoveSchedule(m.ctx, scheduleID); err != nil {
		return errors.Wrap(err, errors.DiskProbeScheduleDeleteFailed).
			WithMetadata("schedule_id", scheduleID)
	}

	m.logger.Info("probe schedule deleted", "schedule_id", scheduleID)

	return nil
}

// EnableProbeSchedule enables a probe schedule
func (m *Manager) EnableProbeSchedule(scheduleID string) error {
	// Get schedule
	schedule, err := m.stateManager.GetProbeSchedule(scheduleID)
	if err != nil {
		return errors.Wrap(err, errors.DiskProbeScheduleNotFound).
			WithMetadata("schedule_id", scheduleID)
	}

	// If already enabled, nothing to do
	if schedule.Enabled {
		return nil
	}

	// Enable and update
	schedule.Enabled = true
	if err := m.UpdateProbeSchedule(scheduleID, schedule); err != nil {
		return err
	}

	m.logger.Info("probe schedule enabled", "schedule_id", scheduleID)

	return nil
}

// DisableProbeSchedule disables a probe schedule
func (m *Manager) DisableProbeSchedule(scheduleID string) error {
	// Get schedule
	schedule, err := m.stateManager.GetProbeSchedule(scheduleID)
	if err != nil {
		return errors.Wrap(err, errors.DiskProbeScheduleNotFound).
			WithMetadata("schedule_id", scheduleID)
	}

	// If already disabled, nothing to do
	if !schedule.Enabled {
		return nil
	}

	// Disable and update
	schedule.Enabled = false
	if err := m.UpdateProbeSchedule(scheduleID, schedule); err != nil {
		return err
	}

	m.logger.Info("probe schedule disabled", "schedule_id", scheduleID)

	return nil
}
