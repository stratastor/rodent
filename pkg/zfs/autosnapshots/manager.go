// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package autosnapshots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"maps"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"gopkg.in/yaml.v3"
)

const (
	configFileName        = "zfs.snapshots.rodent.yml"
	schedulerJobNameFmt   = "snapshot-policy-%s-schedule-%d"
	errorFileSuffixFmt    = ".error.%s"
	defaultErrorBackupFmt = "2006-01-02-150405"
)

// Manager implements the SchedulerInterface for managing ZFS auto-snapshots
type Manager struct {
	logger     logger.Logger
	configPath string
	config     SnapshotConfig
	dsManager  *dataset.Manager
	scheduler  gocron.Scheduler
	jobMapping map[string][]string // Maps policyID to list of job IDs
	mu         sync.RWMutex
	started    bool // Track if the manager has been started
}

// Global instance and mutex for singleton pattern
var (
	globalManager *Manager
	initMutex     sync.Mutex
)

// GetManager returns the global manager instance, creating it if necessary
func GetManager(dsManager *dataset.Manager, cfgDir string) (*Manager, error) {
	initMutex.Lock()
	defer initMutex.Unlock()

	if globalManager == nil {
		var err error
		globalManager, err = newManager(dsManager, cfgDir)
		if err != nil {
			return nil, err
		}
	}

	return globalManager, nil
}

// NewManager creates a new snapshot manager (deprecated, use GetManager instead)
// This function now redirects to GetManager for backward compatibility
func NewManager(dsManager *dataset.Manager, cfgDir string) (*Manager, error) {
	return GetManager(dsManager, cfgDir)
}

// newManager creates a new snapshot manager (internal implementation)
func newManager(dsManager *dataset.Manager, cfgDir string) (*Manager, error) {
	// Initialize logger
	l, err := logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "snapshot")
	if err != nil {
		return nil, errors.Wrap(err, errors.LoggerError)
	}

	l.Info("Initializing snapshot manager")

	// Ensure the config directory exists
	configDir := config.GetConfigDir()
	if cfgDir != "" {
		configDir = cfgDir
	}

	l.Debug("Ensuring config directory exists", "dir", configDir)
	// Check if directory already exists before creation
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		l.Debug("Config directory does not exist, creating it", "dir", configDir)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			l.Error("Failed to create config directory", "dir", configDir, "error", err)
			return nil, errors.Wrap(err, errors.FSError).WithMetadata("path", configDir)
		}
	} else if err != nil {
		l.Error("Failed to check config directory", "dir", configDir, "error", err)
		return nil, errors.Wrap(err, errors.FSError).WithMetadata("path", configDir)
	} else {
		l.Debug("Config directory already exists", "dir", configDir)
	}

	configPath := filepath.Join(configDir, configFileName)
	l.Debug("Using config path", "path", configPath)

	// Create the scheduler with default options
	l.Debug("Creating scheduler")
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		l.Error("Failed to create scheduler", "error", err)
		return nil, errors.Wrap(err, errors.SchedulerError)
	}

	// Initialize the manager with empty config
	manager := &Manager{
		logger:     l,
		configPath: configPath,
		dsManager:  dsManager,
		scheduler:  scheduler,
		jobMapping: make(map[string][]string),
		config: SnapshotConfig{
			Policies: make([]SnapshotPolicy, 0),
			Monitors: make(map[string]JobMonitor),
		},
	}

	l.Info("Snapshot manager initialized successfully")
	return manager, nil
}

