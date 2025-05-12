// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package addc

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/stratastor/logger"
	rodentCfg "github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/internal/services"
	"github.com/stratastor/rodent/internal/services/config"
	"github.com/stratastor/rodent/internal/services/docker"
	"github.com/stratastor/rodent/internal/templates"
	tglClient "github.com/stratastor/rodent/internal/toggle/client"
)

var (
	// Runtime paths for files
	servicesDir            string
	defaultAdDcComposePath string

	// Template file names (no paths needed as they are embedded)
	adDcComposeTemplate = "dc-addc.yml.tmpl"
)

func init() {
	servicesDir = rodentCfg.GetServicesDir()
	defaultAdDcComposePath = servicesDir + "/addc/dc-addc.yml"
}

// AdDcConfig contains configuration data for AD DC
type AdDcConfig struct {
	ContainerName string
	Hostname      string
	Realm         string
	Domain        string
	AdminPassword string
	DnsForwarder  string
	EtcVolume     string
	PrivateVolume string
	VarVolume     string
}

// Client handles interactions with AD DC
type Client struct {
	logger        logger.Logger
	dockerSvc     *docker.Client
	composeFile   string
	configManager *config.ServiceConfigManager
	toggleClient  *tglClient.Client // Optional, for reporting
}

// NewClient creates a new AD DC client
func NewClient(logger logger.Logger) (*Client, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Create Docker client
	dockerSvc, err := docker.NewClient(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Create config manager
	configManager := config.NewServiceConfigManager(logger)

	// Get embedded templates
	composeTemplate, err := templates.GetAddcTemplate(adDcComposeTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to load compose template: %w", err)
	}

	// Output paths should still be resolved since they're written to disk
	composePath := defaultAdDcComposePath

	// Register templates with embedded content
	configManager.RegisterTemplate("addc-compose", &config.ConfigTemplate{
		Name:        "addc-compose",
		Content:     composeTemplate,
		OutputPath:  composePath,
		Permissions: 0644,
		BackupPath:  composePath + ".bak",
	})

	// Create client
	client := &Client{
		logger:        logger,
		dockerSvc:     dockerSvc,
		composeFile:   defaultAdDcComposePath,
		configManager: configManager,
	}

	// Optional: Register state callback for reporting to Toggle
	cfg := rodentCfg.GetConfig()
	if cfg.StrataSecure && cfg.Toggle.JWT != "" {
		toggleClient, err := tglClient.NewClient(logger, cfg.Toggle.JWT, cfg.Toggle.BaseURL)
		if err != nil {
			logger.Warn("Failed to create Toggle client, service reporting disabled", "err", err)
		} else {
			client.toggleClient = toggleClient
			configManager.RegisterStateCallback(client.reportConfigChange)
		}
	}

	return client, nil
}

// Name returns the name of the service
func (c *Client) Name() string {
	return "addc"
}

// Status returns the current status of AD DC with detailed information
func (c *Client) Status(ctx context.Context) ([]services.ServiceStatus, error) {
	// Get detailed container information if running
	containers, err := c.dockerSvc.ComposeStatus(ctx, c.composeFile, "addc")
	if err != nil {
		c.logger.Warn("Failed to get detailed container status", "err", err)
		return []services.ServiceStatus{}, nil
	}

	return containers, nil
}

// Start starts the AD DC service
func (c *Client) Start(ctx context.Context) error {
	// Before starting, ensure config is up to date
	if err := c.UpdateConfig(ctx, nil); err != nil {
		return fmt.Errorf("failed to update configuration before starting: %w", err)
	}

	return c.dockerSvc.ComposeUp(ctx, c.composeFile, true)
}

// Stop stops the AD DC service
func (c *Client) Stop(ctx context.Context) error {
	return c.dockerSvc.ComposeDown(ctx, c.composeFile, false)
}

// Restart restarts the AD DC service
func (c *Client) Restart(ctx context.Context) error {
	return c.dockerSvc.ComposeRestart(ctx, c.composeFile)
}

// UpdateConfig updates the configuration files for AD DC
// If config is nil, it will use the values from the global config
func (c *Client) UpdateConfig(ctx context.Context, config *AdDcConfig) error {
	var adDcConfig AdDcConfig

	if config == nil {
		// Use config from the global config if none provided
		cfg := rodentCfg.GetConfig()
		adDcConfig = AdDcConfig{
			ContainerName: cfg.AD.DC.ContainerName,
			Hostname:      cfg.AD.DC.Hostname,
			Realm:         cfg.AD.DC.Realm,
			Domain:        cfg.AD.DC.Domain,
			AdminPassword: cfg.AD.AdminPassword,
			DnsForwarder:  cfg.AD.DC.DnsForwarder,
			EtcVolume:     cfg.AD.DC.EtcVolume,
			PrivateVolume: cfg.AD.DC.PrivateVolume,
			VarVolume:     cfg.AD.DC.VarVolume,
		}
	} else {
		// Use the provided config
		adDcConfig = *config
	}

	// Ensure directory exists
	if err := common.EnsureDir(filepath.Dir(c.composeFile), 0755); err != nil {
		return fmt.Errorf("failed to create directory for AD DC: %w", err)
	}

	// Update docker-compose with the configuration
	if err := c.configManager.UpdateConfig(ctx, "addc-compose", adDcConfig); err != nil {
		return fmt.Errorf("failed to update docker-compose configuration: %w", err)
	}

	c.logger.Info("Successfully updated AD DC configuration",
		"realm", adDcConfig.Realm,
		"domain", adDcConfig.Domain,
		"container", adDcConfig.ContainerName)

	return nil
}

// reportConfigChange reports configuration changes to the Toggle service
func (c *Client) reportConfigChange(
	ctx context.Context,
	serviceName string,
	state config.ServiceState,
) error {
	if c.toggleClient == nil {
		return nil // Toggle reporting disabled
	}

	// Report to Toggle service
	return c.toggleClient.ReportServiceConfigChange(ctx, serviceName, tglClient.ConfigChangeData{
		ServiceName: serviceName,
		ConfigPath:  state.ConfigPath,
		UpdatedAt:   state.UpdatedAt,
		Status:      state.Status,
	})
}
