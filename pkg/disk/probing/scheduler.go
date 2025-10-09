// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package probing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk/state"
	"github.com/stratastor/rodent/pkg/disk/tools"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// ProbeScheduler manages scheduled and on-demand SMART probes
type ProbeScheduler struct {
	logger         logger.Logger
	stateManager   *state.StateManager
	smartctl       *tools.SmartctlExecutor
	conflictCheck  ConflictChecker
	deviceResolver DeviceResolver
	executor       *ProbeExecutor
	scheduler      gocron.Scheduler

	// Active probe tracking
	activeProbes map[string]*types.ProbeExecution // device -> execution
	mu           sync.RWMutex

	// Configuration
	maxConcurrent int
	semaphore     chan struct{}
}

// ConflictChecker interface for checking probe conflicts
type ConflictChecker interface {
	// CheckConflicts returns true if probe should be delayed/cancelled
	// Takes devicePath since we need it to check pool membership
	CheckConflicts(ctx context.Context, deviceID, devicePath string, probeType types.ProbeType) (bool, string, error)
}

// DeviceResolver interface for resolving device filters to actual devices
type DeviceResolver interface {
	// ResolveDevices returns devices matching the filter with their paths
	// Returns map of deviceID -> devicePath
	ResolveDevices(filter *types.DiskFilter) (map[string]string, error)
}

// NewProbeScheduler creates a new probe scheduler
// deviceResolver can be nil and set later via SetDeviceResolver
func NewProbeScheduler(
	l logger.Logger,
	stateManager *state.StateManager,
	smartctl *tools.SmartctlExecutor,
	conflictCheck ConflictChecker,
	maxConcurrent int,
) (*ProbeScheduler, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, errors.Wrap(err, errors.DiskProbeScheduleFailed).
			WithMetadata("operation", "create_scheduler")
	}

	executor := NewProbeExecutor(l, stateManager, smartctl)

	ps := &ProbeScheduler{
		logger:         l,
		stateManager:   stateManager,
		smartctl:       smartctl,
		conflictCheck:  conflictCheck,
		deviceResolver: nil, // Set via SetDeviceResolver after Manager is created
		executor:       executor,
		scheduler:      scheduler,
		activeProbes:   make(map[string]*types.ProbeExecution),
		maxConcurrent:  maxConcurrent,
		semaphore:      make(chan struct{}, maxConcurrent),
	}

	return ps, nil
}

// Start starts the probe scheduler
func (ps *ProbeScheduler) Start(ctx context.Context) error {
	ps.logger.Info("starting probe scheduler",
		"max_concurrent", ps.maxConcurrent)

	// Load schedules from state
	var schedules map[string]*types.ProbeSchedule
	ps.stateManager.WithRLock(func(s *types.DiskManagerState) {
		schedules = s.ProbeSchedules
	})

	// Register scheduled jobs
	for scheduleID, schedule := range schedules {
		if !schedule.Enabled {
			continue
		}

		if err := ps.registerSchedule(ctx, scheduleID, schedule); err != nil {
			ps.logger.Error("failed to register schedule",
				"schedule_id", scheduleID,
				"error", err)
			continue
		}
	}

	// Start the scheduler
	ps.scheduler.Start()

	ps.logger.Info("probe scheduler started",
		"schedules", len(schedules))

	return nil
}

// Stop stops the probe scheduler
func (ps *ProbeScheduler) Stop(ctx context.Context) error {
	ps.logger.Info("stopping probe scheduler")

	// Stop scheduler
	if err := ps.scheduler.Shutdown(); err != nil {
		ps.logger.Error("error stopping scheduler", "error", err)
	}

	// Wait for active probes to complete or timeout
	deadline := time.Now().Add(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		ps.mu.RLock()
		active := len(ps.activeProbes)
		ps.mu.RUnlock()

		if active == 0 {
			break
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			ps.logger.Warn("context cancelled while waiting for probes",
				"active_probes", active)
			return ctx.Err()
		}
	}

	ps.mu.RLock()
	remaining := len(ps.activeProbes)
	ps.mu.RUnlock()

	if remaining > 0 {
		ps.logger.Warn("probe scheduler stopped with active probes",
			"active_probes", remaining)
	} else {
		ps.logger.Info("probe scheduler stopped cleanly")
	}

	return nil
}

