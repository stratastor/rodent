// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2024 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package autotransfers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/kballard/go-shellquote"
	"gopkg.in/yaml.v3"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/autosnapshots"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
)

// Manager manages transfer policies and their scheduled execution
type Manager struct {
	logger          logger.Logger
	configPath      string
	config          TransferPolicyConfig
	snapshotManager *autosnapshots.Manager
	transferManager *dataset.TransferManager
	scheduler       gocron.Scheduler
	jobMapping      map[string][]uuid.UUID // policyID -> []jobIDs
	mu              sync.RWMutex
	started         bool
}

// Singleton instance
var (
	globalManager *Manager
	initMutex     sync.Mutex
)

// GetManager returns the singleton transfer policy manager instance
func GetManager(
	snapshotMgr *autosnapshots.Manager,
	transferMgr *dataset.TransferManager,
	logCfg logger.Config,
) (*Manager, error) {
	initMutex.Lock()
	defer initMutex.Unlock()

	if globalManager != nil {
		return globalManager, nil
	}

	l, err := logger.NewTag(logCfg, "zfs-transfer-policy")
	if err != nil {
		return nil, errors.Wrap(err, errors.LoggerError)
	}

	policiesDir := config.GetPoliciesDir()
	transferPoliciesDir := filepath.Join(policiesDir, "transfers")

	// Ensure transfers subdirectory exists
	if err := os.MkdirAll(transferPoliciesDir, 0755); err != nil {
		return nil, errors.New(
			errors.TransferPolicySchedulerError,
			fmt.Sprintf("failed to create transfer policies directory: %v", err),
		)
	}

	configPath := filepath.Join(transferPoliciesDir, "zfs.transfer-policies.rodent.yml")

	// Create scheduler
	sched, err := gocron.NewScheduler()
	if err != nil {
		return nil, errors.Wrap(err, errors.TransferPolicySchedulerError)
	}

	m := &Manager{
		logger:          l,
		configPath:      configPath,
		snapshotManager: snapshotMgr,
		transferManager: transferMgr,
		scheduler:       sched,
		jobMapping:      make(map[string][]uuid.UUID),
		config: TransferPolicyConfig{
			Policies: []TransferPolicy{},
			Monitors: make(map[string]*TransferPolicyMonitor),
		},
	}

	// Load existing policies
	if err := m.LoadConfig(); err != nil {
		l.Warn("Failed to load transfer policies config, starting with empty config", "error", err)
	}

	globalManager = m
	return m, nil
}

// Start begins the transfer policy scheduler
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return errors.New(
			errors.TransferPolicyInvalidState,
			"transfer policy manager already started",
		)
	}

	// Create jobs for all enabled policies
	for i := range m.config.Policies {
		policy := &m.config.Policies[i]
		if policy.Enabled {
			if err := m.createJobsForPolicy(policy); err != nil {
				m.logger.Error(
					"Failed to create jobs for policy",
					"policy_id",
					policy.ID,
					"error",
					err,
				)
			}
		}
	}

	// Start scheduler
	m.scheduler.Start()
	m.started = true
	m.logger.Info("Transfer policy manager started")
	return nil
}

// Stop gracefully stops the transfer policy scheduler
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return errors.New(errors.TransferPolicyInvalidState, "transfer policy manager not started")
	}

	// Stop scheduler (gracefully waits for running jobs)
	if err := m.scheduler.Shutdown(); err != nil {
		return errors.Wrap(err, errors.TransferPolicySchedulerError)
	}

	m.started = false
	m.logger.Info("Transfer policy manager stopped")
	return nil
}

