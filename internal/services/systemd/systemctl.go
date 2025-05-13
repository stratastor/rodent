// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package systemd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/services"
)

// ServiceStatus represents systemd service status information
type ServiceStatus struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	Status  string `json:"status"`
	Health  string `json:"health"`
	State   string `json:"state"`
}

// Verify that ServiceStatus implements ServiceStatus interface
var _ services.ServiceStatus = (*ServiceStatus)(nil)

func (s ServiceStatus) String() string {
	return fmt.Sprintf(
		"%s (%s) is %s [%s]",
		s.Name,
		s.Service,
		s.State,
		s.Status,
	)
}

func (s ServiceStatus) InstanceGist() string {
	return s.String()
}

func (s ServiceStatus) InstanceName() string {
	return s.Name
}

func (s ServiceStatus) InstanceService() string {
	return s.Service
}

func (s ServiceStatus) InstanceStatus() string {
	return s.Status
}

func (s ServiceStatus) InstanceHealth() string {
	return s.Health
}

func (s ServiceStatus) InstanceState() string {
	return s.State
}

// Client provides a systemd service management client
type Client struct {
	logger       logger.Logger
	systemctlBin string
}

// NewClient creates a new systemd client
func NewClient(logger logger.Logger) (*Client, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Check if systemctl is installed
	systemctlBin, err := exec.LookPath("systemctl")
	if err != nil {
		return nil, fmt.Errorf("systemctl is not available or not in PATH: %w", err)
	}

	return &Client{
		logger:       logger,
		systemctlBin: systemctlBin,
	}, nil
}

// GetServiceStatus returns the status of a systemd service
func (c *Client) GetServiceStatus(ctx context.Context, serviceName string) (*ServiceStatus, error) {
	// Ensure service name has .service suffix
	serviceUnit := serviceName
	if !strings.HasSuffix(serviceUnit, ".service") {
		serviceUnit = serviceName + ".service"
	}

	// Execute systemctl to get service status
	output, err := command.ExecCommand(
		ctx,
		c.logger,
		c.systemctlBin,
		"status",
		serviceUnit,
		"--no-pager",
	)

	// Parse the systemctl output for status info
	state := "unknown"
	statusFull := string(output)
	status := "Unknown status"
	health := "unknown"

	// If there's an error, check if it's because the service is not active
	if err != nil {
		if strings.Contains(statusFull, "inactive") {
			state = "stopped"
			status = "Inactive (dead)"
			health = "inactive"
			// This is not a real error, so we can clear the error
			err = nil
		} else if strings.Contains(statusFull, "failed") {
			state = "failed"
			status = "Failed"
			health = "failed"
			// Failed status is not a real error for our purposes
			err = nil
		} else {
			c.logger.Warn("Error checking service status",
				"service", serviceName,
				"err", err,
				"output", statusFull)

			status = fmt.Sprintf("Error checking status: %v", err)
			state = "error"
			health = "error"
		}
	} else {
		// Extract just the "Active:" line for the status
		lines := strings.Split(statusFull, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Active:") {
				status = strings.TrimPrefix(line, "Active:")
				status = strings.TrimSpace(status)
				break
			}
		}

		if strings.Contains(statusFull, "Active: active (running)") {
			state = "running"
			health = "healthy"
		} else if strings.Contains(statusFull, "Active: inactive (dead)") {
			state = "stopped"
			health = "inactive"
		} else if strings.Contains(statusFull, "Active: failed") {
			state = "failed"
			health = "failed"
		}
	}

	serviceStatus := &ServiceStatus{
		Name:    serviceName,
		Service: serviceUnit,
		State:   state,
		Status:  status,
		Health:  health,
	}

	return serviceStatus, err
}

// StartService starts a systemd service
func (c *Client) StartService(ctx context.Context, serviceName string) error {
	// Ensure service name has .service suffix
	serviceUnit := serviceName
	if !strings.HasSuffix(serviceUnit, ".service") {
		serviceUnit = serviceName + ".service"
	}

	// Execute systemctl to start the service
	_, err := command.ExecCommand(
		ctx,
		c.logger,
		"sudo", // Using sudo as systemctl start requires root privileges
		c.systemctlBin,
		"start",
		serviceUnit,
	)
	if err != nil {
		return fmt.Errorf("failed to start service %s: %w", serviceName, err)
	}

	return nil
}

