// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

// Package managers provides a centralized registry for shared manager instances.
// This ensures both HTTP routes (pkg/server) and gRPC handlers (internal/toggle)
// use the same manager instances, avoiding duplicate managers and race conditions.
//
// Usage:
//   - HTTP routes (routes.go) call Set* functions after creating managers
//   - gRPC handlers (handlers.go) call Get* functions to retrieve existing managers
//   - Get* functions return nil if the manager hasn't been set yet
package managers

import (
	"sync"

	"github.com/stratastor/rodent/pkg/zfs/autosnapshots"
	"github.com/stratastor/rodent/pkg/zfs/autotransfers"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
)

var (
	mu sync.RWMutex

	// Core ZFS managers
	datasetManager  *dataset.Manager
	transferManager *dataset.TransferManager

	// Policy managers
	snapshotManager       *autosnapshots.Manager
	transferPolicyManager *autotransfers.Manager
)

// SetDatasetManager sets the shared dataset manager instance
func SetDatasetManager(m *dataset.Manager) {
	mu.Lock()
	defer mu.Unlock()
	datasetManager = m
}

// GetDatasetManager returns the shared dataset manager, or nil if not set
func GetDatasetManager() *dataset.Manager {
	mu.RLock()
	defer mu.RUnlock()
	return datasetManager
}

// SetTransferManager sets the shared transfer manager instance
func SetTransferManager(m *dataset.TransferManager) {
	mu.Lock()
	defer mu.Unlock()
	transferManager = m
}

// GetTransferManager returns the shared transfer manager, or nil if not set
func GetTransferManager() *dataset.TransferManager {
	mu.RLock()
	defer mu.RUnlock()
	return transferManager
}

// SetSnapshotManager sets the shared snapshot policy manager instance
func SetSnapshotManager(m *autosnapshots.Manager) {
	mu.Lock()
	defer mu.Unlock()
	snapshotManager = m
}

// GetSnapshotManager returns the shared snapshot policy manager, or nil if not set
func GetSnapshotManager() *autosnapshots.Manager {
	mu.RLock()
	defer mu.RUnlock()
	return snapshotManager
}

// SetTransferPolicyManager sets the shared transfer policy manager instance
func SetTransferPolicyManager(m *autotransfers.Manager) {
	mu.Lock()
	defer mu.Unlock()
	transferPolicyManager = m
}

// GetTransferPolicyManager returns the shared transfer policy manager, or nil if not set
func GetTransferPolicyManager() *autotransfers.Manager {
	mu.RLock()
	defer mu.RUnlock()
	return transferPolicyManager
}