// createJob creates a gocron job for the given policy and schedule
func (m *Manager) createJob(policy SnapshotPolicy, scheduleIndex int) (string, error) {
	if scheduleIndex >= len(policy.Schedules) {
		m.logger.Error("Schedule index out of range",
			"policy_id", policy.ID,
			"policy_name", policy.Name,
			"schedule_index", scheduleIndex,
			"schedules_len", len(policy.Schedules))
		return "", errors.New(errors.ZFSRequestValidationError, "schedule index out of range")
	}

	schedule := policy.Schedules[scheduleIndex]
	if !schedule.Enabled {
		m.logger.Debug("Skipping disabled schedule",
			"policy_id", policy.ID,
			"policy_name", policy.Name,
			"schedule_index", scheduleIndex,
			"schedule_type", schedule.Type)
		return "", nil // Skip disabled schedules
	}

	m.logger.Debug("Creating job for schedule",
		"policy_id", policy.ID,
		"policy_name", policy.Name,
		"schedule_index", scheduleIndex,
		"schedule_type", schedule.Type)

	jobName := fmt.Sprintf(schedulerJobNameFmt, policy.ID, scheduleIndex)
	jobOpts := []gocron.JobOption{
		gocron.WithName(jobName),
		gocron.WithTags(policy.Name, policy.Dataset, fmt.Sprintf("schedule-%d", scheduleIndex)),
	}

	// Add job-specific options
	if schedule.LimitedRuns > 0 {
		jobOpts = append(jobOpts, gocron.WithLimitedRuns(uint(schedule.LimitedRuns)))
	}

	if !schedule.StartTime.IsZero() {
		jobOpts = append(jobOpts, gocron.WithStartAt(gocron.WithStartDateTime(schedule.StartTime)))
	}

	if !schedule.EndTime.IsZero() {
		jobOpts = append(jobOpts, gocron.WithStopAt(gocron.WithStopDateTime(schedule.EndTime)))
	}

	// Create a task function that will run the snapshot
	taskFn := func(ctx context.Context) (any, error) {
		start := time.Now()
		result, err := m.createSnapshot(policy.ID, scheduleIndex)
		duration := time.Since(start)

		// Update the monitor
		m.mu.Lock()
		monitor, exists := m.config.Monitors[policy.ID]
		if !exists {
			monitor = JobMonitor{
				PolicyID:   policy.ID,
				ScheduleID: scheduleIndex,
			}
		}

		monitor.LastRunAt = time.Now()
		monitor.LastDuration = duration
		monitor.RunCount = monitor.RunCount + 1

		if err != nil {
			monitor.Status = "error"
			monitor.LastError = err.Error()
		} else {
			monitor.Status = "success"
			monitor.LastError = ""
		}

		m.config.Monitors[policy.ID] = monitor

		// Update the policy
		for i, p := range m.config.Policies {
			if p.ID == policy.ID {
				m.config.Policies[i].LastRunAt = time.Now()
				m.config.Policies[i].LastRunStatus = monitor.Status
				m.config.Policies[i].LastRunError = monitor.LastError
				break
			}
		}
		m.mu.Unlock()

		// Save config with updated monitor status
		_ = m.SaveConfig(false)

		return result, err
	}

	// Create the job based on the schedule type
	var job gocron.Job
	var err error

	// Set up the job definition based on schedule type
	var jobDef gocron.JobDefinition

	switch schedule.Type {
	case ScheduleTypeSecondly:
		jobDef = gocron.DurationJob(time.Duration(schedule.Interval) * time.Second)

	case ScheduleTypeMinutely:
		jobDef = gocron.DurationJob(time.Duration(schedule.Interval) * time.Minute)

	case ScheduleTypeHourly:
		jobDef = gocron.DurationJob(time.Duration(schedule.Interval) * time.Hour)

	case ScheduleTypeDaily:
		hour, min, sec := parseAtTime(schedule.AtTime)
		jobDef = gocron.DailyJob(
			schedule.Interval,
			gocron.NewAtTimes(
				gocron.NewAtTime(hour, min, sec),
			),
		)

	case ScheduleTypeWeekly:
		hour, min, sec := parseAtTime(schedule.AtTime)
		jobDef = gocron.WeeklyJob(
			schedule.Interval,
			gocron.NewWeekdays(
				schedule.WeekDay,
			),
			gocron.NewAtTimes(
				gocron.NewAtTime(hour, min, sec),
			),
		)

	case ScheduleTypeMonthly:
		hour, min, sec := parseAtTime(schedule.AtTime)
		jobDef = gocron.MonthlyJob(
			schedule.Interval,
			gocron.NewDaysOfTheMonth(
				schedule.DayOfMonth,
			),
			gocron.NewAtTimes(
				gocron.NewAtTime(hour, min, sec),
			),
		)

	case ScheduleTypeYearly:
		// Using CronJob for yearly schedules since YearlyJob isn't available in gocron v2
		// Note: Interval is not supported for yearly schedules with CronJob
		// Cron expression with seconds: second minute hour day month day-of-week
		hour, min, sec := parseAtTime(schedule.AtTime)
		cronExpr := fmt.Sprintf("%d %d %d %d %d *",
			sec, min, hour, schedule.DayOfMonth, int(schedule.Month))
		jobDef = gocron.CronJob(
			cronExpr,
			true, // true indicates with seconds
		)

	case ScheduleTypeOneTime:
		jobDef = gocron.OneTimeJob(gocron.OneTimeJobStartDateTime(schedule.StartTime))

	case ScheduleTypeDuration:
		jobDef = gocron.DurationJob(schedule.Duration)

	case ScheduleTypeRandom:
		jobDef = gocron.DurationRandomJob(schedule.MinDuration, schedule.MaxDuration)

	default:
		return "", errors.New(errors.ZFSRequestValidationError, "unsupported schedule type")
	}

	// Add singleton mode option to prevent concurrent job executions
	// For any secondly schedules, use LimitModeReschedule to avoid immediate retries that might conflict
	if schedule.Type == ScheduleTypeSecondly {
		jobOpts = append(jobOpts, gocron.WithSingletonMode(gocron.LimitModeReschedule))
	} else {
		jobOpts = append(jobOpts, gocron.WithSingletonMode(gocron.LimitModeWait))
	}

	// Create the event listener for job events
	beforeRunListener := gocron.BeforeJobRuns(func(jobID uuid.UUID, jobName string) {
		m.mu.Lock()
		defer m.mu.Unlock()

		m.logger.Debug("Before job run event triggered",
			"policy_id", policy.ID,
			"policy_name", policy.Name,
			"schedule_index", scheduleIndex,
			"job_id", jobID.String(),
			"job_name", jobName)

		// Update the monitor's status before the job runs
		// Initialize Monitors map if it's nil
		if m.config.Monitors == nil {
			m.config.Monitors = make(map[string]JobMonitor)
			m.logger.Warn("Monitors map was nil during job execution, reinitializing it",
				"policy_id", policy.ID,
				"policy_name", policy.Name)
		}

		monitor, exists := m.config.Monitors[policy.ID]
		if !exists {
			monitor = JobMonitor{
				PolicyID:   policy.ID,
				ScheduleID: scheduleIndex,
			}
		}

		monitor.Status = "running"
		m.config.Monitors[policy.ID] = monitor
	})

	// Handle job completion via AfterJobRuns since WhenJobReachesExecutionLimit isn't available
	// We'll use a counter in our monitor to track limited runs
	limitRunsHandler := gocron.AfterJobRuns(func(jobID uuid.UUID, jobName string) {
		m.mu.Lock()
		defer m.mu.Unlock()

		m.logger.Debug("After job run event triggered",
			"policy_id", policy.ID,
			"policy_name", policy.Name,
			"schedule_index", scheduleIndex,
			"job_id", jobID.String(),
			"job_name", jobName)

		// Initialize Monitors map if it's nil
		if m.config.Monitors == nil {
			m.config.Monitors = make(map[string]JobMonitor)
			m.logger.Warn("Monitors map was nil during job completion, reinitializing it",
				"policy_id", policy.ID,
				"policy_name", policy.Name)
		}

		// Update the monitor with run count
		monitor, exists := m.config.Monitors[policy.ID]
		if !exists {
			monitor = JobMonitor{
				PolicyID:   policy.ID,
				ScheduleID: scheduleIndex,
			}
		}

		// Increment run count
		monitor.RunCount++

		// If this job has a limited number of runs and we've reached the limit,
		// mark it as completed
		if schedule.LimitedRuns > 0 && monitor.RunCount >= schedule.LimitedRuns {
			monitor.Status = "completed"
			m.logger.Info("Job reached limited runs limit",
				"policy_id", policy.ID,
				"policy_name", policy.Name,
				"schedule_index", scheduleIndex,
				"run_count", monitor.RunCount,
				"limited_runs", schedule.LimitedRuns)
		} else {
			monitor.Status = "success"
		}

		m.config.Monitors[policy.ID] = monitor
	})

	// Create the job with the task function and options
	job, err = m.scheduler.NewJob(
		jobDef,
		gocron.NewTask(taskFn),
		append(jobOpts,
			gocron.WithEventListeners(
				beforeRunListener,
				limitRunsHandler,
			),
		)...,
	)

	if err != nil {
		m.logger.Error("Failed to create job",
			"policy_id", policy.ID,
			"policy_name", policy.Name,
			"schedule_index", scheduleIndex,
			"schedule_type", schedule.Type,
			"error", err)
		return "", errors.Wrap(err, errors.SchedulerError)
	}

	jobID := job.ID().String()
	m.logger.Info("Created job successfully",
		"policy_id", policy.ID,
		"policy_name", policy.Name,
		"schedule_index", scheduleIndex,
		"schedule_type", schedule.Type,
		"job_id", jobID)

	// Return the job ID for mapping
	return jobID, nil
}

// parseAtTime parses a time string in the format "HH:MM" or "HH:MM:SS"
func parseAtTime(atTime string) (hour, min, sec uint) {
	parts := strings.Split(atTime, ":")
	if len(parts) >= 1 {
		fmt.Sscanf(parts[0], "%d", &hour)
	}
	if len(parts) >= 2 {
		fmt.Sscanf(parts[1], "%d", &min)
	}
	if len(parts) >= 3 {
		fmt.Sscanf(parts[2], "%d", &sec)
	}
	return
}