// AddPolicy creates a new transfer policy
func (m *Manager) AddPolicy(ctx context.Context, params EditTransferPolicyParams) (string, error) {
	if err := ValidateEditTransferPolicyParams(&params); err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify snapshot policy exists
	if _, err := m.snapshotManager.GetPolicy(params.SnapshotPolicyID); err != nil {
		return "", errors.New(errors.TransferPolicySnapshotPolicyNotFound,
			fmt.Sprintf("snapshot policy %s not found", params.SnapshotPolicyID))
	}

	// Create new policy
	policyID := common.UUID7()
	now := time.Now()

	policy := TransferPolicy{
		ID:               policyID,
		Name:             params.Name,
		Description:      params.Description,
		SnapshotPolicyID: params.SnapshotPolicyID,
		TransferConfig:   params.TransferConfig,
		Schedules:        params.Schedules,
		RetentionPolicy:  params.RetentionPolicy,
		Enabled:          params.Enabled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := ValidateTransferPolicy(&policy); err != nil {
		return "", err
	}

	// Associate with snapshot policy FIRST, before modifying our config
	// This ensures the snapshot policy accepts the association before we commit
	if err := m.snapshotManager.UpdateTransferPolicyAssociation("", params.SnapshotPolicyID, policyID); err != nil {
		return "", errors.Wrap(err, errors.TransferPolicySchedulerError)
	}

	// Now that association succeeded, add policy and monitor to config
	// Initialize monitor
	m.config.Monitors[policyID] = &TransferPolicyMonitor{
		PolicyID: policyID,
		Status:   string(TransferPolicyStatusIdle),
	}

	// Add to config
	m.config.Policies = append(m.config.Policies, policy)

	// Create scheduler jobs if enabled and scheduler is running
	if policy.Enabled && m.started {
		if err := m.createJobsForPolicy(&policy); err != nil {
			m.logger.Error(
				"Failed to create jobs for new policy",
				"policy_id",
				policyID,
				"error",
				err,
			)
			// Don't fail policy creation, just log the error
		}
	}

	// Save config with timeout protection
	if err := m.saveConfigWithTimeout(); err != nil {
		return "", err
	}

	m.logger.Info("Transfer policy created", "policy_id", policyID, "name", policy.Name)
	return policyID, nil
}

// UpdatePolicy updates an existing transfer policy
func (m *Manager) UpdatePolicy(ctx context.Context, params EditTransferPolicyParams) error {
	if params.ID == "" {
		return errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required for update")
	}

	if err := ValidateEditTransferPolicyParams(&params); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find policy
	policyIdx := -1
	for i, p := range m.config.Policies {
		if p.ID == params.ID {
			policyIdx = i
			break
		}
	}

	if policyIdx == -1 {
		return errors.New(
			errors.TransferPolicyNotFound,
			fmt.Sprintf("policy %s not found", params.ID),
		)
	}

	// Handle snapshot policy association changes
	oldSnapshotPolicyID := m.config.Policies[policyIdx].SnapshotPolicyID
	snapshotPolicyChanged := params.SnapshotPolicyID != oldSnapshotPolicyID

	if snapshotPolicyChanged {
		// Verify new snapshot policy exists
		if _, err := m.snapshotManager.GetPolicy(params.SnapshotPolicyID); err != nil {
			return errors.New(errors.TransferPolicySnapshotPolicyNotFound,
				fmt.Sprintf("snapshot policy %s not found", params.SnapshotPolicyID))
		}

		// Atomically update association: remove from old, add to new
		if err := m.snapshotManager.UpdateTransferPolicyAssociation(oldSnapshotPolicyID, params.SnapshotPolicyID, params.ID); err != nil {
			return errors.Wrap(err, errors.TransferPolicySchedulerError)
		}
	}

	// Remove old jobs
	m.removeJobsForPolicy(params.ID)

	// Preserve critical fields if not provided in update (fallback)
	oldPolicy := m.config.Policies[policyIdx]
	if params.TransferConfig.ReceiveConfig.RemoteConfig.PrivateKey == "" {
		params.TransferConfig.ReceiveConfig.RemoteConfig.PrivateKey =
			oldPolicy.TransferConfig.ReceiveConfig.RemoteConfig.PrivateKey
	}

	// Update policy fields (preserve CreatedAt and runtime fields)
	m.config.Policies[policyIdx] = TransferPolicy{
		ID:               params.ID,
		Name:             params.Name,
		Description:      params.Description,
		SnapshotPolicyID: params.SnapshotPolicyID,
		TransferConfig:   params.TransferConfig,
		Schedules:        params.Schedules,
		RetentionPolicy:  params.RetentionPolicy,
		Enabled:          params.Enabled,
		CreatedAt:        oldPolicy.CreatedAt,
		UpdatedAt:        time.Now(),
		LastRunAt:        oldPolicy.LastRunAt,
		LastRunStatus:    oldPolicy.LastRunStatus,
		LastRunError:     oldPolicy.LastRunError,
		LastTransferID:   oldPolicy.LastTransferID,
	}

	// Validate updated policy
	if err := ValidateTransferPolicy(&m.config.Policies[policyIdx]); err != nil {
		// Rollback
		m.config.Policies[policyIdx] = oldPolicy
		return err
	}

	// Create new jobs if enabled and scheduler is running
	if m.config.Policies[policyIdx].Enabled && m.started {
		if err := m.createJobsForPolicy(&m.config.Policies[policyIdx]); err != nil {
			m.logger.Error(
				"Failed to create jobs for updated policy",
				"policy_id",
				params.ID,
				"error",
				err,
			)
		}
	}

	// Save config with timeout protection
	if err := m.saveConfigWithTimeout(); err != nil {
		return err
	}

	m.logger.Info("Transfer policy updated", "policy_id", params.ID, "name", params.Name)
	return nil
}

// RemovePolicy deletes a transfer policy and optionally its associated transfers
func (m *Manager) RemovePolicy(ctx context.Context, policyID string, deleteTransfers bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find policy
	policyIdx := -1
	for i, p := range m.config.Policies {
		if p.ID == policyID {
			policyIdx = i
			break
		}
	}

	if policyIdx == -1 {
		return errors.New(
			errors.TransferPolicyNotFound,
			fmt.Sprintf("policy %s not found", policyID),
		)
	}

	// Get snapshot policy ID before removing
	snapshotPolicyID := m.config.Policies[policyIdx].SnapshotPolicyID

	// Remove scheduler jobs
	m.removeJobsForPolicy(policyID)

	// Delete associated transfers if requested
	if deleteTransfers {
		if err := m.deletePolicyTransfers(policyID); err != nil {
			m.logger.Warn("Failed to delete policy transfers", "policy_id", policyID, "error", err)
			// Don't fail policy deletion
		}
	}

	// Remove snapshot policy association
	if err := m.snapshotManager.UpdateTransferPolicyAssociation(snapshotPolicyID, "", policyID); err != nil {
		m.logger.Warn("Failed to remove snapshot policy association", "error", err)
		// Don't fail policy deletion
	}

	// Remove from config
	m.config.Policies = append(m.config.Policies[:policyIdx], m.config.Policies[policyIdx+1:]...)
	delete(m.config.Monitors, policyID)

	// Save config with timeout protection
	if err := m.saveConfigWithTimeout(); err != nil {
		return err
	}

	m.logger.Info(
		"Transfer policy removed",
		"policy_id",
		policyID,
		"deleted_transfers",
		deleteTransfers,
	)
	return nil
}

// GetPolicy returns a transfer policy by ID with enriched monitor status
func (m *Manager) GetPolicy(policyID string) (*TransferPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, policy := range m.config.Policies {
		if policy.ID == policyID {
			// Create copy and enrich with monitor status
			policyCopy := policy
			if monitor, exists := m.config.Monitors[policyID]; exists {
				monitorCopy := *monitor
				policyCopy.MonitorStatus = &monitorCopy
			}
			return &policyCopy, nil
		}
	}

	return nil, errors.New(
		errors.TransferPolicyNotFound,
		fmt.Sprintf("policy %s not found", policyID),
	)
}

// ListPolicies returns all transfer policies with enriched monitor status
func (m *Manager) ListPolicies() ([]TransferPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]TransferPolicy, len(m.config.Policies))
	for i, policy := range m.config.Policies {
		policies[i] = policy
		if monitor, exists := m.config.Monitors[policy.ID]; exists {
			monitorCopy := *monitor
			policies[i].MonitorStatus = &monitorCopy
		}
	}

	return policies, nil
}

