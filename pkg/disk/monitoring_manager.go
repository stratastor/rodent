// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"github.com/stratastor/rodent/pkg/disk/types"
)

// ============================================================================
// Monitoring Configuration Operations
// ============================================================================

// GetMonitoringConfig returns the current monitoring configuration
func (m *Manager) GetMonitoringConfig() *types.MonitoringConfig {
	config := m.configManager.Get()
	return &config.Monitoring
}

// SetMonitoringConfig updates the monitoring configuration
func (m *Manager) SetMonitoringConfig(monitoring *types.MonitoringConfig) error {
	config := m.configManager.Get()
	config.Monitoring = *monitoring

	if err := m.configManager.Update(config); err != nil {
		return err
	}

	m.logger.Info("monitoring configuration updated")
	return nil
}
