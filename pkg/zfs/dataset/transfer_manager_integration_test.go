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

	// Start the transfer
	transferID, err := transferManager.StartTransfer(ctx, transferConfig)
	if err != nil {
		t.Fatalf("Failed to start transfer: %v", err)
	}

	// Clean up any remote filesystem that might have been created before stopping
	defer cleanupRemoteFilesystem(t, testConfig, transferConfig.ReceiveConfig.Target)

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
