// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/stratastor/logger"
	generalCmd "github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/pkg/errors"
)

// HostnameManager manages system hostname operations
type HostnameManager struct {
	executor CommandExecutor
	logger   logger.Logger
}

// NewHostnameManager creates a new hostname manager
func NewHostnameManager(logger logger.Logger) *HostnameManager {
	return &HostnameManager{
		executor: &commandExecutorWrapper{
			executor: generalCmd.NewCommandExecutor(true), // Use sudo for hostname operations
		},
		logger: logger,
	}
}

// GetHostnameInfo gets comprehensive hostname information
func (hm *HostnameManager) GetHostnameInfo(ctx context.Context) (*HostnameInfo, error) {
	info := &HostnameInfo{}

	// Get static hostname
	result, err := hm.executor.ExecuteCommand(ctx, "hostnamectl", "hostname")
	if err == nil {
		info.Static = strings.TrimSpace(result.Stdout)
	}

	// Get all hostname information using hostnamectl status
	result, err = hm.executor.ExecuteCommand(ctx, "hostnamectl", "status")
	if err != nil {
		hm.logger.Warn("Failed to get hostname status", "error", err)
	} else {
		hm.parseHostnameStatus(result.Stdout, info)
	}

	return info, nil
}

// parseHostnameStatus parses hostnamectl status output
func (hm *HostnameManager) parseHostnameStatus(output string, info *HostnameInfo) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "Static hostname":
				if info.Static == "" {
					info.Static = value
				}
			case "Transient hostname":
				info.Transient = value
			case "Pretty hostname":
				info.Pretty = value
			case "Icon name":
				info.IconName = value
			case "Chassis":
				info.Chassis = value
			case "Machine ID":
				info.MachineID = value
			case "Boot ID":
				info.BootID = value
			}
		}
	}
}

// GetHostname gets the current system hostname
func (hm *HostnameManager) GetHostname(ctx context.Context) (string, error) {
	result, err := hm.executor.ExecuteCommand(ctx, "hostnamectl", "hostname")
	if err != nil {
		return "", errors.Wrap(err, errors.ServerInternalError).
			WithMetadata("operation", "get_hostname")
	}

	hostname := strings.TrimSpace(result.Stdout)
	if hostname == "" {
		return "", errors.New(errors.ServerInternalError, "Empty hostname returned")
	}

	return hostname, nil
}

// SetHostname sets the system hostname
func (hm *HostnameManager) SetHostname(ctx context.Context, request SetHostnameRequest) error {
	// Validate hostname
	if err := hm.validateHostname(request.Hostname); err != nil {
		return err
	}

	hm.logger.Info("Setting system hostname", "hostname", request.Hostname, "static", request.Static)

	// Build hostnamectl command arguments
	args := []string{}

	if request.Static {
		args = append(args, "set-hostname", "--static", request.Hostname)
	} else {
		args = append(args, "set-hostname", request.Hostname)
	}

	// Set the hostname
	result, err := hm.executor.ExecuteCommand(ctx, "hostnamectl", args...)
	if err != nil {
		hm.logger.Error("Failed to set hostname", "hostname", request.Hostname, "error", err)
		return errors.Wrap(err, errors.ServerInternalError).
			WithMetadata("operation", "set_hostname").
			WithMetadata("hostname", request.Hostname).
			WithMetadata("output", result.Stdout)
	}

	// Set pretty hostname if provided
	if request.Pretty != "" {
		_, err = hm.executor.ExecuteCommand(ctx, "hostnamectl", "set-hostname", "--pretty", request.Pretty)
		if err != nil {
			hm.logger.Warn("Failed to set pretty hostname", "pretty", request.Pretty, "error", err)
			// Don't fail the entire operation if pretty hostname fails
		}
	}

	hm.logger.Info("Successfully set system hostname", "hostname", request.Hostname)
	return nil
}

// validateHostname validates hostname according to RFC standards
func (hm *HostnameManager) validateHostname(hostname string) error {
	if hostname == "" {
		return errors.New(errors.ServerRequestValidation, "Hostname cannot be empty")
	}

	// Check length (RFC 1123: max 253 characters for FQDN, max 63 for labels)
	if len(hostname) > 253 {
		return errors.New(errors.ServerRequestValidation, "Hostname too long (max 253 characters)")
	}

	// Check for valid characters and format
	// RFC 1123 allows letters, digits, and hyphens
	// Cannot start or end with hyphen
	// Each label (part separated by dots) cannot exceed 63 characters
	hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	if !hostnameRegex.MatchString(hostname) {
		return errors.New(errors.ServerRequestValidation, "Invalid hostname format")
	}

	// Check each label length
	labels := strings.Split(hostname, ".")
	for _, label := range labels {
		if len(label) > 63 {
			return errors.New(errors.ServerRequestValidation, fmt.Sprintf("Hostname label '%s' too long (max 63 characters)", label))
		}
		if label == "" {
			return errors.New(errors.ServerRequestValidation, "Hostname cannot contain empty labels")
		}
	}

	// Additional restrictions for system hostnames
	// Cannot be just numeric (could conflict with IP addresses)
	if regexp.MustCompile(`^[0-9]+$`).MatchString(hostname) {
		return errors.New(errors.ServerRequestValidation, "Hostname cannot be purely numeric")
	}

	// Cannot start with hyphen or dot
	if strings.HasPrefix(hostname, "-") || strings.HasPrefix(hostname, ".") {
		return errors.New(errors.ServerRequestValidation, "Hostname cannot start with hyphen or dot")
	}

	// Cannot end with hyphen or dot
	if strings.HasSuffix(hostname, "-") || strings.HasSuffix(hostname, ".") {
		return errors.New(errors.ServerRequestValidation, "Hostname cannot end with hyphen or dot")
	}

	return nil
}