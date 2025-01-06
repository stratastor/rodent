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

package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
)

// TODO: When verbose and parsable flags are set on applicable operations, output should be returned to the caller

// Manager handles ZFS dataset operations
type Manager struct {
	executor *command.CommandExecutor
}

func NewManager(executor *command.CommandExecutor) *Manager {
	return &Manager{executor: executor}
}

// List returns a list of datasets
func (m *Manager) List(ctx context.Context, cfg ListConfig) (ListResult, error) {
	// A comma-separated list of types to display, where type  is  one  of:
	// filesystem, snapshot, volume, bookmark, or all.
	args := []string{"list", "-t"}
	listTypes := []string{}
	inputTypes := strings.Split(cfg.Type, ",")

	for _, t := range inputTypes {
		switch t {
		case "filesystem", "fs":
			listTypes = append(listTypes, "filesystem")
		case "snapshot", "snap":
			listTypes = append(listTypes, "snapshot")
		case "volume", "vol":
			listTypes = append(listTypes, "volume")
		case "bookmark":
			listTypes = append(listTypes, "bookmark")
		case "all", "":
			listTypes = append(listTypes, "all")
		default:
			return ListResult{}, errors.New(errors.ZFSNameInvalid, "List type must be one of: filesystem, snapshot, volume, bookmark, all")
		}
	}
	if len(listTypes) == 0 {
		args = append(args, "all")
	} else {
		args = append(args, strings.Join(listTypes, ","))
	}

	if cfg.Recursive {
		args = append(args, "-r")
	}
	if cfg.Depth > 0 {
		args = append(args, "-d", fmt.Sprintf("%d", cfg.Depth))
	}
	if len(cfg.Properties) > 0 {
		args = append(args, "-o", strings.Join(cfg.Properties, ","))
	}
	if cfg.Parsable {
		args = append(args, "-p")
	}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	if cfg.Name != "" {
		args = append(args, cfg.Name)
	}

	result := ListResult{}

	out, err := m.executor.Execute(ctx, opts, "zfs list", args...)
	if err != nil {
		if len(out) > 0 {
			return result, errors.Wrap(err, errors.ZFSDatasetList).
				WithMetadata("output", string(out))
		}
		return result, errors.Wrap(err, errors.ZFSDatasetList)
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return ListResult{}, errors.Wrap(err, errors.CommandOutputParse)
	}

	return result, nil
}

// Destroy removes a dataset
func (m *Manager) Destroy(ctx context.Context, dc DestroyConfig) error {
	args := []string{"destroy"}

	if dc.RecursiveDestroyChildren {
		args = append(args, "-r")
	} else if dc.RecursiveDestroyDependents {
		args = append(args, "-R")
	}
	if dc.Force {
		args = append(args, "-f")
	}
	if dc.DryRun {
		args = append(args, "-n")
	}
	if dc.Parsable {
		args = append(args, "-p")
	}
	if dc.Verbose {
		args = append(args, "-v")
	}

	args = append(args, dc.Name)

	opts := command.CommandOptions{}

	out, err := m.executor.Execute(ctx, opts, "zfs destroy", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSDatasetDestroy).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSDatasetDestroy)
	}

	return nil
}

// GetProperty gets a dataset property
func (m *Manager) GetProperty(ctx context.Context, cfg PropertyConfig) (ListResult, error) {
	name := cfg.Name
	property := cfg.Property

	args := []string{"get", "-H", "-p", "-o", "value,source", property, name}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	result := ListResult{}

	out, err := m.executor.Execute(ctx, opts, "zfs get", args...)
	if err != nil {
		return result, errors.Wrap(err, errors.ZFSDatasetGetProperty)
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return result, errors.Wrap(err, errors.CommandOutputParse)
	}

	ds, ok := result.Datasets[name]
	if !ok {
		return result, errors.New(errors.ZFSDatasetNotFound,
			fmt.Sprintf("dataset %s not found", name))
	}

	_, ok = ds.Properties[property]
	if !ok {
		return result, errors.New(errors.ZFSDatasetPropertyNotFound,
			fmt.Sprintf("property %s not found", property))
	}

	return result, nil
}

// ListProperties returns all properties of a dataset
func (m *Manager) ListProperties(ctx context.Context, cfg NameConfig) (ListResult, error) {
	name := cfg.Name
	args := []string{"get", "all", "-H", "-p", name}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	result := ListResult{}

	out, err := m.executor.Execute(ctx, opts, "zfs get", args...)
	if err != nil {
		return result, errors.Wrap(err, errors.ZFSDatasetGetProperty)
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return result, errors.Wrap(err, errors.CommandOutputParse)
	}

	_, ok := result.Datasets[name]
	if !ok {
		return result, errors.New(errors.ZFSDatasetNotFound,
			fmt.Sprintf("dataset %s not found", name))
	}

	return result, nil
}

