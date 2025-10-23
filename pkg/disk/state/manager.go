// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// StateManager manages disk manager state with JSON persistence
type StateManager struct {
	logger      logger.Logger
	statePath   string
	state       *types.DiskManagerState
	mu          sync.RWMutex
	saveTimer   *time.Timer
	saveDelay   time.Duration
	savePending bool
}

// NewStateManager creates a new state manager
func NewStateManager(l logger.Logger, stateFileName string) *StateManager {
	if stateFileName == "" {
		stateFileName = types.DefaultStateFile
	}

	statePath := filepath.Join(config.GetDiskDir(), stateFileName)

	return &StateManager{
		logger:    l,
		statePath: statePath,
		state:     types.NewDiskManagerState(),
		saveDelay: types.DefaultStateSaveDelay,
	}
}

// Load loads state from disk
func (sm *StateManager) Load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if state file exists
	if _, err := os.Stat(sm.statePath); os.IsNotExist(err) {
		sm.logger.Info("state file not found, starting with empty state",
			"path", sm.statePath)
		return nil
	}

	// Read state file
	data, err := os.ReadFile(sm.statePath)
	if err != nil {
		return errors.Wrap(err, errors.DiskStateLoadFailed).
			WithMetadata("path", sm.statePath)
	}

	// Parse JSON
	var state types.DiskManagerState
	if err := json.Unmarshal(data, &state); err != nil {
		sm.logger.Warn("failed to parse state file, backing up and starting fresh",
			"error", err,
			"path", sm.statePath)

		// Backup corrupted state
		backupPath := sm.statePath + ".corrupted." + time.Now().Format("20060102-150405")
		if err := os.Rename(sm.statePath, backupPath); err != nil {
			sm.logger.Error("failed to backup corrupted state", "error", err)
		}

		return nil
	}

	sm.state = &state
	sm.logger.Info("state loaded successfully",
		"path", sm.statePath,
		"devices", len(state.Devices),
		"probes", len(state.ProbeExecutions))

	return nil
}

// Save saves state to disk immediately
func (sm *StateManager) Save() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.saveUnlocked()
}

// saveUnlocked saves without acquiring lock (caller must hold lock)
func (sm *StateManager) saveUnlocked() error {
	sm.state.UpdatedAt = time.Now()

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return errors.Wrap(err, errors.DiskStateSaveFailed).
			WithMetadata("path", sm.statePath)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(sm.statePath), 0755); err != nil {
		return errors.Wrap(err, errors.DiskStateSaveFailed).
			WithMetadata("path", sm.statePath).
			WithMetadata("operation", "mkdir")
	}

	// Write to temporary file first
	tempPath := sm.statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return errors.Wrap(err, errors.DiskStateSaveFailed).
			WithMetadata("path", tempPath).
			WithMetadata("operation", "write_temp")
	}

	// Backup current state if it exists
	if _, err := os.Stat(sm.statePath); err == nil {
		backupPath := sm.statePath + ".backup"
		if err := os.Rename(sm.statePath, backupPath); err != nil {
			sm.logger.Warn("failed to backup current state", "error", err)
		}
	}

	// Atomic rename
	if err := os.Rename(tempPath, sm.statePath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return errors.Wrap(err, errors.DiskStateSaveFailed).
			WithMetadata("path", sm.statePath).
			WithMetadata("operation", "rename")
	}

	sm.logger.Debug("state saved", "path", sm.statePath)
	sm.savePending = false

	return nil
}

// SaveDebounced schedules a save operation with debouncing
func (sm *StateManager) SaveDebounced() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.scheduleSaveUnlocked()
}

// scheduleSaveUnlocked schedules a save without acquiring the lock
// Must be called with sm.mu held
func (sm *StateManager) scheduleSaveUnlocked() {
	// Cancel existing timer if any
	if sm.saveTimer != nil {
		sm.saveTimer.Stop()
	}

	sm.savePending = true

	// Schedule save after delay
	sm.saveTimer = time.AfterFunc(sm.saveDelay, func() {
		if err := sm.Save(); err != nil {
			sm.logger.Error("failed to save state", "error", err)
		}
	})
}

// Get returns a reference to the state (caller should use locking methods)
func (sm *StateManager) Get() *types.DiskManagerState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.state
}

