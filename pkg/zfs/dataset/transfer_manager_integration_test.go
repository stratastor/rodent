// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2024 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/zfs/command"
)

// Test configuration from environment variables
type TestConfig struct {
	TargetUsername   string // RODENT_TEST_TARGET_USERNAME
	TargetIP         string // RODENT_TEST_TARGET_IP
	TargetFilesystem string // RODENT_TEST_TARGET_FILESYSTEM
	SSHKeyPath       string // RODENT_TEST_SSH_KEY_PATH
	SourceFilesystem string // RODENT_TEST_SOURCE_FILESYSTEM
}

func getTestConfig(t *testing.T) TestConfig {
	config := TestConfig{
		TargetUsername:   getEnvOrDefault("RODENT_TEST_TARGET_USERNAME", "rodent"),
		TargetIP:         getEnvOrDefault("RODENT_TEST_TARGET_IP", "172.31.14.189"),
		TargetFilesystem: getEnvOrDefault("RODENT_TEST_TARGET_FILESYSTEM", "store/newFS"),
		SSHKeyPath: getEnvOrDefault(
			"RODENT_TEST_SSH_KEY_PATH",
			"/home/rodent/.rodent/ssh/01978d99-b37f-7032-8f59-94d42795652f/id_ed25519",
		),
		SourceFilesystem: getEnvOrDefault("RODENT_TEST_SOURCE_FILESYSTEM", "tank/standardFS"),
	}

	// Validate required configuration
	if config.TargetIP == "" {
		t.Skip("RODENT_TEST_TARGET_IP not set, skipping integration tests")
	}
	if config.SSHKeyPath == "" {
		t.Skip("RODENT_TEST_SSH_KEY_PATH not set, skipping integration tests")
	}

	t.Logf(
		"Integration test config: Target=%s@%s, Source=%s, Target=%s, SSH=%s",
		config.TargetUsername,
		config.TargetIP,
		config.SourceFilesystem,
		config.TargetFilesystem,
		config.SSHKeyPath,
	)

	return config
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func setupTransferManager(t *testing.T) (*TransferManager, *Manager) {
	// Ensure config directories exist
	if err := config.EnsureDirectories(); err != nil {
		t.Fatalf("Failed to ensure config directories: %v", err)
	}

	// Create logger
	logCfg := logger.Config{LogLevel: "debug"}

	// Create command executor with sudo
	executor := command.NewCommandExecutor(true, logCfg)

	// Create dataset manager
	datasetManager := NewManager(executor)

	// Create transfer manager
	transferManager, err := NewTransferManager(logCfg)
	if err != nil {
		t.Fatalf("Failed to create transfer manager: %v", err)
	}

	return transferManager, datasetManager
}

func createTestSnapshot(t *testing.T, manager *Manager, filesystem string) string {
	ctx := context.Background()
	timestamp := time.Now().Format("20060102-150405")
	snapshotName := fmt.Sprintf("test-transfer-%s", timestamp)

	snapConfig := SnapshotConfig{
		NameConfig: NameConfig{
			Name: filesystem,
		},
		SnapName: snapshotName,
	}

	err := manager.CreateSnapshot(ctx, snapConfig)
	if err != nil {
		t.Fatalf("Failed to create test snapshot %s: %v", snapshotName, err)
	}

	t.Logf("Created test snapshot: %s", snapshotName)
	return fmt.Sprintf("%s@%s", filesystem, snapshotName)
}

func cleanupSnapshot(t *testing.T, manager *Manager, snapshotName string) {
	ctx := context.Background()
	destroyConfig := DestroyConfig{
		NameConfig: NameConfig{
			Name: snapshotName,
		},
	}

	_, err := manager.Destroy(ctx, destroyConfig)
	if err != nil {
		t.Logf("Warning: Failed to cleanup snapshot %s: %v", snapshotName, err)
	} else {
		t.Logf("Cleaned up snapshot: %s", snapshotName)
	}
}

func verifyRemoteFilesystem(t *testing.T, config TestConfig, targetPath string) {
	// Verify the filesystem exists on the remote target
	checkCmd := fmt.Sprintf("ssh -i %s %s@%s 'zfs list %s'",
		config.SSHKeyPath, config.TargetUsername, config.TargetIP, targetPath)

	t.Logf("Verifying remote filesystem exists: %s", targetPath)

	// Execute the verification command
	if err := exec.Command("bash", "-c", checkCmd).Run(); err != nil {
		t.Errorf("Remote filesystem %s was not created successfully: %v", targetPath, err)
	} else {
		t.Logf("✅ Confirmed: Remote filesystem %s exists", targetPath)
	}
}

func cleanupRemoteFilesystem(t *testing.T, config TestConfig, targetPath string) {
	// Clean up the transferred filesystem on the remote target
	cleanupCmd := fmt.Sprintf("ssh -i %s %s@%s 'sudo zfs destroy -r %s'",
		config.SSHKeyPath, config.TargetUsername, config.TargetIP, targetPath)

	t.Logf("Cleaning up remote filesystem: %s", targetPath)

	// Execute the cleanup command
	if err := exec.Command("bash", "-c", cleanupCmd).Run(); err != nil {
		t.Logf("Warning: Failed to cleanup remote filesystem %s: %v", targetPath, err)
	} else {
		t.Logf("✅ Cleaned up remote filesystem: %s", targetPath)
	}
}

func TestTransferManager_StartTransfer_Basic(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	// Create a test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	ctx := context.Background()

	// Configure the transfer
	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: snapshotName,
			Verbose:  true,
			Parsable: false,
		},
		ReceiveConfig: ReceiveConfig{
			Target: testConfig.TargetFilesystem + "/test-basic" + time.Now().
				Format("20060102-150405"),
			Resumable: true,
			Force:     true,
			Verbose:   true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	// Start the transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	t.Logf("Started transfer with ID: %s", transferID)

	// Monitor transfer progress
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Transfer timed out after 5 minutes")

		case <-ticker.C:
			transfer, err := transferManager.GetTransfer(transferID)
			if err != nil {
				t.Fatalf("Failed to get transfer status: %v", err)
			}

			t.Logf("Transfer %s status: %s, elapsed: %ds",
				transferID, transfer.Status, transfer.Progress.ElapsedTime)

			switch transfer.Status {
			case TransferStatusCompleted:
				t.Logf("Transfer completed successfully!")

				// Verify the filesystem was created on the remote target
				verifyRemoteFilesystem(t, testConfig, transferConfig.ReceiveConfig.Target)

				// Clean up the remote filesystem
				defer cleanupRemoteFilesystem(t, testConfig, transferConfig.ReceiveConfig.Target)

				return
			case TransferStatusFailed:
				t.Fatalf("Transfer failed: %s", transfer.ErrorMessage)
			case TransferStatusCancelled:
				t.Fatalf("Transfer was cancelled: %s", transfer.ErrorMessage)
			}
		}
	}
}