// InheritProperty clears the specified property, causing it to be inherited from an ancestor, restored to default if no ancestor has the property set, or with the -S option reverted to the received value if one exists
func (m *Manager) InheritProperty(ctx context.Context, cfg InheritConfig) error {
	args := []string{"inherit"}

	if cfg.Recursive {
		args = append(args, "-r")
	}
	if cfg.Revert {
		args = append(args, "-S")
	}

	args = append(args, cfg.Property)
	for _, name := range cfg.Names {
		args = append(args, name)
	}

	opts := command.CommandOptions{}

	_, err := m.executor.Execute(ctx, opts, "zfs inherit", args...)
	if err != nil {
		return errors.Wrap(err, errors.ZFSDatasetSetProperty)
	}

	return nil
}

// SetProperty sets a property on a dataset
func (m *Manager) SetProperty(ctx context.Context, cfg SetPropertyConfig) error {
	// TODO: Accommmodate multiple property values
	// TODO: Accommodate multiple dataset names
	args := []string{"set", fmt.Sprintf("%s=%s", cfg.Property, shellquote.Join(cfg.Value)), cfg.Name}

	opts := command.CommandOptions{}

	_, err := m.executor.Execute(ctx, opts, "zfs set", args...)
	if err != nil {
		return errors.Wrap(err, errors.ZFSDatasetSetProperty)
	}

	return nil
}

// CreateFilesystem creates a new ZFS filesystem
func (m *Manager) CreateFilesystem(ctx context.Context, cfg FilesystemConfig) error {
	args := []string{"create"}

	if cfg.Parents {
		args = append(args, "-p")
	}
	if cfg.DoNotMount {
		args = append(args, "-u")
	}
	if cfg.DryRun {
		args = append(args, "-n")
	}
	if cfg.Parsable {
		args = append(args, "-P")
	}
	if cfg.Verbose {
		args = append(args, "-v")
	}

	for k, v := range cfg.Properties {
		quotedValue := shellquote.Join(v)
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, quotedValue))
	}

	args = append(args, cfg.Name)

	opts := command.CommandOptions{}

	out, err := m.executor.Execute(ctx, opts, "zfs create", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSDatasetCreate).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSDatasetCreate)
	}

	return nil
}

// CreateVolume creates a new ZFS volume
func (m *Manager) CreateVolume(ctx context.Context, cfg VolumeConfig) error {
	args := []string{"create"}

	if cfg.Size != "" {
		args = append(args, "-V", cfg.Size)
	} else {
		return errors.New(errors.ZFSInvalidSize, "volume size not specified")
	}
	if cfg.Sparse {
		args = append(args, "-s")
	}

	if size, ok := cfg.Properties["blocksize"]; ok {
		args = append(args, "-b", size)
		// Remove from properties to avoid duplication
		delete(cfg.Properties, "blocksize")
	} else if cfg.BlockSize != "" {
		args = append(args, "-b", cfg.BlockSize)
	}

	if cfg.Parents {
		args = append(args, "-p")
	}
	if cfg.DryRun {
		args = append(args, "-n")
	}
	if cfg.Parsable {
		args = append(args, "-P")
	}
	if cfg.Verbose {
		args = append(args, "-v")
	}

	for k, v := range cfg.Properties {
		quotedValue := shellquote.Join(v)
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, quotedValue))
	}

	args = append(args, cfg.Name)

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs create", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSDatasetCreate).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSDatasetCreate)
	}
	return nil
}

// CreateSnapshot creates a new ZFS snapshot
func (m *Manager) CreateSnapshot(ctx context.Context, cfg SnapshotConfig) error {
	args := []string{"snapshot"}

	// Add recursive flag if specified
	if cfg.Recursive {
		args = append(args, "-r")
	}

	// Add properties if specified
	for k, v := range cfg.Properties {
		quotedValue := shellquote.Join(v)
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, quotedValue))
	}

	// Form snapshot name as dataset@snapshot
	snapStr := fmt.Sprintf("%s@%s", cfg.Name, cfg.SnapName)
	args = append(args, snapStr)

	opts := command.CommandOptions{}
	out, err := m.executor.Execute(ctx, opts, "zfs snapshot", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSSnapshotFailed).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSSnapshotFailed)
	}

	return nil
}

