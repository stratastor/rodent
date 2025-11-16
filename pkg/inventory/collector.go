// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/constants"
	"github.com/stratastor/rodent/pkg/disk"
	diskTypes "github.com/stratastor/rodent/pkg/disk/types"
	netTypes "github.com/stratastor/rodent/pkg/netmage/types"
	"github.com/stratastor/rodent/pkg/shares"
	"github.com/stratastor/rodent/pkg/system"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
	"github.com/stratastor/rodent/pkg/zfs/snapshot"
)

// Collector aggregates inventory data from all Rodent subsystems
type Collector struct {
	// Managers (some may be nil if not initialized)
	diskManager     *disk.Manager
	poolManager     *pool.Manager
	datasetManager  *dataset.Manager
	networkManager  netTypes.Manager
	systemManager   *system.Manager
	sharesManager   shares.SharesManager
	snapshotManager snapshot.SchedulerInterface

	logger logger.Logger
	mu     sync.RWMutex
}

// NewCollector creates a new inventory collector
func NewCollector(
	diskMgr *disk.Manager,
	poolMgr *pool.Manager,
	datasetMgr *dataset.Manager,
	networkMgr netTypes.Manager,
	systemMgr *system.Manager,
	sharesMgr shares.SharesManager,
	snapshotMgr snapshot.SchedulerInterface,
	log logger.Logger,
) *Collector {
	return &Collector{
		diskManager:     diskMgr,
		poolManager:     poolMgr,
		datasetManager:  datasetMgr,
		networkManager:  networkMgr,
		systemManager:   systemMgr,
		sharesManager:   sharesMgr,
		snapshotManager: snapshotMgr,
		logger:          log,
	}
}

// CollectInventory aggregates inventory data from all subsystems
func (c *Collector) CollectInventory(
	ctx context.Context,
	opts CollectOptions,
) (*RodentInventory, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	inventory := &RodentInventory{
		Timestamp:  time.Now(),
		Version:    constants.Version,
		CommitSHA:  constants.CommitSHA,
		BuildTime:  constants.BuildTime,
		APIVersion: constants.APIVersion,
	}

	// Collect data in parallel using goroutines and wait group
	var wg sync.WaitGroup
	errChan := make(chan error, 6) // Buffer for potential errors from each subsystem

	// System info (always collect hostname)
	if opts.ShouldInclude("system") && c.systemManager != nil {
		wg.Go(func() {
			if summary, err := c.collectSystemSummary(ctx, opts.DetailLevel); err != nil {
				c.logger.Warn("Failed to collect system summary", "error", err)
				errChan <- fmt.Errorf("system: %w", err)
			} else {
				inventory.System = summary
				// Set hostname from system info
				if summary != nil {
					inventory.Hostname = summary.Hostname
				}
			}
		})
	}

	// Disks
	if opts.ShouldInclude("disks") && c.diskManager != nil {
		wg.Go(func() {
			if summary, err := c.collectDiskSummary(ctx, opts.DetailLevel); err != nil {
				c.logger.Warn("Failed to collect disk summary", "error", err)
				errChan <- fmt.Errorf("disks: %w", err)
			} else {
				inventory.Disks = summary
			}
		})
	}

	// ZFS
	if opts.ShouldInclude("zfs") && c.poolManager != nil {
		wg.Go(func() {
			if summary, err := c.collectZFSSummary(ctx, opts.DetailLevel); err != nil {
				c.logger.Warn("Failed to collect ZFS summary", "error", err)
				errChan <- fmt.Errorf("zfs: %w", err)
			} else {
				inventory.ZFS = summary
			}
		})
	}

	// Network
	if opts.ShouldInclude("network") && c.networkManager != nil {
		wg.Go(func() {
			if summary, err := c.collectNetworkSummary(ctx, opts.DetailLevel); err != nil {
				c.logger.Warn("Failed to collect network summary", "error", err)
				errChan <- fmt.Errorf("network: %w", err)
			} else {
				inventory.Network = summary
			}
		})
	}

	// Shares
	if opts.ShouldInclude("shares") && c.sharesManager != nil {
		wg.Go(func() {
			if summary, err := c.collectSharesSummary(ctx, opts.DetailLevel); err != nil {
				c.logger.Warn("Failed to collect shares summary", "error", err)
				errChan <- fmt.Errorf("shares: %w", err)
			} else {
				inventory.Shares = summary
			}
		})
	}

	// Snapshot Policies
	if opts.ShouldInclude("snapshot_policies") && c.snapshotManager != nil {
		wg.Go(func() {
			if summary, err := c.collectSnapshotPoliciesSummary(ctx, opts.DetailLevel); err != nil {
				c.logger.Warn("Failed to collect snapshot policies summary", "error", err)
				errChan <- fmt.Errorf("snapshot_policies: %w", err)
			} else {
				inventory.SnapshotPolicies = summary
			}
		})
	}

	// Resources (requires system and other managers)
	if opts.ShouldInclude("resources") {
		wg.Go(func() {
			if summary, err := c.collectResourcesSummary(ctx); err != nil {
				c.logger.Warn("Failed to collect resources summary", "error", err)
				errChan <- fmt.Errorf("resources: %w", err)
			} else {
				inventory.Resources = summary
			}
		})
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check if we got any critical errors (currently we log and continue)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	// If we have errors, log them but don't fail the entire request
	if len(errs) > 0 {
		c.logger.Warn("Some inventory subsystems failed to collect", "error_count", len(errs))
	}

	return inventory, nil
}

// collectSystemSummary collects system information
func (c *Collector) collectSystemSummary(
	ctx context.Context,
	_ DetailLevel, // Reserved for future use
) (*SystemSummary, error) {
	if c.systemManager == nil {
		return nil, fmt.Errorf("system manager not available")
	}

	summary := &SystemSummary{}

	// Get OS info
	osInfo, err := c.systemManager.GetOSInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get os info: %w", err)
	}
	summary.OS = osInfo

	// Get hardware info
	hwInfo, err := c.systemManager.GetHardwareInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get hardware info: %w", err)
	}
	summary.Hardware = hwInfo

	// Get performance info
	perfInfo, err := c.systemManager.GetPerformanceInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get performance info: %w", err)
	}
	summary.Performance = perfInfo
	summary.Uptime = perfInfo.UptimeSeconds

	// Get system health
	health, err := c.systemManager.GetSystemHealth(ctx)
	if err != nil {
		c.logger.Warn("Failed to get system health", "error", err)
		summary.Health = "unknown"
	} else {
		// Extract status from health map
		if status, ok := health["status"].(string); ok {
			summary.Health = status
		} else {
			summary.Health = "unknown"
		}
	}

	hostname, err := c.systemManager.GetHostname(ctx)
	if err != nil {
		c.logger.Warn("Failed to get system hostname", "error", err)
	} else {
		summary.Hostname = hostname
	}

	return summary, nil
}

