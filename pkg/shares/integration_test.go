package shares_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/pkg/facl"
	"github.com/stratastor/rodent/pkg/shares/smb"
)

const (
	testFileContent = "This is a test file created by Rodent SMB share integration tests."
)

// TestEnvironment holds the test configuration
type TestEnvironment struct {
	TestUser         string
	TestUserPassword string
	TestFilesystem   string
	TestRealm        string
	Hostname         string
	MountPoint       string
	CleanupFuncs     []func()
}

// newTestEnvironment creates a new test environment
func newTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()

	// Get test parameters from environment variables
	testUser := os.Getenv("RODENT_TEST_USER")
	testUserPassword := os.Getenv("RODENT_TEST_USER_PASS")
	testFilesystem := os.Getenv("RODENT_TEST_FS")
	testRealm := os.Getenv("RODENT_TEST_REALM")
	hostname, _ := os.Hostname()

	// Skip if required env vars are not set
	if testUser == "" || testUserPassword == "" || testFilesystem == "" {
		t.Skip(
			"Required environment variables RODENT_TEST_USER, RODENT_TEST_USER_PASS, and RODENT_TEST_FS must be set",
		)
	}

	// Check if SMB service is running
	if err := exec.Command("sudo", "systemctl", "is-active", "smbd").Run(); err != nil {
		t.Skip("SMB service is not running, skipping tests")
	}

	// Check if the test filesystem exists
	if _, err := os.Stat(testFilesystem); os.IsNotExist(err) {
		t.Skipf("Test filesystem '%s' does not exist", testFilesystem)
	}

	// Create temporary mount point
	mountPoint, err := os.MkdirTemp("/tmp/rodent-test", "rodent-test-mount-*")
	if err != nil {
		t.Fatalf("Failed to create temp mount directory: %v", err)
	}

	env := &TestEnvironment{
		TestUser:         testUser,
		TestUserPassword: testUserPassword,
		TestFilesystem:   testFilesystem,
		TestRealm:        testRealm,
		Hostname:         hostname,
		MountPoint:       mountPoint,
		CleanupFuncs:     []func(){},
	}

	// Register cleanup for mount point
	env.addCleanup(func() {
		// Try to unmount if mounted
		exec.Command("sudo", "umount", mountPoint).Run()
		os.RemoveAll(mountPoint)
	})

	return env
}

// addCleanup adds a cleanup function to be executed on teardown
func (e *TestEnvironment) addCleanup(fn func()) {
	e.CleanupFuncs = append(e.CleanupFuncs, fn)
}

// cleanup executes all registered cleanup functions
func (e *TestEnvironment) cleanup() {
	for i := len(e.CleanupFuncs) - 1; i >= 0; i-- {
		e.CleanupFuncs[i]()
	}
}