func TestTransferManager_PauseResumeTransfer(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	// Create a test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	ctx := context.Background()

	// Configure the transfer with a large dataset to allow time for pause/resume
	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: snapshotName,
			Verbose:  true,
			Parsable: false,
		},
		ReceiveConfig: ReceiveConfig{
			Target: testConfig.TargetFilesystem + "/test-pause-resume" + time.Now().
				Format("20060102-150405"),
			Resumable: true, // Essential for pause/resume
			Force:     true,
			Verbose:   true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	// Start the transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}
	// Clean up the remote filesystem
	defer cleanupRemoteFilesystem(t, testConfig, transferConfig.ReceiveConfig.Target)

	t.Logf("Started transfer with ID: %s", transferID)

	// Wait a bit for transfer to start
	time.Sleep(2 * time.Second)
	t.Logf("Slept for 2 seconds, now pausing transfer %s", transferID)

	// Pause the transfer
	err = transferManager.PauseTransfer(transferID)
	if err != nil {
		t.Fatalf("Failed to pause transfer: %v", err)
	}

	t.Logf("Paused transfer %s", transferID)

	// Verify transfer is paused
	transfer, err := transferManager.GetTransfer(transferID)
	if err != nil {
		t.Fatalf("Failed to get transfer status: %v", err)
	}

	if transfer.Status != TransferStatusPaused {
		t.Fatalf("Expected transfer status to be paused, got: %s", transfer.Status)
	}

	t.Logf("Transfer successfully paused")

	// Wait a moment
	time.Sleep(10 * time.Second)
	t.Logf("Slept for 10 seconds, now resuming transfer %s", transferID)

	// Resume the transfer
	err = transferManager.ResumeTransfer(ctx, transferID)
	if err != nil {
		t.Fatalf("Failed to resume transfer: %v", err)
	}

	t.Logf("Resumed transfer %s", transferID)

	// Monitor until completion
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Transfer timed out after 5 minutes")

		case <-ticker.C:
			transfer, err := transferManager.GetTransfer(transferID)
			if err != nil {
				t.Fatalf("Failed to get transfer status: %v", err)
			}

			t.Logf("Transfer %s status: %s, elapsed: %ds",
				transferID, transfer.Status, transfer.Progress.ElapsedTime)

			switch transfer.Status {
			case TransferStatusCompleted:
				t.Logf("Transfer completed successfully after pause/resume!")

				// Verify the filesystem was created on the remote target
				verifyRemoteFilesystem(t, testConfig, transferConfig.ReceiveConfig.Target)

				return
			case TransferStatusFailed:
				t.Fatalf("Transfer failed after resume: %s", transfer.ErrorMessage)
			case TransferStatusCancelled:
				t.Fatalf("Transfer was cancelled after resume: %s", transfer.ErrorMessage)
			}
		}
	}
}