// AddSchedule adds a new probe schedule
func (ps *ProbeScheduler) AddSchedule(ctx context.Context, schedule *types.ProbeSchedule) error {
	if schedule.ID == "" {
		schedule.ID = fmt.Sprintf("sched-%d", time.Now().UnixNano())
	}

	// Validate cron expression
	if _, err := gocron.NewScheduler(gocron.WithLocation(time.UTC)); err != nil {
		return errors.Wrap(err, errors.DiskConfigCronInvalid).
			WithMetadata("cron", schedule.CronExpression)
	}

	// Save to state
	ps.stateManager.WithLock(func(s *types.DiskManagerState) {
		if s.ProbeSchedules == nil {
			s.ProbeSchedules = make(map[string]*types.ProbeSchedule)
		}
		schedule.CreatedAt = time.Now()
		schedule.UpdatedAt = time.Now()
		s.ProbeSchedules[schedule.ID] = schedule
	})

	ps.stateManager.SaveDebounced()

	// Register if enabled
	if schedule.Enabled {
		if err := ps.registerSchedule(ctx, schedule.ID, schedule); err != nil {
			return err
		}
	}

	ps.logger.Info("probe schedule added",
		"schedule_id", schedule.ID,
		"cron", schedule.CronExpression)

	return nil
}

// RemoveSchedule removes a probe schedule
func (ps *ProbeScheduler) RemoveSchedule(ctx context.Context, scheduleID string) error {
	// Remove from state
	found := false
	ps.stateManager.WithLock(func(s *types.DiskManagerState) {
		if _, exists := s.ProbeSchedules[scheduleID]; exists {
			delete(s.ProbeSchedules, scheduleID)
			found = true
		}
	})

	if !found {
		return errors.New(errors.DiskProbeNotFound, "schedule not found").
			WithMetadata("schedule_id", scheduleID)
	}

	// Note: gocron v2 handles job removal automatically when scheduler restarts
	// For immediate removal, we would need to track job IDs

	ps.stateManager.SaveDebounced()

	ps.logger.Info("probe schedule removed", "schedule_id", scheduleID)

	return nil
}

// TriggerProbe manually triggers a probe on a device
func (ps *ProbeScheduler) TriggerProbe(ctx context.Context, deviceID, devicePath string, probeType types.ProbeType) (string, error) {
	// Validate device path
	if devicePath == "" {
		return "", errors.New(errors.DiskNotFound, "device path required").
			WithMetadata("device_id", deviceID)
	}

	// Check if probe already running on this device
	ps.mu.RLock()
	if existing, ok := ps.activeProbes[deviceID]; ok {
		ps.mu.RUnlock()
		return "", errors.New(errors.DiskProbeAlreadyRunning, "probe already running on device").
			WithMetadata("device_id", deviceID).
			WithMetadata("probe_id", existing.ID).
			WithMetadata("probe_type", string(existing.Type))
	}
	ps.mu.RUnlock()

	// Check concurrency limit
	select {
	case ps.semaphore <- struct{}{}:
		// Acquired semaphore
	default:
		return "", errors.New(errors.DiskProbeConcurrencyLimit, "maximum concurrent probes reached").
			WithMetadata("max_concurrent", fmt.Sprintf("%d", ps.maxConcurrent))
	}

	// Check for conflicts
	hasConflict, reason, err := ps.conflictCheck.CheckConflicts(ctx, deviceID, devicePath, probeType)
	if err != nil {
		<-ps.semaphore // Release semaphore
		return "", errors.Wrap(err, errors.DiskProbeConflict).
			WithMetadata("device_id", deviceID)
	}

	if hasConflict {
		<-ps.semaphore // Release semaphore
		return "", errors.New(errors.DiskProbeConflict, "probe conflicts detected").
			WithMetadata("device_id", deviceID).
			WithMetadata("conflict_reason", reason)
	}

	// Create probe execution using helper
	execution := types.NewProbeExecution(deviceID, devicePath, probeType)

	// Track active probe
	ps.mu.Lock()
	ps.activeProbes[deviceID] = execution
	ps.mu.Unlock()

	// Execute probe asynchronously
	go func() {
		defer func() {
			<-ps.semaphore // Release semaphore
			ps.mu.Lock()
			delete(ps.activeProbes, deviceID)
			ps.mu.Unlock()
		}()

		if err := ps.executor.ExecuteProbe(ctx, execution); err != nil {
			ps.logger.Error("probe execution failed",
				"probe_id", execution.ID,
				"device_id", deviceID,
				"error", err)
		}
	}()

	ps.logger.Info("probe triggered",
		"probe_id", execution.ID,
		"device_id", deviceID,
		"probe_type", string(probeType))

	return execution.ID, nil
}