// createSnapshot creates a snapshot for the given policy and schedule
func (m *Manager) createSnapshot(policyID string, scheduleIndex int) (CreateSnapshotResult, error) {
	m.logger.Debug("Creating snapshot",
		"policy_id", policyID,
		"schedule_index", scheduleIndex)

	m.mu.RLock()
	var policy SnapshotPolicy
	found := false

	for _, p := range m.config.Policies {
		if p.ID == policyID {
			policy = p
			found = true
			break
		}
	}
	m.mu.RUnlock()

	if !found {
		m.logger.Error("Policy not found",
			"policy_id", policyID,
			"schedule_index", scheduleIndex)
		return CreateSnapshotResult{
			ScheduleIndex: scheduleIndex,
		}, errors.New(errors.NotFoundError, "policy not found")
	}

	// Validate scheduleIndex is within range
	if scheduleIndex >= len(policy.Schedules) {
		m.logger.Error("Schedule index out of range",
			"policy_id", policyID,
			"policy_name", policy.Name,
			"schedule_index", scheduleIndex,
			"schedules_len", len(policy.Schedules))
		return CreateSnapshotResult{
			PolicyID:      policyID,
			ScheduleIndex: scheduleIndex,
		}, errors.New(errors.ZFSRequestValidationError, "schedule index out of range")
	}

	m.logger.Debug("Creating snapshot for dataset",
		"policy_id", policyID,
		"policy_name", policy.Name,
		"dataset", policy.Dataset,
		"schedule_index", scheduleIndex)

	// Generate snapshot name based on pattern
	snapName := expandSnapNamePattern(
		policyID,
		policy.Name,
		scheduleIndex,
		policy.SnapNamePattern,
		time.Now(),
	)

	// Create snapshot config
	snapshotCfg := dataset.SnapshotConfig{
		NameConfig: dataset.NameConfig{
			Name: policy.Dataset,
		},
		SnapName:   snapName,
		Recursive:  policy.Recursive,
		Properties: policy.Properties,
	}

	// Create the snapshot
	ctx := context.Background()
	m.logger.Debug("Calling dataset manager to create snapshot",
		"policy_id", policyID,
		"dataset", policy.Dataset,
		"snap_name", snapName,
		"recursive", policy.Recursive)

	err := m.dsManager.CreateSnapshot(ctx, snapshotCfg)
	if err != nil {
		m.logger.Error("Failed to create snapshot",
			"policy_id", policyID,
			"policy_name", policy.Name,
			"dataset", policy.Dataset,
			"snap_name", snapName,
			"error", err)
		return CreateSnapshotResult{
			PolicyID:      policyID,
			ScheduleIndex: scheduleIndex,
			DatasetName:   policy.Dataset,
			Error:         err,
		}, err
	}

	m.logger.Debug("Created snapshot successfully",
		"policy_id", policyID,
		"policy_name", policy.Name,
		"dataset", policy.Dataset,
		"snap_name", snapName)

	// Update policy status
	m.mu.Lock()
	for i, p := range m.config.Policies {
		if p.ID == policyID {
			m.config.Policies[i].LastRunAt = time.Now()
			m.config.Policies[i].LastRunStatus = "success"
			m.config.Policies[i].LastRunError = ""
			break
		}
	}
	m.mu.Unlock()

	// Prune old snapshots if retention policy is set
	prunedSnapshots := []string{}
	if policy.RetentionPolicy.Count > 0 || policy.RetentionPolicy.OlderThan > 0 {
		m.logger.Debug("Pruning old snapshots based on retention policy",
			"policy_id", policyID,
			"policy_name", policy.Name,
			"dataset", policy.Dataset,
			"retention_count", policy.RetentionPolicy.Count,
			"retention_older_than", policy.RetentionPolicy.OlderThan)

		prunedSnapshots, err = m.pruneSnapshots(policy)
		if err != nil {
			// Log the error but don't fail the snapshot creation
			m.logger.Error("Snapshot pruning failed",
				"policy_id", policyID,
				"policy_name", policy.Name,
				"dataset", policy.Dataset,
				"error", err)

			// Update the error string
			m.mu.Lock()
			for i, p := range m.config.Policies {
				if p.ID == policyID {
					m.config.Policies[i].LastRunError = fmt.Sprintf(
						"Snapshot created but pruning failed: %s",
						err.Error(),
					)
					break
				}
			}
			m.mu.Unlock()
		} else if len(prunedSnapshots) > 0 {
			m.logger.Debug("Successfully pruned snapshots",
				"policy_id", policyID,
				"policy_name", policy.Name,
				"dataset", policy.Dataset,
				"pruned_count", len(prunedSnapshots))
		}
	}

	// Save config after successful snapshot
	if err := m.SaveConfig(false); err != nil {
		// Log but don't fail
		m.logger.Error("Failed to save config after snapshot creation",
			"policy_id", policyID,
			"policy_name", policy.Name,
			"dataset", policy.Dataset,
			"snap_name", snapName,
			"error", err)
	}

	return CreateSnapshotResult{
		PolicyID:        policyID,
		ScheduleIndex:   scheduleIndex,
		DatasetName:     policy.Dataset,
		SnapshotName:    snapName,
		CreatedAt:       time.Now(),
		PrunedSnapshots: prunedSnapshots,
	}, nil
}

// listPolicySnapshots lists all snapshots associated with a given policy
func (m *Manager) listPolicySnapshots(policy SnapshotPolicy) ([]struct {
	Name      string
	CreatedAt time.Time
}, error) {
	// Get all snapshots for this dataset
	ctx := context.Background()
	listCfg := dataset.ListConfig{
		Name:       policy.Dataset,
		Type:       "snapshot",
		Parsable:   true,
		Properties: []string{"creation"},
	}

	suffix := policy.ID
	// Append the last portion of the UUID to the result
	if parts := strings.Split(policy.ID, "-"); len(parts) > 0 {
		suffix = parts[len(parts)-1]
	}

	result, err := m.dsManager.List(ctx, listCfg)
	if err != nil {
		return nil, errors.Wrap(err, errors.ZFSDatasetList)
	}

	snapshots := []struct {
		Name      string
		CreatedAt time.Time
	}{}

	// Extract snapshots and creation times
	for name, ds := range result.Datasets {
		// Skip snapshots that don't belong to this dataset
		if !strings.HasPrefix(name, policy.Dataset+"@") {
			continue
		}

		// Skip snapshots that don't belong to this policy ID
		snapName := strings.Split(name, "@")[1]
		if !strings.HasSuffix(snapName, suffix) {
			continue
		}

		// Skip snapshots in the keep list
		if slices.Contains(policy.RetentionPolicy.KeepNamedSnap, snapName) {
			continue
		}

		// Get creation time from dataset properties
		creationTime := time.Time{}
		if createProp, ok := ds.Properties["creation"]; ok {
			switch v := createProp.Value.(type) {
			case float64:
				creationTime = time.Unix(int64(v), 0)
			case string:
				if epoch, err := strconv.ParseInt(v, 10, 64); err == nil {
					creationTime = time.Unix(epoch, 0)
				}
			}
		}

		snapshots = append(snapshots, struct {
			Name      string
			CreatedAt time.Time
		}{
			Name:      name,
			CreatedAt: creationTime,
		})
	}

	// Sort snapshots by creation time (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	return snapshots, nil
}

