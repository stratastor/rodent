// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package transfers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/snapshot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTransferStatusPersistence_Integration tests that transfer status is correctly persisted to disk
// This test verifies fix: transfer status corrections are saved to disk during loadExistingTransfers
func TestTransferStatusPersistence_Integration(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	// Ensure config directories exist
	if err := config.EnsureDirectories(); err != nil {
		t.Fatalf("Failed to ensure config directories: %v", err)
	}

	cfg := config.GetConfig()
	logCfg := logger.Config{LogLevel: cfg.Server.LogLevel}

	ctx := context.Background()

	t.Logf("\n=== Phase 1: Setup and Run Transfers ===")

	// Create managers for initial setup
	executor := command.NewCommandExecutor(true, logCfg)
	datasetMgr := dataset.NewManager(executor)
	snapshotMgr, err := snapshot.GetManager(datasetMgr, "")
	require.NoError(t, err, "Failed to create snapshot manager")
	transferMgr, err := dataset.NewTransferManager(logCfg)
	require.NoError(t, err, "Failed to create transfer manager")
	policyMgr, err := GetManager(snapshotMgr, transferMgr, logCfg)
	require.NoError(t, err, "Failed to create transfer policy manager")

	// Create a snapshot policy that runs every 1 minute
	snapPolicyParams := snapshot.EditPolicyParams{
		Name:        "test-snap-persistence",
		Description: "Test snapshot policy for status persistence",
		Dataset:     "tiny1/split",
		Schedules: []snapshot.ScheduleSpec{
			{
				Type:     snapshot.ScheduleTypeMinutely,
				Interval: 1,
				Enabled:  true,
			},
		},
		RetentionPolicy: snapshot.RetentionPolicy{
			Count: 10,
		},
		Enabled: true,
	}

	snapPolicyID, err := snapshotMgr.AddPolicy(snapPolicyParams)
	require.NoError(t, err, "Failed to create snapshot policy")
	t.Logf("Created snapshot policy: %s (runs every 1 minute)", snapPolicyID)

	// Cleanup snapshot policy after test
	defer func() {
		_ = snapshotMgr.RemovePolicy(snapPolicyID, true)
	}()

	// Create a transfer policy with a NEW target filesystem
	transferParams := EditTransferPolicyParams{
		Name:             "test-transfer-persistence",
		Description:      "Test transfer policy for status persistence",
		SnapshotPolicyID: snapPolicyID,
		TransferConfig: dataset.TransferConfig{
			SendConfig: dataset.SendConfig{
				Replicate:    true,
				SkipMissing:  true,
				Properties:   true,
				LargeBlocks:  true,
				Intermediary: true,
				Compressed:   true,
				Verbose:      true,
			},
			ReceiveConfig: dataset.ReceiveConfig{
				Target:    "tank/test-status-persistence",
				Force:     true,
				Resumable: false, // Non-resumable to test TransferStatusUnknown
				Verbose:   true,
			},
			LogConfig: &dataset.TransferLogConfig{
				MaxSizeBytes:     10240,
				TruncateOnFinish: true,
				RetainOnFailure:  true,
				HeaderLines:      20,
				FooterLines:      20,
			},
		},
		Schedules: []snapshot.ScheduleSpec{
			{
				Type:     snapshot.ScheduleTypeMinutely,
				Interval: 2,
				Enabled:  true,
			},
		},
		RetentionPolicy: TransferRetentionPolicy{
			KeepCount: 5,
		},
		Enabled: true,
	}

	policyID, err := policyMgr.AddPolicy(ctx, transferParams)
	require.NoError(t, err, "Failed to create transfer policy")
	t.Logf("Created transfer policy: %s (runs every 2 minutes)", policyID)

	// Cleanup transfer policy after test
	defer func() {
		// Re-create managers for cleanup if needed
		cleanupMgr, err := GetManager(snapshotMgr, transferMgr, logCfg)
		if err == nil {
			_ = cleanupMgr.RemovePolicy(ctx, policyID, true)
		}
	}()

	// Start both managers
	err = snapshotMgr.Start()
	require.NoError(t, err, "Failed to start snapshot manager")

	err = policyMgr.Start()
	require.NoError(t, err, "Failed to start transfer policy manager")

	t.Logf("Both managers started - snapshot policy: every 1 min, transfer policy: every 2 min")
	t.Logf("Running for 3 minutes to initiate transfers...")

	// Wait 130 seconds for first transfer to start
	time.Sleep(130 * time.Second)

	// Check status after first transfer
	transferPolicy, err := policyMgr.GetPolicy(policyID)
	require.NoError(t, err, "Failed to get transfer policy")

	var lastTransferID string
	if transferPolicy.LastTransferID != "" {
		lastTransferID = transferPolicy.LastTransferID
		t.Logf("Transfer initiated: %s", lastTransferID)

		transferInfo, err := transferMgr.GetTransfer(lastTransferID)
		if err != nil {
			t.Logf("Failed to get transfer info: %v", err)
		} else {
			t.Logf("Transfer status before stop: %s", transferInfo.Status)
			if transferInfo.PID > 0 {
				t.Logf("Transfer PID: %d", transferInfo.PID)
			}
		}
	} else {
		t.Logf("No transfer initiated yet, waiting more...")
		time.Sleep(60 * time.Second)

		transferPolicy, err = policyMgr.GetPolicy(policyID)
		require.NoError(t, err)
		lastTransferID = transferPolicy.LastTransferID
	}

	require.NotEmpty(t, lastTransferID, "Should have at least one transfer initiated")

	t.Logf("\n=== Phase 2: Stop Managers (Simulate Restart) ===")

	// Stop managers and shutdown active transfers
	policyMgr.Stop()
	snapshotMgr.Stop()
	_ = transferMgr.Shutdown(10 * time.Second)

	t.Logf("Managers stopped and transfers shut down - simulating service restart")
	time.Sleep(2 * time.Second)

	t.Logf("\n=== Phase 3: Reload and Verify Status Persistence ===")

	// Create NEW transfer manager (simulating restart)
	// This will trigger loadExistingTransfers() which reads state from disk
	newTransferMgr, err := dataset.NewTransferManager(logCfg)
	require.NoError(t, err, "Failed to create new transfer manager")

	t.Logf("New transfer manager created - this loaded existing transfers from disk")

	// Get transfer via GetTransfer (loads from disk)
	transferViaGetTransfer, err := newTransferMgr.GetTransfer(lastTransferID)
	require.NoError(t, err, "Should be able to get transfer via GetTransfer")

	t.Logf("Transfer status via GetTransfer: %s", transferViaGetTransfer.Status)

	// Get transfer via ListTransfers (uses in-memory activeTransfers map)
	allTransfers := newTransferMgr.ListTransfersByType(dataset.TransferTypeAll)
	var transferViaListAll *dataset.TransferInfo
	for _, tr := range allTransfers {
		if tr.ID == lastTransferID {
			transferViaListAll = tr
			break
		}
	}

	// Get active transfers
	activeTransfers := newTransferMgr.ListTransfers()
	var transferViaListActive *dataset.TransferInfo
	for _, tr := range activeTransfers {
		if tr.ID == lastTransferID {
			transferViaListActive = tr
			break
		}
	}

	t.Logf("\n=== Verification Results ===")
	t.Logf("Transfer ID: %s", lastTransferID)
	t.Logf("Status via GetTransfer (from disk): %s", transferViaGetTransfer.Status)

	if transferViaListAll != nil {
		t.Logf("Status via ListTransfersByType (all): %s", transferViaListAll.Status)
	} else {
		t.Logf("Transfer NOT found in ListTransfersByType(all)")
	}

	if transferViaListActive != nil {
		t.Logf("Status via ListTransfers (active): %s", transferViaListActive.Status)
		t.Logf("WARNING: Transfer still appears in active list - might still be running")
	} else {
		t.Logf("Transfer NOT in active list (expected if process died)")
	}

	// The key verification: status should be consistent
	// If process is dead and transfer is non-resumable, both should show Unknown or Paused
	if transferViaGetTransfer.Status == dataset.TransferStatusRunning {
		// If GetTransfer shows Running but process is dead, that would be the bug
		if transferViaListActive == nil {
			t.Errorf("BUG: GetTransfer shows Running but transfer not in active list")
			t.Errorf("This means status was not persisted to disk during loadExistingTransfers")
		}
	}

	// Verify status is not "running" if it's not actually running
	if transferViaListActive == nil &&
		transferViaGetTransfer.Status == dataset.TransferStatusRunning {
		assert.Fail(t, "Transfer shows as Running in GetTransfer but not in active list",
			"This indicates bug: status corrections not persisted to disk")
	} else {
		t.Logf("Status consistency verified: GetTransfer and ListTransfers are consistent")
	}

	// Additional checks
	t.Logf("\n=== Additional Checks ===")

	// Check if status is one of the expected values after restart
	validStatuses := []dataset.TransferStatus{
		dataset.TransferStatusCompleted,
		dataset.TransferStatusFailed,
		dataset.TransferStatusUnknown,
		dataset.TransferStatusPaused,
		dataset.TransferStatusRunning, // Only valid if actually in active list
	}

	statusValid := false
	for _, validStatus := range validStatuses {
		if transferViaGetTransfer.Status == validStatus {
			statusValid = true
			break
		}
	}
	assert.True(t, statusValid, "Transfer status should be one of the valid statuses")

	t.Logf("Transfer final status: %s", transferViaGetTransfer.Status)
	t.Logf("Test completed - bug (status persistence) verification done")
}
