// pkg/zfs/dataset/dataset.go

package dataset

import (
	"context"
	"encoding/json"
	"fmt"

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

// // Create creates a new ZFS dataset
// func (m *Manager) Create(ctx context.Context, cfg CreateConfig) error {
// 	args := []string{"create"}

// 	// Add create options
// 	if cfg.Parents {
// 		args = append(args, "-p")
// 	}

// 	// Add properties
// 	for k, v := range cfg.Properties {
// 		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
// 	}

// 	if cfg.MountPoint != "" {
// 		args = append(args, "-m", cfg.MountPoint)
// 	}

// 	// Add dataset name
// 	args = append(args, cfg.Name)

// 	opts := command.CommandOptions{
// 		Flags: command.FlagForce,
// 	}

// 	_, err := m.executor.Execute(ctx, opts, "zfs create", args...)
// 	if err != nil {
// 		return errors.Wrap(err, errors.ZFSDatasetCreate)
// 	}

// 	return nil
// }

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

// GetProperty gets a specific property of a dataset
func (m *Manager) GetProperty(ctx context.Context, name, property string) (Property, error) {
	args := []string{"get", property}
	if name != "" {
		args = append(args, name)
	}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	out, err := m.executor.Execute(ctx, opts, "zfs get", args...)
	if err != nil {
		return Property{}, errors.Wrap(err, errors.ZFSDatasetGetProperty)
	}

	var result struct {
		Properties map[string]Property `json:"properties"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return Property{}, errors.Wrap(err, errors.CommandOutputParse)
	}

	prop, ok := result.Properties[property]
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

	if cfg.Recursive {
		args = append(args, "-r")
	}

	for k, v := range cfg.Properties {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, fmt.Sprintf("%s@%s", cfg.Dataset, cfg.Name))

	_, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs snapshot", args...)
	return errors.Wrap(err, errors.ZFSSnapshotFailed)
}

// Clone creates a clone from a snapshot
func (m *Manager) Clone(ctx context.Context, cfg CloneConfig) error {
	args := []string{"clone"}

	for k, v := range cfg.Properties {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, cfg.Snapshot, cfg.Name)

	_, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs clone", args...)
	return errors.Wrap(err, errors.ZFSCloneError)
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
	args := []string{"list", "-t", "snapshot", "-H"}

	if opts.Recursive {
		args = append(args, "-r")
	}

	if opts.Dataset != "" {
		args = append(args, opts.Dataset)
	}

	// Add JSON output flag for parsing
	cmdOpts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	out, err := m.executor.Execute(ctx, cmdOpts, "zfs list", args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ZFSSnapshotList)
	}

	var snapshots []SnapshotInfo
	if err := json.Unmarshal(out, &snapshots); err != nil {
		return nil, errors.Wrap(err, errors.CommandOutputParse)
	}

	return snapshots, nil
}

// Missing implementations for dataset Manager
func (m *Manager) DestroySnapshot(ctx context.Context, dataset, snapshot string) error {
	snapshotPath := fmt.Sprintf("%s@%s", dataset, snapshot)
	args := []string{"destroy", snapshotPath}

	opts := command.CommandOptions{
		Flags: command.FlagForce,
	}

	_, err := m.executor.Execute(ctx, opts, "zfs destroy", args...)
	return errors.Wrap(err, errors.ZFSSnapshotDestroy)
}

func (m *Manager) CreateBookmark(ctx context.Context, snapshot, bookmark string) error {
	args := []string{"bookmark", snapshot, bookmark}

	_, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs bookmark", args...)
	return errors.Wrap(err, errors.ZFSBookmarkFailed)
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
