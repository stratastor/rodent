// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"github.com/stratastor/rodent/pkg/disk/types"
)

// ============================================================================
// Configuration Operations (Priority 1 API)
// ============================================================================

// GetConfig returns the current disk manager configuration
func (m *Manager) GetConfig() *types.DiskManagerConfig {
	return m.configManager.Get()
}

// UpdateConfig updates the disk manager configuration
func (m *Manager) UpdateConfig(config *types.DiskManagerConfig) error {
	// Validate and update configuration
	if err := m.configManager.Update(config); err != nil {
		return err
	}

	m.logger.Info("configuration updated")

	return nil
}

// ReloadConfig reloads configuration from disk
func (m *Manager) ReloadConfig() error {
	if err := m.configManager.Load(); err != nil {
		return err
	}

	m.logger.Info("configuration reloaded from disk")

	return nil
}
