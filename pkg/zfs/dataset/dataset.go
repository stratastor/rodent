package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
)

// TODO: Validate dataset names with relevant functions from common/zfs_namecheck.go

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

// SetProperty sets a property on a dataset
func (m *Manager) SetProperty(ctx context.Context, cfg SetPropertyConfig) error {
	args := []string{"set", fmt.Sprintf("%s=%s", cfg.Property, cfg.Value), cfg.Name}

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
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
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
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
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
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
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
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
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
		args = append(args, "-o", fmt.Sprintf("mountpoint=%s", cfg.TempMountPoint))
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