// EnablePolicy enables a transfer policy and starts its scheduler jobs
func (m *Manager) EnablePolicy(ctx context.Context, policyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find policy
	policyIdx := -1
	for i, p := range m.config.Policies {
		if p.ID == policyID {
			policyIdx = i
			break
		}
	}

	if policyIdx == -1 {
		return errors.New(
			errors.TransferPolicyNotFound,
			fmt.Sprintf("policy %s not found", policyID),
		)
	}

	// Check if already enabled
	if m.config.Policies[policyIdx].Enabled {
		return nil // Already enabled, nothing to do
	}

	// Enable the policy
	m.config.Policies[policyIdx].Enabled = true
	m.config.Policies[policyIdx].UpdatedAt = time.Now()

	// Create scheduler jobs if scheduler is running
	if m.started {
		if err := m.createJobsForPolicy(&m.config.Policies[policyIdx]); err != nil {
			m.logger.Error(
				"Failed to create jobs for enabled policy",
				"policy_id",
				policyID,
				"error",
				err,
			)
			// Don't fail enable operation
		}
	}

	// Save config with timeout protection
	if err := m.saveConfigWithTimeout(); err != nil {
		return err
	}

	m.logger.Info("Transfer policy enabled", "policy_id", policyID)
	return nil
}

// DisablePolicy disables a transfer policy and stops its scheduler jobs
func (m *Manager) DisablePolicy(ctx context.Context, policyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find policy
	policyIdx := -1
	for i, p := range m.config.Policies {
		if p.ID == policyID {
			policyIdx = i
			break
		}
	}

	if policyIdx == -1 {
		return errors.New(
			errors.TransferPolicyNotFound,
			fmt.Sprintf("policy %s not found", policyID),
		)
	}

	// Check if already disabled
	if !m.config.Policies[policyIdx].Enabled {
		return nil // Already disabled, nothing to do
	}

	// Disable the policy
	m.config.Policies[policyIdx].Enabled = false
	m.config.Policies[policyIdx].UpdatedAt = time.Now()

	// Remove scheduler jobs
	m.removeJobsForPolicy(policyID)

	// Save config with timeout protection
	if err := m.saveConfigWithTimeout(); err != nil {
		return err
	}

	m.logger.Info("Transfer policy disabled", "policy_id", policyID)
	return nil
}

// RunPolicy manually executes a transfer policy immediately
func (m *Manager) RunPolicy(
	ctx context.Context,
	params RunTransferPolicyParams,
) (*CreateTransferResult, error) {
	start := time.Now()

	// Find policy and initialize monitor under lock
	m.mu.Lock()
	var policy *TransferPolicy
	var policyIdx int
	for i := range m.config.Policies {
		if m.config.Policies[i].ID == params.PolicyID {
			policy = &m.config.Policies[i]
			policyIdx = i
			break
		}
	}

	if policy == nil {
		m.mu.Unlock()
		return nil, errors.New(
			errors.TransferPolicyNotFound,
			fmt.Sprintf("policy %s not found", params.PolicyID),
		)
	}

	// Get or initialize monitor
	monitor, exists := m.config.Monitors[policy.ID]
	if !exists {
		monitor = &TransferPolicyMonitor{
			PolicyID:      policy.ID,
			ScheduleIndex: 0, // Manual run uses schedule index 0
		}
		m.config.Monitors[policy.ID] = monitor
	}
	monitor.Status = string(TransferPolicyStatusRunning)
	m.mu.Unlock()

	// Execute transfer without holding the lock
	result, err := m.executeTransferForPolicy(ctx, policy, params.SnapshotOverride)

	// Update monitor and policy under lock
	m.mu.Lock()
	duration := time.Since(start)
	monitor.LastRunAt = &start
	monitor.RunCount++
	monitor.LastDuration = duration

	if err != nil {
		monitor.Status = string(TransferPolicyStatusError)
		monitor.LastError = err.Error()
		monitor.LastSkipped = false
		monitor.LastSkipReason = ""
		m.logger.Error("Manual transfer policy execution failed",
			"policy_id", policy.ID,
			"error", err)
	} else if result.Status == dataset.TransferStatusSkipped {
		monitor.Status = string(TransferPolicyStatusIdle)
		monitor.LastError = ""
		monitor.CurrentTransferID = result.TransferID
		monitor.LastSkipped = true
		monitor.LastSkipReason = fmt.Sprintf("target already has snapshot: %s", result.SourceSnapshot)
		monitor.SkipCount++
	} else {
		monitor.Status = string(TransferPolicyStatusIdle)
		monitor.LastError = ""
		monitor.CurrentTransferID = result.TransferID
		monitor.LastSkipped = false
		monitor.LastSkipReason = ""
	}

	// Update policy fields
	m.config.Policies[policyIdx].LastRunAt = monitor.LastRunAt
	if err != nil {
		m.config.Policies[policyIdx].LastRunStatus = "error"
		m.config.Policies[policyIdx].LastRunError = err.Error()
	} else if result.Status == dataset.TransferStatusSkipped {
		m.config.Policies[policyIdx].LastRunStatus = "skipped"
		m.config.Policies[policyIdx].LastRunError = ""
		m.config.Policies[policyIdx].LastTransferID = result.TransferID
	} else {
		m.config.Policies[policyIdx].LastRunStatus = "success"
		m.config.Policies[policyIdx].LastRunError = ""
		m.config.Policies[policyIdx].LastTransferID = result.TransferID
	}

	// Save config asynchronously
	go func() {
		if saveErr := m.SaveConfig(false); saveErr != nil {
			m.logger.Warn("Failed to save config after manual policy execution", "error", saveErr)
		}
	}()

	m.mu.Unlock()

	return result, err
}

// createJobsForPolicy creates gocron jobs for all schedules in a policy
func (m *Manager) createJobsForPolicy(policy *TransferPolicy) error {
	if !policy.Enabled {
		return nil
	}

	// Clear existing job mappings
	m.jobMapping[policy.ID] = []uuid.UUID{}

	for scheduleIdx, schedule := range policy.Schedules {
		if !schedule.Enabled {
			continue
		}

		job, err := m.createJob(policy, scheduleIdx, schedule)
		if err != nil {
			m.logger.Error("Failed to create job for schedule",
				"policy_id", policy.ID,
				"schedule_index", scheduleIdx,
				"error", err)
			continue
		}

		m.jobMapping[policy.ID] = append(m.jobMapping[policy.ID], job.ID())
		m.logger.Debug("Created job for policy schedule",
			"policy_id", policy.ID,
			"schedule_index", scheduleIdx,
			"job_id", job.ID())
	}

	return nil
}