// pruneSnapshots prunes old snapshots based on the retention policy
func (m *Manager) pruneSnapshots(policy SnapshotPolicy) ([]string, error) {
	prunedSnapshots := []string{}

	// Get all snapshots for this policy
	snapshots, err := m.listPolicySnapshots(policy)
	if err != nil {
		return prunedSnapshots, err
	}

	// Apply retention policy
	ctx := context.Background()
	for i, snap := range snapshots {
		shouldDelete := false

		// Apply count-based retention
		if policy.RetentionPolicy.Count > 0 && i >= policy.RetentionPolicy.Count {
			shouldDelete = true
		}

		// Apply time-based retention
		if policy.RetentionPolicy.OlderThan > 0 {
			if time.Since(snap.CreatedAt) > policy.RetentionPolicy.OlderThan {
				shouldDelete = true
			}
		}

		if shouldDelete {
			// Delete the snapshot
			destroyCfg := dataset.DestroyConfig{
				NameConfig: dataset.NameConfig{
					Name: snap.Name,
				},
				Force: policy.RetentionPolicy.ForceDestroy,
				// TODO: Support DeferDestroy in the SnapshotPolicy
				DeferDestroy: true,
				// TODO: Support RecursiveDestroyDependents in the SnapshotPolicy
				RecursiveDestroyChildren: policy.Recursive,
			}

			_, err := m.dsManager.Destroy(ctx, destroyCfg)
			if err != nil {
				// Continue with other snapshots
				continue
			}

			prunedSnapshots = append(prunedSnapshots, snap.Name)
		}
	}

	return prunedSnapshots, nil
}

// expandSnapNamePattern expands a snapshot name pattern with current time
// Supports both strftime-style format codes (%Y, %m, etc.) and well-formed placeholders
// ({timestamp}, {date}, {time}, {policy_id}, {policy_name}, {sequence})
func expandSnapNamePattern(
	id string,
	policyName string,
	idx int,
	pattern string,
	t time.Time,
) string {
	result := pattern

	// Replace well-formed placeholders first (matching buildSnapshotPatternRegex)
	// {timestamp} - Full timestamp: YYYY-MM-DD-HHMMSS
	timestamp := fmt.Sprintf("%04d-%02d-%02d-%02d%02d%02d",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	result = strings.ReplaceAll(result, "{timestamp}", timestamp)

	// {date} - Date only: YYYY-MM-DD
	date := fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())
	result = strings.ReplaceAll(result, "{date}", date)

	// {time} - Time only: HHMMSS
	timeStr := fmt.Sprintf("%02d%02d%02d", t.Hour(), t.Minute(), t.Second())
	result = strings.ReplaceAll(result, "{time}", timeStr)

	// {policy_id} - Full policy UUID
	result = strings.ReplaceAll(result, "{policy_id}", id)

	// {policy_name} - Policy name
	result = strings.ReplaceAll(result, "{policy_name}", policyName)

	// {sequence} - Schedule index
	result = strings.ReplaceAll(result, "{sequence}", fmt.Sprintf("%d", idx))

	// Replace strftime-style format codes (for backwards compatibility)
	result = strings.ReplaceAll(result, "%Y", fmt.Sprintf("%04d", t.Year()))
	result = strings.ReplaceAll(result, "%m", fmt.Sprintf("%02d", t.Month()))
	result = strings.ReplaceAll(result, "%d", fmt.Sprintf("%02d", t.Day()))
	result = strings.ReplaceAll(result, "%H", fmt.Sprintf("%02d", t.Hour()))
	result = strings.ReplaceAll(result, "%M", fmt.Sprintf("%02d", t.Minute()))
	result = strings.ReplaceAll(result, "%S", fmt.Sprintf("%02d", t.Second()))

	// Append the schedule index and last portion of the UUID to the result
	// Format: {pattern}-{schedule_index}-{policy_id_suffix}
	if parts := strings.Split(id, "-"); len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		result = result + "-" + fmt.Sprintf("%d", idx) + "-" + lastPart
	}

	return result
}

// AddPolicy adds a new policy to the manager
func (m *Manager) AddPolicy(params EditPolicyParams) (string, error) {
	m.logger.Info("Adding new snapshot policy",
		"name", params.Name,
		"dataset", params.Dataset,
		"schedules_count", len(params.Schedules))

	// Create a new policy
	policy := NewSnapshotPolicy(params)

	// Validate the policy
	if err := ValidatePolicy(policy); err != nil {
		m.logger.Error("Policy validation failed",
			"name", params.Name,
			"dataset", params.Dataset,
			"error", err)
		return "", err
	}

	m.logger.Debug("Policy validation successful",
		"id", policy.ID,
		"name", policy.Name)

	// Check if policy with the same ID already exists
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.config.Policies {
		if p.ID == policy.ID {
			m.logger.Error("Policy with same ID already exists",
				"id", policy.ID,
				"name", policy.Name)
			return "", errors.New(
				errors.ZFSRequestValidationError,
				"policy with the same ID already exists",
			)
		}
	}

	m.logger.Debug("No duplicate policy found", "id", policy.ID)

	// Add policy to the config
	m.config.Policies = append(m.config.Policies, policy)

	// Create jobs for each schedule
	if policy.Enabled {
		m.jobMapping[policy.ID] = []string{}
		for i, schedule := range policy.Schedules {
			if schedule.Enabled {
				jobID, err := m.createJob(policy, i)
				if err != nil {
					return policy.ID, errors.Wrap(err, errors.SchedulerError).
						WithMetadata("schedule_index", fmt.Sprintf("%d", i))
				}
				if jobID != "" {
					m.jobMapping[policy.ID] = append(m.jobMapping[policy.ID], jobID)
				}
			}
		}
	}

	// Save the updated config with a timeout to avoid hangs
	m.logger.Debug("Adding policy: About to save config", "policy_id", policy.ID)

	saveDone := make(chan error, 1)
	go func() {
		saveDone <- m.SaveConfig(true) // Skip lock since we already hold it
	}()

	// Wait for save with a timeout
	select {
	case err := <-saveDone:
		if err != nil {
			m.logger.Error("Failed to save config after adding policy",
				"policy_id", policy.ID,
				"policy_name", policy.Name,
				"error", err)
			return policy.ID, errors.Wrap(err, errors.ConfigWriteError)
		}
		m.logger.Debug("Config save completed successfully", "policy_id", policy.ID)
	case <-time.After(5 * time.Second):
		m.logger.Error("Timeout while saving config after adding policy",
			"policy_id", policy.ID,
			"policy_name", policy.Name)
		return policy.ID, errors.New(errors.ConfigWriteError, "timeout while saving config")
	}

	m.logger.Info("Successfully added policy",
		"policy_id", policy.ID,
		"policy_name", policy.Name,
		"dataset", policy.Dataset,
		"enabled_schedules", len(policy.Schedules))
	return policy.ID, nil
}

