// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateScheduleSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    ScheduleSpec
		wantErr bool
	}{
		{
			name: "valid hourly",
			spec: ScheduleSpec{
				Type:     ScheduleTypeHourly,
				Interval: 2,
				Enabled:  true,
			},
			wantErr: false,
		},
		{
			name: "invalid hourly - zero interval",
			spec: ScheduleSpec{
				Type:     ScheduleTypeHourly,
				Interval: 0,
				Enabled:  true,
			},
			wantErr: true,
		},
		{
			name: "valid daily",
			spec: ScheduleSpec{
				Type:     ScheduleTypeDaily,
				Interval: 1,
				AtTime:   "12:00",
				Enabled:  true,
			},
			wantErr: false,
		},
		{
			name: "invalid daily - missing time",
			spec: ScheduleSpec{
				Type:     ScheduleTypeDaily,
				Interval: 1,
				Enabled:  true,
			},
			wantErr: true,
		},
		{
			name: "valid weekly",
			spec: ScheduleSpec{
				Type:     ScheduleTypeWeekly,
				Interval: 1,
				WeekDay:  time.Monday,
				AtTime:   "12:00",
				Enabled:  true,
			},
			wantErr: false,
		},
		{
			name: "valid monthly",
			spec: ScheduleSpec{
				Type:       ScheduleTypeMonthly,
				Interval:   1,
				DayOfMonth: 15,
				AtTime:     "12:00",
				Enabled:    true,
			},
			wantErr: false,
		},
		{
			name: "invalid monthly - day out of range",
			spec: ScheduleSpec{
				Type:       ScheduleTypeMonthly,
				Interval:   1,
				DayOfMonth: 32,
				AtTime:     "12:00",
				Enabled:    true,
			},
			wantErr: true,
		},
		{
			name: "valid duration",
			spec: ScheduleSpec{
				Type:     ScheduleTypeDuration,
				Duration: 1 * time.Hour,
				Enabled:  true,
			},
			wantErr: false,
		},
		{
			name: "invalid duration - zero duration",
			spec: ScheduleSpec{
				Type:     ScheduleTypeDuration,
				Duration: 0,
				Enabled:  true,
			},
			wantErr: true,
		},
		{
			name: "valid random",
			spec: ScheduleSpec{
				Type:        ScheduleTypeRandom,
				MinDuration: 1 * time.Hour,
				MaxDuration: 2 * time.Hour,
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "invalid random - min >= max",
			spec: ScheduleSpec{
				Type:        ScheduleTypeRandom,
				MinDuration: 2 * time.Hour,
				MaxDuration: 1 * time.Hour,
				Enabled:     true,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			spec: ScheduleSpec{
				Type:    "invalid",
				Enabled: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScheduleSpec(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  SnapshotPolicy
		wantErr bool
	}{
		{
			name: "valid policy",
			policy: SnapshotPolicy{
				ID:      "test-id",
				Name:    "test-policy",
				Dataset: "tank/data",
				Schedules: []ScheduleSpec{
					{
						Type:     ScheduleTypeHourly,
						Interval: 1,
						Enabled:  true,
					},
				},
				Recursive:       true,
				SnapNamePattern: "auto-%Y-%m-%d",
				Enabled:         true,
			},
			wantErr: false,
		},
		{
			name: "invalid - missing name",
			policy: SnapshotPolicy{
				ID:      "test-id",
				Dataset: "tank/data",
				Schedules: []ScheduleSpec{
					{
						Type:     ScheduleTypeHourly,
						Interval: 1,
						Enabled:  true,
					},
				},
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "invalid - missing dataset",
			policy: SnapshotPolicy{
				ID:   "test-id",
				Name: "test-policy",
				Schedules: []ScheduleSpec{
					{
						Type:     ScheduleTypeHourly,
						Interval: 1,
						Enabled:  true,
					},
				},
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "invalid - no schedules",
			policy: SnapshotPolicy{
				ID:      "test-id",
				Name:    "test-policy",
				Dataset: "tank/data",
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "invalid - too many schedules",
			policy: SnapshotPolicy{
				ID:      "test-id",
				Name:    "test-policy",
				Dataset: "tank/data",
				Schedules: []ScheduleSpec{
					{Type: ScheduleTypeHourly, Interval: 1, Enabled: true},
					{Type: ScheduleTypeDaily, Interval: 1, AtTime: "12:00", Enabled: true},
					{
						Type:     ScheduleTypeWeekly,
						Interval: 1,
						WeekDay:  time.Monday,
						AtTime:   "12:00",
						Enabled:  true,
					},
					{
						Type:       ScheduleTypeMonthly,
						Interval:   1,
						DayOfMonth: 15,
						AtTime:     "12:00",
						Enabled:    true,
					},
					{
						Type:       ScheduleTypeYearly,
						Interval:   1,
						Month:      time.January,
						DayOfMonth: 1,
						AtTime:     "12:00",
						Enabled:    true,
					},
					{Type: ScheduleTypeDuration, Duration: 1 * time.Hour, Enabled: true},
				},
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "invalid - invalid schedule",
			policy: SnapshotPolicy{
				ID:      "test-id",
				Name:    "test-policy",
				Dataset: "tank/data",
				Schedules: []ScheduleSpec{
					{
						Type:     ScheduleTypeHourly,
						Interval: 0, // Invalid
						Enabled:  true,
					},
				},
				Enabled: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePolicy(tt.policy)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewSnapshotPolicy(t *testing.T) {
	params := EditPolicyParams{
		Name:        "test-policy",
		Description: "Test description",
		Dataset:     "tank/data",
		Schedules: []ScheduleSpec{
			{
				Type:     ScheduleTypeHourly,
				Interval: 1,
				Enabled:  true,
			},
		},
		Recursive: true,
		RetentionPolicy: RetentionPolicy{
			Count: 10,
		},
		Properties: map[string]string{
			"compression": "lz4",
		},
		Enabled: true,
	}

	policy := NewSnapshotPolicy(params)

	assert.NotEmpty(t, policy.ID)
	assert.Equal(t, params.Name, policy.Name)
	assert.Equal(t, params.Description, policy.Description)
	assert.Equal(t, params.Dataset, policy.Dataset)
	assert.Equal(t, params.Schedules, policy.Schedules)
	assert.Equal(t, params.Recursive, policy.Recursive)
	assert.Equal(t, params.RetentionPolicy, policy.RetentionPolicy)
	assert.Equal(t, params.Properties, policy.Properties)
	assert.Equal(t, params.Enabled, policy.Enabled)
	assert.False(t, policy.CreatedAt.IsZero())
	assert.False(t, policy.UpdatedAt.IsZero())

	// Test with empty name pattern
	params.SnapNamePattern = ""
	policy = NewSnapshotPolicy(params)
	assert.Equal(t, "autosnap-test-policy-%Y-%m-%d-%H%M%S", policy.SnapNamePattern)

	// Test with custom name pattern
	params.SnapNamePattern = "custom-%Y%m%d"
	policy = NewSnapshotPolicy(params)
	assert.Equal(t, "custom-%Y%m%d", policy.SnapNamePattern)

	// Test with existing ID
	params.ID = "existing-id"
	policy = NewSnapshotPolicy(params)
	assert.Equal(t, "existing-id", policy.ID)
}

// TestExpandSnapNamePattern tests the pattern expansion for snapshot names
func TestExpandSnapNamePattern(t *testing.T) {
	// Mock fixed time for testing
	fixedTime := time.Date(2025, 5, 15, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{
			name:     "basic pattern",
			pattern:  "auto-%Y-%m-%d",
			expected: "auto-2025-05-15",
		},
		{
			name:     "full pattern",
			pattern:  "auto-%Y-%m-%d-%H%M%S",
			expected: "auto-2025-05-15-143045",
		},
		{
			name:     "custom pattern",
			pattern:  "backup_pool_data_%Y%m%d_%H%M",
			expected: "backup_pool_data_20250515_1430",
		},
		{
			name:     "no pattern",
			pattern:  "snapshot",
			expected: "snapshot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandSnapNamePattern(tt.pattern, fixedTime)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Basic integration test that requires a real ZFS dataset
// This test will be skipped if no test filesystem is provided
func TestManager_Integration(t *testing.T) {
	// Get test filesystem from environment
	testFS := os.Getenv("RODENT_TEST_FS")
	if testFS == "" {
		t.Skip("Skipping integration test - RODENT_TEST_FS environment variable not set")
	}
	t.Logf("Using test filesystem: %s", testFS)

	// Create a temp dir for config
	tempDir, err := os.MkdirTemp("", "snapshot-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a real dataset manager
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "debug"})
	dsManager := dataset.NewManager(executor)

	// Test constructor
	manager, err := NewManager(dsManager)
	require.NoError(t, err)
	t.Logf("Created manager: %+v", manager)

	// Override config path for testing
	// manager.configPath = filepath.Join(tempDir, "test-config.yml")
	manager.setConfigPath(filepath.Join(tempDir, "test-config.yml"))
	t.Logf("Using config path: %s", manager.configPath)

	// Test adding a policy
	params := EditPolicyParams{
		Name:        "test-policy",
		Description: "Test description",
		Dataset:     testFS,
		Schedules: []ScheduleSpec{
			{
				Type:     ScheduleTypeHourly,
				Interval: 1,
				Enabled:  true,
			},
		},
		Recursive: true,
		RetentionPolicy: RetentionPolicy{
			Count: 10,
		},
		Properties: map[string]string{
			"compression": "lz4",
		},
		Enabled: true,
	}

	policyID, err := manager.AddPolicy(params)
	require.NoError(t, err)
	assert.NotEmpty(t, policyID)

	// Test getting the policy
	policy, err := manager.GetPolicy(policyID)
	require.NoError(t, err)
	assert.Equal(t, params.Name, policy.Name)

	// Test listing policies
	policies, err := manager.ListPolicies()
	require.NoError(t, err)
	assert.Len(t, policies, 1)

	// Test updating the policy
	updateParams := EditPolicyParams{
		ID:          policyID,
		Name:        "updated-policy",
		Description: "Updated description",
		Dataset:     testFS,
		Schedules: []ScheduleSpec{
			{
				Type:     ScheduleTypeHourly,
				Interval: 2,
				Enabled:  true,
			},
		},
		Recursive: true,
		RetentionPolicy: RetentionPolicy{
			Count: 20,
		},
		Properties: map[string]string{
			"compression": "lz4",
		},
		Enabled: true,
	}

	err = manager.UpdatePolicy(updateParams)
	require.NoError(t, err)

	// Test getting the updated policy
	updatedPolicy, err := manager.GetPolicy(policyID)
	require.NoError(t, err)
	assert.Equal(t, updateParams.Name, updatedPolicy.Name)
	assert.Equal(t, updateParams.RetentionPolicy.Count, updatedPolicy.RetentionPolicy.Count)

	// Only run the snapshot test if RUN_SNAPSHOT_TEST is set
	if runTest := os.Getenv("RUN_SNAPSHOT_TEST"); runTest != "" && runTest != "0" &&
		runTest != "false" {
		t.Logf("Running snapshot test for policy ID: %s", policyID)
		// Test running the policy
		runParams := RunPolicyParams{
			ID:            policyID,
			ScheduleIndex: 0,
		}

		result, err := manager.RunPolicy(runParams)
		require.NoError(t, err)
		assert.Equal(t, policyID, result.PolicyID)
		assert.Equal(t, testFS, result.DatasetName)
		assert.NotEmpty(t, result.SnapshotName)
	}

	// Test removing the policy
	err = manager.RemovePolicy(policyID)
	require.NoError(t, err)

	// Test that the policy was removed
	_, err = manager.GetPolicy(policyID)
	assert.Error(t, err)
}
