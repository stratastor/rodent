// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

// Package privilege provides controlled access to privileged operations
package privilege

import (
	"context"
	"io/fs"
)

// FileOperations defines operations that may require root privileges
type FileOperations interface {
	// ReadFile reads a file that may require elevated privileges
	ReadFile(ctx context.Context, path string) ([]byte, error)
	
	// WriteFile writes to a file that may require elevated privileges
	WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error
	
	// AppendFile appends to a file that may require elevated privileges
	AppendFile(ctx context.Context, path string, data []byte) error
	
	// DeleteFile removes a file that may require elevated privileges
	DeleteFile(ctx context.Context, path string) error
	
	// CopyFile copies a file with potential privilege elevation
	CopyFile(ctx context.Context, src, dst string) error
	
	// Exists checks if a privileged file exists
	Exists(ctx context.Context, path string) (bool, error)
	
	// ExecuteCommand runs a command with elevated privileges
	ExecuteCommand(ctx context.Context, command string, args ...string) ([]byte, error)
}