// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package netmage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/internal/system/privilege"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/netmage/types"
	"gopkg.in/yaml.v3"
)

const (
	NetplanConfigPath = "/etc/netplan/90-rodent-netmage.yaml"
)

// NetplanCommand wraps netplan commands for network configuration management
type NetplanCommand struct {
	executor  *CommandExecutor
	sudoOps   *privilege.SudoFileOperations
	backupDir string
}

// NewNetplanCommand creates a new Netplan command wrapper
func NewNetplanCommand(
	executor *CommandExecutor,
	sudoOps *privilege.SudoFileOperations,
) *NetplanCommand {
	backupDir := filepath.Join(config.GetConfigDir(), "backup", "netplan")

	nc := &NetplanCommand{
		executor:  executor,
		sudoOps:   sudoOps,
		backupDir: backupDir,
	}

	// Ensure netmage config file exists with minimal content
	nc.ensureConfigFileExists()

	return nc
}

// GetConfig retrieves the current netplan configuration
func (nc *NetplanCommand) GetConfig(ctx context.Context) (*types.NetplanConfig, error) {
	result, err := nc.executor.ExecuteCommand(ctx, "netplan", "get", "all")
	if err != nil {
		return nil, errors.Wrap(err, errors.NetplanGetFailed)
	}

	var config types.NetplanConfig
	if err := yaml.Unmarshal([]byte(result.Stdout), &config); err != nil {
		return nil, errors.Wrap(err, errors.NetplanYAMLParseError)
	}

	return &config, nil
}

// SetConfig updates the netplan configuration with validation
func (nc *NetplanCommand) SetConfig(ctx context.Context, config *types.NetplanConfig) error {
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		return errors.Wrap(err, errors.NetplanYAMLParseError)
	}

	// Create temporary directory structure for validation
	tempDir, err := nc.createTempConfigForValidation(ctx, yamlData)
	if err != nil {
		return errors.Wrap(err, errors.NetplanFileOperationFailed)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Validate the temporary configuration
	if err := nc.validateConfigFile(ctx, tempDir); err != nil {
		return errors.Wrap(err, errors.NetplanYAMLValidationError)
	}

	// Write validated config to actual location
	if err := nc.sudoOps.WriteFile(ctx, NetplanConfigPath, yamlData, 0600); err != nil {
		return errors.Wrap(err, errors.NetplanFileOperationFailed)
	}

	return nil
}

// Apply applies the current netplan configuration with validation
func (nc *NetplanCommand) Apply(ctx context.Context) error {
	// Validate current configuration before applying
	if err := nc.ValidateConfig(ctx); err != nil {
		return errors.Wrap(err, errors.NetplanYAMLValidationError)
	}

	result, err := nc.executor.ExecuteCommand(ctx, "netplan", "apply")
	if err != nil {
		return errors.Wrap(err, errors.NetplanApplyFailed).WithMetadata("output", result.Stderr)
	}

	return nil
}

// Try tries a netplan configuration with automatic rollback
func (nc *NetplanCommand) Try(
	ctx context.Context,
	timeout time.Duration,
) (*types.NetplanTryResult, error) {
	// Validate current configuration before trying
	if err := nc.ValidateConfig(ctx); err != nil {
		return &types.NetplanTryResult{
			Success:    false,
			Applied:    false,
			RolledBack: false,
			Error:      err.Error(),
			Message:    "Configuration validation failed",
		}, errors.Wrap(err, errors.NetplanYAMLValidationError)
	}

	timeoutStr := strconv.Itoa(int(timeout.Seconds()))
	result, err := nc.executor.ExecuteCommand(ctx, "netplan", "try", "--timeout", timeoutStr)

	tryResult := &types.NetplanTryResult{
		Success: err == nil,
		Applied: err == nil,
	}

	if err != nil {
		tryResult.Error = result.Stderr
		tryResult.Message = "Netplan try failed"
		tryResult.RolledBack = true
		return tryResult, errors.Wrap(err, errors.NetplanTryFailed)
	}

	tryResult.Message = "Configuration applied successfully"
	return tryResult, nil
}

// GetStatus retrieves netplan status
func (nc *NetplanCommand) GetStatus(
	ctx context.Context,
	iface string,
) (*types.NetplanStatus, error) {
	args := []string{"status", "--all", "--verbose", "-f", "json"}
	if iface != "" {
		args = []string{"status", iface, "--verbose", "-f", "json"}
	}

	result, err := nc.executor.ExecuteCommand(ctx, "netplan", args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.NetplanStatusFailed)
	}

	var status types.NetplanStatus
	if err := json.Unmarshal([]byte(result.Stdout), &status); err != nil {
		return nil, errors.Wrap(err, errors.NetplanYAMLParseError)
	}

	return &status, nil
}

// GetDiff retrieves differences between current state and netplan config
func (nc *NetplanCommand) GetDiff(ctx context.Context) (*types.NetplanDiff, error) {
	result, err := nc.executor.ExecuteCommand(
		ctx,
		"netplan",
		"status",
		"--all",
		"--verbose",
		"-f",
		"json",
		"--diff",
	)
	if err != nil {
		return nil, errors.Wrap(err, errors.NetplanDiffFailed)
	}

	var diff types.NetplanDiff
	if err := json.Unmarshal([]byte(result.Stdout), &diff); err != nil {
		return nil, errors.Wrap(err, errors.NetplanYAMLParseError)
	}

	return &diff, nil
}

