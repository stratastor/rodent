// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/stratastor/rodent/internal/constants"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"gopkg.in/yaml.v3"
)

const (
	configFileName        = "zfs.snapshots.rodent.yml"
	schedulerJobNameFmt   = "snapshot-policy-%s-schedule-%d"
	backupFileSuffixFmt   = ".backup.%s"
	errorFileSuffixFmt    = ".error.%s"
	defaultErrorBackupFmt = "2006-01-02-150405"
)

// Manager implements the SchedulerInterface for managing ZFS auto-snapshots
type Manager struct {
	configPath      string
	config          SnapshotConfig
	dsManager       *dataset.Manager
	scheduler       gocron.Scheduler
	jobMapping      map[string][]string // Maps policyID to list of job IDs
	lastErrorBackup time.Time
	mu              sync.RWMutex
}

// NewManager creates a new snapshot manager
func NewManager(dsManager *dataset.Manager) (*Manager, error) {
	// Ensure the config directory exists
	configDir := constants.SystemConfigDir
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, errors.Wrap(err, errors.FSError).WithMetadata("path", configDir)
	}

	configPath := filepath.Join(configDir, configFileName)

	// Create the scheduler with default options
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, errors.Wrap(err, errors.SchedulerError)
	}

	manager := &Manager{
		configPath: configPath,
		dsManager:  dsManager,
		scheduler:  scheduler,
		jobMapping: make(map[string][]string),
		config: SnapshotConfig{
			Policies: make([]SnapshotPolicy, 0),
			Monitors: make(map[string]JobMonitor),
		},
	}

	return manager, nil
}