// createJob creates a single gocron job for a schedule
func (m *Manager) createJob(
	policy *TransferPolicy,
	scheduleIdx int,
	schedule autosnapshots.ScheduleSpec,
) (gocron.Job, error) {
	// Define the task function that will execute the transfer
	taskFn := func(ctx context.Context) (any, error) {
		start := time.Now()

		// Get or initialize monitor
		m.mu.Lock()
		monitor, exists := m.config.Monitors[policy.ID]
		if !exists {
			monitor = &TransferPolicyMonitor{
				PolicyID:      policy.ID,
				ScheduleIndex: scheduleIdx,
			}
			m.config.Monitors[policy.ID] = monitor
		}
		monitor.Status = string(TransferPolicyStatusRunning)
		m.mu.Unlock()

		// Execute transfer
		result, err := m.executeTransferForPolicy(ctx, policy, "")

		// Update monitor
		m.mu.Lock()
		duration := time.Since(start)
		monitor.LastRunAt = &start
		monitor.RunCount++
		monitor.LastDuration = duration

		if err != nil {
			monitor.Status = string(TransferPolicyStatusError)
			monitor.LastError = err.Error()
			monitor.LastSkipped = false
			monitor.LastSkipReason = ""
			m.logger.Error("Transfer policy execution failed",
				"policy_id", policy.ID,
				"error", err)
		} else if result.Status == dataset.TransferStatusSkipped {
			// Track skipped transfer
			monitor.Status = string(TransferPolicyStatusIdle)
			monitor.LastError = ""
			monitor.CurrentTransferID = result.TransferID
			monitor.LastSkipped = true
			monitor.LastSkipReason = fmt.Sprintf("target already has snapshot: %s", result.SourceSnapshot)
			monitor.SkipCount++
		} else {
			monitor.Status = string(TransferPolicyStatusIdle)
			monitor.LastError = ""
			monitor.CurrentTransferID = result.TransferID
			monitor.LastSkipped = false
			monitor.LastSkipReason = ""
		}

		// Update policy fields
		for i := range m.config.Policies {
			if m.config.Policies[i].ID == policy.ID {
				m.config.Policies[i].LastRunAt = monitor.LastRunAt
				if err != nil {
					m.config.Policies[i].LastRunStatus = "error"
					m.config.Policies[i].LastRunError = err.Error()
				} else if result.Status == dataset.TransferStatusSkipped {
					m.config.Policies[i].LastRunStatus = "skipped"
					m.config.Policies[i].LastRunError = ""
					m.config.Policies[i].LastTransferID = result.TransferID
				} else {
					m.config.Policies[i].LastRunStatus = "success"
					m.config.Policies[i].LastRunError = ""
					m.config.Policies[i].LastTransferID = result.TransferID
				}
				break
			}
		}

		// Save config asynchronously
		go func() {
			if saveErr := m.SaveConfig(false); saveErr != nil {
				m.logger.Warn("Failed to save config after policy execution", "error", saveErr)
			}
		}()

		m.mu.Unlock()

		return result, err
	}

	// Build job definition based on schedule type
	var jobDef gocron.JobDefinition
	var err error

	switch schedule.Type {
	case autosnapshots.ScheduleTypeSecondly:
		interval := time.Duration(schedule.Interval) * time.Second
		jobDef = gocron.DurationJob(interval)

	case autosnapshots.ScheduleTypeMinutely:
		interval := time.Duration(schedule.Interval) * time.Minute
		jobDef = gocron.DurationJob(interval)

	case autosnapshots.ScheduleTypeHourly:
		interval := time.Duration(schedule.Interval) * time.Hour
		jobDef = gocron.DurationJob(interval)

	case autosnapshots.ScheduleTypeDaily:
		hour, min, sec := autosnapshots.ParseAtTime(schedule.AtTime)
		jobDef = gocron.DailyJob(uint(schedule.Interval), gocron.NewAtTimes(
			gocron.NewAtTime(hour, min, sec),
		))

	case autosnapshots.ScheduleTypeWeekly:
		hour, min, sec := autosnapshots.ParseAtTime(schedule.AtTime)
		jobDef = gocron.WeeklyJob(uint(schedule.Interval),
			gocron.NewWeekdays(schedule.WeekDay),
			gocron.NewAtTimes(
				gocron.NewAtTime(hour, min, sec),
			))

	case autosnapshots.ScheduleTypeDuration:
		jobDef = gocron.DurationJob(schedule.Duration)

	case autosnapshots.ScheduleTypeRandom:
		jobDef = gocron.DurationRandomJob(schedule.MinDuration, schedule.MaxDuration)

	case autosnapshots.ScheduleTypeOneTime:
		jobDef = gocron.OneTimeJob(gocron.OneTimeJobStartDateTime(schedule.StartTime))

	default:
		return nil, errors.New(errors.TransferPolicySchedulerError,
			fmt.Sprintf("unsupported schedule type: %s", schedule.Type))
	}

	// Create the job with task and options
	job, err := m.scheduler.NewJob(
		jobDef,
		gocron.NewTask(taskFn, context.Background()),
		gocron.WithSingletonMode(gocron.LimitModeWait), // Wait if previous execution still running
		gocron.WithEventListeners(
			gocron.BeforeJobRuns(func(jobID uuid.UUID, jobName string) {
				m.mu.Lock()
				if monitor, exists := m.config.Monitors[policy.ID]; exists {
					monitor.Status = string(TransferPolicyStatusRunning)
				}
				m.mu.Unlock()
			}),
			gocron.AfterJobRuns(func(jobID uuid.UUID, jobName string) {
				m.mu.Lock()
				if monitor, exists := m.config.Monitors[policy.ID]; exists {
					if monitor.Status == string(TransferPolicyStatusRunning) {
						monitor.Status = string(TransferPolicyStatusIdle)
					}
				}
				m.mu.Unlock()
			}),
		),
	)

	if err != nil {
		return nil, errors.Wrap(err, errors.TransferPolicySchedulerError)
	}

	return job, nil
}

// removeJobsForPolicy removes all scheduler jobs for a policy
func (m *Manager) removeJobsForPolicy(policyID string) {
	jobIDs, exists := m.jobMapping[policyID]
	if !exists {
		return
	}

	for _, jobID := range jobIDs {
		if err := m.scheduler.RemoveJob(jobID); err != nil {
			m.logger.Warn(
				"Failed to remove job",
				"policy_id",
				policyID,
				"job_id",
				jobID,
				"error",
				err,
			)
		}
	}

	delete(m.jobMapping, policyID)
}