// collectDiskSummary collects disk inventory summary
func (c *Collector) collectDiskSummary(
	_ context.Context, // Reserved for future use
	level DetailLevel,
) (*DiskSummary, error) {
	if c.diskManager == nil {
		return nil, fmt.Errorf("disk manager not available")
	}

	summary := &DiskSummary{}

	// Get disk inventory
	disks := c.diskManager.GetInventory(nil) // nil filter = all disks
	if disks == nil {
		return summary, nil
	}

	summary.TotalCount = len(disks)

	var totalCapacity uint64
	var usedCapacity uint64

	// Calculate summaries
	for _, disk := range disks {
		totalCapacity += disk.SizeBytes

		// Count by health
		switch disk.Health {
		case diskTypes.HealthHealthy:
			summary.HealthyCount++
		case diskTypes.HealthWarning:
			summary.WarningCount++
		case diskTypes.HealthFailed, diskTypes.HealthCritical:
			summary.FailedCount++
		}

		// Count available disks
		if disk.IsAvailable() {
			summary.AvailableCount++
		}

		// Estimate used capacity (disks in pools)
		if disk.PoolName != "" {
			usedCapacity += disk.SizeBytes
		}
	}

	summary.TotalCapacity = totalCapacity
	summary.UsedCapacity = usedCapacity

	// Include devices if detail level is basic or full
	if level == DetailLevelBasic || level == DetailLevelFull {
		// Convert slice to map keyed by device ID
		summary.Devices = make(map[string]*diskTypes.PhysicalDisk)
		for _, disk := range disks {
			summary.Devices[disk.DeviceID] = disk
		}
	}

	// Get statistics
	stats := c.diskManager.GetStatistics()
	if stats != nil {
		summary.Statistics = stats
	}

	return summary, nil
}

