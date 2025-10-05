// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/services"
)

// Docker service constants
const (
	DefaultDockerComposeDir = "/home/ubuntu/rodent/scripts"
)

// ContainerStatus represents Docker container status information
type ContainerStatus struct {
	ID      string `json:"ID"`
	Name    string `json:"Name"`
	Image   string `json:"Image"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Health  string `json:"Health"`
	Service string `json:"Service"`
}

// Verify that ContainerStatus implements ServiceStatus interface
var _ services.ServiceStatus = (*ContainerStatus)(nil)

func (c ContainerStatus) String() string {
	return fmt.Sprintf(
		"%s (%s) is %s [%s]",
		c.Name,
		c.Service,
		c.State,
		c.Status,
	)
}

func (c ContainerStatus) InstanceGist() string {
	return c.String()
}

func (c ContainerStatus) InstanceName() string {
	return c.Name
}

func (c ContainerStatus) InstanceService() string {
	return c.Service
}

func (c ContainerStatus) InstanceStatus() string {
	return c.Status
}

func (c ContainerStatus) InstanceHealth() string {
	return c.Health
}

func (c ContainerStatus) InstanceState() string {
	return c.State
}

// Client handles interactions with Docker
type Client struct {
	logger     logger.Logger
	dockerBin  string
	controlSvc string
}

// NewClient creates a new Docker client
func NewClient(logger logger.Logger) (*Client, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Check if docker is installed
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker is not available or not in PATH: %w", err)
	}

	controlBin, err := exec.LookPath("systemctl")
	if err != nil {
		return nil, fmt.Errorf("systemctl is not available or not in PATH: %w", err)
	}

	return &Client{
		logger:     logger,
		dockerBin:  dockerBin,
		controlSvc: controlBin,
	}, nil
}

// DockerBin returns the path to the Docker binary
func (c *Client) DockerBin() string {
	return c.dockerBin
}

// Name returns the name of the service
func (c *Client) Name() string {
	return "docker"
}

// Status returns the current status of Docker
func (c *Client) Status(ctx context.Context) ([]services.ServiceStatus, error) {
	// Execute systemctl to get Docker service status
	output, err := command.ExecCommand(
		ctx,
		c.logger,
		c.controlSvc,
		"status",
		"docker.service",
		"--no-pager",
	)
	if err != nil {
		c.logger.Warn("Failed to get Docker service status", "err", err)
		// If systemctl fails, return an error status
		return []services.ServiceStatus{
			ContainerStatus{
				Name:    "docker",
				Service: "docker.service",
				State:   "error",
				Status:  fmt.Sprintf("Error checking status: %v", err),
			},
		}, nil
	}

	// Parse the systemctl output for status info
	state := "unknown"
	statusFull := string(output)
	status := "Unknown status"

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
	} else if strings.Contains(statusFull, "Active: inactive (dead)") {
		state = "stopped"
	} else if strings.Contains(statusFull, "Active: failed") {
		state = "failed"
	}

	dockerStatus := ContainerStatus{
		Name:    "docker",
		Service: "docker.service",
		State:   state,
		Status:  status,
	}

	// Convert to []services.ServiceStatus
	serviceStatuses := make([]services.ServiceStatus, 1)
	serviceStatuses[0] = dockerStatus

	return serviceStatuses, nil
}

// Start is a no-op for Docker as it should be managed by systemd
func (c *Client) Start(ctx context.Context) error {
	c.logger.Info("Docker service is typically managed by systemd, not starting directly")
	return nil
}

// Stop is a no-op for Docker as it should be managed by systemd
func (c *Client) Stop(ctx context.Context) error {
	c.logger.Info("Docker service is typically managed by systemd, not stopping directly")
	return nil
}

// Restart is a no-op for Docker as it should be managed by systemd
func (c *Client) Restart(ctx context.Context) error {
	c.logger.Info("Docker service is typically managed by systemd, not restarting directly")
	return nil
}

// ComposeUp starts services defined in a docker-compose file
func (c *Client) ComposeUp(ctx context.Context, composeFilePath string, detach bool) error {
	args := []string{"compose", "-f", composeFilePath, "up"}
	if detach {
		args = append(args, "-d")
	}

	// Docker compose up can take a long time when pulling images
	// Use a 5-minute timeout instead of the default 30 seconds
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	c.logger.Debug("Starting docker-compose services", "args", args)
	_, err := command.ExecCommand(timeoutCtx, c.logger, c.dockerBin, args...)
	if err != nil {
		return fmt.Errorf("failed to start docker-compose services: %w", err)
	}

	return nil
}

// ComposeDown stops services defined in a docker-compose file
func (c *Client) ComposeDown(
	ctx context.Context,
	composeFilePath string,
	removeVolumes bool,
) error {
	args := []string{"compose", "-f", composeFilePath, "down"}
	if removeVolumes {
		args = append(args, "-v")
	}

	_, err := command.ExecCommand(ctx, c.logger, c.dockerBin, args...)
	if err != nil {
		return fmt.Errorf("failed to stop docker-compose services: %w", err)
	}

	return nil
}

// ComposeRestart restarts services defined in a docker-compose file
func (c *Client) ComposeRestart(ctx context.Context, composeFilePath string) error {
	args := []string{"compose", "-f", composeFilePath, "restart"}

	_, err := command.ExecCommand(ctx, c.logger, c.dockerBin, args...)
	if err != nil {
		return fmt.Errorf("failed to restart docker-compose services: %w", err)
	}

	return nil
}

// ComposeStatus returns detailed information about containers of a service
func (c *Client) ComposeStatus(
	ctx context.Context,
	composeFilePath string,
	serviceName string,
) ([]services.ServiceStatus, error) {
	output, err := command.ExecCommand(
		ctx,
		c.logger,
		c.dockerBin,
		"compose",
		"-f",
		composeFilePath,
		"ps",
		"--format",
		"json",
		serviceName,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get service container details: %w", err)
	}

	// No output means no containers
	if len(output) == 0 {
		return nil, nil
	}

	// Parse JSON output (potentially multiple JSON objects)
	jsonLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(jsonLines) == 0 {
		return nil, nil
	}

	var containerStatuses []ContainerStatus
	for _, line := range jsonLines {
		var container ContainerStatus
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			return nil, fmt.Errorf("failed to parse container status: %w", err)
		}
		containerStatuses = append(containerStatuses, container)
	}

	// Convert to []services.ServiceStatus
	serviceStatuses := make([]services.ServiceStatus, len(containerStatuses))
	for i, container := range containerStatuses {
		serviceStatuses[i] = container
	}

	return serviceStatuses, nil
}
