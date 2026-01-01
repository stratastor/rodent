// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2024 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package autotransfers

import (
	"fmt"
	"time"

	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/autosnapshots"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
)

// Helper functions from common package for middleware
var ReadResetBody = common.ReadResetBody
var ResetBody = common.ResetBody

// TransferPolicy defines a scheduled ZFS transfer policy
// It orchestrates periodic transfers by creating individual transfer instances
// via the TransferManager for each scheduled execution
type TransferPolicy struct {
	// Core identification
	ID          string `json:"id"          yaml:"id"`
	Name        string `json:"name"        yaml:"name"`
	Description string `json:"description" yaml:"description"`

	// Snapshot policy association
	// The snapshot policy determines which snapshots are available to transfer
	// and helps match snapshot name patterns
	SnapshotPolicyID string `json:"snapshot_policy_id" yaml:"snapshot_policy_id"`

	// Transfer configuration
	// Note: The SendConfig.Snapshot field will be dynamically determined
	// from the associated snapshot policy's latest snapshot at transfer time
	TransferConfig dataset.TransferConfig `json:"transfer_config" yaml:"transfer_config"`

	// Scheduling - supports multiple schedules per policy
	Schedules []autosnapshots.ScheduleSpec `json:"schedules" yaml:"schedules"`

	// Retention policy for transfer entries (not snapshots)
	// Controls automatic cleanup of completed/failed transfer records
	RetentionPolicy TransferRetentionPolicy `json:"retention_policy" yaml:"retention_policy"`

	// Policy state
	Enabled        bool       `json:"enabled"                    yaml:"enabled"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty"      yaml:"last_run_at,omitempty"`
	LastRunStatus  string     `json:"last_run_status,omitempty"  yaml:"last_run_status,omitempty"`
	LastRunError   string     `json:"last_run_error,omitempty"   yaml:"last_run_error,omitempty"`
	LastTransferID string     `json:"last_transfer_id,omitempty" yaml:"last_transfer_id,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`

	// Runtime monitor status (enriched at read-time, not persisted)
	MonitorStatus *TransferPolicyMonitor `json:"monitor_status,omitempty" yaml:"-"`
}

// TransferRetentionPolicy defines retention rules for transfer records
type TransferRetentionPolicy struct {
	// Keep only the N most recent transfers (0 = unlimited)
	KeepCount int `json:"keep_count" yaml:"keep_count"`

	// Delete transfers older than this duration (0 = no age limit)
	OlderThan time.Duration `json:"older_than" yaml:"older_than"`

	// Only apply retention to completed transfers
	CompletedOnly bool `json:"completed_only" yaml:"completed_only"`

	// Always keep failed transfers (for debugging)
	KeepFailed bool `json:"keep_failed" yaml:"keep_failed"`

	// Specific transfer IDs to never delete
	KeepTransferIDs []string `json:"keep_transfer_ids,omitempty" yaml:"keep_transfer_ids,omitempty"`
}

// TransferPolicyMonitor tracks runtime execution status of a policy
// This is similar to JobMonitor in snapshot policies
type TransferPolicyMonitor struct {
	PolicyID      string        `json:"policy_id"             yaml:"policy_id"`
	ScheduleIndex int           `json:"schedule_index"        yaml:"schedule_index"`
	Status        string        `json:"status"                yaml:"status"` // idle, running, waiting, error
	LastRunAt     *time.Time    `json:"last_run_at,omitempty" yaml:"last_run_at,omitempty"`
	NextRunAt     *time.Time    `json:"next_run_at,omitempty" yaml:"next_run_at,omitempty"`
	RunCount      int           `json:"run_count"             yaml:"run_count"`
	LastDuration  time.Duration `json:"last_duration"         yaml:"last_duration"`
	LastError     string        `json:"last_error"            yaml:"last_error"`

	// Transfer-specific tracking
	CurrentTransferID string `json:"current_transfer_id,omitempty" yaml:"current_transfer_id,omitempty"`
	BlockedReason     string `json:"blocked_reason,omitempty"      yaml:"blocked_reason,omitempty"` // e.g., "previous transfer still running"

	// Skip tracking (when target is already in sync)
	LastSkipped    bool   `json:"last_skipped,omitempty"     yaml:"last_skipped,omitempty"`
	LastSkipReason string `json:"last_skip_reason,omitempty" yaml:"last_skip_reason,omitempty"`
	SkipCount      int    `json:"skip_count,omitempty"       yaml:"skip_count,omitempty"`
}

// TransferPolicyConfig is the overall configuration structure
// Persisted to YAML file for durability across restarts
type TransferPolicyConfig struct {
	Policies []TransferPolicy                  `json:"policies" yaml:"policies"`
	Monitors map[string]*TransferPolicyMonitor `json:"monitors" yaml:"monitors"` // PolicyID -> Monitor
}

// TransferPolicyStatus represents different policy states
type TransferPolicyStatus string

const (
	TransferPolicyStatusIdle    TransferPolicyStatus = "idle"    // Enabled and waiting for next schedule
	TransferPolicyStatusRunning TransferPolicyStatus = "running" // Currently executing a transfer
	TransferPolicyStatusWaiting TransferPolicyStatus = "waiting" // Scheduled but waiting (e.g., for previous transfer)
	TransferPolicyStatusPaused  TransferPolicyStatus = "paused"  // Policy is disabled
	TransferPolicyStatusError   TransferPolicyStatus = "error"   // Last execution failed
)