// TestSMBShareLifecycle tests the complete lifecycle of an SMB share
func TestSMBShareLifecycle(t *testing.T) {
	env := newTestEnvironment(t)
	defer env.cleanup()

	// Initialize logger
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "smb-test")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Initialize managers
	executor := command.NewCommandExecutor(true)
	aclManager := facl.NewACLManager(log, nil)

	// Create SMB manager
	smbManager, err := smb.NewManager(log, executor, aclManager)
	if err != nil {
		t.Fatalf("Failed to create SMB manager: %v", err)
	}

	// Create SMB service manager
	smbService := smb.NewServiceManager(log)

	// Generate a unique share name
	shareName := fmt.Sprintf("test-share-%d", time.Now().Unix())
	t.Logf("Using share name: %s", shareName)

	// Create test files in the filesystem
	testFile := filepath.Join(env.TestFilesystem, "test-file.txt")
	if err := os.WriteFile(testFile, []byte(testFileContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	env.addCleanup(func() {
		os.Remove(testFile)
	})

	ctx := context.Background()

	// Step 1: Verify SMB service is running
	t.Run("VerifySMBService", func(t *testing.T) {
		status, err := smbService.Status(ctx)
		if err != nil {
			t.Fatalf("Failed to get SMB service status: %v", err)
		}

		if status != "active" {
			// Try to start the service
			err := smbService.Start(ctx)
			if err != nil {
				t.Fatalf("SMB service is not running and failed to start it: %v", err)
			}

			// Check again
			status, err = smbService.Status(ctx)
			if err != nil || status != "active" {
				t.Fatalf("Failed to start SMB service: %v", err)
			}
		}

		t.Logf("SMB service is running")
	})

	// Step 2: Create a new SMB share
	t.Run("CreateSMBShare", func(t *testing.T) {
		smbConfig := &smb.SMBShareConfig{
			Name:        shareName,
			Description: "Rodent integration test share",
			Path:        env.TestFilesystem,
			Enabled:     true,
			ReadOnly:    false,
			Browsable:   true,
			GuestOk:     false,
			ValidUsers:  []string{"AD\\" + env.TestUser},
			Tags: map[string]string{
				"purpose": "testing",
				"owner":   "rodent",
			},
			InheritACLs:   true,
			MapACLInherit: true,
			CustomParameters: map[string]string{
				"create mask":          "0644",
				"directory mask":       "0755",
				"vfs objects":          "acl_xattr",
				"map archive":          "no",
				"map readonly":         "no",
				"store dos attributes": "yes",
			},
		}

		err := smbManager.CreateShare(ctx, smbConfig)
		if err != nil {
			t.Fatalf("Failed to create SMB share: %v", err)
		}

		// Register cleanup for the share
		env.addCleanup(func() {
			smbManager.DeleteShare(context.Background(), shareName)
		})

		t.Logf("SMB share created successfully")
	})

	// Step 3: Verify the share exists
	t.Run("VerifyShareExists", func(t *testing.T) {
		exists, err := smbManager.Exists(ctx, shareName)
		if err != nil {
			t.Fatalf("Failed to check if share exists: %v", err)
		}

		if !exists {
			t.Fatalf("Share should exist but doesn't")
		}

		// Get share details
		share, err := smbManager.GetSMBShare(ctx, shareName)
		if err != nil {
			t.Fatalf("Failed to get share details: %v", err)
		}
		t.Logf("Share details: %+v", share)

		if share.Name != shareName {
			t.Errorf("Share name mismatch: got %s, want %s", share.Name, shareName)
		}

		if share.Path != env.TestFilesystem {
			t.Errorf("Share path mismatch: got %s, want %s", share.Path, env.TestFilesystem)
		}

		t.Logf("Share exists and details match")
	})

	// Step 4: Mount the share
	t.Run("MountShare", func(t *testing.T) {
		// Wait for a moment to ensure SMB service has reloaded configuration
		time.Sleep(5 * time.Second)

		// Construct the mount command
		mountCmd := fmt.Sprintf(
			"sudo mount -t cifs //%s/%s %s -o username=%s@%s,password=%s,vers=3.0",
			env.Hostname,
			shareName,
			env.MountPoint,
			env.TestUser,
			env.TestRealm,
			env.TestUserPassword,
		)

		// Execute the mount command, but remove password from output
		t.Logf("Mounting with: %s", strings.Replace(mountCmd, env.TestUserPassword, "********", -1))

		cmd := exec.Command("sudo", "mount", "-t", "cifs",
			fmt.Sprintf("//%s/%s", env.Hostname, shareName),
			env.MountPoint,
			"-o", fmt.Sprintf("username=%s@%s,password=%s,vers=3.0",
				env.TestUser, env.TestRealm, env.TestUserPassword))

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Mount failed. Checking if share exists:")
			smbclientCmd := exec.Command(
				"smbclient",
				"-L",
				"//"+env.Hostname+"/"+shareName,
				"-U",
				env.TestUser,
				"--password",
				env.TestUserPassword,
			)
			t.Logf("Running smbclient command: %s", smbclientCmd.String())
			if smbclientOutput, smbclientErr := smbclientCmd.CombinedOutput(); smbclientErr == nil {
				t.Logf("Available shares: %s", string(smbclientOutput))
			} else {
				t.Logf("Failed to list shares: %v", smbclientErr)
			}
			t.Fatalf("Failed to mount share: %v\nOutput: %s", err, output)
		}

		// Verify the mount worked by checking for the test file
		mountedTestFile := filepath.Join(env.MountPoint, "test-file.txt")
		data, err := os.ReadFile(mountedTestFile)
		if err != nil {
			t.Fatalf("Failed to read test file from mounted share: %v", err)
		}

		if string(data) != testFileContent {
			t.Errorf("Test file content mismatch: got %s, want %s", string(data), testFileContent)
		}

		t.Logf("Successfully mounted share and verified content")
	})

	// Step 5: Get share statistics
	t.Run("GetShareStats", func(t *testing.T) {
		// Get basic stats
		stats, err := smbManager.GetShareStats(ctx, shareName)
		if err != nil {
			t.Fatalf("Failed to get share statistics: %v", err)
		}

		t.Logf("Share stats: active connections: %d, open files: %d",
			stats.ActiveConnections, stats.OpenFiles)

		// Get detailed stats
		detailedStats, err := smbManager.GetSMBShareStats(ctx, shareName)
		if err != nil {
			t.Fatalf("Failed to get detailed share statistics: %v", err)
		}

		t.Logf("Detailed stats: sessions: %d, files: %d",
			detailedStats.ActiveSessions, detailedStats.OpenFiles)

		// Should have at least one active session since we're mounted
		if detailedStats.ActiveSessions < 1 {
			t.Logf(
				"Warning: Expected at least one active session, got %d",
				detailedStats.ActiveSessions,
			)
		}
	})

	// Step 6: Create and read file via mount
	t.Run("CreateFileViaMountPoint", func(t *testing.T) {
		// Create a new file via the mount point
		newFileName := "rodent-test-write.txt"
		newFileContent := "This file was written via CIFS mount in Rodent integration test."
		newFilePath := filepath.Join(env.MountPoint, newFileName)

		err := os.WriteFile(newFilePath, []byte(newFileContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write new file via mount point: %v", err)
		}

		// Verify the file exists in the original filesystem
		origFilePath := filepath.Join(env.TestFilesystem, newFileName)
		origData, err := os.ReadFile(origFilePath)
		if err != nil {
			t.Fatalf("Failed to read file from original filesystem: %v", err)
		}

		if string(origData) != newFileContent {
			t.Errorf("File content mismatch: got %s, want %s", string(origData), newFileContent)
		}

		t.Logf("Successfully created and verified file via mount point")
	})

	// Step 7: Update the share configuration
	t.Run("UpdateShare", func(t *testing.T) {
		// Get current config
		share, err := smbManager.GetSMBShare(ctx, shareName)
		if err != nil {
			t.Fatalf("Failed to get share details: %v", err)
		}

		// Update configuration (make it read-only)
		share.ReadOnly = true
		share.Description = "Updated test share description"
		share.CustomParameters["strict locking"] = "no"

		err = smbManager.UpdateShare(ctx, shareName, share)
		if err != nil {
			t.Fatalf("Failed to update share: %v", err)
		}

		// Get updated config to verify
		updatedShare, err := smbManager.GetSMBShare(ctx, shareName)
		if err != nil {
			t.Fatalf("Failed to get updated share details: %v", err)
		}

		if !updatedShare.ReadOnly {
			t.Errorf("Share should be read-only after update")
		}

		if updatedShare.Description != "Updated test share description" {
			t.Errorf("Share description was not updated properly")
		}

		if val, ok := updatedShare.CustomParameters["strict locking"]; !ok || val != "no" {
			t.Errorf("Custom parameter 'strict locking' was not updated properly")
		}

		t.Logf("Successfully updated share configuration")
	})

	// Step 8: Verify read-only by attempting to write (should fail)
	t.Run("VerifyReadOnly", func(t *testing.T) {
		// Unmount and remount to apply new settings
		exec.Command("sudo", "umount", env.MountPoint).Run()
		time.Sleep(5 * time.Second)

		cmd := exec.Command("sudo", "mount", "-t", "cifs",
			fmt.Sprintf("//%s/%s", env.Hostname, shareName),
			env.MountPoint,
			"-o", fmt.Sprintf("username=%s@%s,password=%s,vers=3.0",
				env.TestUser, env.TestRealm, env.TestUserPassword))

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to remount share: %v\nOutput: %s", err, output)
		}

		// Try to write a file (should fail due to read-only)
		readOnlyTest := filepath.Join(env.MountPoint, "should-fail.txt")
		err = os.WriteFile(readOnlyTest, []byte("This write should fail"), 0644)

		if err == nil {
			t.Fatalf("Write succeeded but should have failed (share is read-only)")
		} else {
			t.Logf("Write correctly failed on read-only share: %v", err)
		}
	})

	// Step 9: Unmount and clean up
	t.Run("Unmount", func(t *testing.T) {
		cmd := exec.Command("sudo", "umount", env.MountPoint)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to unmount share: %v\nOutput: %s", err, output)
		}

		t.Logf("Share unmounted successfully")
	})

	// Step 10: Delete the share
	t.Run("DeleteShare", func(t *testing.T) {
		err := smbManager.DeleteShare(ctx, shareName)
		if err != nil {
			t.Fatalf("Failed to delete share: %v", err)
		}

		// Verify it's gone
		exists, err := smbManager.Exists(ctx, shareName)
		if err != nil {
			t.Fatalf("Failed to check if share exists: %v", err)
		}

		if exists {
			t.Errorf("Share should be deleted but still exists")
		}

		t.Logf("Share deleted successfully")
	})
}

