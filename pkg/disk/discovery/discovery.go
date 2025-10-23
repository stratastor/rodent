// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk/parsers"
	"github.com/stratastor/rodent/pkg/disk/tools"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/system"
)

// ZFSPoolManager interface for ZFS pool operations (minimal subset needed for discovery)
type ZFSPoolManager interface {
	// GetPoolForDevice returns the pool name for a device, if any
	GetPoolForDevice(ctx context.Context, devicePath string) (string, bool, error)
}

// Discoverer handles disk discovery operations
type Discoverer struct {
	logger         logger.Logger
	lsblk          *tools.LsblkExecutor
	smartctl       *tools.SmartctlExecutor
	udevadm        *tools.UdevadmExecutor
	toolChecker    *tools.ToolChecker
	zfsPoolManager ZFSPoolManager
	envDetector    *system.EnvironmentDetector
	mu             sync.RWMutex
	lastScan       time.Time
	deviceCache    map[string]*types.PhysicalDisk // Keyed by device path
}

// NewDiscoverer creates a new disk discoverer
func NewDiscoverer(
	l logger.Logger,
	lsblk *tools.LsblkExecutor,
	smartctl *tools.SmartctlExecutor,
	udevadm *tools.UdevadmExecutor,
	toolChecker *tools.ToolChecker,
	zfsPoolManager ZFSPoolManager,
	envDetector *system.EnvironmentDetector,
) *Discoverer {
	return &Discoverer{
		logger:         l,
		lsblk:          lsblk,
		smartctl:       smartctl,
		udevadm:        udevadm,
		toolChecker:    toolChecker,
		zfsPoolManager: zfsPoolManager,
		envDetector:    envDetector,
		deviceCache:    make(map[string]*types.PhysicalDisk),
	}
}

// DiscoverAll discovers all physical disks on the system
func (d *Discoverer) DiscoverAll(ctx context.Context) ([]*types.PhysicalDisk, error) {
	d.logger.Info("starting disk discovery")
	startTime := time.Now()

	// Get all block devices using lsblk
	devices, err := d.discoverBlockDevices(ctx)
	if err != nil {
		return nil, errors.Wrap(err, errors.DiskDiscoveryFailed).
			WithMetadata("operation", "discover_block_devices")
	}

	d.logger.Debug("discovered block devices", "count", len(devices))

	// Enrich with udev information
	if d.toolChecker.IsAvailable("udevadm") {
		d.enrichWithUdev(ctx, devices)
	} else {
		d.logger.Warn("udevadm not available, skipping udev enrichment")
	}

	// Enrich with SMART information (if available)
	if d.toolChecker.IsAvailable("smartctl") {
		d.enrichWithSMART(ctx, devices)
	} else {
		d.logger.Warn("smartctl not available, skipping SMART enrichment")
	}

	// Enrich with ZFS pool membership (if ZFS pool manager available)
	if d.zfsPoolManager != nil {
		d.enrichWithPoolMembership(ctx, devices)
	} else {
		d.logger.Debug("ZFS pool manager not available, skipping pool membership check")
	}

	// Update cache
	d.mu.Lock()
	d.deviceCache = make(map[string]*types.PhysicalDisk)
	for _, dev := range devices {
		d.deviceCache[dev.DevicePath] = dev
	}
	d.lastScan = time.Now()
	d.mu.Unlock()

	d.logger.Info("disk discovery completed",
		"total_disks", len(devices),
		"duration", time.Since(startTime))

	return devices, nil
}

// discoverBlockDevices discovers block devices using lsblk
func (d *Discoverer) discoverBlockDevices(ctx context.Context) ([]*types.PhysicalDisk, error) {
	// Execute lsblk
	output, err := d.lsblk.ListDisks(ctx)
	if err != nil {
		return nil, errors.Wrap(err, errors.DiskDiscoveryFailed).
			WithMetadata("tool", "lsblk")
	}

	// Parse lsblk output
	blockDevices, err := parsers.ParseLsblkJSON(output)
	if err != nil {
		return nil, err
	}

	// Filter only physical disks
	physicalDevices := parsers.FilterPhysicalDisks(blockDevices)

	// Convert to PhysicalDisk types
	var disks []*types.PhysicalDisk
	for _, bd := range physicalDevices {
		disk := bd.ToPhysicalDisk()
		disk.State = types.DiskStateDiscovered
		disk.Health = types.HealthUnknown
		disks = append(disks, disk)
	}

	return disks, nil
}

