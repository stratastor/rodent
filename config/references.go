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
	configDir   string // Directory for configuration files
	servicesDir string // Directory for service configurations
	keysDir     string // Directory for keys
	sshDir      string // Directory for SSH configurations
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
