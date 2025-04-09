// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package smb

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
)

// ServiceManager implements SMB service management
type ServiceManager struct {
	logger logger.Logger
}

// NewServiceManager creates a new SMB service manager
func NewServiceManager(logger logger.Logger) *ServiceManager {
	return &ServiceManager{
		logger: logger,
	}
}

// Start starts the SMB service
func (m *ServiceManager) Start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sudo", "systemctl", "start", "smbd")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, errors.SharesServiceFailed).
			WithMetadata("operation", "start").
			WithMetadata("service", "smbd")
	}

	// Also start winbind if available
	winbindCmd := exec.CommandContext(ctx, "sudo", "systemctl", "start", "winbind")
	if err := winbindCmd.Run(); err != nil {
		m.logger.Warn("Failed to start winbind service", "error", err)
	}

	// Verify service is running
	return m.waitForService(ctx, true)
}

// Stop stops the SMB service
func (m *ServiceManager) Stop(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sudo", "systemctl", "stop", "smbd")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, errors.SharesServiceFailed).
			WithMetadata("operation", "stop").
			WithMetadata("service", "smbd")
	}

	// Verify service is stopped
	return m.waitForService(ctx, false)
}

// Restart restarts the SMB service
func (m *ServiceManager) Restart(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sudo", "systemctl", "restart", "smbd")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, errors.SharesServiceFailed).
			WithMetadata("operation", "restart").
			WithMetadata("service", "smbd")
	}

	// Also restart winbind if available
	winbindCmd := exec.CommandContext(ctx, "sudo", "systemctl", "restart", "winbind")
	if err := winbindCmd.Run(); err != nil {
		m.logger.Warn("Failed to restart winbind service", "error", err)
	}

	// Verify service is running
	return m.waitForService(ctx, true)
}

// Status returns the status of the SMB service
func (m *ServiceManager) Status(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "status", "smbd")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Not necessarily an error, could be inactive
		if strings.Contains(string(out), "inactive") {
			return "inactive", nil
		}

		return "", errors.Wrap(err, errors.SharesServiceFailed).
			WithMetadata("operation", "status").
			WithMetadata("service", "smbd")
	}

	// Parse status output
	status := string(out)
	if strings.Contains(status, "Active: active") {
		return "active", nil
	} else if strings.Contains(status, "Active: inactive") {
		return "inactive", nil
	} else if strings.Contains(status, "Active: failed") {
		return "failed", nil
	}

	return "unknown", nil
}

// ReloadConfig reloads the SMB service configuration
func (m *ServiceManager) ReloadConfig(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sudo", "smbcontrol", "smbd", "reload-config")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, errors.SharesServiceFailed).
			WithMetadata("operation", "reload_config").
			WithMetadata("service", "smbd")
	}

	return nil
}

// waitForService waits for the SMB service to reach the desired state
func (m *ServiceManager) waitForService(ctx context.Context, running bool) error {
	maxWait := 10 * time.Second
	interval := 500 * time.Millisecond

	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		status, err := m.Status(ctx)
		if err != nil {
			return err
		}

		if running && status == "active" {
			return nil
		} else if !running && status == "inactive" {
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.New(errors.SharesServiceFailed, "Context canceled while waiting for service state change").
				WithMetadata("service", "smbd").
				WithMetadata("desired_state", fmt.Sprintf("%v", running))
		case <-time.After(interval):
			// Continue waiting
		}
	}

	state := "running"
	if !running {
		state = "stopped"
	}

	return errors.New(errors.SharesServiceFailed, "Timed out waiting for service to be "+state).
		WithMetadata("service", "smbd")
}