// executeTransferForPolicy executes a transfer for a policy
func (m *Manager) executeTransferForPolicy(
	ctx context.Context,
	policy *TransferPolicy,
	snapshotOverride string,
) (*CreateTransferResult, error) {
	// Check if previous transfer is still running
	if policy.LastTransferID != "" {
		lastTransfer, err := m.transferManager.GetTransfer(policy.LastTransferID)
		if err == nil &&
			(lastTransfer.Status == dataset.TransferStatusRunning ||
				lastTransfer.Status == dataset.TransferStatusStarting ||
				lastTransfer.Status == dataset.TransferStatusPaused) {
			// Previous transfer still active, skip this execution
			m.logger.Info("Skipping transfer execution - previous transfer still active",
				"policy_id", policy.ID,
				"transfer_id", policy.LastTransferID,
				"status", lastTransfer.Status)

			// Update monitor with blocked reason
			if monitor, exists := m.config.Monitors[policy.ID]; exists {
				monitor.BlockedReason = fmt.Sprintf(
					"Previous transfer %s still %s",
					policy.LastTransferID,
					lastTransfer.Status,
				)
			}

			return nil, errors.New(errors.TransferPolicyTransferRunning,
				fmt.Sprintf("previous transfer %s still active", policy.LastTransferID))
		}
	}

	// Determine source snapshot
	var sourceSnapshot string
	if snapshotOverride != "" {
		sourceSnapshot = snapshotOverride
	} else {
		// Get latest snapshot from associated snapshot policy
		snapshot, err := m.getLatestSnapshotFromPolicy(policy.SnapshotPolicyID)
		if err != nil {
			return nil, err
		}
		sourceSnapshot = snapshot
	}

	// Extract source dataset from snapshot (format: dataset@snapshot)
	snapshotParts := strings.Split(sourceSnapshot, "@")
	if len(snapshotParts) != 2 {
		return nil, errors.New(errors.TransferPolicyInvalidConfig,
			fmt.Sprintf("invalid snapshot format: %s", sourceSnapshot))
	}
	sourceDataset := snapshotParts[0]

	// Prepare transfer config with snapshot
	transferCfg := policy.TransferConfig
	transferCfg.SendConfig.Snapshot = sourceSnapshot

	// Find the most recent common snapshot between source and target for incremental transfer
	// This uses ZFS GUIDs to reliably identify common snapshots
	targetDataset := transferCfg.ReceiveConfig.Target
	commonSnapshot, err := m.findMostRecentCommonSnapshot(sourceDataset, targetDataset, transferCfg.ReceiveConfig)
	if err != nil {
		// If we can't find common snapshot, log warning and attempt full send
		m.logger.Warn("Failed to find common snapshot, will attempt full send",
			"error", err,
			"source_dataset", sourceDataset,
			"target_dataset", targetDataset)
	} else if commonSnapshot != "" {
		// Check if target already has the latest snapshot (nothing to transfer)
		if commonSnapshot == sourceSnapshot {
			skipReason := fmt.Sprintf("target already has the latest snapshot: %s", sourceSnapshot)
			m.logger.Info("Target already in sync, skipping transfer",
				"snapshot", sourceSnapshot,
				"source_dataset", sourceDataset,
				"target_dataset", targetDataset)

			// Create a skipped transfer record
			transferID, err := m.transferManager.CreateSkippedTransfer(transferCfg, policy.ID, skipReason)
			if err != nil {
				return nil, errors.Wrap(err, errors.ZFSDatasetSend)
			}

			result := &CreateTransferResult{
				PolicyID:       policy.ID,
				TransferID:     transferID,
				SourceSnapshot: sourceSnapshot,
				TargetDataset:  transferCfg.ReceiveConfig.Target,
				CreatedAt:      time.Now(),
				Status:         dataset.TransferStatusSkipped,
			}

			return result, nil
		}

		// Use the full snapshot path for incremental transfer
		// The FromSnapshot field expects the full path: dataset@snapshot
		transferCfg.SendConfig.FromSnapshot = commonSnapshot
		m.logger.Info("Configuring incremental transfer",
			"from_snapshot", transferCfg.SendConfig.FromSnapshot,
			"to_snapshot", sourceSnapshot,
			"source_dataset", sourceDataset,
			"target_dataset", targetDataset)
	} else if transferCfg.SendConfig.Intermediary {
		// Target is new but Intermediary flag is set - we need to send all snapshots
		// Get the oldest snapshot to use as FromSnapshot, transfer_manager will:
		// 1. Send the oldest snapshot as a full send (via performInitialSend)
		// 2. Then send incremental with -I from oldest to latest
		oldestSnapshot, err := m.getOldestSnapshotFromPolicy(policy.SnapshotPolicyID)
		if err != nil {
			m.logger.Warn("Failed to get oldest snapshot for intermediary transfer, will send only latest",
				"error", err,
				"source_dataset", sourceDataset)
		} else if oldestSnapshot != sourceSnapshot {
			// Only set FromSnapshot if oldest differs from latest (i.e., there are intermediary snapshots)
			transferCfg.SendConfig.FromSnapshot = oldestSnapshot
			m.logger.Info("Configuring initial transfer with intermediary snapshots",
				"from_snapshot", oldestSnapshot,
				"to_snapshot", sourceSnapshot,
				"source_dataset", sourceDataset,
				"target_dataset", targetDataset)
		} else {
			m.logger.Debug("Only one snapshot exists, performing simple full send",
				"snapshot", sourceSnapshot,
				"source_dataset", sourceDataset)
		}
	}
	// If commonSnapshot is empty and Intermediary is false, perform simple full send of latest snapshot

	// Start the transfer with policy ID
	transferID, err := m.transferManager.StartTransferWithPolicy(ctx, transferCfg, policy.ID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ZFSDatasetSend)
	}

	result := &CreateTransferResult{
		PolicyID:       policy.ID,
		TransferID:     transferID,
		SourceSnapshot: sourceSnapshot,
		TargetDataset:  transferCfg.ReceiveConfig.Target,
		CreatedAt:      time.Now(),
		Status:         dataset.TransferStatusStarting,
	}

	m.logger.Info("Transfer initiated by policy",
		"policy_id", policy.ID,
		"transfer_id", transferID,
		"snapshot", sourceSnapshot)

	// Apply retention policy asynchronously
	go m.applyRetentionPolicy(policy)

	return result, nil
}