func TestTransferManager_StopTransfer(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	// Create a test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	ctx := context.Background()

	// Configure the transfer
	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: snapshotName,
			Verbose:  true,
			Parsable: false,
		},
		ReceiveConfig: ReceiveConfig{
			Target: testConfig.TargetFilesystem + "/test-stop" + time.Now().
				Format("20060102-150405"),
			Resumable: true,
			Force:     true,
			Verbose:   true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	// Clean up any remote filesystem that might have been created before stopping
	defer cleanupRemoteFilesystem(t, testConfig, transferConfig.ReceiveConfig.Target)

	// Start the transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	t.Logf("Started transfer with ID: %s", transferID)

	// Wait a bit for transfer to start
	time.Sleep(1 * time.Second)
	t.Logf("Slept for 1 second, now stopping transfer %s", transferID)

	// Stop the transfer
	err = transferManager.StopTransfer(transferID)
	if err != nil {
		t.Fatalf("Failed to stop transfer: %v", err)
	}

	t.Logf("Stopped transfer %s", transferID)

	// Verify transfer is stopped
	transfer, err := transferManager.GetTransfer(transferID)
	if err != nil {
		t.Fatalf("Failed to get transfer status: %v", err)
	}

	if transfer.Status != TransferStatusCancelled {
		t.Fatalf("Expected transfer status to be cancelled, got: %s", transfer.Status)
	}

	t.Logf("Transfer successfully stopped with status: %s", transfer.Status)
}

func TestTransferManager_ListTransfers(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	// Create a test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	ctx := context.Background()

	// Start multiple transfers
	transferIDs := make([]string, 2)

	for i := 0; i < 2; i++ {
		transferConfig := TransferConfig{
			SendConfig: SendConfig{
				Snapshot: snapshotName,
				Verbose:  true,
				Parsable: false,
			},
			ReceiveConfig: ReceiveConfig{
				Target:    fmt.Sprintf("%s/test-list-%d", testConfig.TargetFilesystem, i),
				Resumable: true,
				Force:     true,
				Verbose:   true,
				RemoteConfig: RemoteConfig{
					Host:       testConfig.TargetIP,
					User:       testConfig.TargetUsername,
					PrivateKey: testConfig.SSHKeyPath,
					Port:       22,
				},
			},
		}

		transferID, err := transferManager.StartTransfer(ctx, transferConfig)
		if err != nil {
			t.Fatalf("Failed to start transfer %d: %v", i, err)
		}

		transferIDs[i] = transferID
		t.Logf("Started transfer %d with ID: %s", i, transferID)
	}

	// List transfers
	transfers := transferManager.ListTransfers()

	if len(transfers) < 2 {
		t.Fatalf("Expected at least 2 transfers, got %d", len(transfers))
	}

	// Verify our transfers are in the list
	foundTransfers := 0
	for _, transfer := range transfers {
		for _, expectedID := range transferIDs {
			if transfer.ID == expectedID {
				foundTransfers++
				t.Logf("Found transfer %s with status %s", transfer.ID, transfer.Status)
			}
		}
	}

	if foundTransfers != 2 {
		t.Fatalf("Expected to find 2 transfers in list, found %d", foundTransfers)
	}

	// Stop all test transfers and clean up remote filesystems
	for i, transferID := range transferIDs {
		err := transferManager.StopTransfer(transferID)
		if err != nil {
			t.Logf("Warning: Failed to stop transfer %s: %v", transferID, err)
		}

		// Clean up the remote filesystem that might have been created
		targetPath := fmt.Sprintf("%s/test-list-%d", testConfig.TargetFilesystem, i)
		cleanupRemoteFilesystem(t, testConfig, targetPath)
	}
}

