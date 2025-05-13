// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package samba

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/services"
	"github.com/stratastor/rodent/internal/services/systemd"
	"github.com/stratastor/rodent/pkg/shares/smb"
)

// Constants for service names
const (
	SMBServiceName     = "smbd"
	WinbindServiceName = "winbind"
	NMBServiceName     = "nmbd"
)

// Client handles interactions with Samba service
type Client struct {
	logger         logger.Logger
	serviceManager *smb.ServiceManager
	systemdClient  *systemd.Client
}

// NewClient creates a new Samba service client
func NewClient(logger logger.Logger) (*Client, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Check for smbcontrol binary to verify Samba is installed
	_, err := exec.LookPath("smbcontrol")
	if err != nil {
		return nil, fmt.Errorf("samba is not available or not in PATH: %w", err)
	}

	// Create systemd client
	systemdClient, err := systemd.NewClient(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create systemd client: %w", err)
	}

	// Create SMB service manager
	smbServiceManager := smb.NewServiceManager(logger)

	return &Client{
		logger:         logger,
		serviceManager: smbServiceManager,
		systemdClient:  systemdClient,
	}, nil
}

// Name returns the name of the service
func (c *Client) Name() string {
	return "samba"
}

// Status returns the current status of all Samba-related services
func (c *Client) Status(ctx context.Context) ([]services.ServiceStatus, error) {
	// Collect statuses from all Samba-related services
	var serviceStatuses []services.ServiceStatus

	// Check SMB status
	smbStatus, err := c.systemdClient.GetServiceStatus(ctx, SMBServiceName)
	if err != nil {
		c.logger.Warn("Failed to get SMB service status", "err", err)
		// Continue with other services
	} else {
		serviceStatuses = append(serviceStatuses, smbStatus)
	}

	// Check Winbind status
	winbindStatus, err := c.systemdClient.GetServiceStatus(ctx, WinbindServiceName)
	if err != nil {
		c.logger.Debug("Failed to get Winbind service status", "err", err)
		// Continue without Winbind
	} else {
		serviceStatuses = append(serviceStatuses, winbindStatus)
	}

	// Check NMB status
	nmbStatus, err := c.systemdClient.GetServiceStatus(ctx, NMBServiceName)
	if err != nil {
		c.logger.Debug("Failed to get NMB service status", "err", err)
		// Continue without NMB
	} else {
		serviceStatuses = append(serviceStatuses, nmbStatus)
	}

	// If we couldn't get any status, return an error
	if len(serviceStatuses) == 0 {
		return nil, fmt.Errorf("failed to get status for any Samba-related services")
	}

	return serviceStatuses, nil
}

// Start starts the Samba service and related services
func (c *Client) Start(ctx context.Context) error {
	// Start SMB service
	if err := c.systemdClient.StartService(ctx, SMBServiceName); err != nil {
		return fmt.Errorf("failed to start SMB service: %w", err)
	}

	// Try to start NMB service (but don't fail if it can't be started)
	if err := c.systemdClient.StartService(ctx, NMBServiceName); err != nil {
		c.logger.Warn("Failed to start NMB service", "err", err)
	}

	// Try to start Winbind service (but don't fail if it can't be started)
	if err := c.systemdClient.StartService(ctx, WinbindServiceName); err != nil {
		c.logger.Warn("Failed to start Winbind service", "err", err)
	}

	return nil
}

// Stop stops the Samba service and related services
func (c *Client) Stop(ctx context.Context) error {
	var errors []error

	// Try to stop Winbind service first
	if err := c.systemdClient.StopService(ctx, WinbindServiceName); err != nil {
		c.logger.Warn("Failed to stop Winbind service", "err", err)
		// Don't fail the operation, just log and continue
	}

	// Try to stop NMB service
	if err := c.systemdClient.StopService(ctx, NMBServiceName); err != nil {
		c.logger.Warn("Failed to stop NMB service", "err", err)
		// Don't fail the operation, just log and continue
	}

	// Stop SMB service
	if err := c.systemdClient.StopService(ctx, SMBServiceName); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop SMB service: %w", err))
	}

	// If we had any errors, return the first one
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// Restart restarts the Samba service and related services
func (c *Client) Restart(ctx context.Context) error {
	// Restart SMB service
	if err := c.systemdClient.RestartService(ctx, SMBServiceName); err != nil {
		return fmt.Errorf("failed to restart SMB service: %w", err)
	}

	// Try to restart NMB service (but don't fail if it can't be restarted)
	if err := c.systemdClient.RestartService(ctx, NMBServiceName); err != nil {
		c.logger.Warn("Failed to restart NMB service", "err", err)
	}

	// Try to restart Winbind service (but don't fail if it can't be restarted)
	if err := c.systemdClient.RestartService(ctx, WinbindServiceName); err != nil {
		c.logger.Warn("Failed to restart Winbind service", "err", err)
	}

	return nil
}

// ReloadConfig reloads the Samba service configuration
func (c *Client) ReloadConfig(ctx context.Context) error {
	// Use the existing SMB service manager's reload functionality
	// which uses smbcontrol for a more graceful configuration reload
	if err := c.serviceManager.ReloadConfig(ctx); err != nil {
		c.logger.Warn(
			"Failed to reload config with smbcontrol, falling back to systemd reload",
			"err",
			err,
		)

		// Fall back to systemd reload
		return c.systemdClient.ReloadService(ctx, SMBServiceName)
	}

	return nil
}

// GetServiceManager returns the underlying SMB service manager
func (c *Client) GetServiceManager() *smb.ServiceManager {
	return c.serviceManager
}

// GetSystemdClient returns the underlying systemd client
func (c *Client) GetSystemdClient() *systemd.Client {
	return c.systemdClient
}
