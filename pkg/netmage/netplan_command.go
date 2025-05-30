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
func NewNetplanCommand(executor *CommandExecutor, sudoOps *privilege.SudoFileOperations) *NetplanCommand {
	backupDir := filepath.Join(config.GetConfigDir(), "backup", "netplan")
	
	return &NetplanCommand{
		executor:  executor,
		sudoOps:   sudoOps,
		backupDir: backupDir,
	}
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

	// Write to temporary file first for validation
	tempConfigPath := NetplanConfigPath + ".tmp"
	if err := nc.sudoOps.WriteFile(ctx, tempConfigPath, yamlData, 0644); err != nil {
		return errors.Wrap(err, errors.NetplanFileOperationFailed)
	}

	// Validate the temporary configuration
	if err := nc.validateConfigFile(ctx, tempConfigPath); err != nil {
		// Clean up temporary file on validation failure
		nc.sudoOps.DeleteFile(ctx, tempConfigPath)
		return errors.Wrap(err, errors.NetplanYAMLValidationError)
	}

	// Move temporary file to actual config path
	if err := nc.sudoOps.CopyFile(ctx, tempConfigPath, NetplanConfigPath); err != nil {
		nc.sudoOps.DeleteFile(ctx, tempConfigPath)
		return errors.Wrap(err, errors.NetplanFileOperationFailed)
	}

	// Clean up temporary file
	nc.sudoOps.DeleteFile(ctx, tempConfigPath)

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
func (nc *NetplanCommand) Try(ctx context.Context, timeout time.Duration) (*types.NetplanTryResult, error) {
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
func (nc *NetplanCommand) GetStatus(ctx context.Context, iface string) (*types.NetplanStatus, error) {
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
	result, err := nc.executor.ExecuteCommand(ctx, "netplan", "status", "--all", "--verbose", "-f", "json", "--diff")
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

	// Write to temporary file first for validation
	tempConfigPath := NetplanConfigPath + ".tmp"
	if err := nc.sudoOps.WriteFile(ctx, tempConfigPath, backupData, 0644); err != nil {
		return errors.Wrap(err, errors.NetplanRestoreFailed)
	}

	// Validate the backup configuration
	if err := nc.validateConfigFile(ctx, tempConfigPath); err != nil {
		nc.sudoOps.DeleteFile(ctx, tempConfigPath)
		return errors.Wrap(err, errors.NetplanRestoreFailed).WithMetadata("reason", "backup validation failed")
	}

	// Move temporary file to actual config path
	if err := nc.sudoOps.CopyFile(ctx, tempConfigPath, NetplanConfigPath); err != nil {
		nc.sudoOps.DeleteFile(ctx, tempConfigPath)
		return errors.Wrap(err, errors.NetplanRestoreFailed)
	}

	// Clean up temporary file
	nc.sudoOps.DeleteFile(ctx, tempConfigPath)

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
		args = []string{"--root-dir", "/", "get", "all"}
	} else {
		// Validate all current configurations
		args = []string{"get", "all"}
	}

	result, err := nc.executor.ExecuteCommand(ctx, "netplan", args...)
	if err != nil {
		return errors.Wrap(err, errors.NetplanYAMLValidationError).WithMetadata("output", result.Stderr)
	}
	return nil
}