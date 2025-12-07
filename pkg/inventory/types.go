// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"time"

	diskTypes "github.com/stratastor/rodent/pkg/disk/types"
	netTypes "github.com/stratastor/rodent/pkg/netmage/types"
	"github.com/stratastor/rodent/pkg/shares"
	"github.com/stratastor/rodent/pkg/system"
	"github.com/stratastor/rodent/pkg/zfs/autosnapshots"
	"github.com/stratastor/rodent/pkg/zfs/autotransfers"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

// DetailLevel specifies the level of detail to include in the inventory
type DetailLevel string

const (
	DetailLevelSummary DetailLevel = "summary" // Just counts and key metrics
	DetailLevelBasic   DetailLevel = "basic"   // Include essential lists (default)
	DetailLevelFull    DetailLevel = "full"    // Include all details
)

// CollectOptions specifies what to include in the inventory collection
type CollectOptions struct {
	DetailLevel DetailLevel // Level of detail to include
	Include     []string    // Specific sections to include (empty = all)
	Exclude     []string    // Specific sections to exclude
}

// ShouldInclude returns true if the given section should be included
func (o *CollectOptions) ShouldInclude(section string) bool {
	// Check if explicitly excluded
	for _, excl := range o.Exclude {
		if excl == section {
			return false
		}
	}

	// If include list is empty, include all (except excluded)
	if len(o.Include) == 0 {
		return true
	}

	// Check if explicitly included
	for _, incl := range o.Include {
		if incl == section {
			return true
		}
	}

	return false
}

// RodentInventory represents the complete inventory of a Rodent instance
type RodentInventory struct {
	// Metadata
	Hostname   string    `json:"hostname"`    // System hostname
	Timestamp  time.Time `json:"timestamp"`   // Inventory collection timestamp
	Version    string    `json:"version"`     // Rodent version
	CommitSHA  string    `json:"commit_sha"`  // Git commit SHA
	BuildTime  string    `json:"build_time"`  // Build timestamp
	APIVersion string    `json:"api_version"` // API version

	// System summary
	System *SystemSummary `json:"system,omitempty"`

	// Disk summary
	Disks *DiskSummary `json:"disks,omitempty"`

	// ZFS summary
	ZFS *ZFSSummary `json:"zfs,omitempty"`

	// Network summary
	Network *NetworkSummary `json:"network,omitempty"`

	// Shares summary
	Shares *SharesSummary `json:"shares,omitempty"`

	// Snapshot policies summary
	SnapshotPolicies *SnapshotPoliciesSummary `json:"snapshot_policies,omitempty"`

	// Transfer policies summary
	TransferPolicies *TransferPoliciesSummary `json:"transfer_policies,omitempty"`

	// Services summary
	Services *ServicesSummary `json:"services,omitempty"`

	// Resource allocations and utilization
	Resources *ResourcesSummary `json:"resources,omitempty"`
}

// SystemSummary represents system-level summary information
type SystemSummary struct {
	OS          *system.OSInfo          `json:"os"`
	Hardware    *system.HardwareInfo    `json:"hardware"`
	Performance *system.PerformanceInfo `json:"performance"`
	Hostname    string                  `json:"hostname"`
	Health      string                  `json:"health"` // Overall health status
	Uptime      uint64                  `json:"uptime_seconds"`
}

// DiskSummary represents disk inventory summary
type DiskSummary struct {
	TotalCount     int                                `json:"total_count"`
	AvailableCount int                                `json:"available_count"`
	HealthyCount   int                                `json:"healthy_count"`
	WarningCount   int                                `json:"warning_count"`
	FailedCount    int                                `json:"failed_count"`
	TotalCapacity  uint64                             `json:"total_capacity_bytes"`
	UsedCapacity   uint64                             `json:"used_capacity_bytes"`
	Devices        map[string]*diskTypes.PhysicalDisk `json:"devices,omitempty"` // Included in basic/full detail level
	Statistics     *diskTypes.GlobalStatistics        `json:"statistics,omitempty"`
}

// ZFSSummary represents ZFS pools and datasets summary
type ZFSSummary struct {
	PoolCount       int                   `json:"pool_count"`
	FilesystemCount int                   `json:"filesystem_count"`
	VolumeCount     int                   `json:"volume_count"`
	SnapshotCount   int                   `json:"snapshot_count"`
	TotalCapacity   uint64                `json:"total_capacity_bytes"`
	UsedCapacity    uint64                `json:"used_capacity_bytes"`
	PoolHealth      map[string]string     `json:"pool_health,omitempty"` // pool_name -> health_status
	Pools           map[string]*pool.Pool `json:"pools,omitempty"`       // Included in basic/full detail level
	Filesystems     []string              `json:"filesystems,omitempty"` // Filesystem names, included in basic/full detail level
	Volumes         []string              `json:"volumes,omitempty"`     // Volume names, included in basic/full detail level
}

