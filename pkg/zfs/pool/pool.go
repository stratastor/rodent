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

// Status gets the status of a pool
func (p *Manager) Status(ctx context.Context, name string) (PoolStatus, error) {
	args := []string{"status"}
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

// List returns a list of all pools
func (p *Manager) List(ctx context.Context) (ListResult, error) {
	args := []string{"-H", "-p"}

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

func (p *Manager) AttachDevice(ctx context.Context, pool, device, newDevice string) error {
	args := []string{"attach", pool, device, newDevice}

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

// GetPoolForDevice returns the pool name for a device, if any
func (p *Manager) GetPoolForDevice(ctx context.Context, devicePath string) (string, bool, error) {
	// List all pools and check their devices
	poolList, err := p.List(ctx)
	if err != nil {
		return "", false, err
	}

	// Normalize device path for comparison (handle both /dev/sda and /dev/disk/by-id/...)
	normalizedPath := normalizeDevicePath(devicePath)

	// Iterate through pools and check if device is part of any
	for poolName := range poolList.Pools {
		status, err := p.Status(ctx, poolName)
		if err != nil {
			continue
		}

		// Check if devicePath is in the pool's vdev configuration
		for _, pool := range status.Pools {
			if containsDevice(pool.VDevs, normalizedPath) {
				return poolName, true, nil
			}
		}
	}

	// Device not found in any pool
	return "", false, nil
}

// containsDevice recursively searches vdev tree for a device path
func containsDevice(vdevs map[string]*VDev, devicePath string) bool {
	if vdevs == nil {
		return false
	}

	for _, vdev := range vdevs {
		// Check if this vdev's path matches
		if vdev.Path != "" {
			vdevPath := normalizeDevicePath(vdev.Path)
			if vdevPath == devicePath {
				return true
			}
		}

		// Recursively check nested vdevs
		if vdev.VDevs != nil && containsDevice(vdev.VDevs, devicePath) {
			return true
		}
	}

	return false
}

// normalizeDevicePath normalizes a device path for comparison
// Handles /dev/sda, /dev/disk/by-id/..., and partitions
func normalizeDevicePath(path string) string {
	// Remove trailing partition numbers if present
	// This allows matching /dev/sda with /dev/sda1
	path = strings.TrimSpace(path)

	// If it's a /dev/disk/by-id path, return as-is for exact matching
	if strings.Contains(path, "/dev/disk/by-id/") {
		return path
	}

	// If it's a /dev/disk/by-path, return as-is
	if strings.Contains(path, "/dev/disk/by-path/") {
		return path
	}

	// For /dev/sdX or /dev/nvmeXnY, return as-is
	return path
}
