// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package probing

import (
	"context"
	"fmt"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk/state"
	"github.com/stratastor/rodent/pkg/disk/types"
)

// ZFSPoolManager interface for ZFS pool operations
type ZFSPoolManager interface {
	// IsPoolScrubbing returns true if pool is currently scrubbing
	IsPoolScrubbing(ctx context.Context, poolName string) (bool, error)

	// IsPoolResilvering returns true if pool is currently resilvering
	IsPoolResilvering(ctx context.Context, poolName string) (bool, error)

	// GetPoolForDevice returns the pool name for a device, if any
	GetPoolForDevice(ctx context.Context, devicePath string) (string, bool, error)
}

// ZFSConflictChecker checks for ZFS operation conflicts
type ZFSConflictChecker struct {
	logger       logger.Logger
	stateManager *state.StateManager
	poolManager  ZFSPoolManager
}

// NewZFSConflictChecker creates a new ZFS conflict checker
func NewZFSConflictChecker(
	l logger.Logger,
	stateManager *state.StateManager,
	poolManager ZFSPoolManager,
) *ZFSConflictChecker {
	return &ZFSConflictChecker{
		logger:       l,
		stateManager: stateManager,
		poolManager:  poolManager,
	}
}

// CheckConflicts checks if a probe would conflict with ongoing operations
func (zcc *ZFSConflictChecker) CheckConflicts(
	ctx context.Context,
	deviceID string,
	devicePath string,
	probeType types.ProbeType,
) (bool, string, error) {
	// Check if device is in ZFS pool
	poolName, inPool, err := zcc.poolManager.GetPoolForDevice(ctx, devicePath)
	if err != nil {
		zcc.logger.Warn("failed to check pool membership",
			"device_id", deviceID,
			"device_path", devicePath,
			"error", err)
		// Don't fail the probe if we can't check pool membership
		return false, "", nil
	}

	if !inPool {
		// Device not in pool, no ZFS conflicts possible
		return false, "", nil
	}

	// Check for scrub operation
	scrubbing, err := zcc.poolManager.IsPoolScrubbing(ctx, poolName)
	if err != nil {
		zcc.logger.Warn("failed to check scrub status",
			"pool", poolName,
			"error", err)
	} else if scrubbing {
		reason := fmt.Sprintf("pool '%s' is currently scrubbing", poolName)
		zcc.logger.Debug("probe conflict detected",
			"device_id", deviceID,
			"pool", poolName,
			"conflict", "scrub")
		return true, reason, nil
	}

	// Check for resilver operation
	resilvering, err := zcc.poolManager.IsPoolResilvering(ctx, poolName)
	if err != nil {
		zcc.logger.Warn("failed to check resilver status",
			"pool", poolName,
			"error", err)
	} else if resilvering {
		reason := fmt.Sprintf("pool '%s' is currently resilvering", poolName)
		zcc.logger.Debug("probe conflict detected",
			"device_id", deviceID,
			"pool", poolName,
			"conflict", "resilver")
		return true, reason, nil
	}

	// For extensive probes, add additional checks
	if probeType == types.ProbeTypeExtensive {
		// Check pool health - don't run extensive tests on degraded pools
		// This would be implemented when we have pool health status available
		// For now, we allow extensive probes if no active operations
	}

	// No conflicts detected
	return false, "", nil
}

// NoOpConflictChecker is a no-operation conflict checker for testing
type NoOpConflictChecker struct{}

// NewNoOpConflictChecker creates a conflict checker that never reports conflicts
func NewNoOpConflictChecker() *NoOpConflictChecker {
	return &NoOpConflictChecker{}
}

// CheckConflicts always returns no conflicts
func (n *NoOpConflictChecker) CheckConflicts(
	ctx context.Context,
	deviceID string,
	devicePath string,
	probeType types.ProbeType,
) (bool, string, error) {
	return false, "", nil
}