// UpdatePolicy updates an existing policy
func (m *Manager) UpdatePolicy(params EditPolicyParams) error {
	if params.ID == "" {
		return errors.New(errors.ZFSRequestValidationError, "policy ID is required for updates")
	}

	// Find the policy
	m.mu.Lock()
	defer m.mu.Unlock()

	policyIndex := -1
	for i, p := range m.config.Policies {
		if p.ID == params.ID {
			policyIndex = i
			break
		}
	}

	if policyIndex == -1 {
		return errors.New(errors.NotFoundError, "policy not found")
	}

	// Create an updated policy
	updatedPolicy := NewSnapshotPolicy(params)
	updatedPolicy.CreatedAt = m.config.Policies[policyIndex].CreatedAt
	updatedPolicy.LastRunAt = m.config.Policies[policyIndex].LastRunAt
	updatedPolicy.LastRunStatus = m.config.Policies[policyIndex].LastRunStatus
	updatedPolicy.LastRunError = m.config.Policies[policyIndex].LastRunError

	// Validate the updated policy
	if err := ValidatePolicy(updatedPolicy); err != nil {
		return err
	}

	// Remove existing jobs for this policy
	if jobIDs, ok := m.jobMapping[updatedPolicy.ID]; ok {
		for _, jobID := range jobIDs {
			jobUUID, parseErr := uuid.Parse(jobID)
			if parseErr != nil {
				// Log parse error but continue
				continue
			}
			if err := m.scheduler.RemoveJob(jobUUID); err != nil {
				// Log but continue
			}
		}
		delete(m.jobMapping, updatedPolicy.ID)
	}

	// Update the policy in the config
	m.config.Policies[policyIndex] = updatedPolicy

	// Create new jobs if the policy is enabled
	if updatedPolicy.Enabled {
		m.jobMapping[updatedPolicy.ID] = []string{}
		for i, schedule := range updatedPolicy.Schedules {
			if schedule.Enabled {
				jobID, err := m.createJob(updatedPolicy, i)
				if err != nil {
					return errors.Wrap(err, errors.SchedulerError).
						WithMetadata("schedule_index", fmt.Sprintf("%d", i))
				}
				if jobID != "" {
					m.jobMapping[updatedPolicy.ID] = append(m.jobMapping[updatedPolicy.ID], jobID)
				}
			}
		}
	}

	// Save the updated config with a timeout to avoid hangs
	m.logger.Debug("UpdatePolicy: About to save config", "policy_id", updatedPolicy.ID)

	saveDone := make(chan error, 1)
	go func() {
		saveDone <- m.SaveConfig(true) // Skip lock since we already hold it
	}()

	// Wait for save with a timeout
	select {
	case err := <-saveDone:
		if err != nil {
			m.logger.Error("Failed to save config after updating policy",
				"policy_id", updatedPolicy.ID,
				"policy_name", updatedPolicy.Name,
				"error", err)
			return errors.Wrap(err, errors.ConfigWriteError)
		}
		m.logger.Debug("Config save completed successfully", "policy_id", updatedPolicy.ID)
	case <-time.After(5 * time.Second):
		m.logger.Error("Timeout while saving config after updating policy",
			"policy_id", updatedPolicy.ID,
			"policy_name", updatedPolicy.Name)
		return errors.New(errors.ConfigWriteError, "timeout while saving config")
	}

	return nil
}

// RemovePolicy removes a policy
func (m *Manager) RemovePolicy(policyID string, removeSnapshots bool) error {
	m.logger.Debug("Removing policy", "policy_id", policyID, "remove_snapshots", removeSnapshots)

	// Find the policy
	m.mu.Lock()
	defer m.mu.Unlock()

	policyIndex := -1
	var policy SnapshotPolicy
	for i, p := range m.config.Policies {
		if p.ID == policyID {
			policyIndex = i
			policy = p
			break
		}
	}

	if policyIndex == -1 {
		m.logger.Warn("Policy not found for removal", "policy_id", policyID)
		return errors.New(errors.NotFoundError, "policy not found")
	}

	// Check if policy is referenced by any transfer policies
	if len(policy.TransferPolicyIDs) > 0 {
		m.logger.Warn("Policy cannot be deleted - referenced by transfer policies",
			"policy_id", policyID,
			"transfer_policy_count", len(policy.TransferPolicyIDs))
		return errors.New(
			errors.ZFSSnapshotPolicyError,
			fmt.Sprintf(
				"cannot delete policy: referenced by %d transfer policies",
				len(policy.TransferPolicyIDs),
			),
		)
	}

	m.logger.Debug("Found policy for removal",
		"policy_id", policyID,
		"policy_name", policy.Name,
		"policy_index", policyIndex)

	// If requested, remove all snapshots associated with this policy
	var deletedSnapshots []string
	if removeSnapshots {
		// Create a modified policy with unlimited retention for deletion
		deletionPolicy := policy

		// Set retention to delete all snapshots
		// By setting Count to 0 and OlderThan to 0, no snapshots will be kept
		deletionPolicy.RetentionPolicy.Count = 0
		deletionPolicy.RetentionPolicy.OlderThan = 0
		deletionPolicy.RetentionPolicy.KeepNamedSnap = []string{}

		// Need to temporarily release the lock while pruning snapshots
		m.mu.Unlock()
		m.logger.Info("Removing all snapshots associated with policy",
			"policy_id", policyID,
			"policy_name", policy.Name,
			"dataset", policy.Dataset)

		// Since pruneSnapshots only deletes snapshots that match retention criteria,
		// we need to force it to consider all snapshots as candidates for deletion
		snapshots, err := m.listPolicySnapshots(deletionPolicy)
		if err != nil {
			m.mu.Lock() // Reacquire lock before error return
			m.logger.Error("Failed to list snapshots for policy",
				"policy_id", policyID,
				"policy_name", policy.Name,
				"error", err)
			return errors.Wrap(err, errors.ZFSDatasetList)
		}

		// Delete each snapshot individually
		ctx := context.Background()
		for _, snap := range snapshots {
			destroyCfg := dataset.DestroyConfig{
				NameConfig: dataset.NameConfig{
					Name: snap.Name,
				},
				Force:                    policy.RetentionPolicy.ForceDestroy,
				DeferDestroy:             true,
				RecursiveDestroyChildren: policy.Recursive,
			}

			result, err := m.dsManager.Destroy(ctx, destroyCfg)
			if err != nil {
				m.logger.Warn("Failed to delete snapshot",
					"snapshot", snap.Name,
					"error", err)
				continue
			}

			deletedSnapshots = append(deletedSnapshots, result.Destroyed...)
		}

		m.logger.Info("Removed snapshots for policy",
			"policy_id", policyID,
			"policy_name", policy.Name,
			"removed_count", len(deletedSnapshots))

		// Reacquire lock before continuing
		m.mu.Lock()
	}

	// Remove jobs for this policy
	if jobIDs, ok := m.jobMapping[policyID]; ok {
		m.logger.Debug("Removing jobs for policy",
			"policy_id", policyID,
			"jobs_count", len(jobIDs))

		for _, jobID := range jobIDs {
			jobUUID, parseErr := uuid.Parse(jobID)
			if parseErr != nil {
				m.logger.Warn("Failed to parse job UUID",
					"job_id", jobID,
					"error", parseErr)
				continue
			}
			if err := m.scheduler.RemoveJob(jobUUID); err != nil {
				m.logger.Warn("Failed to remove job",
					"job_id", jobID,
					"error", err)
			} else {
				m.logger.Debug("Removed job successfully", "job_id", jobID)
			}
		}
		delete(m.jobMapping, policyID)
	}

	// Remove the policy from the config
	m.config.Policies = append(
		m.config.Policies[:policyIndex],
		m.config.Policies[policyIndex+1:]...)
	m.logger.Debug("Removed policy from config", "policy_id", policyID)

	// Remove monitors for this policy
	delete(m.config.Monitors, policyID)
	m.logger.Debug("Removed monitors for policy", "policy_id", policyID)

	// Save the updated config with a timeout to avoid hangs
	m.logger.Debug("RemovePolicy: About to save config", "policy_id", policyID)

	saveDone := make(chan error, 1)
	go func() {
		saveDone <- m.SaveConfig(true) // Skip lock since we already hold it
	}()

	// Wait for save with a timeout
	select {
	case err := <-saveDone:
		if err != nil {
			m.logger.Error("Failed to save config after removing policy",
				"policy_id", policyID,
				"policy_name", policy.Name,
				"error", err)
			return errors.Wrap(err, errors.ConfigWriteError)
		}
		m.logger.Debug("Config save completed successfully after removing policy",
			"policy_id", policyID)
	case <-time.After(5 * time.Second):
		m.logger.Error("Timeout while saving config after removing policy",
			"policy_id", policyID,
			"policy_name", policy.Name)
		return errors.New(errors.ConfigWriteError, "timeout while saving config")
	}

	m.logger.Info("Successfully removed policy",
		"policy_id", policyID,
		"policy_name", policy.Name,
		"removed_snapshots", len(deletedSnapshots))
	return nil
}

