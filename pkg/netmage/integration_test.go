// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package netmage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/netmage/types"
)

func TestSafeApplyConfig_Integration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}

	ctx := context.Background()

	// Create logger
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "netmage-test")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create manager
	manager, err := NewManager(ctx, log, types.RendererNetworkd)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Get current configuration as baseline
	currentConfig, err := manager.GetNetplanConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get current config: %v", err)
	}

	t.Logf("Current config: %+v", currentConfig)

	// Test 1: Validate current configuration with safe apply (no changes)
	t.Run("ValidateCurrentConfig", func(t *testing.T) {
		// First, let's manually execute netplan status and see what we get
		executor := NewCommandExecutor(true)

		t.Log("=== Manual netplan status command ===")
		rawResult, err := executor.ExecuteCommand(
			ctx,
			"netplan",
			"status",
			"--all",
			"--verbose",
			"-f",
			"json",
		)
		if err != nil {
			t.Logf("Raw netplan command failed: %v", err)
		} else {
			t.Logf("Raw netplan stdout (%d bytes): %s", len(rawResult.Stdout), rawResult.Stdout)
			if rawResult.Stderr != "" {
				t.Logf("Raw netplan stderr: %s", rawResult.Stderr)
			}
		}

		t.Log("=== Manager GetNetplanStatus call ===")
		parsedStatus, err := manager.GetNetplanStatus(ctx, "")
		if err != nil {
			t.Logf("Manager GetNetplanStatus failed: %v", err)
		} else {
			t.Logf("Parsed status interface count: %d", len(parsedStatus.Interfaces))
			if parsedStatus.NetplanGlobalState != nil {
				t.Logf("Global state online: %v", parsedStatus.NetplanGlobalState.Online)
			}
			for name, iface := range parsedStatus.Interfaces {
				t.Logf("Interface %s: backend=%s, admin=%s, oper=%s, type=%s",
					name, iface.Backend, iface.AdminState, iface.OperState, iface.Type)
			}
		}

		t.Log("=== Manager GetRoutes call ===")
		routes, err := manager.GetRoutes(ctx, "main")
		if err != nil {
			t.Logf("Manager GetRoutes failed: %v", err)
		} else {
			t.Logf("Parsed routes count: %d", len(routes))
			for i, route := range routes {
				if i < 10 { // Show first 10 routes
					t.Logf("Route %d: to=%s, via=%s, table=%s, family=%v",
						i, route.To, route.Via, route.Table, route.Family)
				}
			}
		}

		options := &types.SafeConfigOptions{
			ConnectivityTargets:     []string{"172.31.0.1"}, // Test gateway only
			ConnectivityTimeout:     10 * time.Second,
			ConnectivityInterval:    2 * time.Second,
			MaxConnectivityFailures: 2,
			SkipPreValidation:       false,
			SkipPostValidation:      false,
			AutoBackup:              true,
			AutoRollback:            true,
			RollbackTimeout:         30 * time.Second,
			BackupDescription:       "Test validation of current config",
			GracePeriod:             5 * time.Second,
			ValidateInterfaces:      true,
			ValidateRoutes:          true,
			ValidateConnectivity:    true,
		}

		result, err := manager.SafeApplyConfig(ctx, currentConfig, options)
		if err != nil {
			t.Errorf("SafeApplyConfig failed: %v", err)
			if result != nil {
				t.Logf("Result: %+v", result)
			}
			return
		}

		if !result.Success {
			t.Errorf("SafeApplyConfig was not successful: %s", result.Message)
		}

		if !result.Applied {
			t.Errorf("Configuration was not applied")
		}

		if result.RolledBack {
			t.Errorf("Configuration was unexpectedly rolled back")
		}

		t.Logf("SafeApplyConfig succeeded: %s", result.Message)
		t.Logf("Total duration: %v", result.TotalDuration)
		t.Logf("Backup ID: %s", result.BackupID)

		// Validate results
		if result.PreValidation != nil && !result.PreValidation.Success {
			t.Errorf("Pre-validation failed: %v", result.PreValidation.Errors)
		}

		if result.PostValidation != nil && !result.PostValidation.Success {
			t.Errorf("Post-validation failed: %v", result.PostValidation.Errors)
		}

		if result.Connectivity != nil {
			if !result.Connectivity.InitialSuccess {
				t.Errorf("Initial connectivity test failed")
			}
			if !result.Connectivity.FinalSuccess {
				t.Errorf("Final connectivity test failed")
			}
			t.Logf("Connectivity checks: %d total, %d failed",
				result.Connectivity.TotalChecks,
				result.Connectivity.FailedChecks)
		}
	})

	// Test 2: Test with invalid configuration (should fail validation)
	t.Run("InvalidConfig", func(t *testing.T) {
		invalidConfig := &types.NetplanConfig{
			Network: &types.NetworkConfig{
				Version:  2,
				Renderer: types.RendererNetworkd,
				Ethernets: map[string]*types.EthernetConfig{
					"invalid-interface-name-that-is-way-too-long": {
						BaseDeviceConfig: types.BaseDeviceConfig{
							Addresses: []string{"invalid-ip-address"},
						},
					},
				},
			},
		}

		options := DefaultSafeConfigOptions()
		options.ConnectivityTargets = []string{"172.31.0.1"}
		options.GracePeriod = 5 * time.Second

		result, err := manager.SafeApplyConfig(ctx, invalidConfig, options)

		// This should fail during validation
		if err == nil {
			t.Errorf("Expected SafeApplyConfig to fail with invalid config, but it succeeded")
		}

		if result != nil && result.Success {
			t.Errorf("Expected SafeApplyConfig result to be unsuccessful")
		}

		t.Logf("SafeApplyConfig correctly failed with invalid config: %v", err)
		if result != nil && result.PreValidation != nil {
			t.Logf("Pre-validation errors: %v", result.PreValidation.Errors)
		}
	})

	// Test 3: Test backup and restore functionality
	t.Run("BackupRestore", func(t *testing.T) {
		// Create a backup
		backupID, err := manager.BackupNetplanConfig(ctx)
		if err != nil {
			t.Fatalf("Failed to create backup: %v", err)
		}
		t.Logf("Created backup: %s", backupID)

		// List backups
		backups, err := manager.ListBackups(ctx)
		if err != nil {
			t.Fatalf("Failed to list backups: %v", err)
		}
		t.Logf("Found %d backups", len(backups))

		// Verify our backup exists
		found := false
		for _, backup := range backups {
			if backup.ID == backupID {
				found = true
				t.Logf("Backup found: %s, size: %d bytes, created: %v",
					backup.ID, backup.Size, backup.Timestamp)
				break
			}
		}

		if !found {
			t.Errorf("Created backup %s not found in backup list", backupID)
		}

		// Test restore (should be a no-op since we're restoring the same config)
		err = manager.RestoreNetplanConfig(ctx, backupID)
		if err != nil {
			t.Errorf("Failed to restore backup: %v", err)
		} else {
			t.Logf("Successfully restored backup: %s", backupID)
		}
	})

	// Test 4: Test DefaultSafeConfigOptions
	t.Run("DefaultOptions", func(t *testing.T) {
		options := DefaultSafeConfigOptions()

		if options == nil {
			t.Fatal("DefaultSafeConfigOptions returned nil")
		}

		// Verify sensible defaults
		if len(options.ConnectivityTargets) == 0 {
			t.Error("Default options should have connectivity targets")
		}

		if options.ConnectivityTimeout == 0 {
			t.Error("Default options should have connectivity timeout")
		}

		if !options.AutoBackup {
			t.Error("Default options should enable auto backup")
		}

		if !options.AutoRollback {
			t.Error("Default options should enable auto rollback")
		}

		t.Logf("Default options: %+v", options)
	})

	// Test 5: Global DNS management
	t.Run("GlobalDNSManagement", func(t *testing.T) {
		// Get current DNS configuration for restoration later
		originalDNS, err := manager.GetGlobalDNS(ctx)
		if err != nil {
			t.Fatalf("Failed to get original DNS config: %v", err)
		}
		t.Logf(
			"Original DNS config: addresses=%v, search=%v",
			originalDNS.Addresses,
			originalDNS.Search,
		)

		// Test setting new DNS configuration
		testDNS := &types.NameserverConfig{
			Addresses: []string{"172.31.11.191", "1.1.1.1"},
			Search:    []string{"ad.strata.internal"},
		}

		t.Logf(
			"Setting test DNS config: addresses=%v, search=%v",
			testDNS.Addresses,
			testDNS.Search,
		)
		err = manager.SetGlobalDNS(ctx, testDNS)
		if err != nil {
			t.Errorf("Failed to set test DNS config: %v", err)
			return
		}

		// Wait a moment for systemd-resolved to restart and apply changes
		time.Sleep(3 * time.Second)

		// Verify the DNS was set correctly
		currentDNS, err := manager.GetGlobalDNS(ctx)
		if err != nil {
			t.Errorf("Failed to get DNS config after setting: %v", err)
		} else {
			t.Logf("DNS config after setting: addresses=%v, search=%v", currentDNS.Addresses, currentDNS.Search)

			// Check if search domain was applied
			if len(currentDNS.Search) > 0 && currentDNS.Search[0] == "ad.strata.internal" {
				t.Logf("Search domain correctly set to: %v", currentDNS.Search)
			} else {
				t.Logf("Search domain may not be updated yet in netplan status: %v", currentDNS.Search)
			}
		}

		// Test DNS resolution functionality using dig
		t.Logf("Testing DNS resolution with new config...")

		// Create a non-sudo command executor for dig
		digExecutor := NewCommandExecutor(false)
		digResult, err := digExecutor.ExecuteCommand(ctx, "dig", "+short", "ad.strata.internal")
		if err != nil {
			t.Logf("DNS resolution test failed (this may be expected): %v", err)
		} else {
			t.Logf("DNS resolution result: %s", digResult.Stdout)
		}

		// Check systemd-resolved status
		resolvectlResult, err := digExecutor.ExecuteCommand(
			ctx,
			"resolvectl",
			"status",
			"--no-pager",
		)
		if err != nil {
			t.Logf("Failed to get resolvectl status: %v", err)
		} else {
			t.Logf("Resolvectl status:\n%s", resolvectlResult.Stdout)
		}

		// Restore original DNS configuration
		t.Logf(
			"Restoring original DNS config: addresses=%v, search=%v",
			originalDNS.Addresses,
			originalDNS.Search,
		)
		err = manager.SetGlobalDNS(ctx, originalDNS)
		if err != nil {
			t.Errorf("Failed to restore original DNS config: %v", err)
		} else {
			t.Logf("Successfully restored original DNS configuration")
		}

		// Wait for restoration to take effect
		time.Sleep(3 * time.Second)

		// Verify restoration
		restoredDNS, err := manager.GetGlobalDNS(ctx)
		if err != nil {
			t.Errorf("Failed to verify DNS restoration: %v", err)
		} else {
			t.Logf("DNS config after restoration: addresses=%v, search=%v", restoredDNS.Addresses, restoredDNS.Search)
		}
	})
}