// StopService stops a systemd service
func (c *Client) StopService(ctx context.Context, serviceName string) error {
	// Ensure service name has .service suffix
	serviceUnit := serviceName
	if !strings.HasSuffix(serviceUnit, ".service") {
		serviceUnit = serviceName + ".service"
	}

	// Execute systemctl to stop the service
	_, err := command.ExecCommand(
		ctx,
		c.logger,
		"sudo", // Using sudo as systemctl stop requires root privileges
		c.systemctlBin,
		"stop",
		serviceUnit,
	)
	if err != nil {
		return fmt.Errorf("failed to stop service %s: %w", serviceName, err)
	}

	return nil
}

// RestartService restarts a systemd service
func (c *Client) RestartService(ctx context.Context, serviceName string) error {
	// Ensure service name has .service suffix
	serviceUnit := serviceName
	if !strings.HasSuffix(serviceUnit, ".service") {
		serviceUnit = serviceName + ".service"
	}

	// Execute systemctl to restart the service
	_, err := command.ExecCommand(
		ctx,
		c.logger,
		"sudo", // Using sudo as systemctl restart requires root privileges
		c.systemctlBin,
		"restart",
		serviceUnit,
	)
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %w", serviceName, err)
	}

	return nil
}

// ReloadService reloads a systemd service configuration
func (c *Client) ReloadService(ctx context.Context, serviceName string) error {
	// Ensure service name has .service suffix
	serviceUnit := serviceName
	if !strings.HasSuffix(serviceUnit, ".service") {
		serviceUnit = serviceName + ".service"
	}

	// Execute systemctl to reload the service
	_, err := command.ExecCommand(
		ctx,
		c.logger,
		"sudo", // Using sudo as systemctl reload requires root privileges
		c.systemctlBin,
		"reload",
		serviceUnit,
	)
	
	// If reload fails (some services don't support reload), try restart
	if err != nil {
		c.logger.Warn("Service reload failed, attempting restart", 
			"service", serviceName, 
			"err", err)
		
		return c.RestartService(ctx, serviceName)
	}

	return nil
}

// EnableService enables a systemd service to start on boot
func (c *Client) EnableService(ctx context.Context, serviceName string) error {
	// Ensure service name has .service suffix
	serviceUnit := serviceName
	if !strings.HasSuffix(serviceUnit, ".service") {
		serviceUnit = serviceName + ".service"
	}

	// Execute systemctl to enable the service
	_, err := command.ExecCommand(
		ctx,
		c.logger,
		"sudo", // Using sudo as systemctl enable requires root privileges
		c.systemctlBin,
		"enable",
		serviceUnit,
	)
	if err != nil {
		return fmt.Errorf("failed to enable service %s: %w", serviceName, err)
	}

	return nil
}

// DisableService disables a systemd service from starting on boot
func (c *Client) DisableService(ctx context.Context, serviceName string) error {
	// Ensure service name has .service suffix
	serviceUnit := serviceName
	if !strings.HasSuffix(serviceUnit, ".service") {
		serviceUnit = serviceName + ".service"
	}

	// Execute systemctl to disable the service
	_, err := command.ExecCommand(
		ctx,
		c.logger,
		"sudo", // Using sudo as systemctl disable requires root privileges
		c.systemctlBin,
		"disable",
		serviceUnit,
	)
	if err != nil {
		return fmt.Errorf("failed to disable service %s: %w", serviceName, err)
	}

	return nil
}

// IsSystemdService checks if a service is managed by systemd
func (c *Client) IsSystemdService(ctx context.Context, serviceName string) (bool, error) {
	// Ensure service name has .service suffix
	serviceUnit := serviceName
	if !strings.HasSuffix(serviceUnit, ".service") {
		serviceUnit = serviceName + ".service"
	}

	// Execute systemctl to list services and check if the service exists
	output, err := command.ExecCommand(
		ctx,
		c.logger,
		c.systemctlBin,
		"list-unit-files",
		serviceUnit,
	)
	if err != nil {
		// If the command fails, the service likely doesn't exist
		return false, nil
	}

	// Check if service is in the output
	return strings.Contains(string(output), serviceUnit), nil
}