// EditTransferPolicyParams defines parameters for creating/updating a transfer policy
type EditTransferPolicyParams struct {
	ID               string                       `json:"id,omitempty"`
	Name             string                       `json:"name"`
	Description      string                       `json:"description"`
	SnapshotPolicyID string                       `json:"snapshot_policy_id"`
	TransferConfig   dataset.TransferConfig       `json:"transfer_config"`
	Schedules        []autosnapshots.ScheduleSpec `json:"schedules"`
	RetentionPolicy  TransferRetentionPolicy      `json:"retention_policy"`
	Enabled          bool                         `json:"enabled"`
}

// RunTransferPolicyParams defines parameters for manually running a transfer policy
type RunTransferPolicyParams struct {
	PolicyID string `json:"policy_id"`
	// Optional: override which snapshot to transfer (otherwise uses latest from snapshot policy)
	SnapshotOverride string `json:"snapshot_override,omitempty"`
}

// CreateTransferResult contains the result of a policy-initiated transfer
type CreateTransferResult struct {
	PolicyID       string                 `json:"policy_id"`
	TransferID     string                 `json:"transfer_id"`
	SourceSnapshot string                 `json:"source_snapshot"`
	TargetDataset  string                 `json:"target_dataset"`
	CreatedAt      time.Time              `json:"created_at"`
	Status         dataset.TransferStatus `json:"status"`
}

// TransferPolicyListResult contains the list of policies with count
type TransferPolicyListResult struct {
	Policies []TransferPolicy `json:"policies"`
	Count    int              `json:"count"`
}

// NewTransferPolicy creates a new TransferPolicy from EditTransferPolicyParams
// This is a helper for creating a policy struct for validation
func NewTransferPolicy(params EditTransferPolicyParams) *TransferPolicy {
	return &TransferPolicy{
		ID:               params.ID,
		Name:             params.Name,
		Description:      params.Description,
		SnapshotPolicyID: params.SnapshotPolicyID,
		TransferConfig:   params.TransferConfig,
		Schedules:        params.Schedules,
		RetentionPolicy:  params.RetentionPolicy,
		Enabled:          params.Enabled,
	}
}

// ValidatePolicy is an alias for ValidateTransferPolicy for convenience
func ValidatePolicy(policy *TransferPolicy) error {
	return ValidateTransferPolicy(policy)
}

// ValidateTransferPolicy validates a transfer policy configuration
func ValidateTransferPolicy(policy *TransferPolicy) error {
	if policy.Name == "" {
		return errors.New(errors.TransferPolicyInvalidConfig, "policy name is required")
	}

	if policy.SnapshotPolicyID == "" {
		return errors.New(errors.TransferPolicyInvalidConfig, "snapshot policy ID is required")
	}

	if len(policy.Schedules) == 0 {
		return errors.New(errors.TransferPolicyInvalidConfig, "at least one schedule is required")
	}

	if len(policy.Schedules) > 5 {
		return errors.New(
			errors.TransferPolicyInvalidConfig,
			"maximum 5 schedules allowed per policy",
		)
	}

	// Validate each schedule
	for i, schedule := range policy.Schedules {
		if err := autosnapshots.ValidateScheduleSpec(schedule); err != nil {
			return errors.New(
				errors.TransferPolicyInvalidConfig,
				fmt.Sprintf("schedule %d invalid: %v", i, err),
			)
		}
	}

	// Validate transfer config
	if policy.TransferConfig.SendConfig.Snapshot == "" {
		// Snapshot will be set dynamically, but other send config should be valid
		if policy.TransferConfig.SendConfig.Timeout < 0 {
			return errors.New(errors.TransferPolicyInvalidConfig, "invalid send timeout")
		}
	}

	if policy.TransferConfig.ReceiveConfig.Target == "" {
		return errors.New(errors.TransferPolicyInvalidConfig, "receive target is required")
	}

	// Retention policy validation
	if policy.RetentionPolicy.KeepCount < 0 {
		return errors.New(
			errors.TransferPolicyInvalidConfig,
			"retention keep_count cannot be negative",
		)
	}

	if policy.RetentionPolicy.OlderThan < 0 {
		return errors.New(
			errors.TransferPolicyInvalidConfig,
			"retention older_than cannot be negative",
		)
	}

	return nil
}

// ValidateEditTransferPolicyParams validates parameters for creating/updating a policy
func ValidateEditTransferPolicyParams(params *EditTransferPolicyParams) error {
	if params.Name == "" {
		return errors.New(errors.TransferPolicyInvalidConfig, "policy name is required")
	}

	if params.SnapshotPolicyID == "" {
		return errors.New(errors.TransferPolicyInvalidConfig, "snapshot policy ID is required")
	}

	if len(params.Schedules) == 0 {
		return errors.New(errors.TransferPolicyInvalidConfig, "at least one schedule is required")
	}

	if len(params.Schedules) > 5 {
		return errors.New(
			errors.TransferPolicyInvalidConfig,
			"maximum 5 schedules allowed per policy",
		)
	}

	// Validate each schedule
	for i, schedule := range params.Schedules {
		if err := autosnapshots.ValidateScheduleSpec(schedule); err != nil {
			return errors.New(
				errors.TransferPolicyInvalidConfig,
				fmt.Sprintf("schedule %d invalid: %v", i, err),
			)
		}
	}

	if params.TransferConfig.ReceiveConfig.Target == "" {
		return errors.New(errors.TransferPolicyInvalidConfig, "receive target is required")
	}

	return nil
}
