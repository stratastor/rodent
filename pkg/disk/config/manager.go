// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFileName = "disk-manager.yaml"
)

// ConfigManager manages disk manager configuration with YAML persistence
type ConfigManager struct {
	logger     logger.Logger
	configPath string
	config     *types.DiskManagerConfig
	mu         sync.RWMutex
}

// NewConfigManager creates a new config manager
func NewConfigManager(l logger.Logger) *ConfigManager {
	configPath := filepath.Join(config.GetDiskDir(), DefaultConfigFileName)

	return &ConfigManager{
		logger:     l,
		configPath: configPath,
		config:     types.DefaultDiskManagerConfig(),
	}
}

// Load loads configuration from disk
func (cm *ConfigManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if config file exists
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		cm.logger.Info("config file not found, creating default",
			"path", cm.configPath)

		// Create default config file
		if err := cm.saveUnlocked(); err != nil {
			return errors.Wrap(err, errors.DiskConfigLoadFailed).
				WithMetadata("path", cm.configPath).
				WithMetadata("operation", "create_default")
		}

		return nil
	}

	// Read config file
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return errors.Wrap(err, errors.DiskConfigLoadFailed).
			WithMetadata("path", cm.configPath)
	}

	// Parse YAML
	var cfg types.DiskManagerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return errors.Wrap(err, errors.DiskConfigLoadFailed).
			WithMetadata("path", cm.configPath).
			WithMetadata("operation", "parse_yaml")
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return errors.Wrap(err, errors.DiskConfigInvalid).
			WithMetadata("path", cm.configPath)
	}

	cm.config = &cfg
	cm.logger.Info("configuration loaded successfully",
		"path", cm.configPath)

	return nil
}

// Save saves configuration to disk
func (cm *ConfigManager) Save() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return cm.saveUnlocked()
}

// saveUnlocked saves without acquiring lock (caller must hold lock)
func (cm *ConfigManager) saveUnlocked() error {
	// Validate before saving
	if err := cm.config.Validate(); err != nil {
		return errors.Wrap(err, errors.DiskConfigInvalid).
			WithMetadata("path", cm.configPath)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cm.config)
	if err != nil {
		return errors.Wrap(err, errors.DiskConfigSaveFailed).
			WithMetadata("path", cm.configPath).
			WithMetadata("operation", "marshal_yaml")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cm.configPath), 0755); err != nil {
		return errors.Wrap(err, errors.DiskConfigSaveFailed).
			WithMetadata("path", filepath.Dir(cm.configPath)).
			WithMetadata("operation", "mkdir")
	}

	// Write to temporary file first
	tempPath := cm.configPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return errors.Wrap(err, errors.DiskConfigSaveFailed).
			WithMetadata("path", tempPath).
			WithMetadata("operation", "write_temp")
	}

	// Atomic rename
	if err := os.Rename(tempPath, cm.configPath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return errors.Wrap(err, errors.DiskConfigSaveFailed).
			WithMetadata("path", cm.configPath).
			WithMetadata("operation", "rename")
	}

	cm.logger.Debug("configuration saved", "path", cm.configPath)

	return nil
}

// Get returns a copy of the current configuration
func (cm *ConfigManager) Get() *types.DiskManagerConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a copy to prevent external modification
	cfg := *cm.config
	return &cfg
}

// Update updates the configuration and saves it
func (cm *ConfigManager) Update(cfg *types.DiskManagerConfig) error {
	// Validate first
	if err := cfg.Validate(); err != nil {
		return errors.Wrap(err, errors.DiskConfigInvalid).
			WithMetadata("path", cm.configPath)
	}

	cm.mu.Lock()
	cm.config = cfg
	cm.mu.Unlock()

	return cm.Save()
}

// UpdateDiscovery updates discovery configuration
func (cm *ConfigManager) UpdateDiscovery(discovery types.DiscoveryConfig) error {
	cm.mu.Lock()
	cm.config.Discovery = discovery
	cm.mu.Unlock()

	return cm.Save()
}

// UpdateMonitoring updates monitoring configuration
func (cm *ConfigManager) UpdateMonitoring(monitoring types.MonitoringConfig) error {
	cm.mu.Lock()
	cm.config.Monitoring = monitoring
	cm.mu.Unlock()

	return cm.Save()
}

// UpdateProbing updates probing configuration
func (cm *ConfigManager) UpdateProbing(probing types.ProbingConfig) error {
	cm.mu.Lock()
	cm.config.Probing = probing
	cm.mu.Unlock()

	return cm.Save()
}

// UpdateNaming updates naming configuration
func (cm *ConfigManager) UpdateNaming(naming types.NamingConfig) error {
	cm.mu.Lock()
	cm.config.Naming = naming
	cm.mu.Unlock()

	return cm.Save()
}

// Reload reloads configuration from disk
func (cm *ConfigManager) Reload() error {
	return cm.Load()
}

// GetConfigPath returns the config file path
func (cm *ConfigManager) GetConfigPath() string {
	return cm.configPath
}
