// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package shares

import (
	"context"
	"time"
)

// ShareType represents the type of a share
type ShareType string

const (
	ShareTypeSMB   ShareType = "smb"
	ShareTypeNFS   ShareType = "nfs"
	ShareTypeISCSI ShareType = "iscsi"
)

// ShareStatus represents the status of a share
type ShareStatus string

const (
	ShareStatusActive   ShareStatus = "active"
	ShareStatusInactive ShareStatus = "inactive"
	ShareStatusError    ShareStatus = "error"
)

// ShareConfig contains common configuration for all share types
type ShareConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Path        string            `json:"path"`
	Type        ShareType         `json:"type"`
	Enabled     bool              `json:"enabled"`
	Status      ShareStatus       `json:"status"`
	Created     time.Time         `json:"created"`
	Modified    time.Time         `json:"modified"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// ShareStats represents statistics for a share
type ShareStats struct {
	ActiveConnections int         `json:"active_connections"`
	OpenFiles         int         `json:"open_files"`
	LastAccessed      time.Time   `json:"last_accessed,omitempty"`
	Status            ShareStatus `json:"status"`
	ConfModified      time.Time   `json:"conf_modified"`
}

// SharesManager is the interface that manages shares
type SharesManager interface {
	// Get all shares
	ListShares(ctx context.Context) ([]ShareConfig, error)

	// Get shares by type
	ListSharesByType(ctx context.Context, shareType ShareType) ([]ShareConfig, error)

	// Get a share by name
	GetShare(ctx context.Context, name string) (*ShareConfig, error)

	// Create a new share
	CreateShare(ctx context.Context, config interface{}) error

	// Update an existing share
	UpdateShare(ctx context.Context, name string, config interface{}) error

	// Delete a share
	DeleteShare(ctx context.Context, name string) error

	// Get statistics for a share
	GetShareStats(ctx context.Context, name string) (*ShareStats, error)

	// Check if a share exists
	Exists(ctx context.Context, name string) (bool, error)

	// Reload service configuration
	ReloadConfig(ctx context.Context) error
}

// ServiceManager manages share service operations
type ServiceManager interface {
	// Start the service
	Start(ctx context.Context) error

	// Stop the service
	Stop(ctx context.Context) error

	// Restart the service
	Restart(ctx context.Context) error

	// Get service status
	Status(ctx context.Context) (string, error)

	// Reload service configuration
	ReloadConfig(ctx context.Context) error
}
