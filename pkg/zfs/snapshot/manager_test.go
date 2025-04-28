// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/zfs/command"
	ds "github.com/stratastor/rodent/pkg/zfs/dataset"
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
	testFS := os.Getenv("RODENT_TEST_FS_NAME")
	if testFS == "" {
		t.Skip("Skipping integration test - RODENT_TEST_FS environment variable not set")
	}
	t.Logf("Using test filesystem: %s", testFS)

	// Create a temp dir for config
	tempDir, err := os.MkdirTemp("", "snapshot-test-")
	require.NoError(t, err)

	// Only clean up if the test succeeds
	defer func() {
		if !t.Failed() {
			os.RemoveAll(tempDir)
		} else {
			t.Logf("Test failed. Preserving test directory for inspection: %s", tempDir)
		}
	}()

	// Create a real dataset manager
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "debug"})
	dsManager := ds.NewManager(executor)

	// Create a test config file path and make sure the directory exists
	testConfigDir := filepath.Join(tempDir, "config")
	err = os.MkdirAll(testConfigDir, 0755)
	require.NoError(t, err, "Failed to create test config directory")

	// Test constructor
	manager, err := NewManager(dsManager, testConfigDir)
	require.NoError(t, err)
	t.Logf("Created manager: %+v", manager)
	t.Logf("Using config path: %s", manager.configPath)

	// Write an empty config file to ensure it exists
	err = os.WriteFile(manager.configPath, []byte("# Test config file\n"), 0644)
	require.NoError(t, err, "Failed to write initial test config file")

	// Set a shorter test timeout
	testTimeout := 30 * time.Second
	t.Logf("Using test timeout of %s", testTimeout)

	// Function to check if a file exists with timeout
	fileExistsWithTimeout := func(path string, timeout time.Duration) bool {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			_, err := os.Stat(path)
			if err == nil {
				return true
			}
			time.Sleep(100 * time.Millisecond)
		}
		return false
	}

	// Function to count snapshots for a dataset
	countSnapshots := func(dataset string) (int, error) {
		ctx := context.Background()
		listCfg := ds.ListConfig{
			Name: dataset,
			Type: "snapshot",
		}

		result, err := dsManager.List(ctx, listCfg)
		if err != nil {
			return 0, err
		}

		count := 0
		for name := range result.Datasets {
			if strings.HasPrefix(name, dataset+"@") {
				count++
			}
		}

		return count, nil
	}

	// Function to check if snapshots exist recursively
	checkRecursiveSnapshots := func(dataset, snapName string) (bool, error) {
		ctx := context.Background()
		// List all snapshots recursively
		listCfg := ds.ListConfig{
			Name:      dataset,
			Type:      "all",
			Recursive: true,
		}

		result, err := dsManager.List(ctx, listCfg)
		if err != nil {
			return false, err
		}

		parentSnapshotFound := false
		childrenSnapshotsFound := false

		// Check main dataset snapshot
		for name := range result.Datasets {
			if name == dataset+"@"+snapName {
				parentSnapshotFound = true
			} else if strings.Contains(name, "@"+snapName) && name != dataset+"@"+snapName {
				// Check child dataset snapshots with same name
				childrenSnapshotsFound = true
			}
		}

		return parentSnapshotFound && childrenSnapshotsFound, nil
	}

	// Generate a unique suffix for this test run to avoid snapshot name conflicts
	testUniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano()%10000)

	// Test adding a policy with a secondly schedule for faster testing
	params := EditPolicyParams{
		Name:        "test-policy-" + testUniqueSuffix,
		Description: "Test description",
		Dataset:     testFS,
		Schedules: []ScheduleSpec{
			{
				Type:     ScheduleTypeSecondly, // Use secondly for faster testing
				Interval: 3,                    // Run every 3 seconds for faster testing
				Enabled:  true,
			},
		},
		Recursive: true,
		RetentionPolicy: RetentionPolicy{
			Count: 2, // Keep only 2 snapshots (to verify pruning works)
		},
		Properties: map[string]string{
			"custom:testing": "autosnap-" + testUniqueSuffix,
		},
		Enabled: true,
	}

	t.Log("Adding policy...")
	policyID, err := manager.AddPolicy(params)
	require.NoError(t, err, "Failed to add policy")
	assert.NotEmpty(t, policyID, "Policy ID should not be empty")

	// Wait for config file to be written
	t.Log("Waiting for config file to be updated...")
	configWritten := fileExistsWithTimeout(manager.configPath, testTimeout)
	require.True(t, configWritten, "Config file should be written after adding policy")

	// Check config file content
	configData, err := os.ReadFile(manager.configPath)
	require.NoError(t, err, "Failed to read config file")
	t.Logf("Config file content size: %d bytes", len(configData))
	if len(configData) == 0 {
		t.Error("Config file is empty")
	}

	// Test getting the policy
	policy, err := manager.GetPolicy(policyID)
	require.NoError(t, err)
	assert.Equal(t, params.Name, policy.Name)

	// Test listing policies
	policies, err := manager.ListPolicies()
	require.NoError(t, err)
	assert.Len(t, policies, 1)

	// Function to count only snapshots from our test
	countTestSnapshots := func() (int, error) {
		ctx := context.Background()
		listCfg := ds.ListConfig{
			Name: testFS,
			Type: "snapshot",
		}

		result, err := dsManager.List(ctx, listCfg)
		if err != nil {
			return 0, err
		}

		count := 0
		expectedPrefix := "autosnap-" + params.Name
		for name := range result.Datasets {
			// Only count snapshots from our specific test run
			if strings.HasPrefix(name, testFS+"@") && strings.Contains(name, expectedPrefix) {
				count++
			}
		}

		return count, nil
	}

	// Get initial snapshot count for our test (should be 0)
	initialSnapCount, err := countTestSnapshots()
	require.NoError(t, err)
	t.Logf("Initial test snapshot count: %d", initialSnapCount)

	// Get total snapshot count for information
	totalSnapCount, err := countSnapshots(testFS)
	require.NoError(t, err)
	t.Logf("Initial total snapshot count: %d", totalSnapCount)

	// Start the scheduler to allow auto-snapshots to be created
	t.Log("Starting scheduler...")
	err = manager.Start()
	require.NoError(t, err)

	// Wait for at least 2 snapshot cycles (with 3-second interval)
	t.Log("Waiting for snapshots to be created automatically...")
	time.Sleep(10 * time.Second)

	// Check if snapshots were created from our test run
	testSnapCount, err := countTestSnapshots()
	require.NoError(t, err)
	t.Logf("Test snapshot count after auto-snapshots: %d", testSnapCount)

	// We should have at least 1 new snapshot from our test
	assert.Greater(t, testSnapCount, 0,
		"Expected at least one snapshot to be created by our test policy")

	// Get a list of snapshots to check if they're recursive
	ctx := context.Background()
	listCfg := ds.ListConfig{
		Name: testFS,
		Type: "snapshot",
	}

	snapResult, err := dsManager.List(ctx, listCfg)
	require.NoError(t, err)

	// Find our latest test snapshot
	var latestSnapName string
	expectedPrefix := "autosnap-" + params.Name
	for name := range snapResult.Datasets {
		if strings.HasPrefix(name, testFS+"@") && strings.Contains(name, expectedPrefix) {
			parts := strings.Split(name, "@")
			if len(parts) == 2 && parts[1] > latestSnapName {
				latestSnapName = parts[1]
			}
		}
	}

	// Check if snapshot was created recursively
	if latestSnapName != "" {
		t.Logf("Found test snapshot: %s", latestSnapName)
		hasRecursiveSnaps, err := checkRecursiveSnapshots(testFS, latestSnapName)
		if err != nil {
			t.Logf("Error checking recursive snapshots: %v", err)
		} else {
			// We might not have child datasets in test environment, so don't fail the test if not found
			if !hasRecursiveSnaps {
				t.Logf("Note: Recursive snapshots not found. This could be because the test dataset doesn't have children.")
			} else {
				t.Log("Confirmed: Recursive snapshots created successfully")
			}
		}
	} else {
		t.Log("No snapshots found matching our test policy pattern")
	}

	// Since we set retention count to 2, we should have at most 2 snapshots after enough time
	// for the retention policy to apply
	t.Log("Checking snapshot retention policy...")

	// Wait a bit longer for more snapshots and pruning to occur
	time.Sleep(10 * time.Second)

	// Count snapshots with our specific test policy pattern
	snapResult, err = dsManager.List(ctx, listCfg)
	require.NoError(t, err)

	policySnapCount, err := countTestSnapshots()
	require.NoError(t, err)

	t.Logf("Snapshots from our test policy: %d", policySnapCount)
	assert.LessOrEqual(t, policySnapCount, 2,
		"Expected retention policy to keep at most 2 snapshots")

	// Now stop the scheduler and check that no more snapshots are created
	t.Log("Stopping the scheduler...")
	err = manager.Stop()
	require.NoError(t, err)

	finalSnapCount := policySnapCount

	// Sleep for 6 seconds (longer than our snapshot interval) to see if more snapshots would be created
	t.Log("Waiting to confirm no more snapshots are created...")
	time.Sleep(6 * time.Second)

	// Check snapshot count again
	snapResult, err = dsManager.List(ctx, listCfg)
	require.NoError(t, err)

	currentSnapCount := 0
	expectedPrefix = "autosnap-" + params.Name
	for name := range snapResult.Datasets {
		if strings.HasPrefix(name, testFS+"@") && strings.Contains(name, expectedPrefix) {
			currentSnapCount++
		}
	}

	t.Logf("Snapshot count after stopping scheduler: %d", currentSnapCount)

	// Note: Sometimes the scheduler might have one more job in progress when we stop it
	// so we should make sure no NEW snapshots are being created
	assert.LessOrEqual(t, currentSnapCount, finalSnapCount+1,
		"No significant increase in snapshots should occur after stopping the scheduler")

	// Test removing the policy
	t.Log("Removing policy")
	err = manager.RemovePolicy(policyID, false)
	require.NoError(t, err)

	// Test that the policy was removed
	t.Log("Verifying policy was removed")
	_, err = manager.GetPolicy(policyID)
	assert.Error(t, err)

	// Verify config file was updated properly
	t.Log("Verifying config file exists")
	fileInfo, err := os.Stat(manager.configPath)
	if err != nil {
		t.Logf("Error checking config file: %v", err)
	} else {
		t.Logf("Config file exists: %s, size: %d bytes", manager.configPath, fileInfo.Size())
	}

	t.Log("Integration test completed successfully")
}