// createJob creates a gocron job for the given policy and schedule
func (m *Manager) createJob(policy SnapshotPolicy, scheduleIndex int) (string, error) {
	if scheduleIndex >= len(policy.Schedules) {
		return "", errors.New(errors.ZFSRequestValidationError, "schedule index out of range")
	}

	schedule := policy.Schedules[scheduleIndex]
	if !schedule.Enabled {
		return "", nil // Skip disabled schedules
	}

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
		_ = m.SaveConfig()

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

	// Add singleton mode option based on schedule type
	// if schedule.Type == ScheduleTypeOneTime {
	// 	jobOpts = append(jobOpts, gocron.WithSingletonMode(gocron.LimitModeWait))
	// } else {
	// 	jobOpts = append(jobOpts, gocron.WithSingletonMode(gocron.LimitModeReschedule))
	// }

	// Create the event listener for job events
	beforeRunListener := gocron.BeforeJobRuns(func(jobID uuid.UUID, jobName string) {
		m.mu.Lock()
		defer m.mu.Unlock()

		// Update the monitor's status before the job runs
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
		return "", errors.Wrap(err, errors.SchedulerError)
	}

	// Return the job ID for mapping
	return job.ID().String(), nil
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
		return CreateSnapshotResult{
			ScheduleIndex: scheduleIndex,
		}, errors.New(errors.NotFoundError, "policy not found")
	}
	
	// Validate scheduleIndex is within range
	if scheduleIndex >= len(policy.Schedules) {
		return CreateSnapshotResult{
			PolicyID:      policyID,
			ScheduleIndex: scheduleIndex,
		}, errors.New(errors.ZFSRequestValidationError, "schedule index out of range")
	}

	// Generate snapshot name based on pattern
	snapName := expandSnapNamePattern(policy.SnapNamePattern, time.Now())

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
	err := m.dsManager.CreateSnapshot(ctx, snapshotCfg)
	if err != nil {
		return CreateSnapshotResult{
			PolicyID:      policyID,
			ScheduleIndex: scheduleIndex,
			DatasetName:   policy.Dataset,
			Error:         err,
		}, err
	}

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
		prunedSnapshots, err = m.pruneSnapshots(policy)
		if err != nil {
			// Log the error but don't fail the snapshot creation
			// For now, just update the error string
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
		}
	}

	// Save config after successful snapshot
	if err := m.SaveConfig(); err != nil {
		// Log but don't fail
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

// pruneSnapshots prunes old snapshots based on the retention policy
func (m *Manager) pruneSnapshots(policy SnapshotPolicy) ([]string, error) {
	prunedSnapshots := []string{}

	// Get all snapshots for this dataset
	ctx := context.Background()
	listCfg := dataset.ListConfig{
		Name:      policy.Dataset,
		Type:      "snapshot",
		Recursive: policy.Recursive,
	}

	result, err := m.dsManager.List(ctx, listCfg)
	if err != nil {
		return prunedSnapshots, errors.Wrap(err, errors.ZFSDatasetList)
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

		// Skip snapshots in the keep list
		snapName := strings.Split(name, "@")[1]
		skipSnapshot := false
		for _, keepName := range policy.RetentionPolicy.KeepNamedSnap {
			if snapName == keepName {
				skipSnapshot = true
				break
			}
		}
		if skipSnapshot {
			continue
		}

		// Get creation time from dataset properties
		creationTime := time.Time{}
		if createProp, ok := ds.Properties["creation"]; ok {
			if createVal, ok := createProp.Value.(float64); ok {
				creationTime = time.Unix(int64(createVal), 0)
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

	// Apply retention policy
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
func expandSnapNamePattern(pattern string, t time.Time) string {
	// Simple implementation for common patterns
	result := pattern
	result = strings.ReplaceAll(result, "%Y", fmt.Sprintf("%04d", t.Year()))
	result = strings.ReplaceAll(result, "%m", fmt.Sprintf("%02d", t.Month()))
	result = strings.ReplaceAll(result, "%d", fmt.Sprintf("%02d", t.Day()))
	result = strings.ReplaceAll(result, "%H", fmt.Sprintf("%02d", t.Hour()))
	result = strings.ReplaceAll(result, "%M", fmt.Sprintf("%02d", t.Minute()))
	result = strings.ReplaceAll(result, "%S", fmt.Sprintf("%02d", t.Second()))

	return result
}

// AddPolicy adds a new policy to the manager
func (m *Manager) AddPolicy(params EditPolicyParams) (string, error) {
	// Create a new policy
	policy := NewSnapshotPolicy(params)

	// Validate the policy
	if err := ValidatePolicy(policy); err != nil {
		return "", err
	}

	// Check if policy with the same ID already exists
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.config.Policies {
		if p.ID == policy.ID {
			return "", errors.New(
				errors.ZFSRequestValidationError,
				"policy with the same ID already exists",
			)
		}
	}

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

	// Save the updated config
	if err := m.SaveConfig(); err != nil {
		return policy.ID, errors.Wrap(err, errors.ConfigWriteError)
	}

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

	// Save the updated config
	if err := m.SaveConfig(); err != nil {
		return errors.Wrap(err, errors.ConfigWriteError)
	}

	return nil
}

// RemovePolicy removes a policy
func (m *Manager) RemovePolicy(policyID string) error {
	// Find the policy
	m.mu.Lock()
	defer m.mu.Unlock()

	policyIndex := -1
	for i, p := range m.config.Policies {
		if p.ID == policyID {
			policyIndex = i
			break
		}
	}

	if policyIndex == -1 {
		return errors.New(errors.NotFoundError, "policy not found")
	}

	// Remove jobs for this policy
	if jobIDs, ok := m.jobMapping[policyID]; ok {
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
		delete(m.jobMapping, policyID)
	}

	// Remove the policy from the config
	m.config.Policies = append(
		m.config.Policies[:policyIndex],
		m.config.Policies[policyIndex+1:]...)

	// Remove monitors for this policy
	delete(m.config.Monitors, policyID)

	// Save the updated config
	if err := m.SaveConfig(); err != nil {
		return errors.Wrap(err, errors.ConfigWriteError)
	}

	return nil
}

// GetPolicy gets a policy by ID
func (m *Manager) GetPolicy(policyID string) (SnapshotPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.config.Policies {
		if p.ID == policyID {
			return p, nil
		}
	}

	return SnapshotPolicy{}, errors.New(errors.NotFoundError, "policy not found")
}

// ListPolicies lists all policies
func (m *Manager) ListPolicies() ([]SnapshotPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]SnapshotPolicy, len(m.config.Policies))
	copy(policies, m.config.Policies)

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

// Start starts the scheduler
func (m *Manager) Start() error {
	// Load config first
	if err := m.LoadConfig(); err != nil {
		return err
	}

	// Create jobs for all enabled policies
	m.mu.Lock()
	for _, policy := range m.config.Policies {
		if policy.Enabled {
			m.jobMapping[policy.ID] = []string{}
			for j, schedule := range policy.Schedules {
				if schedule.Enabled {
					jobID, err := m.createJob(policy, j)
					if err != nil {
						m.mu.Unlock()
						return errors.Wrap(err, errors.SchedulerError).
							WithMetadata("policy_id", policy.ID).
							WithMetadata("schedule_index", fmt.Sprintf("%d", j))
					}
					if jobID != "" {
						m.jobMapping[policy.ID] = append(m.jobMapping[policy.ID], jobID)
					}
				}
			}
		}
	}
	m.mu.Unlock()

	// Start the scheduler
	m.scheduler.Start()

	return nil
}

// Stop stops the scheduler
func (m *Manager) Stop() error {
	// Stop the scheduler
	err := m.scheduler.Shutdown()
	if err != nil {
		return errors.Wrap(err, errors.SchedulerError)
	}

	// Save the config
	if err := m.SaveConfig(); err != nil {
		return errors.Wrap(err, errors.ConfigWriteError)
	}

	return nil
}

// LoadConfig loads the config from file
func (m *Manager) LoadConfig() error {
	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// No config file, use empty config
		m.mu.Lock()
		m.config = SnapshotConfig{
			Policies: make([]SnapshotPolicy, 0),
			Monitors: make(map[string]JobMonitor),
		}
		m.mu.Unlock()

		// Create initial config file
		return m.SaveConfig()
	}

	// Read config file
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return errors.Wrap(err, errors.ConfigReadError)
	}

	// Unmarshal config
	var config SnapshotConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		// Backup the bad config file
		backupPath := m.configPath + fmt.Sprintf(
			errorFileSuffixFmt,
			time.Now().Format(defaultErrorBackupFmt),
		)
		if backupErr := os.WriteFile(backupPath, data, 0644); backupErr != nil {
			// Log but continue
		}

		// Use empty config
		m.mu.Lock()
		m.config = SnapshotConfig{
			Policies: make([]SnapshotPolicy, 0),
			Monitors: make(map[string]JobMonitor),
		}
		m.mu.Unlock()

		// Create new config file
		saveErr := m.SaveConfig()
		if saveErr != nil {
			return errors.Wrap(saveErr, errors.ConfigWriteError)
		}

		return errors.Wrap(err, errors.ConfigParseError)
	}

	// Validate all policies
	var validPolicies []SnapshotPolicy
	for _, policy := range config.Policies {
		if err := ValidatePolicy(policy); err == nil {
			validPolicies = append(validPolicies, policy)
		} else {
			// Log invalid policy but continue
		}
	}

	// If some policies were invalid, create a backup
	if len(validPolicies) < len(config.Policies) {
		backupPath := m.configPath + fmt.Sprintf(
			errorFileSuffixFmt,
			time.Now().Format(defaultErrorBackupFmt),
		)
		if backupErr := os.WriteFile(backupPath, data, 0644); backupErr != nil {
			// Log but continue
		}

		// Update config with only valid policies
		config.Policies = validPolicies
	}

	// Set the config
	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	// If any policies were filtered out, save the valid config
	if len(validPolicies) < len(config.Policies) {
		if saveErr := m.SaveConfig(); saveErr != nil {
			return errors.Wrap(saveErr, errors.ConfigWriteError)
		}
	}

	return nil
}

// SaveConfig saves the config to file
func (m *Manager) SaveConfig() error {
	m.mu.RLock()

	// Marshal config to YAML
	data, err := yaml.Marshal(m.config)
	m.mu.RUnlock()

	if err != nil {
		return errors.Wrap(err, errors.ConfigWriteError)
	}

	// Create backup of current config
	if _, err := os.Stat(m.configPath); err == nil {
		currentData, readErr := os.ReadFile(m.configPath)
		if readErr == nil {
			backupPath := m.configPath + fmt.Sprintf(
				backupFileSuffixFmt,
				time.Now().Format(defaultErrorBackupFmt),
			)
			if backupErr := os.WriteFile(backupPath, currentData, 0644); backupErr != nil {
				// Log but continue
			}
		}
	}

	// Write config file
	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return errors.Wrap(err, errors.ConfigWriteError)
	}

	return nil
}
