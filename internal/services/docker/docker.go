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

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/services/command"
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

// Client handles interactions with Docker
type Client struct {
	logger    logger.Logger
	dockerBin string
}

// NewClient creates a new Docker client
func NewClient(logger logger.Logger) (*Client, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Check if docker is installed
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker is not installed or not in PATH: %w", err)
	}

	return &Client{
		logger:    logger,
		dockerBin: dockerBin,
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
func (c *Client) Status(ctx context.Context) (string, error) {
	output, err := command.ExecCommand(
		ctx,
		c.logger,
		c.dockerBin,
		"info",
		"--format",
		"{{json .}}",
	)
	if err != nil {
		return "", fmt.Errorf("failed to get Docker status: %w", err)
	}

	var info map[string]interface{}
	if err := json.Unmarshal(output, &info); err != nil {
		return "", fmt.Errorf("failed to parse Docker info: %w", err)
	}

	version, ok := info["ServerVersion"].(string)
	if !ok {
		return "", fmt.Errorf("failed to get Docker server version")
	}

	return fmt.Sprintf("running (version: %s)", version), nil
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

	_, err := command.ExecCommand(ctx, c.logger, c.dockerBin, args...)
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

// ComposeStatus returns detailed status information for a docker-compose service
func (c *Client) ComposeStatus(ctx context.Context, composeFilePath string) (string, error) {
	// First check if any containers are running
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
	)

	if err != nil {
		return "", fmt.Errorf("failed to check docker-compose status: %w", err)
	}

	// No output means no containers
	if len(output) == 0 {
		return "stopped", nil
	}

	// Parse JSON output (potentially multiple JSON objects)
	jsonLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(jsonLines) == 0 {
		return "stopped", nil
	}

	// Parse the first container's status
	var container ContainerStatus
	if err := json.Unmarshal([]byte(jsonLines[0]), &container); err != nil {
		return "", fmt.Errorf("failed to parse container status: %w", err)
	}

	if container.State == "running" {
		statusInfo := fmt.Sprintf("running (%s)", container.Status)
		if container.Health != "" {
			statusInfo += fmt.Sprintf(" [health: %s]", container.Health)
		}
		return statusInfo, nil
	}

	return container.State, nil
}

// GetServiceContainerDetails returns detailed information about containers of a service
func (c *Client) GetServiceContainerDetails(
	ctx context.Context,
	composeFilePath string,
	serviceName string,
) ([]ContainerStatus, error) {
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

	var containers []ContainerStatus
	for _, line := range jsonLines {
		var container ContainerStatus
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			return nil, fmt.Errorf("failed to parse container status: %w", err)
		}
		containers = append(containers, container)
	}

	return containers, nil
}
