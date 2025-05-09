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
