// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package types

import "time"

// DiskState represents the lifecycle state of a physical disk
type DiskState string

const (
	DiskStateUnknown      DiskState = "UNKNOWN"      // Initial state or error state
	DiskStateDiscovered   DiskState = "DISCOVERED"   // Newly discovered, not yet validated
	DiskStateValidating   DiskState = "VALIDATING"   // Running validation checks
	DiskStateAvailable    DiskState = "AVAILABLE"    // Ready for use in pools
	DiskStateSystem       DiskState = "SYSTEM"       // In use by system (mounted partitions, boot disk, etc.)
	DiskStateOnline       DiskState = "ONLINE"       // Currently part of a ZFS pool (ONLINE state)
	DiskStateFaulted      DiskState = "FAULTED"      // Hardware fault detected (ZFS FAULTED state)
	DiskStateDegraded     DiskState = "DEGRADED"     // Degraded performance or errors (ZFS DEGRADED state)
	DiskStateOffline      DiskState = "OFFLINE"      // Disk offline or removed (ZFS OFFLINE state)
	DiskStateUnavail      DiskState = "UNAVAIL"      // Device cannot be opened (ZFS UNAVAIL state)
	DiskStateRemoving     DiskState = "REMOVING"     // Being removed from pool
	DiskStateRetired      DiskState = "RETIRED"      // Decommissioned/marked for replacement
	DiskStateQuarantined  DiskState = "QUARANTINED"  // Isolated due to errors
	DiskStateUnauthorized DiskState = "UNAUTHORIZED" // Not authorized for use (e.g., unknown disk in secure mode)
)

// HealthStatus represents the overall health assessment of a disk
type HealthStatus string

const (
	HealthUnknown  HealthStatus = "UNKNOWN"  // Health status unknown
	HealthHealthy  HealthStatus = "HEALTHY"  // All checks pass
	HealthWarning  HealthStatus = "WARNING"  // Some thresholds exceeded
	HealthCritical HealthStatus = "CRITICAL" // Critical thresholds exceeded
	HealthFailed   HealthStatus = "FAILED"   // Disk has failed
)

// DeviceType represents the type of storage device
type DeviceType string

const (
	DeviceTypeUnknown DeviceType = "UNKNOWN"
	DeviceTypeHDD     DeviceType = "HDD"    // Spinning disk
	DeviceTypeSSD     DeviceType = "SSD"    // SATA SSD
	DeviceTypeNVMe    DeviceType = "NVME"   // NVMe SSD
	DeviceTypeOptane  DeviceType = "OPTANE" // Intel Optane
)

// InterfaceType represents the disk interface/protocol
type InterfaceType string

const (
	InterfaceUnknown InterfaceType = "UNKNOWN"
	InterfaceSATA    InterfaceType = "SATA"
	InterfaceSAS     InterfaceType = "SAS"
	InterfaceNVMe    InterfaceType = "NVME"
	InterfaceUSB     InterfaceType = "USB"
	InterfaceVirtIO  InterfaceType = "VIRTIO" // Virtual disk
)

// NamingStrategy represents the device naming approach
type NamingStrategy string

const (
	NamingByID   NamingStrategy = "by-id"   // /dev/disk/by-id (default <12 disks)
	NamingByPath NamingStrategy = "by-path" // /dev/disk/by-path (12-24 disks)
	NamingByVdev NamingStrategy = "by-vdev" // /dev/disk/by-vdev (25+ disks)
)

// ProbeType represents the type of SMART probe
type ProbeType string

const (
	ProbeTypeQuick     ProbeType = "quick"     // Quick/short SMART self-test
	ProbeTypeExtensive ProbeType = "extensive" // Extensive/long SMART self-test
)

// ProbeStatus represents the status of a SMART probe operation
type ProbeStatus string

const (
	ProbeStatusScheduled  ProbeStatus = "SCHEDULED"  // Scheduled but not yet started
	ProbeStatusRunning    ProbeStatus = "RUNNING"    // Currently executing
	ProbeStatusCompleted  ProbeStatus = "COMPLETED"  // Successfully completed
	ProbeStatusFailed     ProbeStatus = "FAILED"     // Failed to execute or parse
	ProbeStatusCancelled  ProbeStatus = "CANCELLED"  // Cancelled by user
	ProbeStatusConflicted ProbeStatus = "CONFLICTED" // Skipped due to conflict
	ProbeStatusTimeout    ProbeStatus = "TIMEOUT"    // Exceeded timeout
)

