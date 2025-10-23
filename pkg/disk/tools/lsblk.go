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

// LsblkExecutor wraps lsblk command execution
type LsblkExecutor struct {
	logger   logger.Logger
	executor *command.CommandExecutor
	path     string
}

// NewLsblkExecutor creates a new lsblk executor
func NewLsblkExecutor(l logger.Logger, path string, useSudo bool) *LsblkExecutor {
	executor := command.NewCommandExecutor(useSudo)
	executor.Timeout = 10 * time.Second

	return &LsblkExecutor{
		logger:   l,
		executor: executor,
		path:     path,
	}
}

// ListAll lists all block devices with JSON output
// Common columns: NAME,PATH,TYPE,SIZE,VENDOR,MODEL,SERIAL,WWN,STATE,MOUNTPOINT,FSTYPE
func (l *LsblkExecutor) ListAll(ctx context.Context) ([]byte, error) {
	l.logger.Debug("listing all block devices")
	return l.executor.ExecuteWithCombinedOutput(ctx, l.path,
		"--json",
		"--output", "NAME,PATH,TYPE,SIZE,VENDOR,MODEL,SERIAL,WWN,STATE,MOUNTPOINT,FSTYPE,PHY-SEC,LOG-SEC,ROTA,DISC-GRAN,DISC-MAX,TRAN,HCTL",
		"--bytes",
		"--paths",
	)
}

// ListDisks lists only disk devices (no partitions, loop devices, etc.)
func (l *LsblkExecutor) ListDisks(ctx context.Context) ([]byte, error) {
	l.logger.Debug("listing disk devices")
	return l.executor.ExecuteWithCombinedOutput(ctx, l.path,
		"--json",
		"--output", "NAME,PATH,TYPE,SIZE,VENDOR,MODEL,SERIAL,WWN,STATE,ROTA,PHY-SEC,LOG-SEC,TRAN,HCTL",
		"--bytes",
		"--paths",
		"--nodeps", // No dependencies (partitions)
		"--exclude", "7,11", // Exclude loop and optical devices
	)
}

// ListDisksWithChildren lists disk devices WITH their partitions and mount information
// Used for detecting system disks (disks with mounted partitions)
func (l *LsblkExecutor) ListDisksWithChildren(ctx context.Context) ([]byte, error) {
	l.logger.Debug("listing disk devices with children")
	return l.executor.ExecuteWithCombinedOutput(ctx, l.path,
		"--json",
		"--output", "NAME,PATH,TYPE,SIZE,VENDOR,MODEL,SERIAL,WWN,STATE,MOUNTPOINT,FSTYPE,ROTA,PHY-SEC,LOG-SEC,TRAN,HCTL",
		"--bytes",
		"--paths",
		// NO --nodeps flag, so we get children (partitions)
		"--exclude", "7,11", // Exclude loop and optical devices
	)
}

// GetDevice gets detailed information about a specific device
func (l *LsblkExecutor) GetDevice(ctx context.Context, device string) ([]byte, error) {
	l.logger.Debug("getting device info", "device", device)
	return l.executor.ExecuteWithCombinedOutput(ctx, l.path,
		"--json",
		"--output", "NAME,PATH,TYPE,SIZE,VENDOR,MODEL,SERIAL,WWN,STATE,MOUNTPOINT,FSTYPE,PHY-SEC,LOG-SEC,ROTA,DISC-GRAN,DISC-MAX,TRAN,HCTL",
		"--bytes",
		"--paths",
		device,
	)
}

// ListNVMe lists only NVMe devices
func (l *LsblkExecutor) ListNVMe(ctx context.Context) ([]byte, error) {
	l.logger.Debug("listing NVMe devices")
	return l.executor.ExecuteWithCombinedOutput(ctx, l.path,
		"--json",
		"--output", "NAME,PATH,TYPE,SIZE,MODEL,SERIAL,STATE",
		"--bytes",
		"--paths",
		"--nodeps",
		"--include", "259", // NVMe major device number
	)
}