// TestSMBServiceOperations tests SMB service operations
func TestSMBServiceOperations(t *testing.T) {
	// Skip if running in CI environment
	if os.Getenv("CI") != "" {
		t.Skip("Skipping service operations test in CI environment")
	}

	// Initialize logger
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "smb-test")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create SMB service manager
	smbService := smb.NewServiceManager(log)

	ctx := context.Background()

	t.Run("GetStatus", func(t *testing.T) {
		status, err := smbService.Status(ctx)
		if err != nil {
			t.Fatalf("Failed to get SMB service status: %v", err)
		}
		t.Logf("SMB service status: %s", status)
	})

	// Only run the following tests if we have permission to manage the service
	if os.Geteuid() != 0 && os.Getenv("SUDO_USER") == "" {
		t.Skip("Skipping service management tests that require sudo")
	}

	// Get initial status
	initialStatus, _ := smbService.Status(ctx)

	// Always restore service to its initial state
	defer func() {
		if initialStatus == "active" {
			smbService.Start(ctx)
		} else {
			smbService.Stop(ctx)
		}
	}()

	t.Run("RestartService", func(t *testing.T) {
		err := smbService.Restart(ctx)
		if err != nil {
			t.Fatalf("Failed to restart SMB service: %v", err)
		}

		// Verify service is running
		status, err := smbService.Status(ctx)
		if err != nil || status != "active" {
			t.Errorf("Service should be active after restart, got: %s, err: %v", status, err)
		}

		t.Logf("SMB service restarted successfully")
	})

	t.Run("ReloadConfig", func(t *testing.T) {
		err := smbService.ReloadConfig(ctx)
		if err != nil {
			t.Fatalf("Failed to reload SMB configuration: %v", err)
		}
		t.Logf("SMB configuration reloaded successfully")
	})

	t.Run("StopService", func(t *testing.T) {
		err := smbService.Stop(ctx)
		if err != nil {
			t.Fatalf("Failed to stop SMB service: %v", err)
		}

		// Verify service is stopped
		status, err := smbService.Status(ctx)
		if err != nil || status == "active" {
			t.Errorf("Service should be stopped, got: %s, err: %v", status, err)
		}

		t.Logf("SMB service stopped successfully")
	})

	t.Run("StartService", func(t *testing.T) {
		err := smbService.Start(ctx)
		if err != nil {
			t.Fatalf("Failed to start SMB service: %v", err)
		}

		// Verify service is running
		status, err := smbService.Status(ctx)
		if err != nil || status != "active" {
			t.Errorf("Service should be active after start, got: %s, err: %v", status, err)
		}

		t.Logf("SMB service started successfully")
	})
}