// Exists checks if a dataset exists
func (m *Manager) Exists(ctx context.Context, name string) (bool, error) {
	args := []string{"list", "-H"}

	// Add type flag for bookmarks
	if strings.Contains(name, "#") {
		args = append(args, "-t", "bookmark")
	} else if strings.Contains(name, "@") {
		args = append(args, "-t", "snapshot")
	}

	args = append(args, name)

	opts := command.CommandOptions{}
	_, err := m.executor.Execute(ctx, opts, "zfs list", args...)
	if err != nil {
		// if cmdErr, ok := err.(*errors.RodentError); ok {
		// 	// Check if it's a non-existent dataset error (exit code 1)
		// 	if cmdErr.Code == errors.CommandExecution {
		// 		return false, nil
		// 	}
		// }
		return false, errors.Wrap(err, errors.ZFSDatasetList)
	}

	return true, nil
}

// Clone creates a clone from a snapshot
func (m *Manager) Clone(ctx context.Context, cfg CloneConfig) error {
	args := []string{"clone"}

	if cfg.Parents {
		args = append(args, "-p")
	}

	// Add properties if specified
	for k, v := range cfg.Properties {
		quotedValue := shellquote.Join(v)
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, quotedValue))
	}

	args = append(args, cfg.Name, cfg.CloneName)

	opts := command.CommandOptions{}
	out, err := m.executor.Execute(ctx, opts, "zfs clone", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSCloneError).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSCloneError)
	}

	return nil
}

func (m *Manager) PromoteClone(ctx context.Context, cfg NameConfig) error {
	args := []string{"promote", cfg.Name}

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs promote", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSCloneError).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSCloneError)
	}

	return nil
}

// Rename renames a dataset
func (m *Manager) Rename(ctx context.Context, cfg RenameConfig) error {
	args := []string{"rename"}

	if cfg.Recursive {
		// Only for snapshots
		// TODO: assert snap? ZFS will return an error anyway
		args = append(args, "-r")
	} else {
		if cfg.Force {
			args = append(args, "-f")
		}
		// Volumes can't have -u flag
		// -u and -p options are mutually exclusive even for fs
		if cfg.DoNotMount {
			args = append(args, "-u")
		} else if cfg.Parents {
			args = append(args, "-p")
		}
	}

	args = append(args, cfg.Name, cfg.NewName)

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs rename", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSDatasetRename).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSDatasetRename)
	}
	return nil
}

// Rollback rolls back a dataset to a snapshot
func (m *Manager) Rollback(ctx context.Context, cfg RollbackConfig) error {
	args := []string{"rollback"}

	if cfg.DestroyRecent {
		args = append(args, "-r")
	}
	if cfg.DestroyRecentClones {
		args = append(args, "-R")
		if cfg.ForceUnmount {
			args = append(args, "-f")
		}
	}

	args = append(args, cfg.Name)

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs rollback", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSSnapshotRollback).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSSnapshotRollback)
	}

	return nil
}

// CreateBookmark creates a bookmark from a snapshot
func (m *Manager) CreateBookmark(ctx context.Context, cfg BookmarkConfig) error {
	args := []string{"bookmark", cfg.Name, cfg.BookmarkName}

	opts := command.CommandOptions{}
	out, err := m.executor.Execute(ctx, opts, "zfs bookmark", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSBookmarkFailed).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSBookmarkFailed)
	}

	return nil
}

// Mount operations
func (m *Manager) Mount(ctx context.Context, cfg MountConfig) error {
	args := []string{"mount"}

	if cfg.Recursive {
		args = append(args, "-R")
	}
	if cfg.Force {
		args = append(args, "-f")
	}
	if cfg.Verbose {
		args = append(args, "-v")
	}
	if cfg.TempMountPoint != "" {
		args = append(args, "-o", fmt.Sprintf("mountpoint='%s'", cfg.TempMountPoint))
	}

	for _, opt := range cfg.Options {
		args = append(args, "-o", opt)
	}

	args = append(args, cfg.Name)

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs mount", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSMountError).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSMountError)
	}
	return nil
}

func (m *Manager) Unmount(ctx context.Context, cfg UnmountConfig) error {
	args := []string{"unmount"}

	if cfg.Force {
		args = append(args, "-f")
	}

	args = append(args, cfg.Name)

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs unmount", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSMountError).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSMountError)
	}
	return nil
}