// ProbeResult represents the result of a SMART probe
type ProbeResult string

const (
	ProbeResultPass    ProbeResult = "PASS"    // Self-test passed
	ProbeResultFail    ProbeResult = "FAIL"    // Self-test failed
	ProbeResultAborted ProbeResult = "ABORTED" // Self-test aborted
	ProbeResultUnknown ProbeResult = "UNKNOWN" // Result not available
)

// OperationType represents the type of disk operation tracked in state
type OperationType string

const (
	OperationProbe     OperationType = "PROBE"     // SMART probe operation
	OperationDiscovery OperationType = "DISCOVERY" // Disk discovery operation
	OperationValidate  OperationType = "VALIDATE"  // Disk validation operation
)

// Default configuration values
const (
	// Disk count thresholds for naming strategy selection
	DiskCountThresholdByID   = 11 // Up to 11 disks: use by-id
	DiskCountThresholdByPath = 24 // 12-24 disks: use by-path
	// 25+ disks: use by-vdev

	// Discovery settings
	DefaultDiscoveryInterval  = 5 * time.Minute
	DefaultDiscoveryTimeout   = 30 * time.Second
	DefaultReconcileInterval  = 15 * time.Minute
	DefaultUdevMonitorEnabled = true

	// SMART probe settings
	DefaultQuickProbeInterval     = 24 * time.Hour     // Daily
	DefaultExtensiveProbeInterval = 7 * 24 * time.Hour // Weekly
	DefaultProbeTimeout           = 10 * time.Minute
	DefaultProbeRetention         = 90 * 24 * time.Hour // 90 days
	DefaultMaxConcurrentProbes    = 4                   // Max concurrent SMART probes

	// Health monitoring settings
	DefaultHealthCheckInterval = 5 * time.Minute
	DefaultMetricRetention     = 30 * 24 * time.Hour // 30 days

	// State persistence settings
	DefaultStateFile       = "disk-manager.state.json"
	DefaultStateSaveDelay  = 5 * time.Second // Debounce state saves
	DefaultStateBackupKeep = 3               // Keep 3 backup state files

	// Performance settings
	DefaultCacheSize   = 1000             // Disk inventory cache size
	DefaultCacheTTL    = 10 * time.Minute // Cache entry TTL
	DefaultWorkerCount = 4                // Worker goroutines for concurrent operations
)

// SMART threshold defaults
const (
	// Temperature thresholds (Celsius)
	DefaultTempWarning  = 50
	DefaultTempCritical = 60

	// Reallocated sectors
	DefaultReallocatedSectorsWarning  = 10
	DefaultReallocatedSectorsCritical = 50

	// Pending sectors
	DefaultPendingSectorsWarning  = 5
	DefaultPendingSectorsCritical = 20

	// Power-on hours (5 years = ~43800 hours)
	DefaultPowerOnHoursWarning  = 43800
	DefaultPowerOnHoursCritical = 52560 // 6 years

	// NVMe percentage used (for endurance)
	DefaultNVMePercentUsedWarning  = 80
	DefaultNVMePercentUsedCritical = 90

	// Media errors (NVMe)
	DefaultMediaErrorsWarning  = 10
	DefaultMediaErrorsCritical = 50
)

// Tool paths (may be overridden by configuration)
// Empty strings mean the tool will be found via exec.LookPath() in system PATH
const (
	DefaultSmartctlPath = "" // Will use exec.LookPath("smartctl")
	DefaultLsblkPath    = "" // Will use exec.LookPath("lsblk")
	DefaultLsscsiPath   = "" // Will use exec.LookPath("lsscsi")
	DefaultUdevadmPath  = "" // Will use exec.LookPath("udevadm")
	DefaultSgSesPath    = "" // Will use exec.LookPath("sg_ses")
	DefaultZpoolPath    = "" // Will use exec.LookPath("zpool")
)

// Validation constants
const (
	MaxDevicePathLength = 255
	MaxSerialLength     = 64
	MaxModelLength      = 128
	MaxVendorLength     = 64
	MaxFirmwareLength   = 32
	MaxWWNLength        = 32
)

// Conflict detection settings
const (
	DefaultConflictCheckInterval = 30 * time.Second
	DefaultConflictRetryDelay    = 5 * time.Minute
	DefaultMaxConflictRetries    = 3
)
