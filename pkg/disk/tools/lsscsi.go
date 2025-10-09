// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
)

// LsscsiExecutor wraps lsscsi command execution
type LsscsiExecutor struct {
	logger   logger.Logger
	executor *command.CommandExecutor
	path     string
}

// NewLsscsiExecutor creates a new lsscsi executor
func NewLsscsiExecutor(l logger.Logger, path string, useSudo bool) *LsscsiExecutor {
	executor := command.NewCommandExecutor(useSudo)
	executor.Timeout = 10 * time.Second

	return &LsscsiExecutor{
		logger:   l,
		executor: executor,
		path:     path,
	}
}

// List lists all SCSI/SAS devices with verbose output
// Includes HCTL (Host:Channel:Target:LUN), device type, vendor, model, revision, device path
func (l *LsscsiExecutor) List(ctx context.Context) ([]byte, error) {
	l.logger.Debug("listing SCSI devices")
	return l.executor.ExecuteWithCombinedOutput(ctx, l.path,
		"--verbose",    // Verbose output with more details
		"--size",       // Show device size
		"--transport",  // Show transport information
	)
}

// ListGeneric lists all SCSI generic devices
func (l *LsscsiExecutor) ListGeneric(ctx context.Context) ([]byte, error) {
	l.logger.Debug("listing SCSI generic devices")
	return l.executor.ExecuteWithCombinedOutput(ctx, l.path,
		"--generic",    // Show sg device names
		"--verbose",
		"--size",
	)
}

// ListEnclosures lists only enclosure devices
func (l *LsscsiExecutor) ListEnclosures(ctx context.Context) ([]byte, error) {
	l.logger.Debug("listing enclosure devices")
	return l.executor.ExecuteWithCombinedOutput(ctx, l.path,
		"--generic",
		"--verbose",
		"--scsi_id",
		"--wwn",
	)
}

// GetDevice gets detailed information about a specific SCSI device
func (l *LsscsiExecutor) GetDevice(ctx context.Context, hctl string) ([]byte, error) {
	l.logger.Debug("getting SCSI device info", "hctl", hctl)
	return l.executor.ExecuteWithCombinedOutput(ctx, l.path,
		"--verbose",
		"--size",
		"--transport",
		"--wwn",
		hctl,
	)
}