// Diff shows the difference between snapshots or between a snapshot and filesystem
func (m *Manager) Diff(ctx context.Context, cfg DiffConfig) (DiffResult, error) {
	// Validate number of arguments
	if len(cfg.Names) != 2 {
		return DiffResult{}, errors.New(errors.CommandInvalidInput,
			"Exactly two names required for diff operation")
	}

	args := []string{"diff"}

	// Always use these flags for consistent parsable output
	args = append(args, "-H", "-t", "-F")

	// Add snapshot/filesystem arguments
	args = append(args, cfg.Names...)

	opts := command.CommandOptions{}

	out, err := m.executor.Execute(ctx, opts, "zfs diff", args...)
	if err != nil {
		if len(out) > 0 {
			return DiffResult{}, errors.Wrap(err, errors.ZFSDatasetOperation).
				WithMetadata("output", string(out))
		}
		return DiffResult{}, errors.Wrap(err, errors.ZFSDatasetOperation)
	}

	// Parse the output
	result := DiffResult{
		Changes: make([]DiffEntry, 0),
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Split on tabs
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}

		// Parse timestamp
		timestamp, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}

		entry := DiffEntry{
			Timestamp:  timestamp,
			ChangeType: fields[1],
			FileType:   fields[2],
			Path:       fields[3],
		}

		// Handle rename entries which have an additional field
		if entry.ChangeType == "R" && len(fields) > 4 {
			entry.NewPath = fields[4]
		}

		result.Changes = append(result.Changes, entry)
	}

	return result, nil
}

// Allow grants permissions on a dataset
func (m *Manager) Allow(ctx context.Context, cfg AllowConfig) error {
	args := []string{"allow"}

	// Handle mutually exclusive flags
	switch {
	case cfg.SetName != "":
		// Permission set definition
		if !strings.HasPrefix(cfg.SetName, "@") {
			return errors.New(errors.ZFSNameInvalid, "Permission set name must start with @")
		}
		args = append(args, "-s", cfg.SetName)
	case cfg.Create:
		// Create-time permissions
		args = append(args, "-c")
	case cfg.Everyone:
		// Everyone permissions
		args = append(args, "-e")
		if cfg.Local {
			args = append(args, "-l")
		}
		if cfg.Descendent {
			args = append(args, "-d")
		}
	case len(cfg.Groups) > 0:
		// Group permissions
		args = append(args, "-g")
		args = append(args, strings.Join(cfg.Groups, ","))

		if cfg.Local {
			args = append(args, "-l")
		}
		if cfg.Descendent {
			args = append(args, "-d")
		}
	case len(cfg.Users) > 0:
		// User permissions
		args = append(args, "-u")
		args = append(args, strings.Join(cfg.Users, ","))
		if cfg.Local {
			args = append(args, "-l")
		}
		if cfg.Descendent {
			args = append(args, "-d")
		}
	default:
		// User/group permissions
		if cfg.Local {
			args = append(args, "-l")
		}
		if cfg.Descendent {
			args = append(args, "-d")
		}
	}

	// Add permissions
	if len(cfg.Permissions) > 0 {
		args = append(args, strings.Join(cfg.Permissions, ","))
	}

	// Add dataset name
	args = append(args, cfg.Name)

	// Use CommandOptions.CaptureStderr to capture stderr even on success
	opts := command.CommandOptions{
		CaptureStderr: true,
	}

	out, err := m.executor.Execute(ctx, opts, "zfs allow", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSDatasetOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSDatasetOperation)
	}

	// TODO: Check with upstream OpenZFS
	// A workaround for ZFS bug where it returns success even when it fails
	// Check stderr for error messages even with success exit code
	if strings.Contains(string(out), "invalid user") ||
		strings.Contains(string(out), "error:") {
		return errors.New(errors.ZFSCommandFailed, string(out))
	}

	return nil
}