// collectZFSSummary collects ZFS pools and datasets summary
func (c *Collector) collectZFSSummary(ctx context.Context, level DetailLevel) (*ZFSSummary, error) {
	if c.poolManager == nil {
		return nil, fmt.Errorf("pool manager not available")
	}

	summary := &ZFSSummary{
		PoolHealth: make(map[string]string),
	}

	// List all pools
	poolList, err := c.poolManager.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pools: %w", err)
	}

	summary.PoolCount = len(poolList.Pools)

	var totalCapacity uint64
	var usedCapacity uint64

	// Collect pool details
	for poolName, poolInfo := range poolList.Pools {
		summary.PoolHealth[poolName] = poolInfo.State

		// Extract capacity from properties if available
		if poolInfo.Properties != nil {
			if sizeProp, ok := poolInfo.Properties["size"]; ok {
				// Try string first (ZFS properties come as strings)
				if sizeStr, ok := sizeProp.Value.(string); ok {
					if sizeVal, err := strconv.ParseUint(sizeStr, 10, 64); err == nil {
						totalCapacity += sizeVal
					}
				} else if sizeVal, ok := sizeProp.Value.(uint64); ok {
					totalCapacity += sizeVal
				}
			}
			if allocProp, ok := poolInfo.Properties["allocated"]; ok {
				// Try string first (ZFS properties come as strings)
				if allocStr, ok := allocProp.Value.(string); ok {
					if allocVal, err := strconv.ParseUint(allocStr, 10, 64); err == nil {
						usedCapacity += allocVal
					}
				} else if allocVal, ok := allocProp.Value.(uint64); ok {
					usedCapacity += allocVal
				}
			}
		}
	}

	summary.TotalCapacity = totalCapacity
	summary.UsedCapacity = usedCapacity

	// Include pools if detail level is basic or full
	if level == DetailLevelBasic || level == DetailLevelFull {
		// Convert map[string]pool.Pool to map[string]*pool.Pool
		summary.Pools = make(map[string]*pool.Pool)
		for name, p := range poolList.Pools {
			poolCopy := p
			summary.Pools[name] = &poolCopy
		}
	}

	// Count datasets and snapshots if dataset manager is available
	if c.datasetManager != nil {
		// List filesystems and volumes (exclude snapshots)
		datasetList, err := c.datasetManager.List(ctx, dataset.ListConfig{
			Recursive: true,
			Type:      "filesystem,volume", // Exclude snapshots
		})
		if err == nil {
			// Initialize slices if detail level requires them
			if level == DetailLevelBasic || level == DetailLevelFull {
				summary.Filesystems = make([]string, 0)
				summary.Volumes = make([]string, 0)
			}

			for _, ds := range datasetList.Datasets {
				// Case-insensitive type checking
				dsType := strings.ToLower(ds.Type)
				switch dsType {
				case "filesystem", "fs":
					summary.FilesystemCount++
					// Include filesystem names in basic/full detail level
					if level == DetailLevelBasic || level == DetailLevelFull {
						summary.Filesystems = append(summary.Filesystems, ds.Name)
					}
				case "volume", "vol":
					summary.VolumeCount++
					// Include volume names in basic/full detail level
					if level == DetailLevelBasic || level == DetailLevelFull {
						summary.Volumes = append(summary.Volumes, ds.Name)
					}
				}
			}
		}

		// Count snapshots separately (don't include in list due to potentially high volume)
		snapshotList, err := c.datasetManager.List(ctx, dataset.ListConfig{
			Recursive: true,
			Type:      "snapshot",
		})
		if err == nil {
			summary.SnapshotCount = len(snapshotList.Datasets)
		}
	}

	return summary, nil
}