// UpdateDeviceState updates device state and saves
func (sm *StateManager) UpdateDeviceState(
	deviceID string,
	state types.DiskState,
	health types.HealthStatus,
) {
	sm.mu.Lock()
	sm.state.UpdateDeviceState(deviceID, state, health)
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// AddProbeExecution adds a probe execution to state
func (sm *StateManager) AddProbeExecution(execution *types.ProbeExecution) {
	sm.mu.Lock()
	sm.state.AddProbeExecution(execution)
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// AddProbeSchedule adds a probe schedule to state
func (sm *StateManager) AddProbeSchedule(schedule *types.ProbeSchedule) {
	sm.mu.Lock()
	sm.state.AddProbeSchedule(schedule)
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// RemoveProbeSchedule removes a probe schedule from state
func (sm *StateManager) RemoveProbeSchedule(scheduleID string) {
	sm.mu.Lock()
	sm.state.RemoveProbeSchedule(scheduleID)
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// AddOperation adds an operation to state
func (sm *StateManager) AddOperation(op *types.OperationState) {
	sm.mu.Lock()
	sm.state.AddOperation(op)
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// CompleteOperation marks an operation as completed
func (sm *StateManager) CompleteOperation(opID string, success bool, message string) {
	sm.mu.Lock()
	sm.state.CompleteOperation(opID, success, message)
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// UpdateStatistics updates global statistics
func (sm *StateManager) UpdateStatistics() {
	sm.mu.Lock()
	sm.state.UpdateStatistics()
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// RecordDiscoveryCompleted updates discovery statistics in real-time
func (sm *StateManager) RecordDiscoveryCompleted(devicesFound int) {
	sm.mu.Lock()
	sm.state.Statistics.TotalDiscoveries++
	sm.state.Statistics.LastDiscoveryAt = time.Now()
	sm.state.Statistics.TotalDevicesFound += devicesFound
	sm.state.Statistics.UpdatedAt = time.Now()
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// RecordProbeCompleted updates probe statistics in real-time
func (sm *StateManager) RecordProbeCompleted(probeType string, success bool) {
	sm.mu.Lock()
	sm.state.Statistics.TotalProbes++
	switch probeType {
	case "quick":
		sm.state.Statistics.TotalQuickProbes++
	case "extensive":
		sm.state.Statistics.TotalExtensiveProbes++
	}
	sm.state.Statistics.LastProbeAt = time.Now()
	if success {
		sm.state.Statistics.SuccessfulProbes++
	} else {
		sm.state.Statistics.FailedProbes++
	}
	sm.state.Statistics.UpdatedAt = time.Now()
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// RecordProbeConflict updates conflict statistics in real-time
func (sm *StateManager) RecordProbeConflict() {
	sm.mu.Lock()
	sm.state.Statistics.ConflictedProbes++
	sm.state.Statistics.UpdatedAt = time.Now()
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// RecordError updates error statistics in real-time
func (sm *StateManager) RecordError(errorMessage string) {
	sm.mu.Lock()
	sm.state.Statistics.TotalErrors++
	sm.state.Statistics.LastErrorAt = time.Now()
	sm.state.Statistics.LastErrorMessage = errorMessage
	sm.state.Statistics.UpdatedAt = time.Now()
	sm.mu.Unlock()

	sm.SaveDebounced()
}

// CalculateStatistics calculates statistics on-demand
// This recalculates device state/health distributions from current device states
func (sm *StateManager) CalculateStatistics() *types.GlobalStatistics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Clone the statistics to avoid external modification
	stats := &types.GlobalStatistics{
		TotalDiscoveries:     sm.state.Statistics.TotalDiscoveries,
		LastDiscoveryAt:      sm.state.Statistics.LastDiscoveryAt,
		TotalDevicesFound:    sm.state.Statistics.TotalDevicesFound,
		TotalProbes:          sm.state.Statistics.TotalProbes,
		TotalQuickProbes:     sm.state.Statistics.TotalQuickProbes,
		TotalExtensiveProbes: sm.state.Statistics.TotalExtensiveProbes,
		LastProbeAt:          sm.state.Statistics.LastProbeAt,
		SuccessfulProbes:     sm.state.Statistics.SuccessfulProbes,
		FailedProbes:         sm.state.Statistics.FailedProbes,
		ConflictedProbes:     sm.state.Statistics.ConflictedProbes,
		TotalErrors:          sm.state.Statistics.TotalErrors,
		LastErrorAt:          sm.state.Statistics.LastErrorAt,
		LastErrorMessage:     sm.state.Statistics.LastErrorMessage,
		ManagerStartedAt:     sm.state.Statistics.ManagerStartedAt,
		UpdatedAt:            time.Now(),
	}

	// Recalculate current uptime
	stats.Uptime = time.Since(stats.ManagerStartedAt)

	// Recalculate device state distributions
	stats.CurrentDeviceCount = len(sm.state.Devices)
	stats.AvailableDevices = 0
	stats.InUseDevices = 0
	stats.FaultedDevices = 0
	stats.OfflineDevices = 0

	for _, device := range sm.state.Devices {
		switch device.State {
		case types.DiskStateAvailable:
			stats.AvailableDevices++
		case types.DiskStateInUse:
			stats.InUseDevices++
		case types.DiskStateFaulted:
			stats.FaultedDevices++
		case types.DiskStateOffline:
			stats.OfflineDevices++
		}
	}

	// Recalculate device health distributions
	stats.HealthyDevices = 0
	stats.WarningDevices = 0
	stats.CriticalDevices = 0
	stats.FailedDevices = 0

	for _, device := range sm.state.Devices {
		switch device.Health {
		case types.HealthHealthy:
			stats.HealthyDevices++
		case types.HealthWarning:
			stats.WarningDevices++
		case types.HealthCritical:
			stats.CriticalDevices++
		case types.HealthFailed:
			stats.FailedDevices++
		}
	}

	return stats
}

// CleanupOldExecutions removes old probe executions
func (sm *StateManager) CleanupOldExecutions(retentionPeriod time.Duration) int {
	sm.mu.Lock()
	removed := sm.state.CleanupOldExecutions(retentionPeriod)
	sm.mu.Unlock()

	if removed > 0 {
		sm.SaveDebounced()
		sm.logger.Info("cleaned up old probe executions",
			"removed", removed,
			"retention", retentionPeriod)
	}

	return removed
}

// WithLock executes a function with state locked
func (sm *StateManager) WithLock(fn func(*types.DiskManagerState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	fn(sm.state)
	sm.scheduleSaveUnlocked()
}

// WithRLock executes a function with state read-locked
func (sm *StateManager) WithRLock(fn func(*types.DiskManagerState)) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	fn(sm.state)
}

// Flush forces an immediate save if there's a pending save
func (sm *StateManager) Flush() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.saveTimer != nil {
		sm.saveTimer.Stop()
		sm.saveTimer = nil
	}

	if sm.savePending {
		return sm.saveUnlocked()
	}

	return nil
}

// GetStatePath returns the state file path
func (sm *StateManager) GetStatePath() string {
	return sm.statePath
}
