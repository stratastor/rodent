// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/errors"
)

var APIError = common.APIError
var ReadResetBody = common.ReadResetBody
var ResetBody = common.ResetBody

// ScheduleType represents the type of schedule
type ScheduleType string

const (
	ScheduleTypeSecondly ScheduleType = "secondly"
	ScheduleTypeMinutely ScheduleType = "minutely"
	ScheduleTypeHourly   ScheduleType = "hourly"
	ScheduleTypeDaily    ScheduleType = "daily"
	ScheduleTypeWeekly   ScheduleType = "weekly"
	ScheduleTypeMonthly  ScheduleType = "monthly"
	ScheduleTypeYearly   ScheduleType = "yearly"
	ScheduleTypeOneTime  ScheduleType = "onetime"
	ScheduleTypeDuration ScheduleType = "duration"
	ScheduleTypeRandom   ScheduleType = "random"
)

// RetentionPolicy defines how snapshots are retained or pruned
type RetentionPolicy struct {
	Count         int           `json:"count"           yaml:"count"`           // Number of snapshots to keep
	OlderThan     time.Duration `json:"older_than"      yaml:"older_than"`      // Prune snapshots older than this duration
	ForceDestroy  bool          `json:"force_destroy"   yaml:"force_destroy"`   // Force destroy even if dependencies exist
	KeepNamedSnap []string      `json:"keep_named_snap" yaml:"keep_named_snap"` // List of specific snapshot names to keep
}

// ScheduleSpec defines a specific schedule configuration
type ScheduleSpec struct {
	Type        ScheduleType  `json:"type"         yaml:"type"`         // Type of schedule
	Interval    uint          `json:"interval"     yaml:"interval"`     // Interval count (e.g., every 2 hours)
	AtTime      string        `json:"at_time"      yaml:"at_time"`      // Specific time for daily/weekly/monthly/yearly
	Duration    time.Duration `json:"duration"     yaml:"duration"`     // For duration-based schedules
	MinDuration time.Duration `json:"min_duration" yaml:"min_duration"` // For random schedules
	MaxDuration time.Duration `json:"max_duration" yaml:"max_duration"` // For random schedules
	WeekDay     time.Weekday  `json:"week_day"     yaml:"week_day"`     // Day of week for weekly schedules
	DayOfMonth  int           `json:"day_of_month" yaml:"day_of_month"` // Day of month for monthly schedules
	Month       time.Month    `json:"month"        yaml:"month"`        // Month for yearly schedules
	StartTime   time.Time     `json:"start_time"   yaml:"start_time"`   // Start time for the schedule
	EndTime     time.Time     `json:"end_time"     yaml:"end_time"`     // End time for the schedule
	LimitedRuns int           `json:"limited_runs" yaml:"limited_runs"` // Number of runs to limit to (0 = unlimited)
	Enabled     bool          `json:"enabled"      yaml:"enabled"`      // Whether this schedule is enabled
}

// SnapshotPolicy represents a complete auto-snapshot policy
type SnapshotPolicy struct {
	ID              string            `json:"id"                yaml:"id"`                // Unique identifier
	Name            string            `json:"name"              yaml:"name"`              // User-friendly name
	Description     string            `json:"description"       yaml:"description"`       // Description of the policy
	Dataset         string            `json:"dataset"           yaml:"dataset"`           // ZFS dataset to snapshot
	Schedules       []ScheduleSpec    `json:"schedules"         yaml:"schedules"`         // List of schedules for this policy (max 5)
	Recursive       bool              `json:"recursive"         yaml:"recursive"`         // Whether to snapshot recursively
	SnapNamePattern string            `json:"snap_name_pattern" yaml:"snap_name_pattern"` // Pattern for snapshot names
	RetentionPolicy RetentionPolicy   `json:"retention_policy"  yaml:"retention_policy"`  // Retention/pruning policy
	Properties      map[string]string `json:"properties"        yaml:"properties"`        // ZFS properties to set on snapshots
	Enabled         bool              `json:"enabled"           yaml:"enabled"`           // Whether this policy is enabled
	CreatedAt       time.Time         `json:"created_at"        yaml:"created_at"`        // When this policy was created
	UpdatedAt       time.Time         `json:"updated_at"        yaml:"updated_at"`        // When this policy was last updated
	LastRunAt       time.Time         `json:"last_run_at"       yaml:"last_run_at"`       // When this policy was last executed
	LastRunStatus   string            `json:"last_run_status"   yaml:"last_run_status"`   // Status of the last run
	LastRunError    string            `json:"last_run_error"    yaml:"last_run_error"`    // Error from the last run, if any
	MonitorStatus   *JobMonitor       `json:"monitor_status"    yaml:"-"`                 // Detailed job monitor status (not stored in YAML)
}