// GetPolicy gets a policy by ID with status information
func (m *Manager) GetPolicy(policyID string) (SnapshotPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.config.Policies {
		if p.ID == policyID {
			// Create a copy of the policy to avoid modifying the original
			policy := p

			// Add monitor status information if it exists
			if monitor, exists := m.config.Monitors[policyID]; exists {
				// Status information is already in the policy (LastRunAt, LastRunStatus, LastRunError)
				// But we can enrich it with additional details from the monitor
				policy.MonitorStatus = &monitor
			} else {
				// If no monitor exists yet, create a default one
				policy.MonitorStatus = &JobMonitor{
					PolicyID: policyID,
					Status:   "pending",
				}
			}

			return policy, nil
		}
	}

	return SnapshotPolicy{}, errors.New(errors.NotFoundError, "policy not found")
}

// ListPolicies lists all policies with their status information
func (m *Manager) ListPolicies() ([]SnapshotPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a deep copy of the policies and add monitor information
	policies := make([]SnapshotPolicy, len(m.config.Policies))
	for i, p := range m.config.Policies {
		policies[i] = p

		// Add monitor status information if it exists
		if monitor, exists := m.config.Monitors[p.ID]; exists {
			// Status information is already in the policy (LastRunAt, LastRunStatus, LastRunError)
			// But we can enrich it with additional details from the monitor
			policies[i].MonitorStatus = &monitor
		} else {
			// If no monitor exists yet, create a default one
			policies[i].MonitorStatus = &JobMonitor{
				PolicyID: p.ID,
				Status:   "pending",
			}
		}
	}

	return policies, nil
}

// RunPolicy runs a policy immediately
func (m *Manager) RunPolicy(params RunPolicyParams) (CreateSnapshotResult, error) {
	// Find the policy
	policy, err := m.GetPolicy(params.ID)
	if err != nil {
		return CreateSnapshotResult{
			ScheduleIndex: params.ScheduleIndex,
		}, err
	}

	// Check schedule index
	if params.ScheduleIndex >= len(policy.Schedules) {
		return CreateSnapshotResult{
				PolicyID:      params.ID,
				ScheduleIndex: params.ScheduleIndex,
			}, errors.New(
				errors.ZFSRequestValidationError,
				"schedule index out of range",
			)
	}

	// Create snapshot
	result, err := m.createSnapshot(params.ID, params.ScheduleIndex)
	if err != nil {
		return result, err
	}

	return result, nil
}

