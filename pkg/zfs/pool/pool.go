/*
 * Copyright 2024-2025 Raamsri Kumar <raam@tinkershack.in>
 * Copyright 2024-2025 The StrataSTOR Authors and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
)

// Manager manages ZFS pool operations
type Manager struct {
	executor *command.CommandExecutor
}

func NewManager(executor *command.CommandExecutor) *Manager {
	return &Manager{executor: executor}
}

// buildVDevArgs converts VDevSpec to command arguments
func buildVDevArgs(specs []VDevSpec) []string {
	var args []string
	for _, spec := range specs {
		if spec.Type != "" && spec.Type != "stripe" {
			args = append(args, spec.Type)
		}
		args = append(args, spec.Devices...)
		if len(spec.Children) > 0 {
			args = append(args, buildVDevArgs(spec.Children)...)
		}
	}
	return args
}

// Create creates a new ZFS pool
func (p *Manager) Create(ctx context.Context, cfg CreateConfig) error {
	args := []string{}

	// Add properties
	for k, v := range cfg.Properties {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	// Add features
	for feature, enabled := range cfg.Features {
		if enabled {
			args = append(args, "-o", fmt.Sprintf("feature@%s=enabled", feature))
		}
	}

	if cfg.MountPoint != "" {
		args = append(args, "-m", cfg.MountPoint)
	}

	// Add pool name and vdev specs
	args = append(args, cfg.Name)
	args = append(args, buildVDevArgs(cfg.VDevSpec)...)

	opts := command.CommandOptions{
		Flags: command.FlagForce, // if cfg.Force is true
	}

	out, err := p.executor.Execute(ctx, opts, "zpool create", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolCreate).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolCreate)
	}

	return nil
}

// Import imports a ZFS pool
func (p *Manager) Import(ctx context.Context, cfg ImportConfig) error {
	args := []string{"import"}

	if cfg.Force {
		args = append(args, "-f")
	}

	if cfg.Dir != "" {
		args = append(args, "-d", cfg.Dir)
	}

	for k, v := range cfg.Properties {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	if cfg.Name != "" {
		args = append(args, cfg.Name)
	}

	if len(cfg.Paths) > 0 {
		args = append(args, cfg.Paths...)
	}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool import", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolImport).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolImport)
	}

	return nil
}

// Status gets the comprehensive status of a pool or all pools.
// Internally uses flags to provide complete information: -P (full vdev paths),
// -i (initialization status), -t (TRIM status), -D (deduplication stats).
func (p *Manager) Status(ctx context.Context, name string) (PoolStatus, error) {
	// Build args with comprehensive flags for detailed status information
	args := []string{
		"status",
		"-P", // Display complete vdev paths
		"-i", // Include initialization status
		"-t", // Include TRIM status
		"-D", // Include deduplication statistics
	}

	// Add pool name if specified
	if name != "" {
		args = append(args, name)
	}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	var status PoolStatus

	out, err := p.executor.Execute(ctx, opts, "zpool status", args...)
	if err != nil {
		if len(out) > 0 {
			return status, errors.Wrap(err, errors.ZFSPoolStatus).
				WithMetadata("output", string(out))
		}
		return status, errors.Wrap(err, errors.ZFSPoolStatus)
	}

	if err := json.Unmarshal(out, &status); err != nil {
		return status, errors.Wrap(err, errors.CommandOutputParse)
	}

	return status, nil
}

func (p *Manager) GetProperties(ctx context.Context, name string) (ListResult, error) {
	args := []string{"get", "all", "-H"}
	if name != "" {
		args = append(args, name)
	}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	var result ListResult

	out, err := p.executor.Execute(ctx, opts, "zpool get", args...)
	if err != nil {
		if len(out) > 0 {
			return result, errors.Wrap(err, errors.ZFSPoolGetProperty).
				WithMetadata("output", string(out))
		}
		return result, errors.Wrap(err, errors.ZFSPoolGetProperty)
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return result, errors.Wrap(err, errors.CommandOutputParse)
	}

	return result, nil
}

// GetProperty gets a specific property of a pool
func (p *Manager) GetProperty(ctx context.Context, name, property string) (ListResult, error) {
	args := []string{"get", "-H", property}
	if name != "" {
		args = append(args, name)
	}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	var result ListResult

	out, err := p.executor.Execute(ctx, opts, "zpool get", args...)
	if err != nil {
		if len(out) > 0 {
			return result, errors.Wrap(err, errors.ZFSPoolGetProperty).
				WithMetadata("output", string(out))
		}
		return result, errors.Wrap(err, errors.ZFSPoolGetProperty)
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return result, errors.Wrap(err, errors.CommandOutputParse)
	}

	return result, nil
}

func (p *Manager) SetProperty(ctx context.Context, name, property, value string) error {
	// Format: zpool set property=value poolname
	// For values with spaces, we need to properly escape
	formattedValue := value
	if strings.Contains(value, " ") {
		// Use single quotes to preserve spaces without shell interpretation
		formattedValue = fmt.Sprintf("'%s'", value)
	}

	args := []string{"set", fmt.Sprintf("%s=%s", property, formattedValue), name}

	opts := command.CommandOptions{}
	out, err := p.executor.Execute(ctx, opts, "zpool set", args...)
	if err != nil {
		// Add command output to error context if available
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolSetProperty).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolSetProperty)
	}

	return nil
}

// Export exports a ZFS pool
func (p *Manager) Export(ctx context.Context, name string, force bool) error {
	args := []string{"export"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool export", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolExport).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolExport)
	}
	return nil
}

// Destroy destroys a ZFS pool
func (p *Manager) Destroy(ctx context.Context, name string, force bool) error {
	if name == "" {
		return errors.New(errors.ZFSPoolInvalidName, "pool name cannot be empty")
	}

	args := []string{"destroy"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	opts := command.CommandOptions{}
	out, err := p.executor.Execute(ctx, opts, "zpool destroy", args...)
	if err != nil {
		// Include command output in error if available
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDestroy).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDestroy)
	}

	return nil
}

// Scrub starts/stops a scrub on a pool
func (p *Manager) Scrub(ctx context.Context, name string, stop bool) error {
	args := []string{"scrub"}
	if stop {
		args = append(args, "-s")
	}
	args = append(args, name)

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool scrub", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolScrubFailed).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolScrubFailed)
	}
	return nil
}

// List returns a list of all pools or a specific pool
func (p *Manager) List(ctx context.Context, poolName ...string) (ListResult, error) {
	args := []string{"-H", "-p"}

	// If pool name is provided, add it to args
	if len(poolName) > 0 && poolName[0] != "" {
		args = append(args, poolName[0])
	}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	var result ListResult

	out, err := p.executor.Execute(ctx, opts, "zpool list", args...)
	if err != nil {
		if len(out) > 0 {
			return result, errors.Wrap(err, errors.ZFSPoolList).
				WithMetadata("output", string(out))
		}
		return result, errors.Wrap(err, errors.ZFSPoolList)
	}

	if len(out) > 0 {
		if err := json.Unmarshal(out, &result); err != nil {
			return result, errors.Wrap(err, errors.CommandOutputParse)
		}
	}

	return result, nil
}

// ListImportable lists pools available for import
func (p *Manager) ListImportable(ctx context.Context) (ImportablePoolsResult, error) {
	args := []string{"import"}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool import", args...)
	if err != nil {
		// zpool import returns exit code 1 when no pools are available
		// This is normal, so we parse the output anyway if available
		if len(out) > 0 {
			return parseImportablePoolsOutput(string(out))
		}
		return ImportablePoolsResult{}, errors.Wrap(err, errors.ZFSPoolImport).
			WithMetadata("output", string(out))
	}

	return parseImportablePoolsOutput(string(out))
}

func (p *Manager) Resilver(ctx context.Context, name string) error {
	args := []string{"resilver", name}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool resilver", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolResilverFailed).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolResilverFailed)
	}
	return nil
}

func (p *Manager) AttachDevice(
	ctx context.Context,
	pool, device, newDevice string,
	force bool,
) error {
	args := []string{"attach"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, pool, device, newDevice)

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool attach", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

func (p *Manager) DetachDevice(ctx context.Context, pool, device string) error {
	args := []string{"detach", pool, device}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool detach", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

func (p *Manager) ReplaceDevice(ctx context.Context, pool, oldDevice, newDevice string) error {
	args := []string{"replace", pool, oldDevice, newDevice}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool replace", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// IsPoolScrubbing returns true if pool is currently scrubbing
func (p *Manager) IsPoolScrubbing(ctx context.Context, poolName string) (bool, error) {
	status, err := p.Status(ctx, poolName)
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
func (p *Manager) IsPoolResilvering(ctx context.Context, poolName string) (bool, error) {
	status, err := p.Status(ctx, poolName)
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

// NOTE: GetPoolForDevice, containsDevice, and normalizeDevicePath methods have been removed.
//
// Pool membership is now determined during disk discovery via DEVLINKS matching
// in pkg/disk/discovery and stored in DeviceState.PoolName.
//
// This provides:
// - Robust device path resolution using all device symlinks (DEVLINKS)
// - Persistent pool membership across service restarts
// - No circular dependencies between disk and ZFS packages
// - Consistent data flow: discovery -> state -> API

// Add adds vdevs to an existing pool
func (p *Manager) Add(ctx context.Context, cfg AddConfig) error {
	args := []string{"add"}

	if cfg.Force {
		args = append(args, "-f")
	}

	args = append(args, cfg.Name)
	args = append(args, buildVDevArgs(cfg.VDevSpec)...)

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool add", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Clear clears device errors in a pool
func (p *Manager) Clear(ctx context.Context, cfg ClearConfig) error {
	args := []string{"clear", cfg.Pool}

	if cfg.Device != "" {
		args = append(args, cfg.Device)
	}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool clear", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Offline takes a device offline
func (p *Manager) Offline(ctx context.Context, cfg OfflineConfig) error {
	args := []string{"offline"}

	if cfg.Temporary {
		args = append(args, "-t")
	}

	args = append(args, cfg.Pool, cfg.Device)

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool offline", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Online brings a device online
func (p *Manager) Online(ctx context.Context, cfg OnlineConfig) error {
	args := []string{"online"}

	if cfg.Expand {
		args = append(args, "-e")
	}

	args = append(args, cfg.Pool, cfg.Device)

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool online", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Remove removes a device from a pool
func (p *Manager) Remove(ctx context.Context, pool string, devices []string) error {
	args := []string{"remove", pool}
	args = append(args, devices...)

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool remove", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Initialize initializes devices in a pool
func (p *Manager) Initialize(ctx context.Context, cfg InitializeConfig) error {
	args := []string{"initialize"}

	if cfg.Cancel {
		args = append(args, "-c")
	} else if cfg.Suspend {
		args = append(args, "-s")
	}

	args = append(args, cfg.Pool)

	if len(cfg.Devices) > 0 {
		args = append(args, cfg.Devices...)
	}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool initialize", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Trim trims devices in a pool
func (p *Manager) Trim(ctx context.Context, cfg TrimConfig) error {
	args := []string{"trim"}

	if cfg.Cancel {
		args = append(args, "-c")
	} else if cfg.Suspend {
		args = append(args, "-s")
	}

	if cfg.Secure {
		args = append(args, "-d")
	}

	if cfg.Rate > 0 {
		args = append(args, "-r", fmt.Sprintf("%d", cfg.Rate))
	}

	args = append(args, cfg.Pool)

	if len(cfg.Devices) > 0 {
		args = append(args, cfg.Devices...)
	}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool trim", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Checkpoint creates or discards a checkpoint
func (p *Manager) Checkpoint(ctx context.Context, cfg CheckpointConfig) error {
	args := []string{"checkpoint"}

	if cfg.Discard {
		args = append(args, "-d")
	}

	args = append(args, cfg.Pool)

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool checkpoint", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Reguid regenerates the GUID for a pool
func (p *Manager) Reguid(ctx context.Context, pool string) error {
	args := []string{"reguid", pool}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool reguid", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Reopen reopens all vdevs associated with a pool
func (p *Manager) Reopen(ctx context.Context, pool string) error {
	args := []string{"reopen", pool}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool reopen", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Upgrade upgrades a pool to the latest on-disk version
func (p *Manager) Upgrade(ctx context.Context, pool string, all bool) error {
	args := []string{"upgrade"}

	if all {
		args = append(args, "-a")
	} else if pool != "" {
		args = append(args, pool)
	}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool upgrade", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// History retrieves the command history for a pool
func (p *Manager) History(
	ctx context.Context,
	pool string,
	internal bool,
	longFormat bool,
) (string, error) {
	args := []string{"history"}

	if internal {
		args = append(args, "-i")
	}

	if longFormat {
		args = append(args, "-l")
	}

	if pool != "" {
		args = append(args, pool)
	}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool history", args...)
	if err != nil {
		if len(out) > 0 {
			return "", errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return "", errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}

	return string(out), nil
}

// Events retrieves pool events
// Note: Follow mode (-f) is not supported as it requires streaming
func (p *Manager) Events(ctx context.Context, pool string, verbose bool) (string, error) {
	args := []string{"events"}

	if verbose {
		args = append(args, "-v")
	}

	if pool != "" {
		args = append(args, pool)
	}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool events", args...)
	if err != nil {
		if len(out) > 0 {
			return "", errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return "", errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}

	return string(out), nil
}

// IOStat retrieves comprehensive I/O statistics for pools with per-vdev breakdown.
// Internally uses -P (full paths) and -v (per-vdev stats) flags for consistent device naming
// across APIs and detailed monitoring.
func (p *Manager) IOStat(ctx context.Context, pool string) (IOStatResult, error) {
	args := []string{
		"iostat",
		"-P", // Display full vdev paths for consistency with Status API
		"-v", // Always include per-vdev statistics for comprehensive monitoring
	}

	if pool != "" {
		args = append(args, pool)
	}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool iostat", args...)
	if err != nil {
		if len(out) > 0 {
			return IOStatResult{}, errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return IOStatResult{}, errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}

	return parseIOStatOutput(string(out), true) // Always parse with verbose=true
}

// Wait waits for background activity in a pool to complete
// WARNING: This is a blocking operation and may take a long time.
// Consider using status/events polling instead for long-running operations.
func (p *Manager) Wait(ctx context.Context, cfg WaitConfig) error {
	args := []string{"wait"}

	if len(cfg.Activities) > 0 {
		args = append(args, "-t", strings.Join(cfg.Activities, ","))
	}

	args = append(args, cfg.Pool)

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool wait", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Split splits a mirrored pool
func (p *Manager) Split(ctx context.Context, cfg SplitConfig) error {
	args := []string{"split"}

	// Add properties
	for k, v := range cfg.Properties {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	if cfg.MountPoint != "" {
		args = append(args, "-m", cfg.MountPoint)
	}

	args = append(args, cfg.Pool, cfg.NewPool)

	if len(cfg.Devices) > 0 {
		args = append(args, cfg.Devices...)
	}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool split", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// LabelClear clears label information from a device
func (p *Manager) LabelClear(ctx context.Context, device string, force bool) error {
	args := []string{"labelclear"}

	if force {
		args = append(args, "-f")
	}

	args = append(args, device)

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool labelclear", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// Sync forces all pool transactions to be written to stable storage
func (p *Manager) Sync(ctx context.Context, pool string) error {
	args := []string{"sync"}

	if pool != "" {
		args = append(args, pool)
	}

	out, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool sync", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSPoolDeviceOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
	}
	return nil
}

// parseImportablePoolsOutput parses the text output from 'zpool import'
// and returns a structured ImportablePoolsResult
func parseImportablePoolsOutput(output string) (ImportablePoolsResult, error) {
	result := ImportablePoolsResult{
		Pools: []ImportablePool{},
	}

	if strings.TrimSpace(output) == "" || strings.Contains(output, "no pools available") {
		return result, nil
	}

	lines := strings.Split(output, "\n")
	var currentPool *ImportablePool
	var configLines []string
	inConfig := false

	for _, line := range lines {
		// Check for start of a new pool
		if strings.HasPrefix(line, "  pool:") {
			// Save previous pool if any
			if currentPool != nil {
				if len(configLines) > 0 {
					currentPool.Config = strings.Join(configLines, "\n")
				}
				result.Pools = append(result.Pools, *currentPool)
			}

			// Start new pool
			currentPool = &ImportablePool{
				Name: strings.TrimSpace(strings.TrimPrefix(line, "  pool:")),
			}
			configLines = []string{}
			inConfig = false
			continue
		}

		if currentPool == nil {
			continue
		}

		// Parse pool fields
		if strings.HasPrefix(line, "    id:") {
			currentPool.ID = strings.TrimSpace(strings.TrimPrefix(line, "    id:"))
			inConfig = false
		} else if strings.HasPrefix(line, " state:") {
			currentPool.State = strings.TrimSpace(strings.TrimPrefix(line, " state:"))
			inConfig = false
		} else if strings.HasPrefix(line, "action:") {
			currentPool.Action = strings.TrimSpace(strings.TrimPrefix(line, "action:"))
			inConfig = false
		} else if strings.HasPrefix(line, "config:") {
			inConfig = true
		} else if inConfig && strings.TrimSpace(line) != "" {
			// Collect config lines (preserve original formatting)
			configLines = append(configLines, line)
		}
	}

	// Save the last pool
	if currentPool != nil {
		if len(configLines) > 0 {
			currentPool.Config = strings.Join(configLines, "\n")
		}
		result.Pools = append(result.Pools, *currentPool)
	}

	return result, nil
}

// parseIOStatOutput parses the text output from 'zpool iostat'
// and returns a structured IOStatResult
func parseIOStatOutput(output string, verbose bool) (IOStatResult, error) {
	result := IOStatResult{
		Pools: make(map[string]PoolIOStats),
	}

	if strings.TrimSpace(output) == "" {
		return result, nil
	}

	lines := strings.Split(output, "\n")

	// Skip header lines (first 3 lines: header, column names, separator)
	dataStart := 3
	if dataStart >= len(lines) {
		return result, nil
	}

	var currentPool *PoolIOStats
	var currentPoolName string

	for i := dataStart; i < len(lines); i++ {
		line := lines[i]

		// Skip separator lines
		if strings.HasPrefix(line, "---") || strings.TrimSpace(line) == "" {
			continue
		}

		// Parse the line into fields
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		// Check if this is a pool line (no leading whitespace) or vdev line (leading whitespace)
		isVdev := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')

		if !isVdev {
			// This is a pool line
			poolName := fields[0]
			pool := PoolIOStats{
				Name:       poolName,
				Alloc:      fields[1],
				Free:       fields[2],
				Operations: IOOperations{},
				Bandwidth:  IOBandwidth{},
			}

			// Parse operations (read, write)
			if readOps, err := strconv.ParseInt(fields[3], 10, 64); err == nil {
				pool.Operations.Read = readOps
			}
			if writeOps, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
				pool.Operations.Write = writeOps
			}

			// Parse bandwidth (read, write)
			pool.Bandwidth.Read = fields[5]
			pool.Bandwidth.Write = fields[6]

			if verbose {
				pool.VDevStats = make(map[string]VDevStat)
			}

			currentPoolName = poolName
			currentPool = &pool
			result.Pools[poolName] = pool
		} else if verbose && currentPool != nil {
			// This is a vdev line (only present when -v is used)
			vdevName := strings.TrimSpace(fields[0])
			vdev := VDevStat{
				Alloc:      fields[1],
				Free:       fields[2],
				Operations: IOOperations{},
				Bandwidth:  IOBandwidth{},
			}

			// Parse operations
			if readOps, err := strconv.ParseInt(fields[3], 10, 64); err == nil {
				vdev.Operations.Read = readOps
			}
			if writeOps, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
				vdev.Operations.Write = writeOps
			}

			// Parse bandwidth
			vdev.Bandwidth.Read = fields[5]
			vdev.Bandwidth.Write = fields[6]

			// Update the pool in the map with the vdev
			if pool, ok := result.Pools[currentPoolName]; ok {
				if pool.VDevStats == nil {
					pool.VDevStats = make(map[string]VDevStat)
				}
				pool.VDevStats[vdevName] = vdev
				result.Pools[currentPoolName] = pool
			}
		}
	}

	return result, nil
}