// NetworkInterfaceSummary represents a simplified network interface view
type NetworkInterfaceSummary struct {
	Name      string                  `json:"name"`
	State     netTypes.InterfaceState `json:"state"`
	Type      string                  `json:"type"`
	MAC       string                  `json:"mac_address"`
	IPv4Addrs []string                `json:"ipv4_addresses"`
	IPv6Addrs []string                `json:"ipv6_addresses"`
	Speed     uint64                  `json:"speed_mbps,omitempty"`
	TxBytes   uint64                  `json:"tx_bytes,omitempty"`
	RxBytes   uint64                  `json:"rx_bytes,omitempty"`
}

// NetworkSummary represents network configuration summary
type NetworkSummary struct {
	InterfaceCount int                        `json:"interface_count"`
	ActiveCount    int                        `json:"active_count"`
	Interfaces     []*NetworkInterfaceSummary `json:"interfaces,omitempty"` // Included in basic/full detail level
	GlobalDNS      *netTypes.NameserverConfig `json:"global_dns,omitempty"`
}

// ServicesSummary represents systemd services summary
type ServicesSummary struct {
	TotalCount   int             `json:"total_count"`
	RunningCount int             `json:"running_count"`
	FailedCount  int             `json:"failed_count"`
	Services     []ServiceStatus `json:"services,omitempty"` // Included in basic/full detail level
}

// ServiceStatus represents a service's runtime status
type ServiceStatus struct {
	Name        string `json:"name"`
	State       string `json:"state"`   // running, stopped, failed
	Enabled     bool   `json:"enabled"` // startup enabled/disabled
	Description string `json:"description,omitempty"`
}

// ResourcesSummary represents resource utilization across the system
type ResourcesSummary struct {
	CPU     CPUUtilization     `json:"cpu"`
	Memory  MemoryUtilization  `json:"memory"`
	Storage StorageUtilization `json:"storage"`
	Network NetworkUtilization `json:"network"`
}

// CPUUtilization represents CPU usage metrics
type CPUUtilization struct {
	TotalCores    int     `json:"total_cores"`
	UsagePercent  float64 `json:"usage_percent"`
	LoadAverage1  float64 `json:"load_average_1min"`
	LoadAverage5  float64 `json:"load_average_5min"`
	LoadAverage15 float64 `json:"load_average_15min"`
}

// MemoryUtilization represents memory usage metrics
type MemoryUtilization struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
	SwapTotalBytes uint64  `json:"swap_total_bytes"`
	SwapUsedBytes  uint64  `json:"swap_used_bytes"`
	SwapPercent    float64 `json:"swap_percent"`
}

// StorageUtilization represents storage usage metrics
type StorageUtilization struct {
	TotalCapacityBytes uint64  `json:"total_capacity_bytes"`
	UsedBytes          uint64  `json:"used_bytes"`
	AvailableBytes     uint64  `json:"available_bytes"`
	UsagePercent       float64 `json:"usage_percent"`
	PoolCount          int     `json:"pool_count"`
	DiskCount          int     `json:"disk_count"`

	// Disk breakdown by state
	OnlineDisks       int    `json:"online_disks"`             // Disks in zpools (ONLINE state)
	AvailableDisks    int    `json:"available_disks"`          // Available disks (not in pool)
	SystemDisks       int    `json:"system_disks"`             // System disks
	OnlineCapacity    uint64 `json:"online_capacity_bytes"`    // Total capacity of online disks
	AvailableCapacity uint64 `json:"available_capacity_bytes"` // Total capacity of available disks
	SystemCapacity    uint64 `json:"system_capacity_bytes"`    // Total capacity of system disks
}

// NetworkUtilization represents network usage metrics
type NetworkUtilization struct {
	InterfaceCount int    `json:"interface_count"`
	ActiveCount    int    `json:"active_count"`
	TotalTxBytes   uint64 `json:"total_tx_bytes"`
	TotalRxBytes   uint64 `json:"total_rx_bytes"`
	TotalTxPackets uint64 `json:"total_tx_packets"`
	TotalRxPackets uint64 `json:"total_rx_packets"`
}

// SharesSummary represents file shares summary (SMB/NFS/iSCSI)
type SharesSummary struct {
	TotalCount   int                  `json:"total_count"`
	EnabledCount int                  `json:"enabled_count"`
	SMBCount     int                  `json:"smb_count"`
	NFSCount     int                  `json:"nfs_count"`
	ISCSICount   int                  `json:"iscsi_count"`
	Shares       []shares.ShareConfig `json:"shares,omitempty"` // Included in basic/full detail level
}

// SnapshotPoliciesSummary represents snapshot automation policies summary
type SnapshotPoliciesSummary struct {
	TotalCount   int                            `json:"total_count"`
	EnabledCount int                            `json:"enabled_count"`
	ActiveCount  int                            `json:"active_count"`       // Policies with enabled schedules
	Policies     []autosnapshots.SnapshotPolicy `json:"policies,omitempty"` // Included in basic/full detail level
}

// TransferPoliciesSummary represents transfer automation policies summary
type TransferPoliciesSummary struct {
	TotalCount   int                            `json:"total_count"`
	EnabledCount int                            `json:"enabled_count"`
	ActiveCount  int                            `json:"active_count"`       // Policies with enabled schedules
	RunningCount int                            `json:"running_count"`      // Policies currently executing transfers
	Policies     []autotransfers.TransferPolicy `json:"policies,omitempty"` // Included in basic/full detail level
}