// enrichWithUdev enriches disk information with udev data
func (d *Discoverer) enrichWithUdev(ctx context.Context, disks []*types.PhysicalDisk) {
	for _, disk := range disks {
		output, err := d.udevadm.Info(ctx, disk.DevicePath)
		if err != nil {
			d.logger.Warn("failed to get udev info",
				"device", disk.DevicePath,
				"error", err)
			continue
		}

		// Parse udev properties
		props := parseUdevProperties(string(output))

		// Update disk with udev information
		if id, ok := props["ID_SERIAL"]; ok {
			if disk.Serial == "" {
				disk.Serial = id
			}
		}
		if wwn, ok := props["ID_WWN"]; ok {
			disk.WWN = wwn
		}
		if model, ok := props["ID_MODEL"]; ok {
			if disk.Model == "" {
				disk.Model = model
			}
		}
		if vendor, ok := props["ID_VENDOR"]; ok {
			if disk.Vendor == "" {
				disk.Vendor = vendor
			}
		}

		// Get by-id path
		if byID, ok := props["DEVLINKS"]; ok {
			for _, link := range strings.Fields(byID) {
				if strings.Contains(link, "/dev/disk/by-id/") {
					disk.ByIDPath = link
					break
				}
			}
		}

		// Get by-path path
		if byPath, ok := props["ID_PATH"]; ok {
			disk.ByPathPath = "/dev/disk/by-path/" + byPath
		}

		// Device ID - prefer serial/WWN for true uniqueness
		// Serial is globally unique and stable across boots/controllers
		if disk.Serial != "" {
			disk.DeviceID = disk.Serial
			disk.DeviceIDSource = "serial"
		} else if disk.WWN != "" {
			disk.DeviceID = disk.WWN
			disk.DeviceIDSource = "wwn"
		} else if disk.ByIDPath != "" {
			disk.DeviceID = disk.ByIDPath
			disk.DeviceIDSource = "by-id"
		} else {
			disk.DeviceID = disk.DevicePath
			disk.DeviceIDSource = "path"
		}
	}
}

// enrichWithSMART enriches disk information with SMART data
func (d *Discoverer) enrichWithSMART(ctx context.Context, disks []*types.PhysicalDisk) {
	// 1. Detect virtualization environment first
	var envInfo *system.EnvironmentInfo
	var err error
	if d.envDetector != nil {
		envInfo, err = d.envDetector.DetectEnvironment(ctx)
		if err != nil {
			d.logger.Warn("failed to detect environment", "error", err)
		}
	}

	for _, disk := range disks {
		// 2. Skip SMART on known unsupported platforms (performance optimization)
		if envInfo != nil && d.shouldSkipSMART(envInfo, disk) {
			d.logger.Debug("skipping SMART for virtual/unsupported device",
				"device", disk.DevicePath,
				"hypervisor", envInfo.Hypervisor)
			disk.SMARTAvailable = false
			disk.SMARTEnabled = false
			disk.SMARTTestsSupported = false
			continue
		}

		// 3. Try to get SMART info
		output, err := d.smartctl.GetInfo(ctx, disk.DevicePath)
		if err != nil {
			d.logger.Debug("failed to get SMART info (may not support SMART)",
				"device", disk.DevicePath,
				"error", err)
			disk.SMARTAvailable = false
			disk.SMARTEnabled = false
			disk.SMARTTestsSupported = false
			continue
		}

		// Parse SMART info
		smartInfo, err := parsers.ParseSmartctlJSON(output, disk.DeviceID)
		if err != nil {
			d.logger.Warn("failed to parse SMART info",
				"device", disk.DevicePath,
				"error", err)
			disk.SMARTAvailable = false
			disk.SMARTEnabled = false
			disk.SMARTTestsSupported = false
			continue
		}

		disk.SMARTInfo = smartInfo
		disk.SMARTAvailable = smartInfo.Available
		disk.SMARTEnabled = smartInfo.Enabled

		// Update device type if detected from SMART
		if smartInfo.DeviceType != types.DeviceTypeUnknown {
			disk.Type = smartInfo.DeviceType
		}

		// 4. Check if self-tests are actually supported
		if disk.SMARTAvailable {
			canRunTests, err := d.smartctl.CanRunSelfTests(ctx, disk.DevicePath)
			if err != nil || !canRunTests {
				d.logger.Debug("SMART info available but self-tests not supported",
					"device", disk.DevicePath)
				disk.SMARTTestsSupported = false
			} else {
				disk.SMARTTestsSupported = true
			}
		} else {
			disk.SMARTTestsSupported = false
		}
	}
}

