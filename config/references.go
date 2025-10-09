// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	configDir    string // Directory for configuration files
	servicesDir  string // Directory for service configurations
	keysDir      string // Directory for keys
	sshDir       string // Directory for SSH configurations
	transfersDir string // Directory for managing ZFS dataset transfers
	eventsDir    string // Directory for event logs
	diskDir      string // Directory for disk manager state and config
)

func init() {
	if os.Geteuid() == 0 {
		configDir = "/etc/rodent"
	}

	// Otherwise, use user config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home directory: %v", err))
	}

	configDir = filepath.Join(homeDir, ".rodent")
	servicesDir = filepath.Join(configDir, "services")
	keysDir = filepath.Join(configDir, "keys")
	sshDir = filepath.Join(keysDir, "ssh")
	transfersDir = filepath.Join(configDir, "transfers")
	eventsDir = filepath.Join(configDir, "events")
	diskDir = filepath.Join(configDir, "disk")

	// Ensure the directories exist
	if err := EnsureDirectories(); err != nil {
		panic(fmt.Sprintf("failed to ensure configuration directories: %v", err))
	}
}

// GetConfigDir returns the appropriate configuration directory
// If running as root, it returns the system config directory
// Otherwise, it returns the user config directory
func GetConfigDir() string {
	return configDir
}

// GetServicesDir returns the directory for service configurations
func GetServicesDir() string {
	return servicesDir
}

// GetKeysDir returns the directory for keys
func GetKeysDir() string {
	return keysDir
}

// GetSSHDir returns the directory for SSH configurations
func GetSSHDir() string {
	return sshDir
}

// GetTransfersDir returns the directory for managing ZFS dataset transfers
func GetTransfersDir() string {
	return transfersDir
}

// GetEventsDir returns the directory for event logs
func GetEventsDir() string {
	return eventsDir
}

// GetDiskDir returns the directory for disk manager state and config
func GetDiskDir() string {
	return diskDir
}

// EnsureDirectories creates necessary directories if they do not exist
func EnsureDirectories() error {
	dirs := []string{
		configDir,
		servicesDir,
		keysDir,
		sshDir,
		transfersDir,
		eventsDir,
		diskDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