func TestTransferManager_DeleteTransfer(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	// Create a test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	ctx := context.Background()

	// Configure the transfer
	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: snapshotName,
			Verbose:  true,
			Parsable: false,
		},
		ReceiveConfig: ReceiveConfig{
			Target: testConfig.TargetFilesystem + "/test-delete" + time.Now().
				Format("20060102-150405"),
			Resumable: true,
			Force:     true,
			Verbose:   true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	// Start the transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	t.Logf("Started transfer with ID: %s", transferID)

	// Wait a bit then stop the transfer
	time.Sleep(1 * time.Second)
	t.Logf("Slept for 1 second, now stopping transfer %s", transferID)
	err = transferManager.StopTransfer(transferID)
	if err != nil {
		t.Fatalf("Failed to stop transfer: %v", err)
	}

	// Verify transfer exists
	_, err = transferManager.GetTransfer(transferID)
	if err != nil {
		t.Fatalf("Transfer should exist before deletion: %v", err)
	}

	// Delete the transfer
	err = transferManager.DeleteTransfer(transferID)
	if err != nil {
		t.Fatalf("Failed to delete transfer: %v", err)
	}

	t.Logf("Deleted transfer %s", transferID)

	// Verify transfer no longer exists
	_, err = transferManager.GetTransfer(transferID)
	if err == nil {
		t.Fatalf("Transfer should not exist after deletion")
	}

	t.Logf("Transfer successfully deleted and removed from list")

	// Clean up any remote filesystem that might have been created before stopping
	cleanupRemoteFilesystem(t, testConfig, transferConfig.ReceiveConfig.Target)
}

func TestTransferManager_NetworkResilience(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	// Create a test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	ctx := context.Background()

	// Configure transfer with invalid SSH details to simulate network failure
	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: snapshotName,
			Verbose:  true,
			Parsable: false,
		},
		ReceiveConfig: ReceiveConfig{
			Target: testConfig.TargetFilesystem + "/test-network" + time.Now().
				Format("20060102-150405"),
			Resumable: true,
			Force:     true,
			Verbose:   true,
			RemoteConfig: RemoteConfig{
				Host:       "192.168.1.999", // Invalid IP to simulate network failure
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	// Start the transfer (should fail)
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	t.Logf("Started transfer with ID: %s (should fail due to network)", transferID)

	// Monitor for failure
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Transfer should have failed due to network error within 30 seconds")

		case <-ticker.C:
			transfer, err := transferManager.GetTransfer(transferID)
			if err != nil {
				t.Fatalf("Failed to get transfer status: %v", err)
			}

			t.Logf("Transfer %s status: %s", transferID, transfer.Status)

			if transfer.Status == TransferStatusFailed {
				t.Logf(
					"Transfer failed as expected due to network error: %s",
					transfer.ErrorMessage,
				)
				return
			}
		}
	}
}

// Test automatic initial snapshot handling when initial snapshot is missing
func TestTransferManager_AutoInitialSnapshotSend(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	ctx := context.Background()

	// Create two test snapshots for incremental transfer
	initialSnapshot := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, initialSnapshot)

	// Wait a moment to ensure different timestamps
	time.Sleep(1 * time.Second)

	incrementalSnapshot := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, incrementalSnapshot)

	t.Logf("Created snapshots - Initial: %s, Incremental: %s", initialSnapshot, incrementalSnapshot)

	// Target filesystem path
	targetPath := testConfig.TargetFilesystem + "/test-auto-initial" + time.Now().Format("20060102-150405")
	defer cleanupRemoteFilesystem(t, testConfig, targetPath)

	// Configure incremental transfer (initial snapshot should be missing on target)
	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot:     incrementalSnapshot,
			FromSnapshot: initialSnapshot, // This should trigger auto-initial send
			Verbose:      true,
			Parsable:     false,
		},
		ReceiveConfig: ReceiveConfig{
			Target:    targetPath,
			Resumable: true,
			Force:     true,
			Verbose:   true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	// Start the transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start incremental transfer: %v", err)
	}

	t.Logf("Started incremental transfer with ID: %s", transferID)

	// Monitor transfer progress and verify phases
	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	seenInitialSendPhase := false
	seenIncrementalSendPhase := false

	for {
		select {
		case <-timeout:
			t.Fatalf("Transfer timed out after 10 minutes")

		case <-ticker.C:
			transfer, err := transferManager.GetTransfer(transferID)
			if err != nil {
				t.Fatalf("Failed to get transfer status: %v", err)
			}

			t.Logf("Transfer %s status: %s, phase: %s, description: %s, elapsed: %ds",
				transferID, transfer.Status, transfer.Progress.Phase, 
				transfer.Progress.PhaseDescription, transfer.Progress.ElapsedTime)

			// Track which phases we've seen
			if transfer.Progress.Phase == "initial_send" {
				seenInitialSendPhase = true
				t.Logf("✅ Detected initial send phase: %s", transfer.Progress.PhaseDescription)
			}
			if transfer.Progress.Phase == "incremental_send" {
				seenIncrementalSendPhase = true
				t.Logf("✅ Detected incremental send phase: %s", transfer.Progress.PhaseDescription)
			}

			switch transfer.Status {
			case TransferStatusCompleted:
				t.Logf("Transfer completed successfully!")

				// Verify we saw both phases
				if !seenInitialSendPhase {
					t.Errorf("Expected to see initial_send phase but didn't")
				}
				if !seenIncrementalSendPhase {
					t.Errorf("Expected to see incremental_send phase but didn't")
				}

				// Verify the filesystem was created on the remote target
				verifyRemoteFilesystem(t, testConfig, targetPath)

				return
			case TransferStatusFailed:
				t.Fatalf("Transfer failed: %s", transfer.ErrorMessage)
			case TransferStatusCancelled:
				t.Fatalf("Transfer was cancelled: %s", transfer.ErrorMessage)
			}
		}
	}
}

