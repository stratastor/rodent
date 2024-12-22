// pkg/zfs/pool/pool.go

package pool

import (
	"context"
	"encoding/json"
	"fmt"

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
		if spec.Type != "" {
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
	args := []string{"create"}

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

	_, err := p.executor.Execute(ctx, opts, "zpool create", args...)
	if err != nil {
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

	_, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool import", args...)
	if err != nil {
		return errors.Wrap(err, errors.ZFSPoolImport)
	}

	return nil
}

// Status gets the status of a pool
func (p *Manager) Status(ctx context.Context, name string) (*PoolStatus, error) {
	args := []string{"status"}
	if name != "" {
		args = append(args, name)
	}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	out, err := p.executor.Execute(ctx, opts, "zpool status", args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ZFSPoolStatus)
	}

	var status PoolStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, errors.Wrap(err, errors.CommandOutputParse)
	}

	return &status, nil
}

// GetProperty gets a specific property of a pool
func (p *Manager) GetProperty(ctx context.Context, name, property string) (Property, error) {
	args := []string{"get", property}
	if name != "" {
		args = append(args, name)
	}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	out, err := p.executor.Execute(ctx, opts, "zpool get", args...)
	if err != nil {
		return Property{}, errors.Wrap(err, errors.ZFSPoolGetProperty)
	}

	var result struct {
		Properties map[string]Property `json:"properties"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return Property{}, errors.Wrap(err, errors.CommandOutputParse)
	}

	prop, ok := result.Properties[property]
	if !ok {
		return Property{}, errors.New(errors.ZFSPoolPropertyNotFound,
			fmt.Sprintf("property %s not found", property))
	}

	return prop, nil
}

// Export exports a ZFS pool
func (p *Manager) Export(ctx context.Context, name string, force bool) error {
	args := []string{"export"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	_, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool export", args...)
	return errors.Wrap(err, errors.ZFSPoolExport)
}

// Destroy destroys a ZFS pool
func (p *Manager) Destroy(ctx context.Context, name string, force bool) error {
	args := []string{"destroy"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	_, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool destroy", args...)
	return errors.Wrap(err, errors.ZFSPoolDestroy)
}

// Scrub starts/stops a scrub on a pool
func (p *Manager) Scrub(ctx context.Context, name string, stop bool) error {
	args := []string{"scrub"}
	if stop {
		args = append(args, "-s")
	}
	args = append(args, name)

	_, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool scrub", args...)
	return errors.Wrap(err, errors.ZFSPoolScrubFailed)
}

// List returns a list of all pools
func (p *Manager) List(ctx context.Context) ([]Pool, error) {
	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	out, err := p.executor.Execute(ctx, opts, "zpool list", []string{})
	if err != nil {
		return nil, errors.Wrap(err, errors.ZFSPoolList)
	}

	var result struct {
		Pools []Pool `json:"pools"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, errors.Wrap(err, errors.CommandOutputParse)
	}

	return result.Pools, nil
}