// Backup creates a backup of the current netplan configuration
func (nc *NetplanCommand) Backup(ctx context.Context) (string, error) {
	// Ensure backup directory exists
	if err := common.EnsureDir(nc.backupDir, 0755); err != nil {
		return "", errors.Wrap(err, errors.NetplanBackupFailed)
	}

	// Generate backup ID
	backupID := fmt.Sprintf("backup_%d", time.Now().Unix())
	backupPath := filepath.Join(nc.backupDir, fmt.Sprintf("%s.yaml", backupID))

	// Read current config file
	configData, err := nc.sudoOps.ReadFile(ctx, NetplanConfigPath)
	if err != nil {
		return "", errors.Wrap(err, errors.NetplanBackupFailed)
	}

	// Write backup to user-accessible location
	if err := os.WriteFile(backupPath, configData, 0644); err != nil {
		return "", errors.Wrap(err, errors.NetplanBackupFailed)
	}

	return backupID, nil
}

// Restore restores netplan configuration from a backup
func (nc *NetplanCommand) Restore(ctx context.Context, backupID string) error {
	backupPath := filepath.Join(nc.backupDir, fmt.Sprintf("%s.yaml", backupID))

	// Read backup file
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return errors.Wrap(err, errors.NetplanRestoreFailed)
	}

	// Create temporary directory structure for validation
	tempDir, err := nc.createTempConfigForValidation(ctx, backupData)
	if err != nil {
		return errors.Wrap(err, errors.NetplanRestoreFailed)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Validate the backup configuration
	if err := nc.validateConfigFile(ctx, tempDir); err != nil {
		return errors.Wrap(err, errors.NetplanRestoreFailed).
			WithMetadata("reason", "backup validation failed")
	}

	// Write validated config to actual location
	if err := nc.sudoOps.WriteFile(ctx, NetplanConfigPath, backupData, 0600); err != nil {
		return errors.Wrap(err, errors.NetplanRestoreFailed)
	}

	return nil
}

// ListBackups lists available netplan configuration backups
func (nc *NetplanCommand) ListBackups(ctx context.Context) ([]*types.ConfigBackup, error) {
	entries, err := os.ReadDir(nc.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*types.ConfigBackup{}, nil
		}
		return nil, errors.Wrap(err, errors.NetplanBackupFailed)
	}

	var backups []*types.ConfigBackup
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Extract backup ID from filename
		name := entry.Name()
		backupID := name[:len(name)-5] // Remove .yaml extension

		backup := &types.ConfigBackup{
			ID:          backupID,
			Timestamp:   info.ModTime(),
			Description: fmt.Sprintf("Netplan backup %s", backupID),
			FilePath:    filepath.Join(nc.backupDir, name),
			Size:        info.Size(),
		}
		backups = append(backups, backup)
	}

	return backups, nil
}

// ValidateConfig validates netplan configuration by running "netplan get all"
func (nc *NetplanCommand) ValidateConfig(ctx context.Context) error {
	return nc.validateConfigFile(ctx, "")
}

// validateConfigFile validates a specific netplan configuration file
// If configPath is empty, validates all netplan configuration files
func (nc *NetplanCommand) validateConfigFile(ctx context.Context, configPath string) error {
	var args []string
	if configPath != "" {
		// Use --root-dir to validate specific config file
		// We need to validate in the context of /etc/netplan directory
		args = []string{"get", "all", "--root-dir", configPath}
	} else {
		// Validate all current configurations
		args = []string{"get", "all"}
	}

	result, err := nc.executor.ExecuteCommand(ctx, "netplan", args...)
	if err != nil {
		return errors.Wrap(err, errors.NetplanYAMLValidationError).
			WithMetadata("output", result.Stderr)
	}
	return nil
}

// ensureConfigFileExists creates the netmage config file if it doesn't exist
// and ensures correct permissions (600) to avoid netplan warnings
func (nc *NetplanCommand) ensureConfigFileExists() {
	ctx := context.Background()

	// Check if file already exists
	_, err := nc.sudoOps.ReadFile(ctx, NetplanConfigPath)
	if err != nil {
		// Create minimal config file
		minimalConfig := `# Netmage configuration file
# This file is managed by the netmage package
network:
  version: 2
  renderer: networkd
`
		// Write minimal config with correct permissions (ignore errors - it's best effort)
		nc.sudoOps.WriteFile(ctx, NetplanConfigPath, []byte(minimalConfig), 0600)
	} else {
		// File exists, but ensure correct permissions to avoid netplan warnings
		nc.executor.ExecuteCommand(ctx, "chmod", "600", NetplanConfigPath)
	}
}

// createTempConfigForValidation creates a temporary directory structure
// with proper netplan hierarchy for validation using --root-dir
func (nc *NetplanCommand) createTempConfigForValidation(ctx context.Context, configData []byte) (string, error) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "netplan-validation-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}

	// Create nested directory structure: tempDir/etc/netplan/
	netplanDir := filepath.Join(tempDir, "etc", "netplan")
	if err := os.MkdirAll(netplanDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to create netplan directory structure: %v", err)
	}

	// Write config file to temp netplan directory
	tempConfigFile := filepath.Join(netplanDir, "90-rodent-netmage.yaml")
	if err := os.WriteFile(tempConfigFile, configData, 0600); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to write temp config file: %v", err)
	}

	return tempDir, nil
}
