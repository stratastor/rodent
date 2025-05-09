// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package traefik

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/stratastor/logger"
	rodentCfg "github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/services"
	"github.com/stratastor/rodent/internal/services/config"
	"github.com/stratastor/rodent/internal/services/docker"
	"github.com/stratastor/rodent/internal/templates"
	tglClient "github.com/stratastor/rodent/internal/toggle/client"
)

var (
	servicesDir string

	// Template file names (no paths needed as they are embedded)
	traefikComposeTemplate = "dc-traefik.yml.tmpl"
	traefikConfigTemplate  = "config.yml.tmpl"
	traefikTLSTemplate     = "tls.yml.tmpl"

	// Runtime paths for files
	defaultTraefikCertDir     = servicesDir + "/traefik/certs"
	defaultTraefikTLSPath     = servicesDir + "/traefik/tls.yml"
	defaultTraefikComposePath = servicesDir + "/traefik/dc-traefik.yml"
	defaultTraefikConfigPath  = servicesDir + "/traefik/config.yml"
)

func init() {
	servicesDir = rodentCfg.GetServicesDir()
}

// CertificateData contains information about a TLS certificate
type CertificateData struct {
	Domain      string    `yaml:"domain"`
	Certificate string    `yaml:"certificate"`
	PrivateKey  string    `yaml:"privateKey"`
	ExpiresOn   time.Time `yaml:"expiresOn"`
}

// TraefikConfig contains configuration data for Traefik
type TraefikConfig struct {
	Domain              string
	EnableTLS           bool
	CertificateData     *CertificateData
	CorsAllowedOrigins  string
	TraefikApiInsecure  bool
	TraefikDashboard    bool
	TraefikHttpPort     int
	TraefikHttpsPort    int
	EnableToggleReports bool
}

// Client handles interactions with Traefik
type Client struct {
	logger        logger.Logger
	dockerSvc     *docker.Client
	composeFile   string
	configManager *config.ServiceConfigManager
	toggleClient  *tglClient.Client // Optional, for reporting
}

// NewClient creates a new Traefik client
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
	composeTemplate, err := templates.GetTraefikTemplate(traefikComposeTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to load compose template: %w", err)
	}

	configTemplate, err := templates.GetTraefikTemplate(traefikConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to load config template: %w", err)
	}

	tlsTemplate, err := templates.GetTraefikTemplate(traefikTLSTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS template: %w", err)
	}

	// Output paths should still be resolved since they're written to disk
	composePath := defaultTraefikComposePath
	configPath := defaultTraefikConfigPath
	tlsPath := defaultTraefikTLSPath

	// Register templates with embedded content
	configManager.RegisterTemplate("traefik-compose", &config.ConfigTemplate{
		Name:        "traefik-compose",
		Content:     composeTemplate,
		OutputPath:  composePath,
		Permissions: 0644,
		BackupPath:  composePath + ".bak",
	})

	configManager.RegisterTemplate("traefik-config", &config.ConfigTemplate{
		Name:        "traefik-config",
		Content:     configTemplate,
		OutputPath:  configPath,
		Permissions: 0644,
		BackupPath:  configPath + ".bak",
	})

	configManager.RegisterTemplate("traefik-tls", &config.ConfigTemplate{
		Name:        "traefik-tls",
		Content:     tlsTemplate,
		OutputPath:  tlsPath,
		Permissions: 0644,
		BackupPath:  tlsPath + ".bak",
	})

	// Create client
	client := &Client{
		logger:        logger,
		dockerSvc:     dockerSvc,
		composeFile:   defaultTraefikComposePath,
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
	return "traefik"
}

// Status returns the current status of Traefik with detailed information
func (c *Client) Status(ctx context.Context) ([]services.ServiceStatus, error) {
	// Get detailed container information if running
	containers, err := c.dockerSvc.ComposeStatus(ctx, c.composeFile, "traefik")
	if err != nil {
		c.logger.Warn("Failed to get detailed container status", "err", err)
		return []services.ServiceStatus{}, nil
	}

	return containers, nil
}

// Start starts the Traefik service
func (c *Client) Start(ctx context.Context) error {
	return c.dockerSvc.ComposeUp(ctx, c.composeFile, true)
}

// Stop stops the Traefik service
func (c *Client) Stop(ctx context.Context) error {
	return c.dockerSvc.ComposeDown(ctx, c.composeFile, false)
}

// Restart restarts the Traefik service
func (c *Client) Restart(ctx context.Context) error {
	return c.dockerSvc.ComposeRestart(ctx, c.composeFile)
}

// InstallCertificate installs a TLS certificate for Traefik
func (c *Client) InstallCertificate(ctx context.Context, certData CertificateData) error {
	// Write certificate to file
	certFile := filepath.Join(defaultTraefikCertDir, certData.Domain+".pem")
	if err := services.WriteFileWithPerms(certFile, []byte(certData.Certificate), 0644); err != nil {
		return fmt.Errorf("failed to write certificate file: %w", err)
	}
	c.logger.Debug("Wrote certificate to file", "file", certFile)

	// Write private key to file
	keyFile := filepath.Join(defaultTraefikCertDir, certData.Domain+".key")
	if err := services.WriteFileWithPerms(keyFile, []byte(certData.PrivateKey), 0600); err != nil {
		return fmt.Errorf("failed to write private key file: %w", err)
	}
	c.logger.Debug("Wrote private key to file", "file", keyFile)

	// Update TLS configuration using config manager
	traefikConfig := TraefikConfig{
		Domain:              certData.Domain,
		EnableTLS:           true,
		CertificateData:     &certData,
		CorsAllowedOrigins:  fmt.Sprintf("https://%s", certData.Domain),
		TraefikApiInsecure:  false, // Secure defaults
		TraefikDashboard:    false, // Disable dashboard by default
		TraefikHttpPort:     80,    // Default ports
		TraefikHttpsPort:    443,
		EnableToggleReports: c.toggleClient != nil,
	}

	// Update both TLS config and docker-compose file
	if err := c.configManager.UpdateConfig(ctx, "traefik-tls", certData); err != nil {
		return fmt.Errorf("failed to update TLS configuration: %w", err)
	}

	// Update docker-compose with the new domain and TLS settings
	if err := c.configManager.UpdateConfig(ctx, "traefik-compose", traefikConfig); err != nil {
		return fmt.Errorf("failed to update docker-compose configuration: %w", err)
	}

	// Update main traefik config with CORS settings, etc.
	if err := c.configManager.UpdateConfig(ctx, "traefik-config", traefikConfig); err != nil {
		c.logger.Warn("Failed to update Traefik config", "err", err)
		// Continue despite config update failure
	}

	c.logger.Info("Successfully installed certificate",
		"domain", certData.Domain,
		"certFile", certFile,
		"keyFile", keyFile,
		"expiresOn", certData.ExpiresOn)

	// Restart Traefik to apply changes
	err := c.Restart(ctx)
	if err != nil {
		c.logger.Warn("Failed to restart Traefik after certificate installation", "err", err)
	}

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