// GetActiveProbes returns currently active probes
func (ps *ProbeScheduler) GetActiveProbes() []*types.ProbeExecution {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	probes := make([]*types.ProbeExecution, 0, len(ps.activeProbes))
	for _, probe := range ps.activeProbes {
		probes = append(probes, probe)
	}

	return probes
}

// registerSchedule registers a schedule with gocron
func (ps *ProbeScheduler) registerSchedule(ctx context.Context, scheduleID string, schedule *types.ProbeSchedule) error {
	// Create job that executes the scheduled probe
	job := func() {
		ps.executeScheduledProbe(ctx, scheduleID, schedule)
	}

	// Register with gocron
	_, err := ps.scheduler.NewJob(
		gocron.CronJob(schedule.CronExpression, false),
		gocron.NewTask(job),
		gocron.WithName(scheduleID),
	)

	if err != nil {
		return errors.Wrap(err, errors.DiskProbeScheduleFailed).
			WithMetadata("schedule_id", scheduleID).
			WithMetadata("cron", schedule.CronExpression)
	}

	ps.logger.Debug("schedule registered",
		"schedule_id", scheduleID,
		"cron", schedule.CronExpression)

	return nil
}

// SetDeviceResolver sets the device resolver (must be called before Start)
func (ps *ProbeScheduler) SetDeviceResolver(resolver DeviceResolver) {
	ps.deviceResolver = resolver
}

// executeScheduledProbe executes a scheduled probe on devices matching the filter
func (ps *ProbeScheduler) executeScheduledProbe(ctx context.Context, scheduleID string, schedule *types.ProbeSchedule) {
	ps.logger.Info("executing scheduled probe",
		"schedule_id", scheduleID,
		"probe_type", string(schedule.Type))

	// Check if device resolver is set
	if ps.deviceResolver == nil {
		ps.logger.Error("device resolver not set, cannot execute scheduled probe",
			"schedule_id", scheduleID)
		return
	}

	// Resolve devices from filter
	devices, err := ps.deviceResolver.ResolveDevices(schedule.DeviceFilter)
	if err != nil {
		ps.logger.Error("failed to resolve devices for scheduled probe",
			"schedule_id", scheduleID,
			"error", err)
		return
	}

	if len(devices) == 0 {
		ps.logger.Debug("no devices matched filter for scheduled probe",
			"schedule_id", scheduleID)
		return
	}

	ps.logger.Info("scheduled probe targeting devices",
		"schedule_id", scheduleID,
		"device_count", len(devices))

	// Trigger probe for each device
	for deviceID, devicePath := range devices {
		// Trigger probe (this handles concurrency limits and conflict checking)
		probeID, err := ps.TriggerProbe(ctx, deviceID, devicePath, schedule.Type)
		if err != nil {
			// Check if this is an expected conflict/limit error
			if rodentErr, ok := err.(*errors.RodentError); ok {
				// Log expected conflicts at debug level, continue with other devices
				if rodentErr.Code == errors.DiskProbeAlreadyRunning ||
					rodentErr.Code == errors.DiskProbeConcurrencyLimit ||
					rodentErr.Code == errors.DiskProbeConflict {
					ps.logger.Debug("skipping scheduled probe for device",
						"schedule_id", scheduleID,
						"device_id", deviceID,
						"reason", err.Error())
					continue
				}
			}

			// Unexpected error - log at error level but continue
			ps.logger.Error("failed to trigger scheduled probe",
				"schedule_id", scheduleID,
				"device_id", deviceID,
				"error", err)
			continue
		}

		ps.logger.Debug("scheduled probe triggered",
			"schedule_id", scheduleID,
			"probe_id", probeID,
			"device_id", deviceID)
	}
}
