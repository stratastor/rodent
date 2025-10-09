// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package types

import "time"

// DiskManagerState represents the persistent state of the disk manager
type DiskManagerState struct {
	// State metadata
	Version   string    `json:"version"`    // State format version
	UpdatedAt time.Time `json:"updated_at"` // Last state update

	// Device state
	Devices map[string]*DeviceState `json:"devices"` // Keyed by DeviceID

	// Probe state
	ProbeExecutions map[string]*ProbeExecution `json:"probe_executions"` // Keyed by execution ID
	ProbeSchedules  map[string]*ProbeSchedule  `json:"probe_schedules"`  // Keyed by schedule ID
	ProbeHistory    map[string]*ProbeHistory   `json:"probe_history"`    // Keyed by device ID

	// Operation state
	Operations map[string]*OperationState `json:"operations"` // Keyed by operation ID

	// Statistics
	Statistics *GlobalStatistics `json:"statistics"`
}

// DeviceState represents the persistent state of a single device
type DeviceState struct {
	DeviceID string    `json:"device_id"`
	State    DiskState `json:"state"`
	Health   HealthStatus `json:"health"`

	// Timestamps
	FirstSeenAt  time.Time  `json:"first_seen_at"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	LastProbeAt  *time.Time `json:"last_probe_at,omitempty"`
	StateChangedAt time.Time `json:"state_changed_at"`

	// Counters
	ProbeCount       int `json:"probe_count"`
	FailedProbeCount int `json:"failed_probe_count"`
	HealthChanges    int `json:"health_changes"`

	// Last known values
	LastTemperature int    `json:"last_temperature"`
	LastPowerOnHours uint64 `json:"last_power_on_hours"`

	// Metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// OperationState represents the state of an ongoing or completed operation
type OperationState struct {
	ID          string        `json:"id"`           // Operation ID
	Type        OperationType `json:"type"`         // Operation type
	Status      string        `json:"status"`       // Status (running, completed, failed)
	DeviceID    string        `json:"device_id"`    // Target device (if applicable)
	StartedAt   time.Time     `json:"started_at"`   // Start time
	CompletedAt *time.Time    `json:"completed_at,omitempty"` // Completion time
	Progress    int           `json:"progress"`     // Progress percentage
	Message     string        `json:"message"`      // Status message
	Error       string        `json:"error,omitempty"` // Error message if failed
	Metadata    map[string]string `json:"metadata,omitempty"` // Additional metadata
}

// GlobalStatistics represents global disk manager statistics
type GlobalStatistics struct {
	// Discovery stats
	TotalDiscoveries   int       `json:"total_discoveries"`
	LastDiscoveryAt    time.Time `json:"last_discovery_at"`
	TotalDevicesFound  int       `json:"total_devices_found"`
	CurrentDeviceCount int       `json:"current_device_count"`

	// Probe stats
	TotalProbes         int       `json:"total_probes"`
	TotalQuickProbes    int       `json:"total_quick_probes"`
	TotalExtensiveProbes int      `json:"total_extensive_probes"`
	LastProbeAt         time.Time `json:"last_probe_at"`
	SuccessfulProbes    int       `json:"successful_probes"`
	FailedProbes        int       `json:"failed_probes"`
	ConflictedProbes    int       `json:"conflicted_probes"`

	// Health stats
	HealthyDevices  int `json:"healthy_devices"`
	WarningDevices  int `json:"warning_devices"`
	CriticalDevices int `json:"critical_devices"`
	FailedDevices   int `json:"failed_devices"`

	// State stats
	AvailableDevices int `json:"available_devices"`
	InUseDevices     int `json:"in_use_devices"`
	FaultedDevices   int `json:"faulted_devices"`
	OfflineDevices   int `json:"offline_devices"`

	// Error stats
	TotalErrors     int       `json:"total_errors"`
	LastErrorAt     time.Time `json:"last_error_at"`
	LastErrorMessage string   `json:"last_error_message,omitempty"`

	// Uptime
	ManagerStartedAt time.Time     `json:"manager_started_at"`
	Uptime           time.Duration `json:"uptime"` // Current uptime

	// Last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// NewDiskManagerState creates a new empty state
func NewDiskManagerState() *DiskManagerState {
	return &DiskManagerState{
		Version:         "1.0",
		UpdatedAt:       time.Now(),
		Devices:         make(map[string]*DeviceState),
		ProbeExecutions: make(map[string]*ProbeExecution),
		ProbeSchedules:  make(map[string]*ProbeSchedule),
		ProbeHistory:    make(map[string]*ProbeHistory),
		Operations:      make(map[string]*OperationState),
		Statistics:      NewGlobalStatistics(),
	}
}

// NewDeviceState creates a new device state
func NewDeviceState(deviceID string) *DeviceState {
	now := time.Now()
	return &DeviceState{
		DeviceID:       deviceID,
		State:          DiskStateDiscovered,
		Health:         HealthUnknown,
		FirstSeenAt:    now,
		LastSeenAt:     now,
		StateChangedAt: now,
		Metadata:       make(map[string]string),
	}
}

// NewOperationState creates a new operation state
func NewOperationState(id string, opType OperationType, deviceID string) *OperationState {
	return &OperationState{
		ID:        id,
		Type:      opType,
		Status:    "running",
		DeviceID:  deviceID,
		StartedAt: time.Now(),
		Progress:  0,
		Metadata:  make(map[string]string),
	}
}

// NewGlobalStatistics creates a new global statistics object
func NewGlobalStatistics() *GlobalStatistics {
	now := time.Now()
	return &GlobalStatistics{
		ManagerStartedAt: now,
		UpdatedAt:        now,
	}
}

// UpdateDeviceState updates or creates device state
func (s *DiskManagerState) UpdateDeviceState(deviceID string, state DiskState, health HealthStatus) {
	ds, exists := s.Devices[deviceID]
	if !exists {
		ds = NewDeviceState(deviceID)
		s.Devices[deviceID] = ds
	}

	// Track state changes
	if ds.State != state {
		ds.StateChangedAt = time.Now()
	}

	// Track health changes
	if ds.Health != health {
		ds.HealthChanges++
	}

	ds.State = state
	ds.Health = health
	ds.LastSeenAt = time.Now()
	s.UpdatedAt = time.Now()
}

// AddProbeExecution adds a probe execution to state
func (s *DiskManagerState) AddProbeExecution(execution *ProbeExecution) {
	s.ProbeExecutions[execution.ID] = execution

	// Update device state
	if ds, exists := s.Devices[execution.DeviceID]; exists {
		ds.ProbeCount++
		if execution.Status == ProbeStatusFailed {
			ds.FailedProbeCount++
		}
		if execution.CompletedAt != nil {
			ds.LastProbeAt = execution.CompletedAt
		}
	}

	// Update probe history
	if _, exists := s.ProbeHistory[execution.DeviceID]; !exists {
		s.ProbeHistory[execution.DeviceID] = &ProbeHistory{
			DeviceID:   execution.DeviceID,
			Executions: []*ProbeExecution{},
		}
	}
	s.ProbeHistory[execution.DeviceID].Executions = append(
		s.ProbeHistory[execution.DeviceID].Executions,
		execution,
	)
	s.ProbeHistory[execution.DeviceID].UpdatedAt = time.Now()

	s.UpdatedAt = time.Now()
}

// AddProbeSchedule adds a probe schedule to state
func (s *DiskManagerState) AddProbeSchedule(schedule *ProbeSchedule) {
	s.ProbeSchedules[schedule.ID] = schedule
	s.UpdatedAt = time.Now()
}

// RemoveProbeSchedule removes a probe schedule from state
func (s *DiskManagerState) RemoveProbeSchedule(scheduleID string) {
	delete(s.ProbeSchedules, scheduleID)
	s.UpdatedAt = time.Now()
}

// AddOperation adds an operation to state
func (s *DiskManagerState) AddOperation(op *OperationState) {
	s.Operations[op.ID] = op
	s.UpdatedAt = time.Now()
}

// CompleteOperation marks an operation as completed
func (s *DiskManagerState) CompleteOperation(opID string, success bool, message string) {
	if op, exists := s.Operations[opID]; exists {
		now := time.Now()
		op.CompletedAt = &now
		op.Progress = 100
		if success {
			op.Status = "completed"
		} else {
			op.Status = "failed"
			op.Error = message
		}
		op.Message = message
	}
	s.UpdatedAt = time.Now()
}

// UpdateStatistics updates global statistics
func (s *DiskManagerState) UpdateStatistics() {
	stats := s.Statistics
	stats.UpdatedAt = time.Now()
	stats.Uptime = time.Since(stats.ManagerStartedAt)

	// Count devices by state
	stats.AvailableDevices = 0
	stats.InUseDevices = 0
	stats.FaultedDevices = 0
	stats.OfflineDevices = 0
	stats.CurrentDeviceCount = len(s.Devices)

	for _, ds := range s.Devices {
		switch ds.State {
		case DiskStateAvailable:
			stats.AvailableDevices++
		case DiskStateInUse:
			stats.InUseDevices++
		case DiskStateFaulted:
			stats.FaultedDevices++
		case DiskStateOffline:
			stats.OfflineDevices++
		}
	}

	// Count devices by health
	stats.HealthyDevices = 0
	stats.WarningDevices = 0
	stats.CriticalDevices = 0
	stats.FailedDevices = 0

	for _, ds := range s.Devices {
		switch ds.Health {
		case HealthHealthy:
			stats.HealthyDevices++
		case HealthWarning:
			stats.WarningDevices++
		case HealthCritical:
			stats.CriticalDevices++
		case HealthFailed:
			stats.FailedDevices++
		}
	}

	// Count probes
	stats.TotalProbes = len(s.ProbeExecutions)
	stats.SuccessfulProbes = 0
	stats.FailedProbes = 0
	stats.ConflictedProbes = 0
	stats.TotalQuickProbes = 0
	stats.TotalExtensiveProbes = 0

	for _, exec := range s.ProbeExecutions {
		if exec.Type == ProbeTypeQuick {
			stats.TotalQuickProbes++
		} else {
			stats.TotalExtensiveProbes++
		}

		switch exec.Status {
		case ProbeStatusCompleted:
			stats.SuccessfulProbes++
		case ProbeStatusFailed:
			stats.FailedProbes++
		case ProbeStatusConflicted:
			stats.ConflictedProbes++
		}

		if exec.CompletedAt != nil && exec.CompletedAt.After(stats.LastProbeAt) {
			stats.LastProbeAt = *exec.CompletedAt
		}
	}
}

// CleanupOldExecutions removes old probe executions based on retention period
func (s *DiskManagerState) CleanupOldExecutions(retentionPeriod time.Duration) int {
	cutoff := time.Now().Add(-retentionPeriod)
	removed := 0

	// Cleanup executions
	for id, exec := range s.ProbeExecutions {
		if exec.CompletedAt != nil && exec.CompletedAt.Before(cutoff) {
			delete(s.ProbeExecutions, id)
			removed++
		}
	}

	// Cleanup history
	for deviceID, history := range s.ProbeHistory {
		newExecutions := []*ProbeExecution{}
		for _, exec := range history.Executions {
			if exec.CompletedAt == nil || exec.CompletedAt.After(cutoff) {
				newExecutions = append(newExecutions, exec)
			}
		}
		if len(newExecutions) != len(history.Executions) {
			s.ProbeHistory[deviceID].Executions = newExecutions
			s.ProbeHistory[deviceID].UpdatedAt = time.Now()
		}
	}

	if removed > 0 {
		s.UpdatedAt = time.Now()
	}

	return removed
}

// GetDeviceState returns device state by ID
func (s *DiskManagerState) GetDeviceState(deviceID string) (*DeviceState, bool) {
	ds, exists := s.Devices[deviceID]
	return ds, exists
}

// GetProbeExecution returns probe execution by ID
func (s *DiskManagerState) GetProbeExecution(execID string) (*ProbeExecution, bool) {
	exec, exists := s.ProbeExecutions[execID]
	return exec, exists
}

// GetOperation returns operation by ID
func (s *DiskManagerState) GetOperation(opID string) (*OperationState, bool) {
	op, exists := s.Operations[opID]
	return op, exists
}

// ListRunningOperations returns all running operations
func (s *DiskManagerState) ListRunningOperations() []*OperationState {
	running := []*OperationState{}
	for _, op := range s.Operations {
		if op.Status == "running" {
			running = append(running, op)
		}
	}
	return running
}

// ListActiveProbes returns all active probe executions
func (s *DiskManagerState) ListActiveProbes() []*ProbeExecution {
	active := []*ProbeExecution{}
	for _, exec := range s.ProbeExecutions {
		if exec.Status == ProbeStatusRunning || exec.Status == ProbeStatusScheduled {
			active = append(active, exec)
		}
	}
	return active
}
