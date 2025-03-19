// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package traefik

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"text/template"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/constants"
	"github.com/stratastor/rodent/internal/services"
	"github.com/stratastor/rodent/internal/services/docker"
)

// TLS configuration template
const tlsConfigTemplate = `# TLS Configuration
tls:
  certificates:
    - certFile: /certs/{{.Domain}}.pem
      keyFile: /certs/{{.Domain}}.key
      
  options:
    default:
      minVersion: VersionTLS12
      maxVersion: VersionTLS13
      sniStrict: true
      cipherSuites:
        # TLS 1.3 ciphers
        - TLS_AES_256_GCM_SHA384
        - TLS_AES_128_GCM_SHA256
        - TLS_CHACHA20_POLY1305_SHA256
        # TLS 1.2 ciphers (only GCM and CHACHA20)
        - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
        - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
        - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
        - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
        - TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256
        - TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
  stores:
    default:
      defaultCertificate:
        certFile: /certs/{{.Domain}}.pem
        keyFile: /certs/{{.Domain}}.key
`

// CertificateData contains information about a TLS certificate
type CertificateData struct {
	Domain      string    `yaml:"domain"`
	Certificate string    `yaml:"certificate"`
	PrivateKey  string    `yaml:"privateKey"`
	ExpiresOn   time.Time `yaml:"expiresOn"`
}

// Client handles interactions with Traefik
type Client struct {
	logger      logger.Logger
	dockerSvc   *docker.Client
	composeFile string
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

	return &Client{
		logger:      logger,
		dockerSvc:   dockerSvc,
		composeFile: constants.DefaultTraefikComposePath,
	}, nil
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
	certFile := filepath.Join(constants.DefaultTraefikCertDir, certData.Domain+".pem")
	if err := services.WriteFileWithPerms(certFile, []byte(certData.Certificate), 0644); err != nil {
		return fmt.Errorf("failed to write certificate file: %w", err)
	}

	// Write private key to file
	keyFile := filepath.Join(constants.DefaultTraefikCertDir, certData.Domain+".key")
	if err := services.WriteFileWithPerms(keyFile, []byte(certData.PrivateKey), 0600); err != nil {
		return fmt.Errorf("failed to write private key file: %w", err)
	}

	// Update TLS configuration
	if err := c.updateTLSConfig(certData); err != nil {
		return fmt.Errorf("failed to update TLS configuration: %w", err)
	}

	c.logger.Info("Successfully installed certificate",
		"domain", certData.Domain,
		"certFile", certFile,
		"keyFile", keyFile,
		"expiresOn", certData.ExpiresOn)

	// Restart Traefik to apply changes
	// Note: In production this could be optional or delayed
	err := c.Restart(ctx)
	if err != nil {
		c.logger.Warn("Failed to restart Traefik after certificate installation", "err", err)
	}

	return nil
}

// updateTLSConfig updates the Traefik TLS configuration file
func (c *Client) updateTLSConfig(certData CertificateData) error {
	// Create a template
	tmpl, err := template.New("tlsConfig").Parse(tlsConfigTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse TLS config template: %w", err)
	}

	// Render the template with certificate data
	var tlsConfig bytes.Buffer
	if err := tmpl.Execute(&tlsConfig, certData); err != nil {
		return fmt.Errorf("failed to render TLS config template: %w", err)
	}

	// Write the rendered template to the TLS config file using the common function
	if err := services.WriteFileWithPerms(constants.DefaultTraefikTLSPath, tlsConfig.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write TLS config file: %w", err)
	}

	c.logger.Info("Updated Traefik TLS configuration", "path", constants.DefaultTraefikTLSPath)

	return nil
}