// getOldestSnapshotFromPolicy retrieves the oldest snapshot from the associated snapshot policy
// This is used for initial transfers with intermediary snapshots enabled
func (m *Manager) getOldestSnapshotFromPolicy(snapshotPolicyID string) (string, error) {
	return m.getSnapshotFromPolicy(snapshotPolicyID, true)
}

// getLatestSnapshotFromPolicy retrieves the latest snapshot from the associated snapshot policy
func (m *Manager) getLatestSnapshotFromPolicy(snapshotPolicyID string) (string, error) {
	return m.getSnapshotFromPolicy(snapshotPolicyID, false)
}

// getSnapshotFromPolicy retrieves a snapshot from the associated snapshot policy
// If oldest is true, returns the oldest matching snapshot; otherwise returns the latest
func (m *Manager) getSnapshotFromPolicy(snapshotPolicyID string, oldest bool) (string, error) {
	// Get the snapshot policy
	snapPolicy, err := m.snapshotManager.GetPolicy(snapshotPolicyID)
	if err != nil {
		return "", errors.Wrap(err, errors.TransferPolicySnapshotPolicyNotFound)
	}

	// List all snapshots for the dataset, sorted by creation time
	// -S creation = descending (newest first), -s creation = ascending (oldest first)
	sortFlag := "-S"
	if oldest {
		sortFlag = "-s"
	}
	cmd := exec.Command(
		"sudo",
		"zfs",
		"list",
		"-o",
		"name",
		"-H",
		"-t",
		"snap",
		sortFlag,
		"creation",
		snapPolicy.Dataset,
	)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", errors.New(
				errors.ZFSSnapshotList,
				fmt.Sprintf(
					"failed to list snapshots for dataset %s: %s",
					snapPolicy.Dataset,
					string(exitErr.Stderr),
				),
			)
		}
		return "", errors.New(errors.ZFSSnapshotList,
			fmt.Sprintf("failed to list snapshots for dataset %s: %v", snapPolicy.Dataset, err))
	}

	// Parse output and filter by snapshot name pattern
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return "", errors.New(errors.TransferPolicyNoSnapshots,
			fmt.Sprintf("no snapshots found for dataset %s", snapPolicy.Dataset))
	}

	// Convert the snapshot name pattern to a regex pattern
	// The pattern may contain placeholders like {timestamp}, {policy_id}, etc.
	// We need to convert it to a regex that matches actual snapshot names
	patternRegex, err := m.buildSnapshotPatternRegex(snapPolicy.SnapNamePattern)
	if err != nil {
		return "", errors.New(errors.TransferPolicyInvalidConfig,
			fmt.Sprintf("invalid snapshot pattern: %v", err))
	}

	// Find the first (most recent) snapshot that matches the pattern
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Extract snapshot name from full path (dataset@snapshot)
		parts := strings.Split(line, "@")
		if len(parts) != 2 {
			continue
		}

		snapshotName := parts[1]
		if patternRegex.MatchString(snapshotName) {
			m.logger.Debug("Found matching snapshot",
				"snapshot", line,
				"pattern", snapPolicy.SnapNamePattern)
			return line, nil
		}
	}

	return "", errors.New(errors.TransferPolicyNoSnapshots,
		fmt.Sprintf("no snapshots matching pattern '%s' found for dataset %s",
			snapPolicy.SnapNamePattern, snapPolicy.Dataset))
}

// buildSnapshotPatternRegex converts a snapshot name pattern to a regex
// Pattern placeholders like {timestamp}, {policy_id}, etc. are converted to regex wildcards
// Also handles strftime-style format codes like %Y-%m-%d-%H%M%S
func (m *Manager) buildSnapshotPatternRegex(pattern string) (*regexp.Regexp, error) {
	// Escape special regex characters except for placeholders
	regexPattern := regexp.QuoteMeta(pattern)

	// Replace strftime-style format codes (used by snapshot manager)
	// These are escaped by QuoteMeta, so we need to match the escaped versions
	strftimeReplacements := map[string]string{
		regexp.QuoteMeta("%Y"): `\d{4}`, // Year (4 digits)
		regexp.QuoteMeta("%m"): `\d{2}`, // Month (2 digits)
		regexp.QuoteMeta("%d"): `\d{2}`, // Day (2 digits)
		regexp.QuoteMeta("%H"): `\d{2}`, // Hour (2 digits)
		regexp.QuoteMeta("%M"): `\d{2}`, // Minute (2 digits)
		regexp.QuoteMeta("%S"): `\d{2}`, // Second (2 digits)
	}

	for code, regexRepl := range strftimeReplacements {
		regexPattern = strings.ReplaceAll(regexPattern, code, regexRepl)
	}

	// Replace placeholder patterns with regex patterns
	// Common placeholders: {timestamp}, {policy_id}, {policy_name}, {date}, {time}, etc.
	replacements := map[string]string{
		regexp.QuoteMeta("{timestamp}"):   `\d{4}-\d{2}-\d{2}-\d{6}`, // YYYY-MM-DD-HHMMSS
		regexp.QuoteMeta("{date}"):        `\d{4}-\d{2}-\d{2}`,       // YYYY-MM-DD
		regexp.QuoteMeta("{time}"):        `\d{6}`,                   // HHMMSS
		regexp.QuoteMeta("{policy_id}"):   `[a-f0-9\-]+`,             // UUID
		regexp.QuoteMeta("{policy_name}"): `[a-zA-Z0-9\-_]+`,         // Policy name
		regexp.QuoteMeta("{sequence}"):    `\d+`,                     // Sequence number
	}

	for placeholder, regexRepl := range replacements {
		regexPattern = strings.ReplaceAll(regexPattern, placeholder, regexRepl)
	}

	// The snapshot manager appends: -{schedule_index}-{policy_id_suffix}
	// Schedule index is a digit (0-4, max 5 schedules), policy ID suffix is last part of UUID
	// Example: autosnap-policy-%Y-%m-%d-%H%M%S becomes autosnap-policy-2025-11-25-081138-0-d1f36875b92f
	regexPattern = regexPattern + `-\d+-[a-f0-9]+`

	// Anchor the pattern to match the full snapshot name
	regexPattern = "^" + regexPattern + "$"

	return regexp.Compile(regexPattern)
}

