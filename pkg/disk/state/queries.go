// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// ============================================================================
// Probe Execution Queries
// ============================================================================

// GetProbeExecution retrieves a specific probe execution by ID
func (sm *StateManager) GetProbeExecution(probeID string) (*types.ProbeExecution, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	execution, exists := sm.state.ProbeExecutions[probeID]
	if !exists {
		return nil, errors.New(errors.DiskProbeNotFound, "probe execution not found").
			WithMetadata("probe_id", probeID)
	}

	return execution, nil
}

// GetProbeHistory returns probe history for a specific device, limited to the specified count
func (sm *StateManager) GetProbeHistory(deviceID string, limit int) []*types.ProbeExecution {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var history []*types.ProbeExecution

	// Iterate through all probe executions and filter by device ID
	for _, execution := range sm.state.ProbeExecutions {
		if execution.DeviceID == deviceID {
			history = append(history, execution)
		}
	}

	// Sort by start time (newest first)
	// TODO: Implement sorting if needed

	// Apply limit
	if limit > 0 && len(history) > limit {
		history = history[:limit]
	}

	return history
}

// GetActiveProbes returns all currently running probes
func (sm *StateManager) GetActiveProbes() []*types.ProbeExecution {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var activeProbes []*types.ProbeExecution

	for _, execution := range sm.state.ProbeExecutions {
		if execution.Status == types.ProbeStatusRunning || execution.Status == types.ProbeStatusScheduled {
			activeProbes = append(activeProbes, execution)
		}
	}

	return activeProbes
}

// ============================================================================
// Probe Schedule Queries
// ============================================================================

// GetProbeSchedule retrieves a specific probe schedule by ID
func (sm *StateManager) GetProbeSchedule(scheduleID string) (*types.ProbeSchedule, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	schedule, exists := sm.state.ProbeSchedules[scheduleID]
	if !exists {
		return nil, errors.New(errors.DiskProbeScheduleNotFound, "probe schedule not found").
			WithMetadata("schedule_id", scheduleID)
	}

	return schedule, nil
}

// GetProbeSchedules returns all probe schedules
func (sm *StateManager) GetProbeSchedules() []*types.ProbeSchedule {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	schedules := make([]*types.ProbeSchedule, 0, len(sm.state.ProbeSchedules))
	for _, schedule := range sm.state.ProbeSchedules {
		schedules = append(schedules, schedule)
	}

	return schedules
}

// ============================================================================
// Device State Queries
// ============================================================================

// GetDeviceState retrieves the state for a specific device
func (sm *StateManager) GetDeviceState(deviceID string) (*types.DeviceState, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	deviceState, exists := sm.state.Devices[deviceID]
	if !exists {
		return nil, errors.New(errors.DiskNotFound, "device state not found").
			WithMetadata("device_id", deviceID)
	}

	return deviceState, nil
}

// ============================================================================
// Operation Queries
// ============================================================================

// GetOperation retrieves a specific operation by ID
func (sm *StateManager) GetOperation(operationID string) (*types.OperationState, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	operation, exists := sm.state.Operations[operationID]
	if !exists {
		return nil, errors.New(errors.DiskOperationNotFound, "operation not found").
			WithMetadata("operation_id", operationID)
	}

	return operation, nil
}

// GetActiveOperations returns all currently active operations
func (sm *StateManager) GetActiveOperations() []*types.OperationState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var activeOps []*types.OperationState

	for _, op := range sm.state.Operations {
		if op.Status == "running" || op.Status == "pending" {
			activeOps = append(activeOps, op)
		}
	}

	return activeOps
}