// Test incremental transfer when initial snapshot already exists on target
func TestTransferManager_IncrementalWithExistingInitial(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	ctx := context.Background()

	// Create initial snapshot
	initialSnapshot := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, initialSnapshot)

	// Target filesystem path
	targetPath := testConfig.TargetFilesystem + "/test-existing-initial" + time.Now().Format("20060102-150405")
	defer cleanupRemoteFilesystem(t, testConfig, targetPath)

	// First, send the initial snapshot
	initialTransferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: initialSnapshot,
			Verbose:  true,
			Parsable: false,
		},
		ReceiveConfig: ReceiveConfig{
			Target:    targetPath,
			Resumable: true,
			Force:     true,
			Verbose:   true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	initialTransferID, err := transferManager.StartTransfer(ctx, initialTransferConfig)
	if err != nil {
		t.Fatalf("Failed to start initial transfer: %v", err)
	}

	t.Logf("Started initial transfer with ID: %s", initialTransferID)

	// Wait for initial transfer to complete
	waitForTransferCompletion(t, transferManager, initialTransferID, 5*time.Minute)

	// Wait a moment to ensure different timestamps
	time.Sleep(1 * time.Second)

	// Create incremental snapshot
	incrementalSnapshot := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, incrementalSnapshot)

	t.Logf("Created incremental snapshot: %s", incrementalSnapshot)

	// Now send incremental (should skip initial send since it exists)
	incrementalTransferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot:     incrementalSnapshot,
			FromSnapshot: initialSnapshot, // This should NOT trigger auto-initial send
			Verbose:      true,
			Parsable:     false,
		},
		ReceiveConfig: ReceiveConfig{
			Target:    targetPath,
			Resumable: true,
			Force:     true,
			Verbose:   true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	incrementalTransferID, err := transferManager.StartTransfer(ctx, incrementalTransferConfig)
	if err != nil {
		t.Fatalf("Failed to start incremental transfer: %v", err)
	}

	t.Logf("Started incremental transfer with ID: %s", incrementalTransferID)

	// Monitor progress - should only see incremental phase
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	seenInitialSendPhase := false
	seenIncrementalSendPhase := false

	for {
		select {
		case <-timeout:
			t.Fatalf("Transfer timed out after 5 minutes")

		case <-ticker.C:
			transfer, err := transferManager.GetTransfer(incrementalTransferID)
			if err != nil {
				t.Fatalf("Failed to get transfer status: %v", err)
			}

			t.Logf("Transfer %s status: %s, phase: %s, description: %s",
				incrementalTransferID, transfer.Status, transfer.Progress.Phase, 
				transfer.Progress.PhaseDescription)

			// Track phases
			if transfer.Progress.Phase == "initial_send" {
				seenInitialSendPhase = true
			}
			if transfer.Progress.Phase == "incremental_send" {
				seenIncrementalSendPhase = true
			}

			switch transfer.Status {
			case TransferStatusCompleted:
				t.Logf("Incremental transfer completed successfully!")

				// Should NOT have seen initial send phase
				if seenInitialSendPhase {
					t.Errorf("Should not have seen initial_send phase when initial snapshot already exists")
				}
				// Should have seen incremental phase
				if !seenIncrementalSendPhase {
					t.Errorf("Expected to see incremental_send phase")
				}

				return
			case TransferStatusFailed:
				t.Fatalf("Transfer failed: %s", transfer.ErrorMessage)
			case TransferStatusCancelled:
				t.Fatalf("Transfer was cancelled: %s", transfer.ErrorMessage)
			}
		}
	}
}