// TestSMBBulkOperations tests bulk update operations on SMB shares
func TestSMBBulkOperations(t *testing.T) {
	env := newTestEnvironment(t)
	defer env.cleanup()

	// Initialize logger
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "smb-test")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Initialize managers
	executor := command.NewCommandExecutor(true)
	aclManager := facl.NewACLManager(log, nil)

	// Create SMB manager
	smbManager, err := smb.NewManager(log, executor, aclManager)
	if err != nil {
		t.Fatalf("Failed to create SMB manager: %v", err)
	}

	ctx := context.Background()

	// Create test shares
	shareNames := []string{}
	shareCount := 3
	for i := 0; i < shareCount; i++ {
		shareName := fmt.Sprintf("bulk-test-%d-%d", time.Now().Unix(), i)
		shareNames = append(shareNames, shareName)

		smbConfig := &smb.SMBShareConfig{
			Name:        shareName,
			Description: fmt.Sprintf("Bulk test share %d", i),
			Path:        env.TestFilesystem,
			Enabled:     true,
			ReadOnly:    false,
			Browsable:   true,
			GuestOk:     false,
			ValidUsers:  []string{env.TestUser},
			Tags: map[string]string{
				"purpose": "bulk-testing",
				"index":   fmt.Sprintf("%d", i),
			},
		}

		err := smbManager.CreateShare(ctx, smbConfig)
		if err != nil {
			t.Fatalf("Failed to create test share %s: %v", shareName, err)
		}

		// Register cleanup
		env.addCleanup(func() {
			smbManager.DeleteShare(context.Background(), shareName)
		})
	}

	t.Logf("Created %d test shares for bulk operations", shareCount)

	// Test bulk update by name
	t.Run("BulkUpdateByName", func(t *testing.T) {
		bulkConfig := smb.SMBBulkUpdateConfig{
			ShareNames: shareNames[:2], // Update only first two shares
			Parameters: map[string]string{
				"strict locking": "no",
				"hide dot files": "yes",
			},
		}

		results, err := smbManager.BulkUpdateShares(ctx, bulkConfig)
		if err != nil {
			t.Fatalf("Bulk update by name failed: %v", err)
		}

		// Verify results
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}

		for _, result := range results {
			if !result.Success {
				t.Errorf("Bulk update failed for share %s: %s", result.ShareName, result.Error)
			}
		}

		// Verify updates were applied
		for i, name := range shareNames[:2] {
			share, err := smbManager.GetSMBShare(ctx, name)
			if err != nil {
				t.Fatalf("Failed to get share %s: %v", name, err)
			}

			if val, ok := share.CustomParameters["strict locking"]; !ok || val != "no" {
				t.Errorf("Share %d: Parameter 'strict locking' not updated", i)
			}

			if val, ok := share.CustomParameters["hide dot files"]; !ok || val != "yes" {
				t.Errorf("Share %d: Parameter 'hide dot files' not updated", i)
			}
		}

		// Verify third share was not affected
		share, err := smbManager.GetSMBShare(ctx, shareNames[2])
		if err != nil {
			t.Fatalf("Failed to get share %s: %v", shareNames[2], err)
		}

		if _, ok := share.CustomParameters["strict locking"]; ok {
			t.Errorf("Share %d should not have been updated", 2)
		}

		t.Logf("Bulk update by name completed successfully")
	})

	// Test bulk update by tag
	t.Run("BulkUpdateByTag", func(t *testing.T) {
		bulkConfig := smb.SMBBulkUpdateConfig{
			Tags: map[string]string{
				"purpose": "bulk-testing",
				"index":   "0", // Only update share with index 0
			},
			Parameters: map[string]string{
				"ea support":     "yes",
				"level2 oplocks": "no",
			},
		}

		results, err := smbManager.BulkUpdateShares(ctx, bulkConfig)
		if err != nil {
			t.Fatalf("Bulk update by tag failed: %v", err)
		}

		// Verify results
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}

		for _, result := range results {
			if !result.Success {
				t.Errorf("Bulk update failed for share %s: %s", result.ShareName, result.Error)
			}
		}

		// Verify update was applied
		share, err := smbManager.GetSMBShare(ctx, shareNames[0])
		if err != nil {
			t.Fatalf("Failed to get share %s: %v", shareNames[0], err)
		}

		if val, ok := share.CustomParameters["ea support"]; !ok || val != "yes" {
			t.Errorf("Parameter 'ea support' not updated")
		}

		if val, ok := share.CustomParameters["level2 oplocks"]; !ok || val != "no" {
			t.Errorf("Parameter 'level2 oplocks' not updated")
		}

		t.Logf("Bulk update by tag completed successfully")
	})

	// Test bulk update all
	t.Run("BulkUpdateAll", func(t *testing.T) {
		bulkConfig := smb.SMBBulkUpdateConfig{
			All: true,
			Parameters: map[string]string{
				"follow symlinks": "yes",
				"wide links":      "no",
			},
		}

		results, err := smbManager.BulkUpdateShares(ctx, bulkConfig)
		if err != nil {
			t.Fatalf("Bulk update all failed: %v", err)
		}

		// Count successful updates for our test shares
		successCount := 0
		for _, result := range results {
			for _, name := range shareNames {
				if result.ShareName == name && result.Success {
					successCount++
				}
			}
		}

		if successCount != len(shareNames) {
			t.Errorf("Expected %d successful updates, got %d", len(shareNames), successCount)
		}

		// Verify updates were applied to all test shares
		for i, name := range shareNames {
			share, err := smbManager.GetSMBShare(ctx, name)
			if err != nil {
				t.Fatalf("Failed to get share %s: %v", name, err)
			}

			if val, ok := share.CustomParameters["follow symlinks"]; !ok || val != "yes" {
				t.Errorf("Share %d: Parameter 'follow symlinks' not updated", i)
			}

			if val, ok := share.CustomParameters["wide links"]; !ok || val != "no" {
				t.Errorf("Share %d: Parameter 'wide links' not updated", i)
			}
		}

		t.Logf("Bulk update all completed successfully")
	})
}