func TestNetworkManagerBasics_Integration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}

	ctx := context.Background()

	// Create logger
	log, err := logger.NewTag(logger.Config{LogLevel: "info"}, "netmage-basic-test")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create manager
	manager, err := NewManager(ctx, log, types.RendererNetworkd)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test GetSystemNetworkInfo
	t.Run("GetSystemNetworkInfo", func(t *testing.T) {
		info, err := manager.GetSystemNetworkInfo(ctx)
		if err != nil {
			t.Fatalf("Failed to get system network info: %v", err)
		}

		t.Logf("System info: hostname=%s, interfaces=%d, renderer=%s",
			info.Hostname, info.InterfaceCount, info.Renderer)

		if info.InterfaceCount == 0 {
			t.Error("Expected at least one network interface")
		}

		if info.Renderer != types.RendererNetworkd {
			t.Errorf("Expected renderer %s, got %s", types.RendererNetworkd, info.Renderer)
		}
	})

	// Test ListInterfaces
	t.Run("ListInterfaces", func(t *testing.T) {
		interfaces, err := manager.ListInterfaces(ctx)
		if err != nil {
			t.Fatalf("Failed to list interfaces: %v", err)
		}

		if len(interfaces) == 0 {
			t.Error("Expected at least one network interface")
		}

		for _, iface := range interfaces {
			t.Logf("Interface: %s, type=%s, admin=%s, oper=%s, addresses=%d",
				iface.Name, iface.Type, iface.AdminState, iface.OperState, len(iface.IPAddresses))
		}
	})

	// Test GetNetplanStatus
	t.Run("GetNetplanStatus", func(t *testing.T) {
		status, err := manager.GetNetplanStatus(ctx, "")
		if err != nil {
			t.Fatalf("Failed to get netplan status: %v", err)
		}

		if status.NetplanGlobalState == nil {
			t.Error("Expected global state in netplan status")
		}

		if !status.NetplanGlobalState.Online {
			t.Error("Expected system to be online")
		}

		t.Logf("Netplan status: online=%v, interfaces=%d",
			status.NetplanGlobalState.Online, len(status.Interfaces))
	})
}