// findMostRecentCommonSnapshot finds the most recent common snapshot between source and target
// using ZFS GUIDs for reliable matching. Returns the common snapshot name on the source dataset,
// or an empty string if no common snapshot is found or target doesn't exist.
func (m *Manager) findMostRecentCommonSnapshot(
	sourceDataset, targetDataset string,
	recvCfg dataset.ReceiveConfig,
) (string, error) {
	isRemote := recvCfg.RemoteConfig.Host != ""

	// Build SSH command prefix for remote targets
	var sshPrefix []string
	if isRemote {
		var err error
		sshPrefix, err = dataset.BuildSSHCommand(recvCfg.RemoteConfig)
		if err != nil {
			return "", errors.New(errors.ZFSDatasetSend,
				fmt.Sprintf("failed to build SSH command: %v", err))
		}
	}

	// Check if target dataset exists
	var checkCmd *exec.Cmd
	if isRemote {
		cmdStr := fmt.Sprintf("%s sudo zfs list -H -o name %s",
			shellquote.Join(sshPrefix...), shellquote.Join(targetDataset))
		checkCmd = exec.Command("bash", "-c", cmdStr)
		m.logger.Debug("Checking remote target dataset existence", "command", cmdStr)
	} else {
		checkCmd = exec.Command("sudo", "zfs", "list", "-H", "-o", "name", targetDataset)
	}

	if err := checkCmd.Run(); err != nil {
		// Target doesn't exist - this will be a full send
		m.logger.Debug("Target dataset does not exist, will perform full send",
			"target", targetDataset,
			"remote", isRemote)
		return "", nil
	}

	// List source snapshots with GUIDs (sorted by creation, newest first)
	sourceCmd := exec.Command(
		"sudo",
		"zfs",
		"list",
		"-H",
		"-o",
		"name,guid",
		"-t",
		"snap",
		"-S",
		"creation",
		sourceDataset,
	)
	sourceOutput, err := sourceCmd.Output()
	if err != nil {
		return "", errors.New(errors.ZFSSnapshotList,
			fmt.Sprintf("failed to list source snapshots for %s: %v", sourceDataset, err))
	}

	// List target snapshots with GUIDs
	var targetCmd *exec.Cmd
	if isRemote {
		cmdStr := fmt.Sprintf("%s sudo zfs list -H -o name,guid -t snap %s",
			shellquote.Join(sshPrefix...), shellquote.Join(targetDataset))
		targetCmd = exec.Command("bash", "-c", cmdStr)
		m.logger.Debug("Listing remote target snapshots", "command", cmdStr)
	} else {
		targetCmd = exec.Command(
			"sudo",
			"zfs",
			"list",
			"-H",
			"-o",
			"name,guid",
			"-t",
			"snap",
			targetDataset,
		)
	}
	targetOutput, err := targetCmd.Output()
	if err != nil {
		return "", errors.New(errors.ZFSSnapshotList,
			fmt.Sprintf("failed to list target snapshots for %s: %v", targetDataset, err))
	}

	// Parse target snapshots into a GUID -> name map
	targetGUIDs := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(string(targetOutput)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		targetGUIDs[fields[1]] = fields[0] // guid -> full snapshot name
	}

	if len(targetGUIDs) == 0 {
		// Target has no snapshots - this will be a full send
		m.logger.Debug("Target dataset has no snapshots, will perform full send",
			"target", targetDataset,
			"remote", isRemote)
		return "", nil
	}

	// Find the most recent source snapshot that exists on target (by GUID)
	for line := range strings.SplitSeq(strings.TrimSpace(string(sourceOutput)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		sourceSnapshot := fields[0]
		sourceGUID := fields[1]

		// Check if this GUID exists on target
		if _, exists := targetGUIDs[sourceGUID]; exists {
			m.logger.Info("Found most recent common snapshot",
				"source_snapshot", sourceSnapshot,
				"guid", sourceGUID,
				"source_dataset", sourceDataset,
				"target_dataset", targetDataset,
				"remote", isRemote)
			return sourceSnapshot, nil
		}
	}

	// No common snapshots found
	m.logger.Warn("No common snapshots found between source and target",
		"source_dataset", sourceDataset,
		"target_dataset", targetDataset,
		"target_snapshot_count", len(targetGUIDs),
		"remote", isRemote)

	return "", errors.New(errors.ZFSDatasetSend,
		fmt.Sprintf("no common snapshots found between %s and %s", sourceDataset, targetDataset))
}

// deletePolicyTransfers deletes all non-running transfers associated with a policy
func (m *Manager) deletePolicyTransfers(policyID string) error {
	// List all transfers and filter by policy ID
	allTransfers := m.transferManager.ListTransfers()

	deletedCount := 0
	for _, transfer := range allTransfers {
		// Only delete transfers that belong to this policy
		if transfer.PolicyID != policyID {
			continue
		}

		// Don't delete running or paused transfers
		if transfer.Status == dataset.TransferStatusRunning ||
			transfer.Status == dataset.TransferStatusPaused ||
			transfer.Status == dataset.TransferStatusStarting {
			m.logger.Debug("Skipping active transfer",
				"transfer_id", transfer.ID,
				"status", transfer.Status)
			continue
		}

		// Delete the transfer
		if err := m.transferManager.DeleteTransfer(transfer.ID); err != nil {
			m.logger.Warn("Failed to delete transfer",
				"transfer_id", transfer.ID,
				"error", err)
		} else {
			deletedCount++
			m.logger.Debug("Deleted policy transfer",
				"transfer_id", transfer.ID,
				"policy_id", policyID)
		}
	}

	m.logger.Info("Deleted policy transfers",
		"policy_id", policyID,
		"count", deletedCount)

	return nil
}

// applyRetentionPolicy applies retention rules to clean up old transfers
func (m *Manager) applyRetentionPolicy(policy *TransferPolicy) {
	retention := policy.RetentionPolicy

	// Skip if no retention policy is configured
	if retention.KeepCount == 0 && retention.OlderThan == 0 {
		m.logger.Debug("No retention policy configured", "policy_id", policy.ID)
		return
	}

	// List all transfers and filter by policy ID
	allTransfers := m.transferManager.ListTransfers()
	var policyTransfers []*dataset.TransferInfo

	for _, transfer := range allTransfers {
		if transfer.PolicyID == policy.ID {
			policyTransfers = append(policyTransfers, transfer)
		}
	}

	if len(policyTransfers) == 0 {
		m.logger.Debug("No transfers found for policy", "policy_id", policy.ID)
		return
	}

	// Sort transfers by creation time (most recent first)
	sort.Slice(policyTransfers, func(i, j int) bool {
		return policyTransfers[i].CreatedAt.After(policyTransfers[j].CreatedAt)
	})

	deletedCount := 0
	now := time.Now()

	for idx, transfer := range policyTransfers {
		// Check if transfer is in the keep list
		if slices.Contains(retention.KeepTransferIDs, transfer.ID) {
			m.logger.Debug("Keeping transfer (in keep list)",
				"transfer_id", transfer.ID)
			continue
		}

		// Don't delete running or paused transfers
		if transfer.Status == dataset.TransferStatusRunning ||
			transfer.Status == dataset.TransferStatusPaused ||
			transfer.Status == dataset.TransferStatusStarting {
			continue
		}

		// Keep failed transfers if configured
		if retention.KeepFailed && transfer.Status == dataset.TransferStatusFailed {
			m.logger.Debug("Keeping failed transfer",
				"transfer_id", transfer.ID)
			continue
		}

		// Only apply retention to completed transfers if configured
		if retention.CompletedOnly && transfer.Status != dataset.TransferStatusCompleted {
			continue
		}

		shouldDelete := false

		// Apply keep count rule (keep only N most recent)
		if retention.KeepCount > 0 && idx >= retention.KeepCount {
			shouldDelete = true
			m.logger.Debug("Transfer exceeds keep count",
				"transfer_id", transfer.ID,
				"index", idx,
				"keep_count", retention.KeepCount)
		}

		// Apply age-based retention rule
		if retention.OlderThan > 0 {
			age := now.Sub(transfer.CreatedAt)
			if age > retention.OlderThan {
				shouldDelete = true
				m.logger.Debug("Transfer exceeds age limit",
					"transfer_id", transfer.ID,
					"age", age,
					"limit", retention.OlderThan)
			}
		}

		if shouldDelete {
			if err := m.transferManager.DeleteTransfer(transfer.ID); err != nil {
				m.logger.Warn("Failed to delete transfer during retention",
					"transfer_id", transfer.ID,
					"error", err)
			} else {
				deletedCount++
				m.logger.Debug("Deleted transfer by retention policy",
					"transfer_id", transfer.ID,
					"policy_id", policy.ID)
			}
		}
	}

	if deletedCount > 0 {
		m.logger.Info("Applied retention policy",
			"policy_id", policy.ID,
			"deleted_count", deletedCount,
			"total_transfers", len(policyTransfers))
	}
}

// LoadConfig loads the transfer policy configuration from disk
func (m *Manager) LoadConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		m.logger.Info("Transfer policy config file does not exist, starting with empty config")
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return errors.Wrap(err, errors.ConfigReadError)
	}

	var cfg TransferPolicyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// Backup corrupted config
		backupPath := m.configPath + fmt.Sprintf(
			".error.%s",
			time.Now().Format("2006-01-02-150405"),
		)
		if copyErr := os.WriteFile(backupPath, data, 0644); copyErr != nil {
			m.logger.Error("Failed to backup corrupted config", "error", copyErr)
		}
		m.logger.Warn("Corrupted config backed up", "backup_path", backupPath)
		return errors.Wrap(err, errors.ConfigUnmarshalFailed)
	}

	// Validate all policies
	validPolicies := []TransferPolicy{}
	for _, policy := range cfg.Policies {
		if err := ValidateTransferPolicy(&policy); err != nil {
			m.logger.Warn(
				"Invalid policy in config, skipping",
				"policy_id",
				policy.ID,
				"error",
				err,
			)
			continue
		}
		validPolicies = append(validPolicies, policy)
	}

	// If some policies were invalid, save cleaned config
	if len(validPolicies) != len(cfg.Policies) {
		m.logger.Info("Removed invalid policies from config",
			"original_count", len(cfg.Policies),
			"valid_count", len(validPolicies))
		backupPath := m.configPath + fmt.Sprintf(
			".cleaned.%s",
			time.Now().Format("2006-01-02-150405"),
		)
		if copyErr := os.WriteFile(backupPath, data, 0644); copyErr == nil {
			m.logger.Info("Original config backed up before cleaning", "backup_path", backupPath)
		}
		cfg.Policies = validPolicies
	}

	// Ensure monitors map is initialized
	if cfg.Monitors == nil {
		cfg.Monitors = make(map[string]*TransferPolicyMonitor)
	}

	m.config = cfg
	m.logger.Info("Transfer policy config loaded", "policy_count", len(cfg.Policies))
	return nil
}