// collectNetworkSummary collects network configuration summary
func (c *Collector) collectNetworkSummary(
	ctx context.Context,
	level DetailLevel,
) (*NetworkSummary, error) {
	if c.networkManager == nil {
		return nil, fmt.Errorf("network manager not available")
	}

	summary := &NetworkSummary{}

	// List all interfaces
	interfaces, err := c.networkManager.ListInterfaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	summary.InterfaceCount = len(interfaces)

	// Build interface summaries if detail level is basic or full
	if level == DetailLevelBasic || level == DetailLevelFull {
		summary.Interfaces = make([]*NetworkInterfaceSummary, 0, len(interfaces))

		for _, iface := range interfaces {
			ifaceSummary := &NetworkInterfaceSummary{
				Name:  iface.Name,
				State: iface.OperState, // Use operational state
				Type:  string(iface.Type),
				MAC:   iface.MACAddress,
			}

			// Extract IP addresses
			for _, addr := range iface.IPAddresses {
				switch addr.Family {
				case netTypes.FamilyIPv4:
					ifaceSummary.IPv4Addrs = append(ifaceSummary.IPv4Addrs, addr.Address)
				case netTypes.FamilyIPv6:
					ifaceSummary.IPv6Addrs = append(ifaceSummary.IPv6Addrs, addr.Address)
				}
			}

			// Use statistics from interface if available
			if iface.Statistics != nil {
				ifaceSummary.TxBytes = iface.Statistics.TXBytes
				ifaceSummary.RxBytes = iface.Statistics.RXBytes
			}

			// Count active interfaces
			if iface.OperState == netTypes.InterfaceStateUp {
				summary.ActiveCount++
			}

			summary.Interfaces = append(summary.Interfaces, ifaceSummary)
		}
	} else {
		// Just count active interfaces for summary level
		for _, iface := range interfaces {
			if iface.OperState == netTypes.InterfaceStateUp {
				summary.ActiveCount++
			}
		}
	}

	// Get global DNS
	dns, err := c.networkManager.GetGlobalDNS(ctx)
	if err == nil {
		summary.GlobalDNS = dns
	}

	return summary, nil
}

// collectResourcesSummary collects resource utilization summary
func (c *Collector) collectResourcesSummary(ctx context.Context) (*ResourcesSummary, error) {
	summary := &ResourcesSummary{}

	// CPU utilization (from system manager)
	if c.systemManager != nil {
		perfInfo, err := c.systemManager.GetPerformanceInfo(ctx)
		if err == nil && perfInfo != nil {
			summary.CPU = CPUUtilization{
				UsagePercent:  perfInfo.CPUUsage.Total,
				LoadAverage1:  perfInfo.LoadAverage.Load1,
				LoadAverage5:  perfInfo.LoadAverage.Load5,
				LoadAverage15: perfInfo.LoadAverage.Load15,
			}

			// Get CPU core count from hardware info
			hwInfo, err := c.systemManager.GetHardwareInfo(ctx)
			if err == nil && hwInfo != nil {
				summary.CPU.TotalCores = hwInfo.CPU.ProcessorCount
			}
		}

		// Memory utilization
		if err == nil && perfInfo != nil {
			hwInfo, err := c.systemManager.GetHardwareInfo(ctx)
			if err == nil && hwInfo != nil {
				summary.Memory = MemoryUtilization{
					TotalBytes:     hwInfo.Memory.Total,
					UsedBytes:      hwInfo.Memory.Used,
					AvailableBytes: hwInfo.Memory.Available,
					UsagePercent:   hwInfo.Memory.MemoryPercent,
					SwapTotalBytes: hwInfo.Memory.SwapTotal,
					SwapUsedBytes:  hwInfo.Memory.SwapUsed,
					SwapPercent:    hwInfo.Memory.SwapPercent,
				}
			}
		}
	}

	// Storage utilization (from disk and ZFS managers)
	if c.diskManager != nil {
		disks := c.diskManager.GetInventory(nil)
		if disks != nil {
			summary.Storage.DiskCount = len(disks)

			// Categorize disks by state and calculate capacity for each category
			for _, disk := range disks {
				summary.Storage.TotalCapacityBytes += disk.SizeBytes

				switch disk.State {
				case diskTypes.DiskStateOnline:
					summary.Storage.OnlineDisks++
					summary.Storage.OnlineCapacity += disk.SizeBytes
				case diskTypes.DiskStateAvailable:
					summary.Storage.AvailableDisks++
					summary.Storage.AvailableCapacity += disk.SizeBytes
				case diskTypes.DiskStateSystem:
					summary.Storage.SystemDisks++
					summary.Storage.SystemCapacity += disk.SizeBytes
				}
			}
		}
	}

	if c.poolManager != nil {
		poolList, err := c.poolManager.List(ctx)
		if err == nil {
			summary.Storage.PoolCount = len(poolList.Pools)
			for _, pool := range poolList.Pools {
				if pool.Properties != nil {
					if sizeProp, ok := pool.Properties["size"]; ok {
						// Try string first (ZFS properties come as strings)
						if sizeStr, ok := sizeProp.Value.(string); ok {
							if sizeVal, err := strconv.ParseUint(sizeStr, 10, 64); err == nil {
								summary.Storage.UsedBytes += sizeVal
							}
						} else if sizeVal, ok := sizeProp.Value.(uint64); ok {
							summary.Storage.UsedBytes += sizeVal
						}
					}
					if allocProp, ok := pool.Properties["allocated"]; ok {
						// Try string first (ZFS properties come as strings)
						if allocStr, ok := allocProp.Value.(string); ok {
							if allocVal, err := strconv.ParseUint(allocStr, 10, 64); err == nil {
								summary.Storage.UsedBytes += allocVal
							}
						} else if allocVal, ok := allocProp.Value.(uint64); ok {
							summary.Storage.UsedBytes += allocVal
						}
					}
				}
			}
			if summary.Storage.TotalCapacityBytes > 0 {
				summary.Storage.AvailableBytes = summary.Storage.TotalCapacityBytes - summary.Storage.UsedBytes
				summary.Storage.UsagePercent = float64(
					summary.Storage.UsedBytes,
				) / float64(
					summary.Storage.TotalCapacityBytes,
				) * 100
			}
		}
	}

	// Network utilization
	if c.networkManager != nil {
		interfaces, err := c.networkManager.ListInterfaces(ctx)
		if err == nil {
			summary.Network.InterfaceCount = len(interfaces)
			for _, iface := range interfaces {
				if iface.OperState == netTypes.InterfaceStateUp {
					summary.Network.ActiveCount++
				}

				// Use statistics from interface if available
				if iface.Statistics != nil {
					summary.Network.TotalTxBytes += iface.Statistics.TXBytes
					summary.Network.TotalRxBytes += iface.Statistics.RXBytes
					summary.Network.TotalTxPackets += iface.Statistics.TXPackets
					summary.Network.TotalRxPackets += iface.Statistics.RXPackets
				}
			}
		}
	}

	return summary, nil
}

