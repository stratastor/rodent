package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
)

// Manager handles ZFS dataset operations
type Manager struct {
	executor *command.CommandExecutor
}

func NewManager(executor *command.CommandExecutor) *Manager {
	return &Manager{executor: executor}
}

// Create creates a ZFS dataset (filesystem or volume)
func (m *Manager) Create(ctx context.Context, cfg CreateConfig) error {
	args := []string{"create"}

	// Add parent flag if specified
	if cfg.Parents {
		args = append(args, "-p")
	}

	// Add volume flag and size for volumes
	if cfg.Type == "volume" {
		if size, ok := cfg.Properties["volsize"]; ok {
			args = append(args, "-V", size)
			// Remove from properties to avoid duplication
			delete(cfg.Properties, "volsize")
		} else {
			return errors.New(errors.ZFSInvalidSize, "volume size not specified")
		}
	}

	// Add remaining properties if specified
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

// List returns a list of datasets
func (m *Manager) List(ctx context.Context, recursive bool) ([]Dataset, error) {
	args := []string{"list", "-t", "all"}

	if recursive {
		args = append(args, "-r")
	}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	out, err := m.executor.Execute(ctx, opts, "zfs list", args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ZFSDatasetList)
	}

	var result struct {
		Datasets []Dataset `json:"datasets"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, errors.Wrap(err, errors.CommandOutputParse)
	}

	return result.Datasets, nil
}

// Destroy removes a dataset
func (m *Manager) Destroy(ctx context.Context, name string, recursive bool) error {
	args := []string{"destroy"}

	if recursive {
		args = append(args, "-r")
	}

	args = append(args, name)

	opts := command.CommandOptions{
		Flags: command.FlagForce,
	}

	_, err := m.executor.Execute(ctx, opts, "zfs destroy", args...)
	if err != nil {
		return errors.Wrap(err, errors.ZFSDatasetDestroy)
	}

	return nil
}

// GetProperty gets a dataset property
func (m *Manager) GetProperty(ctx context.Context, name, property string) (Property, error) {
	args := []string{"get", "-H", "-p", "-o", "value,source", property, name}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	out, err := m.executor.Execute(ctx, opts, "zfs get", args...)
	if err != nil {
		return Property{}, errors.Wrap(err, errors.ZFSDatasetGetProperty)
	}

	var result struct {
		Datasets map[string]struct {
			Properties map[string]Property `json:"properties"`
		} `json:"datasets"`
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return Property{}, errors.Wrap(err, errors.CommandOutputParse)
	}

	ds, ok := result.Datasets[name]
	if !ok {
		return Property{}, errors.New(errors.ZFSDatasetNotFound,
			fmt.Sprintf("dataset %s not found", name))
	}

	prop, ok := ds.Properties[property]
	if !ok {
		return Property{}, errors.New(errors.ZFSDatasetPropertyNotFound,
			fmt.Sprintf("property %s not found", property))
	}

	return prop, nil
}

// SetProperty sets a property on a dataset
func (m *Manager) SetProperty(ctx context.Context, name, property, value string) error {
	args := []string{"set", fmt.Sprintf("%s=%s", property, value), name}

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

	for k, v := range cfg.Properties {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	if cfg.MountPoint != "" {
		args = append(args, "-m", cfg.MountPoint)
	}

	args = append(args, cfg.Name)

	_, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs create", args...)
	return errors.Wrap(err, errors.ZFSDatasetCreate)
}

// CreateVolume creates a new ZFS volume
func (m *Manager) CreateVolume(ctx context.Context, cfg VolumeConfig) error {
	args := []string{"create", "-V", cfg.Size}

	if cfg.Parents {
		args = append(args, "-p")
	}

	for k, v := range cfg.Properties {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, cfg.Name)

	_, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs create", args...)
	return errors.Wrap(err, errors.ZFSDatasetCreate)
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
	snapName := fmt.Sprintf("%s@%s", cfg.Dataset, cfg.Name)
	args = append(args, snapName)

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

	// Add properties if specified
	for k, v := range cfg.Properties {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, cfg.Snapshot, cfg.Name)

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

func (m *Manager) PromoteClone(ctx context.Context, name string) error {
	args := []string{"promote", name}

	_, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs promote", args...)
	return errors.Wrap(err, errors.ZFSCloneError)
}

// Rename renames a dataset
func (m *Manager) Rename(ctx context.Context, oldName string, cfg RenameConfig) error {
	args := []string{"rename"}

	if cfg.CreateParent {
		args = append(args, "-p")
	}
	if cfg.Force {
		args = append(args, "-f")
	}

	args = append(args, oldName, cfg.NewName)

	_, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs rename", args...)
	return errors.Wrap(err, errors.ZFSDatasetRename)
}

// Rollback rolls back a dataset to a snapshot
func (m *Manager) Rollback(ctx context.Context, dataset string, cfg RollbackConfig) error {
	args := []string{"rollback"}

	if cfg.Force {
		args = append(args, "-f")
	}
	if cfg.Recursive {
		args = append(args, "-r")
	}

	args = append(args, fmt.Sprintf("%s@%s", dataset, cfg.Snapshot))

	_, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs rollback", args...)
	return errors.Wrap(err, errors.ZFSSnapshotRollback)
}

// ListSnapshots returns a list of snapshots for the given dataset
func (m *Manager) ListSnapshots(ctx context.Context, opts SnapshotListOptions) ([]SnapshotInfo, error) {
	args := []string{"list", "-t", "snapshot", "-H", "-p"}

	if opts.Recursive {
		args = append(args, "-r")
	}

	if opts.Dataset != "" {
		args = append(args, opts.Dataset)
	}

	cmdOpts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	out, err := m.executor.Execute(ctx, cmdOpts, "zfs list", args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ZFSSnapshotList)
	}

	var result struct {
		Datasets []SnapshotInfo `json:"datasets"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, errors.Wrap(err, errors.CommandOutputParse).
			WithMetadata("output", string(out))
	}

	return result.Datasets, nil
}

