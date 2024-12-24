// pkg/zfs/pool/pool.go

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
	args := []string{"get", "-H", property}
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
		Pools map[string]struct {
			Properties map[string]Property `json:"properties"`
		} `json:"pools"`
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return Property{}, errors.Wrap(err, errors.CommandOutputParse)
	}

	poolData, ok := result.Pools[name]
	if !ok {
		return Property{}, errors.New(errors.ZFSPoolNotFound,
			fmt.Sprintf("pool %s not found", name))
	}

	prop, ok := poolData.Properties[property]
	if !ok {
		return Property{}, errors.New(errors.ZFSPoolPropertyNotFound,
			fmt.Sprintf("property %s not found", property))
	}

	return prop, nil
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

	_, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool export", args...)
	return errors.Wrap(err, errors.ZFSPoolExport)
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

	_, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool scrub", args...)
	return errors.Wrap(err, errors.ZFSPoolScrubFailed)
}

// List returns a list of all pools
func (p *Manager) List(ctx context.Context) ([]Pool, error) {
	args := []string{"-H", "-p", "-o", "name,size,allocated,free,state,health"}

	opts := command.CommandOptions{
		Flags: command.FlagJSON,
	}

	out, err := p.executor.Execute(ctx, opts, "zpool list", args...)
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

func (p *Manager) Resilver(ctx context.Context, name string) error {
	args := []string{"online", "-e", name}

	_, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool", args...)
	return errors.Wrap(err, errors.ZFSPoolResilverFailed)
}

func (p *Manager) AttachDevice(ctx context.Context, pool, device, newDevice string) error {
	args := []string{"attach", pool, device, newDevice}

	_, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool attach", args...)
	return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
}

func (p *Manager) DetachDevice(ctx context.Context, pool, device string) error {
	args := []string{"detach", pool, device}

	_, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool detach", args...)
	return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
}

func (p *Manager) ReplaceDevice(ctx context.Context, pool, oldDevice, newDevice string) error {
	args := []string{"replace", pool, oldDevice, newDevice}

	_, err := p.executor.Execute(ctx, command.CommandOptions{}, "zpool replace", args...)
	return errors.Wrap(err, errors.ZFSPoolDeviceOperation)
}