// Test snapshot validation with network issues
func TestTransferManager_SnapshotValidationNetworkFailure(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	ctx := context.Background()

	// Create snapshots for incremental transfer
	initialSnapshot := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, initialSnapshot)

	time.Sleep(1 * time.Second)

	incrementalSnapshot := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, incrementalSnapshot)

	// Configure transfer with invalid remote to simulate network issues during validation
	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot:     incrementalSnapshot,
			FromSnapshot: initialSnapshot,
			Verbose:      true,
			Parsable:     false,
		},
		ReceiveConfig: ReceiveConfig{
			Target: testConfig.TargetFilesystem + "/test-validation-network" + time.Now().Format("20060102-150405"),
			Resumable: true,
			Force:     true,
			Verbose:   true,
			RemoteConfig: RemoteConfig{
				Host:       "192.168.1.999", // Invalid IP
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	// Start transfer - should proceed despite validation failure
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	t.Logf("Started transfer with network validation failure: %s", transferID)

	// Should fail due to network error in actual transfer, but should have proceeded past validation
	timeout := time.After(1 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Expected transfer to fail within 1 minute due to network error")

		case <-ticker.C:
			transfer, err := transferManager.GetTransfer(transferID)
			if err != nil {
				t.Fatalf("Failed to get transfer status: %v", err)
			}

			t.Logf("Transfer %s status: %s", transferID, transfer.Status)

			if transfer.Status == TransferStatusFailed {
				t.Logf("Transfer failed as expected due to network error: %s", transfer.ErrorMessage)
				return
			}
		}
	}
}

// Test progress tracking phases for different transfer types
func TestTransferManager_ProgressPhases(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	ctx := context.Background()

	// Test 1: Full send phase
	fullSnapshot := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, fullSnapshot)

	targetPath1 := testConfig.TargetFilesystem + "/test-phases-full" + time.Now().Format("20060102-150405")
	defer cleanupRemoteFilesystem(t, testConfig, targetPath1)

	fullTransferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: fullSnapshot,
			Verbose:  true,
		},
		ReceiveConfig: ReceiveConfig{
			Target:    targetPath1,
			Resumable: true,
			Force:     true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	fullTransferID, err := transferManager.StartTransfer(ctx, fullTransferConfig)
	if err != nil {
		t.Fatalf("Failed to start full transfer: %v", err)
	}

	// Check that full send shows correct phase
	time.Sleep(1 * time.Second) // Allow time for phase to be set
	transfer, err := transferManager.GetTransfer(fullTransferID)
	if err != nil {
		t.Fatalf("Failed to get transfer status: %v", err)
	}

	if transfer.Progress.Phase != "full_send" {
		t.Errorf("Expected phase 'full_send', got '%s'", transfer.Progress.Phase)
	}

	if !contains(transfer.Progress.PhaseDescription, fullSnapshot) {
		t.Errorf("Expected phase description to contain snapshot name '%s', got '%s'", 
			fullSnapshot, transfer.Progress.PhaseDescription)
	}

	t.Logf("✅ Full send phase correct: %s - %s", transfer.Progress.Phase, transfer.Progress.PhaseDescription)

	// Wait for completion
	waitForTransferCompletion(t, transferManager, fullTransferID, 5*time.Minute)
}

// Helper function to wait for transfer completion
func waitForTransferCompletion(t *testing.T, tm *TransferManager, transferID string, timeout time.Duration) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("Transfer %s timed out after %v", transferID, timeout)

		case <-ticker.C:
			transfer, err := tm.GetTransfer(transferID)
			if err != nil {
				t.Fatalf("Failed to get transfer %s status: %v", transferID, err)
			}

			switch transfer.Status {
			case TransferStatusCompleted:
				t.Logf("Transfer %s completed successfully", transferID)
				return
			case TransferStatusFailed:
				t.Fatalf("Transfer %s failed: %s", transferID, transfer.ErrorMessage)
			case TransferStatusCancelled:
				t.Fatalf("Transfer %s was cancelled: %s", transferID, transfer.ErrorMessage)
			}
		}
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr || 
		   len(s) >= len(substr) && s[:len(substr)] == substr ||
		   (len(s) > len(substr) && indexOf(s, substr) >= 0)
}