// JobMonitor monitors job status and execution
type JobMonitor struct {
	PolicyID     string        `json:"policy_id"     yaml:"policy_id"`
	ScheduleID   int           `json:"schedule_id"   yaml:"schedule_id"`
	Status       string        `json:"status"        yaml:"status"`
	LastRunAt    time.Time     `json:"last_run_at"   yaml:"last_run_at"`
	NextRunAt    time.Time     `json:"next_run_at"   yaml:"next_run_at"`
	RunCount     int           `json:"run_count"     yaml:"run_count"`
	LastDuration time.Duration `json:"last_duration" yaml:"last_duration"`
	LastError    string        `json:"last_error"    yaml:"last_error"`
}

// SnapshotConfig wraps the collection of snapshot policies and job monitors
type SnapshotConfig struct {
	Policies []SnapshotPolicy      `json:"policies" yaml:"policies"`
	Monitors map[string]JobMonitor `json:"monitors" yaml:"monitors"`
}

// EditPolicyParams are parameters for creating or updating a policy
type EditPolicyParams struct {
	ID              string            `json:"id,omitempty"` // ID for updates, empty for new policies
	Name            string            `json:"name"`         // Required
	Description     string            `json:"description,omitempty"`
	Dataset         string            `json:"dataset"`   // Required
	Schedules       []ScheduleSpec    `json:"schedules"` // Required, max 5
	Recursive       bool              `json:"recursive"`
	SnapNamePattern string            `json:"snap_name_pattern,omitempty"`
	RetentionPolicy RetentionPolicy   `json:"retention_policy,omitempty"`
	Properties      map[string]string `json:"properties,omitempty"`
	Enabled         bool              `json:"enabled"`
}

// RunPolicyParams are parameters for running a policy immediately
type RunPolicyParams struct {
	ID            string `json:"id"`                // Policy ID
	ScheduleIndex int    `json:"schedule_index"`    // Index of schedule to run
	DryRun        bool   `json:"dry_run,omitempty"` // Just simulate, don't create
}

// CreateSnapshotResult is the result of creating a snapshot
type CreateSnapshotResult struct {
	PolicyID        string    `json:"policy_id"`
	ScheduleIndex   int       `json:"schedule_index"`
	DatasetName     string    `json:"dataset_name"`
	SnapshotName    string    `json:"snapshot_name"`
	CreatedAt       time.Time `json:"created_at"`
	Error           error     `json:"error,omitempty"`
	PrunedSnapshots []string  `json:"pruned_snapshots,omitempty"`
}

// SchedulerInterface defines the interface for the scheduler
type SchedulerInterface interface {
	AddPolicy(params EditPolicyParams) (string, error)
	UpdatePolicy(params EditPolicyParams) error
	RemovePolicy(policyID string, removeSnapshots bool) error
	GetPolicy(policyID string) (SnapshotPolicy, error)
	ListPolicies() ([]SnapshotPolicy, error)
	RunPolicy(params RunPolicyParams) (CreateSnapshotResult, error)
	Start() error
	Stop() error
	LoadConfig() error
	SaveConfig(skipLock bool) error
}

