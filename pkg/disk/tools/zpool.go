// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
)

// ZpoolExecutor wraps zpool command execution for pool status queries
// This is a minimal wrapper for disk discovery to check pool membership
type ZpoolExecutor struct {
	logger   logger.Logger
	executor *command.CommandExecutor
	path     string
}

// NewZpoolExecutor creates a new zpool executor
func NewZpoolExecutor(l logger.Logger, path string, useSudo bool) *ZpoolExecutor {
	executor := command.NewCommandExecutor(useSudo)
	executor.Timeout = 30 * time.Second // Pool status can take time

	return &ZpoolExecutor{
		logger:   l,
		executor: executor,
		path:     path,
	}
}

// PoolStatus represents the minimal structure needed from zpool status -j
type PoolStatus struct {
	Pools map[string]Pool `json:"pools"`
}

// Pool represents a ZFS pool from status output
type Pool struct {
	Name      string            `json:"name"`
	VDevs     map[string]*VDev `json:"vdevs"`
	ScanStats *ScanStats        `json:"scan_stats,omitempty"`
}

// ScanStats represents scan statistics from pool status
type ScanStats struct {
	Function string `json:"function"` // SCRUB, RESILVER, etc.
	State    string `json:"state"`    // SCANNING, FINISHED, etc.
}

// VDev represents a virtual device in the pool
type VDev struct {
	Name  string            `json:"name"`
	State string            `json:"state"` // ONLINE, DEGRADED, FAULTED, UNAVAIL, OFFLINE
	Path  string            `json:"path,omitempty"`
	VDevs map[string]*VDev `json:"vdevs,omitempty"` // Nested vdevs
}

// Status returns the status of all pools in JSON format
func (z *ZpoolExecutor) Status(ctx context.Context) ([]byte, error) {
	z.logger.Debug("getting zpool status")
	return z.executor.ExecuteWithCombinedOutput(ctx, z.path,
		"status",
		"-j", // JSON output
	)
}

// GetPoolStatus parses zpool status JSON output
func (z *ZpoolExecutor) GetPoolStatus(ctx context.Context) (*PoolStatus, error) {
	output, err := z.Status(ctx)
	if err != nil {
		return nil, err
	}

	var status PoolStatus
	if err := json.Unmarshal(output, &status); err != nil {
		z.logger.Warn("failed to parse zpool status JSON", "error", err)
		return nil, err
	}

	return &status, nil
}

// IsPoolScrubbing returns true if pool is currently scrubbing
func (z *ZpoolExecutor) IsPoolScrubbing(ctx context.Context, poolName string) (bool, error) {
	status, err := z.GetPoolStatus(ctx)
	if err != nil {
		return false, err
	}

	// Check if the specified pool has an active scrub
	if pool, exists := status.Pools[poolName]; exists {
		if pool.ScanStats != nil {
			// Function is uppercase "SCRUB" and state is "SCANNING"
			if pool.ScanStats.Function == "SCRUB" && pool.ScanStats.State == "SCANNING" {
				return true, nil
			}
		}
	}

	return false, nil
}

// IsPoolResilvering returns true if pool is currently resilvering
func (z *ZpoolExecutor) IsPoolResilvering(ctx context.Context, poolName string) (bool, error) {
	status, err := z.GetPoolStatus(ctx)
	if err != nil {
		return false, err
	}

	// Check if the specified pool has an active resilver
	if pool, exists := status.Pools[poolName]; exists {
		if pool.ScanStats != nil {
			// Function is uppercase "RESILVER" and state is "SCANNING"
			if pool.ScanStats.Function == "RESILVER" && pool.ScanStats.State == "SCANNING" {
				return true, nil
			}
		}
	}

	return false, nil
}