// Helper function to find substring index
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Test historical transfer listing by type
func TestTransferManager_ListTransfersByType(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	ctx := context.Background()

	// Create test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	// Start a transfer and let it complete
	targetPath := testConfig.TargetFilesystem + "/test-history" + time.Now().Format("20060102-150405")
	defer cleanupRemoteFilesystem(t, testConfig, targetPath)

	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: snapshotName,
			Verbose:  true,
		},
		ReceiveConfig: ReceiveConfig{
			Target:    targetPath,
			Resumable: true,
			Force:     true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	// Start transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	// Wait for completion
	waitForTransferCompletion(t, transferManager, transferID, 5*time.Minute)

	// Test different transfer type listings
	testCases := []struct {
		transferType    TransferType
		expectedCount   int
		shouldContainID bool
	}{
		{TransferTypeActive, 0, false},    // Should be 0 since transfer completed
		{TransferTypeCompleted, 1, true},  // Should contain our completed transfer
		{TransferTypeFailed, 0, false},    // Should be 0 since transfer succeeded
		{TransferTypeAll, 1, true},        // Should contain our transfer
	}

	for _, tc := range testCases {
		t.Run(string(tc.transferType), func(t *testing.T) {
			transfers := transferManager.ListTransfersByType(tc.transferType)

			if len(transfers) < tc.expectedCount {
				t.Errorf("Expected at least %d transfers for type %s, got %d", 
					tc.expectedCount, tc.transferType, len(transfers))
			}

			if tc.shouldContainID {
				found := false
				for _, transfer := range transfers {
					if transfer.ID == transferID {
						found = true
						if tc.transferType == TransferTypeCompleted && transfer.Status != TransferStatusCompleted {
							t.Errorf("Expected completed transfer to have status 'completed', got '%s'", transfer.Status)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected to find transfer %s in %s transfers", transferID, tc.transferType)
				}
			}
		})
	}
}

// Test transfer log functionality
func TestTransferManager_TransferLogs(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	ctx := context.Background()

	// Create test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	targetPath := testConfig.TargetFilesystem + "/test-logs" + time.Now().Format("20060102-150405")
	defer cleanupRemoteFilesystem(t, testConfig, targetPath)

	// Configure transfer with custom log settings
	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: snapshotName,
			Verbose:  true,
		},
		ReceiveConfig: ReceiveConfig{
			Target:    targetPath,
			Resumable: true,
			Force:     true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
		LogConfig: &TransferLogConfig{
			MaxSizeBytes:     5 * 1024, // 5KB to test truncation
			TruncateOnFinish: false,    // Don't truncate for this test
			RetainOnFailure:  true,
			HeaderLines:      10,
			FooterLines:      10,
		},
	}

	// Start transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	// Wait for completion
	waitForTransferCompletion(t, transferManager, transferID, 5*time.Minute)

	// Test GetTransferLog
	t.Run("GetTransferLog", func(t *testing.T) {
		logContent, err := transferManager.GetTransferLog(transferID)
		if err != nil {
			t.Fatalf("Failed to get transfer log: %v", err)
		}

		if logContent == "" {
			t.Error("Expected non-empty log content")
		}

		// Should contain ZFS send output
		if !contains(logContent, "send") && !contains(logContent, snapshotName) {
			t.Error("Expected log to contain transfer-related content")
		}

		t.Logf("Log content length: %d bytes", len(logContent))
	})

	// Test GetTransferLogGist
	t.Run("GetTransferLogGist", func(t *testing.T) {
		logGist, err := transferManager.GetTransferLogGist(transferID)
		if err != nil {
			t.Fatalf("Failed to get transfer log gist: %v", err)
		}

		if logGist == "" {
			t.Error("Expected non-empty log gist")
		}

		// Should be different from full log if truncation occurred
		fullLog, _ := transferManager.GetTransferLog(transferID)
		if len(fullLog) > 100*1024 { // 100KB limit
			if len(logGist) >= len(fullLog) {
				t.Error("Expected gist to be shorter than full log for large files")
			}
			
			// Should contain truncation marker
			if !contains(logGist, "File truncated") {
				t.Error("Expected gist to contain truncation marker for large files")
			}
		}

		t.Logf("Log gist length: %d bytes", len(logGist))
	})

	// Test log for non-existent transfer
	t.Run("NonExistentTransfer", func(t *testing.T) {
		_, err := transferManager.GetTransferLog("non-existent-id")
		if err == nil {
			t.Error("Expected error for non-existent transfer log")
		}

		_, err = transferManager.GetTransferLogGist("non-existent-id")
		if err == nil {
			t.Error("Expected error for non-existent transfer log gist")
		}
	})
}

// Test custom log configuration
func TestTransferManager_CustomLogConfig(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	ctx := context.Background()

	// Create test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	targetPath := testConfig.TargetFilesystem + "/test-custom-log" + time.Now().Format("20060102-150405")
	defer cleanupRemoteFilesystem(t, testConfig, targetPath)

	// Test with custom log configuration
	customLogConfig := &TransferLogConfig{
		MaxSizeBytes:     1024, // 1KB - very small to trigger truncation
		TruncateOnFinish: true,
		RetainOnFailure:  false,
		HeaderLines:      5,
		FooterLines:      5,
	}

	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: snapshotName,
			Verbose:  true,
		},
		ReceiveConfig: ReceiveConfig{
			Target:    targetPath,
			Resumable: true,
			Force:     true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
		LogConfig: customLogConfig,
	}

	// Start transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	// Wait for completion
	waitForTransferCompletion(t, transferManager, transferID, 5*time.Minute)

	// Get transfer to verify log config was preserved
	transfer, err := transferManager.GetTransfer(transferID)
	if err != nil {
		t.Fatalf("Failed to get transfer: %v", err)
	}

	// Verify custom config was preserved
	if transfer.Config.LogConfig == nil {
		t.Fatal("Expected log config to be preserved")
	}

	if transfer.Config.LogConfig.MaxSizeBytes != customLogConfig.MaxSizeBytes {
		t.Errorf("Expected MaxSizeBytes %d, got %d", 
			customLogConfig.MaxSizeBytes, transfer.Config.LogConfig.MaxSizeBytes)
	}

	if transfer.Config.LogConfig.HeaderLines != customLogConfig.HeaderLines {
		t.Errorf("Expected HeaderLines %d, got %d", 
			customLogConfig.HeaderLines, transfer.Config.LogConfig.HeaderLines)
	}
}