// UpdateTransferPolicyAssociation atomically updates transfer policy associations.
// It removes from oldSnapshotPolicyID (if non-empty) and adds to newSnapshotPolicyID (if non-empty).
// This ensures both operations happen in a single save, preventing ghost references.
//
// Use cases:
//   - Create: oldSnapshotPolicyID="", newSnapshotPolicyID="xyz" (add only)
//   - Update: oldSnapshotPolicyID="abc", newSnapshotPolicyID="xyz" (remove + add)
//   - Delete: oldSnapshotPolicyID="abc", newSnapshotPolicyID="" (remove only)
func (m *Manager) UpdateTransferPolicyAssociation(oldSnapshotPolicyID, newSnapshotPolicyID, transferPolicyID string) error {
	// No-op if both are empty or same (and non-empty)
	if oldSnapshotPolicyID == "" && newSnapshotPolicyID == "" {
		return nil
	}
	if oldSnapshotPolicyID == newSnapshotPolicyID {
		m.logger.Debug("Transfer policy association unchanged",
			"snapshot_policy_id", oldSnapshotPolicyID,
			"transfer_policy_id", transferPolicyID)
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find old policy index (if removing)
	oldPolicyIdx := -1
	if oldSnapshotPolicyID != "" {
		for i, p := range m.config.Policies {
			if p.ID == oldSnapshotPolicyID {
				oldPolicyIdx = i
				break
			}
		}
		if oldPolicyIdx == -1 {
			// Old policy not found - this could happen if snapshot policy was deleted
			// Log warning but continue with adding to new policy
			m.logger.Warn("Old snapshot policy not found for disassociation",
				"snapshot_policy_id", oldSnapshotPolicyID,
				"transfer_policy_id", transferPolicyID)
		}
	}

	// Find new policy index (if adding)
	newPolicyIdx := -1
	if newSnapshotPolicyID != "" {
		for i, p := range m.config.Policies {
			if p.ID == newSnapshotPolicyID {
				newPolicyIdx = i
				break
			}
		}
		if newPolicyIdx == -1 {
			return errors.New(
				errors.NotFoundError,
				fmt.Sprintf("snapshot policy %s not found", newSnapshotPolicyID),
			)
		}
	}

	// Perform in-memory modifications

	// Remove from old policy
	if oldPolicyIdx != -1 {
		filtered := []string{}
		for _, id := range m.config.Policies[oldPolicyIdx].TransferPolicyIDs {
			if id != transferPolicyID {
				filtered = append(filtered, id)
			}
		}
		m.config.Policies[oldPolicyIdx].TransferPolicyIDs = filtered
	}

	// Add to new policy (if not already associated)
	if newPolicyIdx != -1 {
		if !slices.Contains(m.config.Policies[newPolicyIdx].TransferPolicyIDs, transferPolicyID) {
			m.config.Policies[newPolicyIdx].TransferPolicyIDs = append(
				m.config.Policies[newPolicyIdx].TransferPolicyIDs,
				transferPolicyID,
			)
		}
	}

	// Single atomic save
	if err := m.SaveConfig(true); err != nil {
		return err
	}

	// Log the result
	if oldSnapshotPolicyID != "" && newSnapshotPolicyID != "" {
		m.logger.Info("Transfer policy association updated",
			"old_snapshot_policy_id", oldSnapshotPolicyID,
			"new_snapshot_policy_id", newSnapshotPolicyID,
			"transfer_policy_id", transferPolicyID)
	} else if newSnapshotPolicyID != "" {
		m.logger.Info("Transfer policy associated with snapshot policy",
			"snapshot_policy_id", newSnapshotPolicyID,
			"transfer_policy_id", transferPolicyID)
	} else {
		m.logger.Info("Transfer policy disassociated from snapshot policy",
			"snapshot_policy_id", oldSnapshotPolicyID,
			"transfer_policy_id", transferPolicyID)
	}

	return nil
}

// GetTransferPolicyAssociations returns the list of transfer policy IDs associated with a snapshot policy
func (m *Manager) GetTransferPolicyAssociations(snapshotPolicyID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find the snapshot policy
	for _, p := range m.config.Policies {
		if p.ID == snapshotPolicyID {
			// Return a copy to prevent external modifications
			result := make([]string, len(p.TransferPolicyIDs))
			copy(result, p.TransferPolicyIDs)
			return result, nil
		}
	}

	return nil, errors.New(
		errors.NotFoundError,
		fmt.Sprintf("snapshot policy %s not found", snapshotPolicyID),
	)
}

// Start starts the scheduler
func (m *Manager) Start() error {
	// First check if already started (with lock)
	m.mu.Lock()
	if m.started {
		m.logger.Info("Snapshot scheduler is already started, skipping initialization")
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock() // Release lock for subsequent operations

	m.logger.Info("Starting snapshot scheduler")

	// Load config first without holding the lock
	if err := m.LoadConfig(); err != nil {
		m.logger.Error("Failed to load config", "error", err)
		return err
	}

	m.logger.Debug("Config loaded successfully")

	// Clean up any existing jobs to avoid duplicates
	m.cleanupExistingJobs()

	// Variables to track statistics
	enabledPolicyCount := 0
	enabledScheduleCount := 0
	createdJobCount := 0

	// Reacquire lock for job creation
	m.mu.Lock()

	// Create jobs for all enabled policies
	for _, policy := range m.config.Policies {
		if policy.Enabled {
			enabledPolicyCount++
			m.logger.Debug("Processing enabled policy",
				"policy_id", policy.ID,
				"policy_name", policy.Name)

			m.jobMapping[policy.ID] = []string{}
			for j, schedule := range policy.Schedules {
				if schedule.Enabled {
					enabledScheduleCount++
					m.logger.Debug("Creating job for enabled schedule",
						"policy_id", policy.ID,
						"policy_name", policy.Name,
						"schedule_index", j,
						"schedule_type", schedule.Type)

					jobID, err := m.createJob(policy, j)
					if err != nil {
						m.mu.Unlock()
						m.logger.Error("Failed to create job",
							"policy_id", policy.ID,
							"policy_name", policy.Name,
							"schedule_index", j,
							"schedule_type", schedule.Type,
							"error", err)
						return errors.Wrap(err, errors.SchedulerError).
							WithMetadata("policy_id", policy.ID).
							WithMetadata("schedule_index", fmt.Sprintf("%d", j))
					}
					if jobID != "" {
						createdJobCount++
						m.jobMapping[policy.ID] = append(m.jobMapping[policy.ID], jobID)
					}
				}
			}
		}
	}

	// Start the scheduler and mark as started
	m.scheduler.Start()
	m.started = true

	// Release the lock before finishing
	m.mu.Unlock()

	m.logger.Info("Snapshot scheduler started",
		"enabled_policies", enabledPolicyCount,
		"enabled_schedules", enabledScheduleCount,
		"created_jobs", createdJobCount)

	return nil
}

// cleanupExistingJobs removes all existing jobs from the scheduler and clears the job mapping
func (m *Manager) cleanupExistingJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get all jobs from the scheduler
	jobs := m.scheduler.Jobs()

	// Log how many jobs we're cleaning up
	if len(jobs) > 0 {
		m.logger.Info("Cleaning up existing jobs before starting", "job_count", len(jobs))
	}

	// Remove each job from the scheduler
	for _, job := range jobs {
		jobID := job.ID()
		err := m.scheduler.RemoveJob(jobID)
		if err != nil {
			m.logger.Warn("Failed to remove job during cleanup",
				"job_id", jobID.String(),
				"error", err)
		} else {
			m.logger.Debug("Removed job during cleanup", "job_id", jobID.String())
		}
	}

	// Clear the job mapping
	m.jobMapping = make(map[string][]string)
}

// Stop stops the scheduler
func (m *Manager) Stop() error {
	m.mu.Lock()

	// Check if already stopped
	if !m.started {
		m.logger.Info("Snapshot scheduler is not running")
		m.mu.Unlock()
		return nil
	}

	m.logger.Info("Stopping snapshot scheduler")
	m.mu.Unlock()

	// Stop the scheduler
	err := m.scheduler.Shutdown()
	if err != nil {
		m.logger.Error("Failed to shut down scheduler", "error", err)
		return errors.Wrap(err, errors.SchedulerError)
	}
	m.logger.Debug("Scheduler shut down successfully")

	// Save the config
	if err := m.SaveConfig(false); err != nil {
		m.logger.Error("Failed to save config during shutdown", "error", err)
		return errors.Wrap(err, errors.ConfigWriteError)
	}

	// Mark as stopped
	m.mu.Lock()
	m.started = false
	m.mu.Unlock()

	m.logger.Info("Snapshot scheduler stopped successfully")
	return nil
}

// LoadConfig loads the config from file
func (m *Manager) LoadConfig() error {
	m.logger.Debug("Loading config from file", "path", m.configPath)

	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		m.logger.Info("Config file does not exist, creating empty config",
			"path", m.configPath)

		// No config file, use empty config
		m.mu.Lock()
		m.config = SnapshotConfig{
			Policies: make([]SnapshotPolicy, 0),
			Monitors: make(map[string]JobMonitor),
		}
		m.mu.Unlock()

		// Create initial config file
		return m.SaveConfig(false)
	}

	// Read config file
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		m.logger.Error("Failed to read config file",
			"path", m.configPath,
			"error", err)
		return errors.Wrap(err, errors.ConfigReadError)
	}

	m.logger.Debug("Read config file successfully",
		"path", m.configPath,
		"size", len(data))

	// Unmarshal config
	var config SnapshotConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		m.logger.Error("Failed to unmarshal config file",
			"path", m.configPath,
			"error", err)

		// Backup the bad config file
		backupPath := m.configPath + fmt.Sprintf(
			errorFileSuffixFmt,
			time.Now().Format(defaultErrorBackupFmt),
		)
		if backupErr := os.WriteFile(backupPath, data, 0644); backupErr != nil {
			m.logger.Error("Failed to create backup of invalid config",
				"backup_path", backupPath,
				"error", backupErr)
			// Log but continue
		} else {
			m.logger.Info("Created backup of invalid config",
				"backup_path", backupPath)
		}

		// Use empty config
		m.mu.Lock()
		m.config = SnapshotConfig{
			Policies: make([]SnapshotPolicy, 0),
			Monitors: make(map[string]JobMonitor),
		}
		m.mu.Unlock()

		// Create new config file
		saveErr := m.SaveConfig(false)
		if saveErr != nil {
			return errors.Wrap(saveErr, errors.ConfigWriteError)
		}

		return errors.Wrap(err, errors.ConfigParseError)
	}

	// Validate all policies
	var validPolicies []SnapshotPolicy
	for i, policy := range config.Policies {
		if err := ValidatePolicy(policy); err == nil {
			validPolicies = append(validPolicies, policy)
		} else {
			m.logger.Warn("Invalid policy in config, skipping",
				"policy_id", policy.ID,
				"policy_name", policy.Name,
				"policy_index", i,
				"error", err)
			// Continue with other policies
		}
	}

	m.logger.Debug("Validated policies",
		"total_policies", len(config.Policies),
		"valid_policies", len(validPolicies))

	// If some policies were invalid, create a backup
	if len(validPolicies) < len(config.Policies) {
		m.logger.Info("Some policies are invalid, creating backup and using only valid policies",
			"invalid_count", len(config.Policies)-len(validPolicies))

		backupPath := m.configPath + fmt.Sprintf(
			errorFileSuffixFmt,
			time.Now().Format(defaultErrorBackupFmt),
		)
		if backupErr := os.WriteFile(backupPath, data, 0644); backupErr != nil {
			m.logger.Error("Failed to create backup of config with invalid policies",
				"backup_path", backupPath,
				"error", backupErr)
			// Log but continue
		} else {
			m.logger.Info("Created backup of config with invalid policies",
				"backup_path", backupPath)
		}

		// Update config with only valid policies
		config.Policies = validPolicies
	}

	// Set the config
	m.mu.Lock()
	m.config = config

	// Ensure Monitors map is initialized
	if m.config.Monitors == nil {
		m.logger.Warn("Monitors map was nil in loaded config, initializing it")
		m.config.Monitors = make(map[string]JobMonitor)
	}
	m.mu.Unlock()

	// If any policies were filtered out, save the valid config
	if len(validPolicies) < len(config.Policies) {
		m.logger.Info("Saving config with only valid policies")
		if saveErr := m.SaveConfig(false); saveErr != nil {
			m.logger.Error("Failed to save config with valid policies", "error", saveErr)
			return errors.Wrap(saveErr, errors.ConfigWriteError)
		}
	}

	m.logger.Info("Successfully loaded config",
		"path", m.configPath,
		"policies_count", len(m.config.Policies),
		"monitors_count", len(m.config.Monitors))
	return nil
}

