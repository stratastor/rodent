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

// Discoverer handles disk discovery operations
type Discoverer struct {
	logger      logger.Logger
	lsblk       *tools.LsblkExecutor
	smartctl    *tools.SmartctlExecutor
	udevadm     *tools.UdevadmExecutor
	zpool       *tools.ZpoolExecutor
	toolChecker *tools.ToolChecker
	envDetector *system.EnvironmentDetector
	mu          sync.RWMutex
	lastScan    time.Time
	deviceCache map[string]*types.PhysicalDisk // Keyed by device path
}

// NewDiscoverer creates a new disk discoverer
func NewDiscoverer(
	l logger.Logger,
	lsblk *tools.LsblkExecutor,
	smartctl *tools.SmartctlExecutor,
	udevadm *tools.UdevadmExecutor,
	zpool *tools.ZpoolExecutor,
	toolChecker *tools.ToolChecker,
	envDetector *system.EnvironmentDetector,
) *Discoverer {
	return &Discoverer{
		logger:      l,
		lsblk:       lsblk,
		smartctl:    smartctl,
		udevadm:     udevadm,
		zpool:       zpool,
		toolChecker: toolChecker,
		envDetector: envDetector,
		deviceCache: make(map[string]*types.PhysicalDisk),
	}
}

// DiscoverAll discovers all physical disks on the system
func (d *Discoverer) DiscoverAll(ctx context.Context) ([]*types.PhysicalDisk, error) {
	d.logger.Info("starting disk discovery")
	startTime := time.Now()

	// Get all block devices using lsblk (with partition/mount info)
	devices, blockDeviceMap, err := d.discoverBlockDevices(ctx)
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

	// Enrich with system usage (check for mounted partitions)
	// This must run BEFORE pool enrichment to set SYSTEM state for boot/system disks
	d.enrichWithSystemUsage(devices, blockDeviceMap)

	// Enrich with SMART information (if available)
	if d.toolChecker.IsAvailable("smartctl") {
		d.enrichWithSMART(ctx, devices)
	} else {
		d.logger.Warn("smartctl not available, skipping SMART enrichment")
	}

	// Enrich with ZFS pool membership (if zpool tool available)
	// This runs AFTER system usage check - pool state overrides AVAILABLE but not SYSTEM
	if d.zpool != nil && d.toolChecker.IsAvailable("zpool") {
		d.enrichWithPoolMembership(ctx, devices)
	} else {
		d.logger.Debug("zpool tool not available, skipping pool membership check")
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
func (d *Discoverer) discoverBlockDevices(ctx context.Context) ([]*types.PhysicalDisk, map[string]*parsers.BlockDevice, error) {
	// Execute lsblk WITH children to get partition/mount info
	output, err := d.lsblk.ListDisksWithChildren(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, errors.DiskDiscoveryFailed).
			WithMetadata("tool", "lsblk")
	}

	// Parse lsblk output
	blockDevices, err := parsers.ParseLsblkJSON(output)
	if err != nil {
		return nil, nil, err
	}

	// Filter only physical disks
	physicalDevices := parsers.FilterPhysicalDisks(blockDevices)

	// Build mapping of device path -> BlockDevice (for checking children/mounts later)
	blockDeviceMap := make(map[string]*parsers.BlockDevice)
	for _, bd := range physicalDevices {
		blockDeviceMap[bd.Path] = bd
	}

	// Convert to PhysicalDisk types
	var disks []*types.PhysicalDisk
	for _, bd := range physicalDevices {
		disk := bd.ToPhysicalDisk()
		// Initial state - will be updated by enrichWithSystemUsage and enrichWithPoolMembership
		disk.State = types.DiskStateAvailable // Default: available (not in pool/system yet)
		disk.Health = types.HealthUnknown
		disks = append(disks, disk)
	}

	return disks, blockDeviceMap, nil
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

		// Get all device links (for pool membership matching)
		if devlinks, ok := props["DEVLINKS"]; ok {
			disk.DevLinks = strings.Fields(devlinks)

			// Also populate ByIDPath for backward compatibility (first by-id link found)
			for _, link := range disk.DevLinks {
				if strings.Contains(link, "/dev/disk/by-id/") && disk.ByIDPath == "" {
					disk.ByIDPath = link
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

// enrichWithSystemUsage checks if disks have mounted partitions (system/boot disks)
// Disks with mounted partitions are marked as SYSTEM state
func (d *Discoverer) enrichWithSystemUsage(disks []*types.PhysicalDisk, blockDeviceMap map[string]*parsers.BlockDevice) {
	for _, disk := range disks {
		// Skip if disk is already in a known state (e.g., will be set by pool membership later)
		// We only check AVAILABLE disks here
		if disk.State != types.DiskStateAvailable {
			continue
		}

		// Get the BlockDevice for this disk to check its children (partitions)
		blockDevice, ok := blockDeviceMap[disk.DevicePath]
		if !ok {
			continue
		}

		// Check if any child partition is mounted
		hasMountedPartition := false
		for _, child := range blockDevice.Children {
			if child.Mountpoint != nil && *child.Mountpoint != "" {
				hasMountedPartition = true
				d.logger.Debug("disk has mounted partition",
					"device", disk.DevicePath,
					"partition", child.Path,
					"mountpoint", *child.Mountpoint)
				break
			}
		}

		// If disk has mounted partitions, mark as SYSTEM (in use by OS)
		if hasMountedPartition {
			disk.State = types.DiskStateSystem
			d.logger.Debug("disk marked as SYSTEM (has mounted partitions)",
				"device", disk.DevicePath,
				"device_id", disk.DeviceID)
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

// VDevInfo holds vdev path and state information
type VDevInfo struct {
	Path  string
	State string
}

// enrichWithPoolMembership enriches disk information with ZFS pool membership and state
// Uses DEVLINKS-based matching for robust device path resolution
func (d *Discoverer) enrichWithPoolMembership(ctx context.Context, disks []*types.PhysicalDisk) {
	// Get all pool status
	poolStatus, err := d.zpool.GetPoolStatus(ctx)
	if err != nil {
		d.logger.Warn("failed to get zpool status", "error", err)
		return
	}

	// For each pool, extract all vdev paths with states
	poolVdevInfo := make(map[string][]VDevInfo) // poolName -> []VDevInfo
	for poolName, pool := range poolStatus.Pools {
		vdevs := d.extractVdevInfo(pool.VDevs)
		poolVdevInfo[poolName] = vdevs
		d.logger.Debug("extracted vdev info for pool",
			"pool", poolName,
			"vdev_count", len(vdevs))
	}

	// Match disks against pool vdev paths using DEVLINKS and extract state
	for _, disk := range disks {
		poolName, vdevState := d.findPoolAndStateForDisk(disk, poolVdevInfo)
		if poolName != "" {
			disk.PoolName = poolName
			// Map ZFS vdev state to DiskState
			disk.State = d.mapZFSStateToDiskState(vdevState)
			d.logger.Debug("device is in ZFS pool",
				"device", disk.DevicePath,
				"device_id", disk.DeviceID,
				"pool", poolName,
				"vdev_state", vdevState,
				"disk_state", disk.State)
		}
	}
}

// extractVdevInfo recursively extracts vdev paths and states from a vdev tree
func (d *Discoverer) extractVdevInfo(vdevs map[string]*tools.VDev) []VDevInfo {
	var vdevInfo []VDevInfo
	for _, vdev := range vdevs {
		if vdev.Path != "" {
			vdevInfo = append(vdevInfo, VDevInfo{
				Path:  vdev.Path,
				State: vdev.State,
			})
		}
		// Recursively extract from nested vdevs
		if vdev.VDevs != nil {
			vdevInfo = append(vdevInfo, d.extractVdevInfo(vdev.VDevs)...)
		}
	}
	return vdevInfo
}

// findPoolAndStateForDisk finds which pool (if any) a disk belongs to and its vdev state
// Uses DEVLINKS for robust matching across different device path formats
// Returns (poolName, vdevState) or ("", "") if not in any pool
func (d *Discoverer) findPoolAndStateForDisk(disk *types.PhysicalDisk, poolVdevInfo map[string][]VDevInfo) (string, string) {
	// Build a set of all device links for quick lookup
	deviceLinks := make(map[string]bool)
	for _, link := range disk.DevLinks {
		deviceLinks[link] = true
	}
	// Also add the main device path
	deviceLinks[disk.DevicePath] = true

	// Check each pool's vdev info
	for poolName, vdevInfoList := range poolVdevInfo {
		for _, vdevInfo := range vdevInfoList {
			// Check if this vdev path matches any of our device links
			if deviceLinks[vdevInfo.Path] {
				return poolName, vdevInfo.State
			}

			// Also check if vdev path is a partition of our device
			// ZFS often uses partitions (e.g., /dev/disk/by-id/xxx-part1)
			// We need to check if the base device matches
			if d.isPartitionOfDevice(vdevInfo.Path, deviceLinks) {
				return poolName, vdevInfo.State
			}
		}
	}

	return "", ""
}

// mapZFSStateToDiskState maps ZFS vdev state to DiskState constant
func (d *Discoverer) mapZFSStateToDiskState(zfsState string) types.DiskState {
	switch strings.ToUpper(zfsState) {
	case "ONLINE":
		return types.DiskStateOnline
	case "DEGRADED":
		return types.DiskStateDegraded
	case "FAULTED":
		return types.DiskStateFaulted
	case "UNAVAIL":
		return types.DiskStateUnavail
	case "OFFLINE":
		return types.DiskStateOffline
	default:
		d.logger.Warn("unknown ZFS vdev state", "state", zfsState)
		return types.DiskStateOnline // Default to online if unknown
	}
}

// isPartitionOfDevice checks if vdevPath is a partition of any device link
// E.g., /dev/disk/by-id/nvme-xxx-part1 is a partition of /dev/disk/by-id/nvme-xxx
func (d *Discoverer) isPartitionOfDevice(vdevPath string, deviceLinks map[string]bool) bool {
	// Check if vdevPath ends with a partition suffix
	// Common patterns: -part1, -part2, p1, p2, 1, 2
	for deviceLink := range deviceLinks {
		// Simple check: if vdevPath starts with deviceLink and has partition suffix
		if strings.HasPrefix(vdevPath, deviceLink) {
			suffix := strings.TrimPrefix(vdevPath, deviceLink)
			// Check if suffix looks like a partition (starts with -, p, or digit)
			if len(suffix) > 0 && (suffix[0] == '-' || suffix[0] == 'p' || (suffix[0] >= '0' && suffix[0] <= '9')) {
				return true
			}
		}
	}
	return false
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
