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

// UdevadmExecutor wraps udevadm command execution
type UdevadmExecutor struct {
	logger   logger.Logger
	executor *command.CommandExecutor
	path     string
}

// NewUdevadmExecutor creates a new udevadm executor
func NewUdevadmExecutor(l logger.Logger, path string, useSudo bool) *UdevadmExecutor {
	executor := command.NewCommandExecutor(useSudo)
	executor.Timeout = 10 * time.Second

	return &UdevadmExecutor{
		logger:   l,
		executor: executor,
		path:     path,
	}
}

// Info gets udev information for a device
// Returns detailed udev properties including ID_SERIAL, ID_WWN, ID_PATH, etc.
func (u *UdevadmExecutor) Info(ctx context.Context, device string) ([]byte, error) {
	u.logger.Debug("getting udev info", "device", device)
	return u.executor.ExecuteWithCombinedOutput(ctx, u.path,
		"info",
		"--query=property",
		"--name="+device,
	)
}

// InfoAll gets all udev information for a device including parent devices
func (u *UdevadmExecutor) InfoAll(ctx context.Context, device string) ([]byte, error) {
	u.logger.Debug("getting all udev info", "device", device)
	return u.executor.ExecuteWithCombinedOutput(ctx, u.path,
		"info",
		"--query=all",
		"--name="+device,
	)
}

// Trigger triggers udev events (used for device re-enumeration)
func (u *UdevadmExecutor) Trigger(ctx context.Context) ([]byte, error) {
	u.logger.Debug("triggering udev events")
	return u.executor.ExecuteWithCombinedOutput(ctx, u.path,
		"trigger",
		"--subsystem-match=block",
	)
}

// Settle waits for udev to process all events
func (u *UdevadmExecutor) Settle(ctx context.Context) ([]byte, error) {
	u.logger.Debug("waiting for udev to settle")
	return u.executor.ExecuteWithCombinedOutput(ctx, u.path,
		"settle",
		"--timeout=10",
	)
}

// Monitor monitors udev events in real-time
// Note: This is a long-running command, should be used with appropriate context timeout
func (u *UdevadmExecutor) Monitor(ctx context.Context) ([]byte, error) {
	u.logger.Debug("monitoring udev events")
	// Override timeout for monitoring
	oldTimeout := u.executor.Timeout
	u.executor.Timeout = 0 // No timeout, use context
	defer func() { u.executor.Timeout = oldTimeout }()

	return u.executor.ExecuteWithCombinedOutput(ctx, u.path,
		"monitor",
		"--subsystem-match=block",
		"--property",
	)
}