// SaveConfig saves the transfer policy configuration to disk
func (m *Manager) SaveConfig(skipLock bool) error {
	if !skipLock {
		m.mu.RLock()
		defer m.mu.RUnlock()
	}

	// Deep copy config to avoid mutations during save
	cfgCopy := TransferPolicyConfig{
		Policies: make([]TransferPolicy, len(m.config.Policies)),
		Monitors: make(map[string]*TransferPolicyMonitor),
	}
	copy(cfgCopy.Policies, m.config.Policies)
	for k, v := range m.config.Monitors {
		monitorCopy := *v
		cfgCopy.Monitors[k] = &monitorCopy
	}

	data, err := yaml.Marshal(&cfgCopy)
	if err != nil {
		return errors.Wrap(err, errors.ConfigMarshalFailed)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return errors.Wrap(err, errors.ConfigWriteError)
	}

	return nil
}

// saveConfigWithTimeout saves config with timeout protection
func (m *Manager) saveConfigWithTimeout() error {
	saveDone := make(chan error, 1)
	go func() {
		saveDone <- m.SaveConfig(true) // skipLock=true since we already hold the lock
	}()

	select {
	case err := <-saveDone:
		return err
	case <-time.After(5 * time.Second):
		return errors.New(errors.ConfigWriteError, "timeout while saving transfer policy config")
	}
}

// CheckSnapshotPolicyInUse checks if a snapshot policy is referenced by any transfer policies
func (m *Manager) CheckSnapshotPolicyInUse(snapshotPolicyID string) (bool, []string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policyIDs := []string{}
	for _, policy := range m.config.Policies {
		if policy.SnapshotPolicyID == snapshotPolicyID {
			policyIDs = append(policyIDs, policy.ID)
		}
	}

	return len(policyIDs) > 0, policyIDs, nil
}