// TestSMBGlobalConfig tests the global SMB configuration
func TestSMBGlobalConfig(t *testing.T) {
	// Initialize logger
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "smb-test")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Initialize managers
	executor := command.NewCommandExecutor(true)
	aclManager := facl.NewACLManager(log, nil)

	// Create SMB manager
	smbManager, err := smb.NewManager(log, executor, aclManager)
	if err != nil {
		t.Fatalf("Failed to create SMB manager: %v", err)
	}

	ctx := context.Background()

	// Get current global config
	originalConfig, err := smbManager.GetGlobalConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get global SMB config: %v", err)
	}

	// Make a copy for restoration
	finalConfig := *originalConfig

	// Register deferred restoration
	defer func() {
		if err := smbManager.UpdateGlobalConfig(context.Background(), &finalConfig); err != nil {
			t.Logf("Failed to restore original global config: %v", err)
		}
	}()

	t.Run("UpdateGlobalConfig", func(t *testing.T) {
		// Update global config
		updatedConfig := *originalConfig
		updatedConfig.WorkGroup = "RODENTTEST"
		updatedConfig.ServerString = "Rodent Test Server"
		updatedConfig.LogLevel = "2" // More verbose logging

		if updatedConfig.CustomParameters == nil {
			updatedConfig.CustomParameters = make(map[string]string)
		}
		updatedConfig.CustomParameters["client min protocol"] = "SMB2"
		updatedConfig.CustomParameters["client max protocol"] = "SMB3"

		err := smbManager.UpdateGlobalConfig(ctx, &updatedConfig)
		if err != nil {
			t.Fatalf("Failed to update global config: %v", err)
		}

		// Get updated config
		newConfig, err := smbManager.GetGlobalConfig(ctx)
		if err != nil {
			t.Fatalf("Failed to get updated global config: %v", err)
		}

		// Verify updates
		if newConfig.WorkGroup != "RODENTTEST" {
			t.Errorf("WorkGroup not updated: expected RODENTTEST, got %s", newConfig.WorkGroup)
		}

		if newConfig.ServerString != "Rodent Test Server" {
			t.Errorf(
				"ServerString not updated: expected 'Rodent Test Server', got %s",
				newConfig.ServerString,
			)
		}

		if newConfig.LogLevel != "2" {
			t.Errorf("LogLevel not updated: expected 2, got %s", newConfig.LogLevel)
		}

		if val, ok := newConfig.CustomParameters["client min protocol"]; !ok || val != "SMB2" {
			t.Errorf("Custom parameter 'client min protocol' not updated correctly")
		}

		t.Logf("Global SMB configuration updated successfully")
	})
}