// collectSharesSummary collects file shares summary (SMB/NFS/iSCSI)
func (c *Collector) collectSharesSummary(
	ctx context.Context,
	level DetailLevel,
) (*SharesSummary, error) {
	if c.sharesManager == nil {
		return nil, fmt.Errorf("shares manager not available")
	}

	summary := &SharesSummary{}

	// Get all shares
	allShares, err := c.sharesManager.ListShares(ctx)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}

	summary.TotalCount = len(allShares)

	// Count by type and enabled status
	for _, share := range allShares {
		if share.Enabled {
			summary.EnabledCount++
		}

		switch share.Type {
		case shares.ShareTypeSMB:
			summary.SMBCount++
		case shares.ShareTypeNFS:
			summary.NFSCount++
		case shares.ShareTypeISCSI:
			summary.ISCSICount++
		}
	}

	// Include full share list in basic/full detail levels
	if level == DetailLevelBasic || level == DetailLevelFull {
		summary.Shares = allShares
	}

	return summary, nil
}

// collectSnapshotPoliciesSummary collects snapshot automation policies summary
func (c *Collector) collectSnapshotPoliciesSummary(
	_ context.Context,
	level DetailLevel,
) (*SnapshotPoliciesSummary, error) {
	if c.snapshotManager == nil {
		return nil, fmt.Errorf("snapshot manager not available")
	}

	summary := &SnapshotPoliciesSummary{}

	// Get all policies
	policies, err := c.snapshotManager.ListPolicies()
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}

	summary.TotalCount = len(policies)

	// Count enabled and active policies
	for _, policy := range policies {
		if policy.Enabled {
			summary.EnabledCount++

			// A policy is considered "active" if it's enabled and has at least one enabled schedule
			hasActiveSchedule := false
			for _, schedule := range policy.Schedules {
				if schedule.Enabled {
					hasActiveSchedule = true
					break
				}
			}
			if hasActiveSchedule {
				summary.ActiveCount++
			}
		}
	}

	// Include full policy list in basic/full detail levels
	if level == DetailLevelBasic || level == DetailLevelFull {
		summary.Policies = policies
	}

	return summary, nil
}
