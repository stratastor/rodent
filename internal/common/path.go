// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BaseDir is the directory where the binary is located
// It's set at initialization in main.go
var BaseDir string

// ResolvePath resolves a path relative to the application
func ResolvePath(relativePath string) string {
	// If path is absolute, return it unchanged
	if filepath.IsAbs(relativePath) {
		return relativePath
	}
	// Check if path exists relative to BaseDir
	path := filepath.Join(BaseDir, relativePath)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// Try development path (one level up from binary)
	path = filepath.Join(BaseDir, "..", relativePath)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// Return the original path joined with BaseDir as fallback
	return filepath.Join(BaseDir, relativePath)
}

// ExpandPath expands a path with tilde (~) to the user's home directory
func ExpandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user's home directory: %w", err)
	}

	return filepath.Join(homeDir, path[1:]), nil
}

// GetConfigDir returns the appropriate configuration directory
// If running as root, it returns the system config directory
// Otherwise, it returns the user config directory
func GetConfigDir() (string, error) {
	// If running as root, use system config directory
	if os.Geteuid() == 0 {
		return "/etc/rodent", nil
	}

	// Otherwise, use user config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".rodent"), nil
}

// EnsureDir ensures a directory exists, creating it if necessary
func EnsureDir(path string, perm os.FileMode) error {
	// Expand path if it contains tilde
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(expandedPath, perm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", expandedPath, err)
	}

	return nil
}