// enrichWithPoolMembership enriches disk information with ZFS pool membership
func (d *Discoverer) enrichWithPoolMembership(ctx context.Context, disks []*types.PhysicalDisk) {
	for _, disk := range disks {
		poolName, inPool, err := d.zfsPoolManager.GetPoolForDevice(ctx, disk.DevicePath)
		if err != nil {
			d.logger.Warn("failed to check pool membership",
				"device", disk.DevicePath,
				"error", err)
			continue
		}

		if inPool {
			disk.PoolName = poolName
			d.logger.Debug("device is in ZFS pool",
				"device", disk.DevicePath,
				"pool", poolName)
		}
	}
}

// shouldSkipSMART determines if SMART should be skipped entirely for a device
// based on the environment and device characteristics
func (d *Discoverer) shouldSkipSMART(envInfo *system.EnvironmentInfo, disk *types.PhysicalDisk) bool {
	if !envInfo.IsVirtualized {
		return false // Physical hardware - always check SMART
	}

	// Known platforms where SMART doesn't work or provides minimal value
	unsupportedHypervisors := map[string]bool{
		"amazon": true, // AWS EBS
		"google": true, // Google Cloud persistent disks
		"azure":  true, // Azure managed disks
	}

	if unsupportedHypervisors[envInfo.Hypervisor] {
		return true
	}

	// Check if it's a cloud block storage device based on model name
	model := strings.ToLower(disk.Model)
	if strings.Contains(model, "amazon elastic block store") ||
		strings.Contains(model, "google persistentdisk") ||
		strings.Contains(model, "virtual disk") {
		return true
	}

	return false
}

// parseUdevProperties parses udev property output into a map
func parseUdevProperties(output string) map[string]string {
	props := make(map[string]string)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Udev properties are in KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			props[parts[0]] = parts[1]
		}
	}

	return props
}

// GetCachedDevices returns cached devices from last scan
func (d *Discoverer) GetCachedDevices() map[string]*types.PhysicalDisk {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return a copy to prevent external modification
	cache := make(map[string]*types.PhysicalDisk, len(d.deviceCache))
	for k, v := range d.deviceCache {
		diskCopy := *v
		cache[k] = &diskCopy
	}

	return cache
}

// GetLastScanTime returns the timestamp of the last scan
func (d *Discoverer) GetLastScanTime() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.lastScan
}

// RefreshDevice refreshes information for a specific device
func (d *Discoverer) RefreshDevice(ctx context.Context, devicePath string) (*types.PhysicalDisk, error) {
	// Get device info from lsblk
	output, err := d.lsblk.GetDevice(ctx, devicePath)
	if err != nil {
		return nil, errors.Wrap(err, errors.DiskDiscoveryFailed).
			WithMetadata("device", devicePath).
			WithMetadata("tool", "lsblk")
	}

	// Parse output
	blockDevices, err := parsers.ParseLsblkJSON(output)
	if err != nil {
		return nil, err
	}

	if len(blockDevices) == 0 {
		return nil, errors.New(errors.DiskNotFound, "device not found").
			WithMetadata("device", devicePath)
	}

	disk := blockDevices[0].ToPhysicalDisk()

	// Enrich with udev
	if d.toolChecker.IsAvailable("udevadm") {
		d.enrichWithUdev(ctx, []*types.PhysicalDisk{disk})
	}

	// Enrich with SMART
	if d.toolChecker.IsAvailable("smartctl") {
		d.enrichWithSMART(ctx, []*types.PhysicalDisk{disk})
	}

	// Update cache
	d.mu.Lock()
	d.deviceCache[devicePath] = disk
	d.mu.Unlock()

	return disk, nil
}