// SaveConfig saves the config to file
// If skipLock is true, the method assumes the caller already holds the lock
// and will skip acquiring it to avoid deadlocks
func (m *Manager) SaveConfig(skipLock bool) error {
	m.logger.Debug("SaveConfig: Starting", "path", m.configPath)

	// Check if we should skip lock acquisition
	if skipLock {
		m.logger.Debug("SaveConfig: Skipping lock acquisition as requested")
	}

	// Take a copy of the config under lock (or use directly if lock already held)
	var configCopy SnapshotConfig

	if !skipLock {
		// Standard path - acquire lock ourselves
		m.mu.RLock()
		m.logger.Debug("SaveConfig: Acquired read lock")

		// Create a deep copy of the config
		configCopy = SnapshotConfig{
			Policies: make([]SnapshotPolicy, len(m.config.Policies)),
			Monitors: make(map[string]JobMonitor, len(m.config.Monitors)),
		}

		// Copy policies
		copy(configCopy.Policies, m.config.Policies)

		// Copy monitors
		maps.Copy(configCopy.Monitors, m.config.Monitors)

		m.mu.RUnlock()
		m.logger.Debug("SaveConfig: Released read lock")
	} else {
		// Lock is already held by caller - use the config directly
		// Create a deep copy of the config without acquiring lock
		configCopy = SnapshotConfig{
			Policies: make([]SnapshotPolicy, len(m.config.Policies)),
			Monitors: make(map[string]JobMonitor, len(m.config.Monitors)),
		}

		// Copy policies
		copy(configCopy.Policies, m.config.Policies)

		// Copy monitors
		maps.Copy(configCopy.Monitors, m.config.Monitors)
	}

	m.logger.Debug("SaveConfig: Config copy taken",
		"policies_count", len(configCopy.Policies),
		"monitors_count", len(configCopy.Monitors))

	// Marshal config to YAML
	data, err := yaml.Marshal(configCopy)
	if err != nil {
		m.logger.Error("SaveConfig: Marshal failed", "error", err)
		return errors.Wrap(err, errors.ConfigWriteError)
	}

	m.logger.Debug("SaveConfig: Marshaled config", "size", len(data))

	// Write to file directly
	m.logger.Debug("SaveConfig: Writing to file", "path", m.configPath)
	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		m.logger.Error("SaveConfig: Write failed",
			"path", m.configPath,
			"error", err)
		return errors.Wrap(err, errors.ConfigWriteError)
	}

	m.logger.Debug("SaveConfig: Successfully saved config file",
		"path", m.configPath,
		"size", len(data),
		"policies_count", len(configCopy.Policies))
	return nil
}