// Unallow revokes permissions on a dataset
func (m *Manager) Unallow(ctx context.Context, cfg UnallowConfig) error {
	args := []string{"unallow"}

	// Handle flags based on configuration
	switch {
	case cfg.SetName != "":
		// Permission set definition
		if !strings.HasPrefix(cfg.SetName, "@") {
			return errors.New(errors.ZFSNameInvalid, "Permission set name must start with @")
		}
		args = append(args, "-s", cfg.SetName)
	case cfg.Create:
		// Create-time permissions
		args = append(args, "-c")
	case cfg.Everyone:
		// Everyone permissions
		args = append(args, "-e")
		if cfg.Local {
			args = append(args, "-l")
		}
		if cfg.Descendent {
			args = append(args, "-d")
		}
	case len(cfg.Groups) > 0:
		// Group permissions
		args = append(args, "-g")
		args = append(args, strings.Join(cfg.Groups, ","))

		if cfg.Local {
			args = append(args, "-l")
		}
		if cfg.Descendent {
			args = append(args, "-d")
		}
	case len(cfg.Users) > 0:
		// User permissions
		args = append(args, "-u")
		args = append(args, strings.Join(cfg.Users, ","))
		if cfg.Local {
			args = append(args, "-l")
		}
		if cfg.Descendent {
			args = append(args, "-d")
		}
	default:
		// User/group permissions
		if cfg.Local {
			args = append(args, "-l")
		}
		if cfg.Descendent {
			args = append(args, "-d")
		}
	}

	if cfg.Recursive {
		args = append(args, "-r")
	}

	// Add permissions to remove
	if len(cfg.Permissions) > 0 {
		args = append(args, strings.Join(cfg.Permissions, ","))
	}

	// Add dataset name
	args = append(args, cfg.Name)

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs unallow", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSDatasetOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSDatasetOperation)
	}
	return nil
}

// ListPermissions gets delegated permissions on a dataset
func (m *Manager) ListPermissions(ctx context.Context, cfg NameConfig) (AllowResult, error) {
	args := []string{"allow", cfg.Name}

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs allow", args...)
	if err != nil {
		return AllowResult{}, errors.Wrap(err, errors.ZFSPermissionError)
	}

	return parseAllowOutput(string(out))
}

// parseAllowOutput parses the output of zfs allow command
func parseAllowOutput(output string) (AllowResult, error) {
	result := AllowResult{
		PermissionSets:  make(map[string][]string),
		Local:           make(map[string][]string),
		Descendent:      make(map[string][]string),
		LocalDescendent: make(map[string][]string),
	}

	var currentSection string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and headers
		if line == "" || strings.HasPrefix(line, "----") {
			continue
		}

		// Detect sections
		switch {
		case strings.HasPrefix(line, "Permission sets:"):
			currentSection = "sets"
			continue
		case strings.HasPrefix(line, "Create time permissions:"):
			currentSection = "create"
			continue
		case strings.HasPrefix(line, "Local permissions:"):
			currentSection = "local"
			continue
		case strings.HasPrefix(line, "Descendent permissions:"):
			currentSection = "descendent"
			continue
		case strings.HasPrefix(line, "Local+Descendent permissions:"):
			currentSection = "local+descendent"
			continue
		}

		// Parse section content
		switch currentSection {
		case "sets":
			if parts := strings.Fields(line); len(parts) >= 2 {
				setName := parts[0]
				perms := strings.Split(parts[1], ",")
				result.PermissionSets[setName] = perms
			}
		case "create":
			result.CreateTime = strings.Split(line, ",")
		case "local", "descendent", "local+descendent":
			if parts := strings.Fields(line); len(parts) >= 2 {
				entity := strings.Join(parts[:2], " ")
				perms := strings.Split(parts[2], ",")
				switch currentSection {
				case "local":
					result.Local[entity] = perms
				case "descendent":
					result.Descendent[entity] = perms
				case "local+descendent":
					result.LocalDescendent[entity] = perms
				}
			}
		}
	}

	return result, nil
}

// TODO: share/unshare is a bit rough; disturbs my peace.
// If sharenfs and sharesmb properties are set to off, it falls to being legacy which is then useless for `zfs share`
// When sharenfs and sharesmb are set to a value, it's exposed by default and requires handling.
// Besides, sharesmb seems to be very rudimentary and needs manual work on smb.conf anyway.
// Share shares a ZFS dataset
func (m *Manager) Share(ctx context.Context, cfg ShareConfig) error {
	args := []string{"share"}

	if cfg.LoadKeys {
		args = append(args, "-l")
	}

	if cfg.All {
		args = append(args, "-a")
	} else if cfg.Name == "" {
		return errors.New(errors.CommandInvalidInput,
			"Dataset name required when not using -a flag")
	} else {
		args = append(args, cfg.Name)
	}

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs share", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSDatasetOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSDatasetOperation)
	}
	return nil
}

// Unshare unshares a ZFS dataset
func (m *Manager) Unshare(ctx context.Context, cfg UnshareConfig) error {
	args := []string{"unshare"}

	if cfg.All {
		args = append(args, "-a")
	} else if cfg.Name == "" {
		return errors.New(errors.CommandInvalidInput,
			"Dataset name or mountpoint required when not using -a flag")
	} else {
		args = append(args, cfg.Name)
	}

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs unshare", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSDatasetOperation).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSDatasetOperation)
	}
	return nil
}