// Test default log configuration behavior
func TestTransferManager_DefaultLogConfig(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	ctx := context.Background()

	// Create test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	targetPath := testConfig.TargetFilesystem + "/test-default-log" + time.Now().Format("20060102-150405")
	defer cleanupRemoteFilesystem(t, testConfig, targetPath)

	// Transfer without explicit log config (should use defaults)
	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: snapshotName,
			Verbose:  true,
		},
		ReceiveConfig: ReceiveConfig{
			Target:    targetPath,
			Resumable: true,
			Force:     true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
		// LogConfig intentionally nil to test defaults
	}

	// Start transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	// Wait for completion
	waitForTransferCompletion(t, transferManager, transferID, 5*time.Minute)

	// Verify that log operations work with default config
	logContent, err := transferManager.GetTransferLog(transferID)
	if err != nil {
		t.Fatalf("Failed to get log with default config: %v", err)
	}

	if logContent == "" {
		t.Error("Expected non-empty log content with default config")
	}

	logGist, err := transferManager.GetTransferLogGist(transferID)
	if err != nil {
		t.Fatalf("Failed to get log gist with default config: %v", err)
	}

	if logGist == "" {
		t.Error("Expected non-empty log gist with default config")
	}

	t.Logf("Default config test - Log: %d bytes, Gist: %d bytes", len(logContent), len(logGist))
}

// Test historical transfer deletion
func TestTransferManager_DeleteHistoricalTransfer(t *testing.T) {
	testConfig := getTestConfig(t)
	transferManager, datasetManager := setupTransferManager(t)

	ctx := context.Background()

	// Create test snapshot
	snapshotName := createTestSnapshot(t, datasetManager, testConfig.SourceFilesystem)
	defer cleanupSnapshot(t, datasetManager, snapshotName)

	targetPath := testConfig.TargetFilesystem + "/test-delete-historical" + time.Now().Format("20060102-150405")
	defer cleanupRemoteFilesystem(t, testConfig, targetPath)

	transferConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: snapshotName,
			Verbose:  true,
		},
		ReceiveConfig: ReceiveConfig{
			Target:    targetPath,
			Resumable: true,
			Force:     true,
			RemoteConfig: RemoteConfig{
				Host:       testConfig.TargetIP,
				User:       testConfig.TargetUsername,
				PrivateKey: testConfig.SSHKeyPath,
				Port:       22,
			},
		},
	}

	// Start and complete transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	waitForTransferCompletion(t, transferManager, transferID, 5*time.Minute)

	// Verify transfer exists in completed transfers
	completedTransfers := transferManager.ListTransfersByType(TransferTypeCompleted)
	found := false
	for _, transfer := range completedTransfers {
		if transfer.ID == transferID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Transfer should exist in completed transfers before deletion")
	}

	// Delete the historical transfer
	err = transferManager.DeleteTransfer(transferID)
	if err != nil {
		t.Fatalf("Failed to delete historical transfer: %v", err)
	}

	// Verify transfer no longer exists
	_, err = transferManager.GetTransfer(transferID)
	if err == nil {
		t.Error("Transfer should not exist after deletion")
	}

	// Verify it's not in completed transfers list
	completedTransfers = transferManager.ListTransfersByType(TransferTypeCompleted)
	for _, transfer := range completedTransfers {
		if transfer.ID == transferID {
			t.Error("Transfer should not appear in completed transfers after deletion")
		}
	}

	// Verify log files are cleaned up
	_, err = transferManager.GetTransferLog(transferID)
	if err == nil {
		t.Error("Transfer log should not exist after deletion")
	}
}