// DestroySnapshot removes a snapshot
func (m *Manager) DestroySnapshot(ctx context.Context, dataset, snapshot string) error {
	// Form full snapshot name
	snapName := fmt.Sprintf("%s@%s", dataset, snapshot)
	args := []string{"destroy", snapName}

	opts := command.CommandOptions{
		Flags: command.FlagForce,
	}

	out, err := m.executor.Execute(ctx, opts, "zfs destroy", args...)
	if err != nil {
		if len(out) > 0 {
			return errors.Wrap(err, errors.ZFSSnapshotDestroy).
				WithMetadata("output", string(out))
		}
		return errors.Wrap(err, errors.ZFSSnapshotDestroy)
	}

	return nil
}

// CreateBookmark creates a bookmark from a snapshot
func (m *Manager) CreateBookmark(ctx context.Context, cfg BookmarkConfig) error {
	args := []string{"bookmark", cfg.Snapshot, cfg.Bookmark}

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

func (m *Manager) ListBookmarks(ctx context.Context, dataset string) ([]Dataset, error) {
	args := []string{"list", "-t", "bookmark"}
	if dataset != "" {
		args = append(args, "-r", dataset)
	}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	out, err := m.executor.Execute(ctx, opts, "zfs list", args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ZFSBookmarkFailed)
	}

	var result struct {
		Datasets []Dataset `json:"datasets"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, errors.Wrap(err, errors.CommandOutputParse)
	}

	return result.Datasets, nil
}

// Mount operations
func (m *Manager) Mount(ctx context.Context, cfg MountConfig) error {
	args := []string{"mount"}

	if cfg.MountPoint != "" {
		args = append(args, "-o", fmt.Sprintf("mountpoint=%s", cfg.MountPoint))
	}

	for _, opt := range cfg.Options {
		args = append(args, "-o", opt)
	}

	args = append(args, cfg.Dataset)

	_, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs mount", args...)
	return errors.Wrap(err, errors.ZFSMountError)
}

func (m *Manager) Unmount(ctx context.Context, dataset string, force bool) error {
	args := []string{"unmount"}

	if force {
		args = append(args, "-f")
	}

	args = append(args, dataset)

	_, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs unmount", args...)
	return errors.Wrap(err, errors.ZFSMountError)
}