// NewSnapshotPolicy creates a new snapshot policy with default values
func NewSnapshotPolicy(params EditPolicyParams) SnapshotPolicy {
	now := time.Now()

	id := params.ID
	if id == "" {
		id = uuid.New().String()
	}

	policy := SnapshotPolicy{
		ID:              id,
		Name:            params.Name,
		Description:     params.Description,
		Dataset:         params.Dataset,
		Schedules:       params.Schedules,
		Recursive:       params.Recursive,
		SnapNamePattern: params.SnapNamePattern,
		RetentionPolicy: params.RetentionPolicy,
		Properties:      params.Properties,
		Enabled:         params.Enabled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if policy.SnapNamePattern == "" {
		policy.SnapNamePattern = fmt.Sprintf("autosnap-%s-%%Y-%%m-%%d-%%H%%M%%S", params.Name)
	}

	return policy
}

// ValidateScheduleSpec validates a schedule specification
func ValidateScheduleSpec(spec ScheduleSpec) error {
	switch spec.Type {
	case ScheduleTypeSecondly, ScheduleTypeMinutely, ScheduleTypeHourly:
		if spec.Interval <= 0 {
			return errors.New(errors.ZFSRequestValidationError, "interval must be greater than 0")
		}
	case ScheduleTypeDaily:
		if spec.AtTime == "" {
			return errors.New(
				errors.ZFSRequestValidationError,
				"at_time must be specified for daily schedules",
			)
		}
	case ScheduleTypeWeekly:
		if spec.AtTime == "" {
			return errors.New(
				errors.ZFSRequestValidationError,
				"at_time must be specified for weekly schedules",
			)
		}
	case ScheduleTypeMonthly:
		if spec.AtTime == "" {
			return errors.New(
				errors.ZFSRequestValidationError,
				"at_time must be specified for monthly schedules",
			)
		}
		if spec.DayOfMonth <= 0 || spec.DayOfMonth > 31 {
			return errors.New(
				errors.ZFSRequestValidationError,
				"day_of_month must be between 1 and 31",
			)
		}
	case ScheduleTypeYearly:
		if spec.AtTime == "" {
			return errors.New(
				errors.ZFSRequestValidationError,
				"at_time must be specified for yearly schedules",
			)
		}
		if spec.DayOfMonth <= 0 || spec.DayOfMonth > 31 {
			return errors.New(
				errors.ZFSRequestValidationError,
				"day_of_month must be between 1 and 31",
			)
		}
	case ScheduleTypeOneTime:
		if spec.StartTime.IsZero() {
			return errors.New(
				errors.ZFSRequestValidationError,
				"start_time must be specified for one-time schedules",
			)
		}
	case ScheduleTypeDuration:
		if spec.Duration <= 0 {
			return errors.New(errors.ZFSRequestValidationError, "duration must be greater than 0")
		}
	case ScheduleTypeRandom:
		if spec.MinDuration <= 0 {
			return errors.New(
				errors.ZFSRequestValidationError,
				"min_duration must be greater than 0",
			)
		}
		if spec.MaxDuration <= 0 {
			return errors.New(
				errors.ZFSRequestValidationError,
				"max_duration must be greater than 0",
			)
		}
		if spec.MinDuration >= spec.MaxDuration {
			return errors.New(
				errors.ZFSRequestValidationError,
				"min_duration must be less than max_duration",
			)
		}
	default:
		return errors.New(errors.ZFSRequestValidationError, "invalid schedule type")
	}

	return nil
}

// ValidatePolicy validates a snapshot policy
func ValidatePolicy(policy SnapshotPolicy) error {
	if policy.Name == "" {
		return errors.New(errors.ZFSRequestValidationError, "name is required")
	}

	if policy.Dataset == "" {
		return errors.New(errors.ZFSRequestValidationError, "dataset is required")
	}

	if len(policy.Schedules) == 0 {
		return errors.New(errors.ZFSRequestValidationError, "at least one schedule is required")
	}

	if len(policy.Schedules) > 5 {
		return errors.New(
			errors.ZFSRequestValidationError,
			"maximum of 5 schedules allowed per policy",
		)
	}

	for i, schedule := range policy.Schedules {
		if err := ValidateScheduleSpec(schedule); err != nil {
			return errors.Wrap(err, errors.ZFSRequestValidationError).
				WithMetadata("schedule_index", fmt.Sprintf("%d", i)).
				WithMetadata("schedule_type", string(schedule.Type))
		}
	}

	return nil
}
