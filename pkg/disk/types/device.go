// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package types

import "time"

// PhysicalDisk represents a physical storage device
type PhysicalDisk struct {
	// Device identification
	DeviceID   string `json:"device_id"`   // Unique device ID (e.g., from by-id)
	DevicePath string `json:"device_path"` // Primary device path (e.g., /dev/sda)
	WWN        string `json:"wwn"`         // World Wide Name (if available)
	Serial     string `json:"serial"`      // Device serial number
	Model      string `json:"model"`       // Device model
	Vendor     string `json:"vendor"`      // Device vendor
	Firmware   string `json:"firmware"`    // Firmware version

	// Device properties
	Type      DeviceType    `json:"type"`       // HDD, SSD, NVMe, etc.
	Interface InterfaceType `json:"interface"`  // SATA, SAS, NVMe, etc.
	SizeBytes uint64        `json:"size_bytes"` // Total device size in bytes

	// Alternative device paths
	ByIDPath   string `json:"by_id_path"`   // /dev/disk/by-id path
	ByPathPath string `json:"by_path_path"` // /dev/disk/by-path path
	ByVdevPath string `json:"by_vdev_path"` // /dev/disk/by-vdev path (if configured)

	// Physical topology (optional, filled by topology discovery)
	Topology *DiskTopology `json:"topology,omitempty"`

	// SMART capability
	SMARTAvailable bool        `json:"smart_available"` // Whether SMART is supported
	SMARTEnabled   bool        `json:"smart_enabled"`   // Whether SMART is enabled
	SMARTInfo      *SMARTInfo  `json:"smart_info,omitempty"` // Latest SMART data

	// Health and state
	State        DiskState    `json:"state"`         // Current lifecycle state
	Health       HealthStatus `json:"health"`        // Overall health status
	HealthReason string       `json:"health_reason"` // Explanation for health status

	// ZFS integration
	PoolName string `json:"pool_name,omitempty"` // Pool name if disk is in use
	VdevGUID string `json:"vdev_guid,omitempty"` // ZFS vdev GUID if in use

	// Timestamps
	DiscoveredAt time.Time  `json:"discovered_at"` // When first discovered
	LastSeenAt   time.Time  `json:"last_seen_at"`  // Last seen during discovery
	UpdatedAt    time.Time  `json:"updated_at"`    // Last updated timestamp
	RemovedAt    *time.Time `json:"removed_at,omitempty"` // When removed/offline

	// Metadata
	Tags     map[string]string `json:"tags,omitempty"`     // User-defined tags
	Notes    string            `json:"notes,omitempty"`    // User notes
	Metadata map[string]string `json:"metadata,omitempty"` // Additional metadata
}

// DiskTopology represents the physical topology location of a disk
type DiskTopology struct {
	// Controller information
	ControllerID   string `json:"controller_id"`   // Controller identifier
	ControllerType string `json:"controller_type"` // HBA, RAID (should be IT mode), etc.
	ControllerPCI  string `json:"controller_pci"`  // PCI address (e.g., 0000:05:00.0)

	// Enclosure information (for SAS/SCSI)
	EnclosureID     string `json:"enclosure_id,omitempty"`      // Enclosure identifier
	EnclosureVendor string `json:"enclosure_vendor,omitempty"`  // Enclosure vendor
	EnclosureModel  string `json:"enclosure_model,omitempty"`   // Enclosure model
	EnclosureSerial string `json:"enclosure_serial,omitempty"`  // Enclosure serial

	// Slot information
	SlotNumber int    `json:"slot_number,omitempty"` // Physical slot number
	Bay        string `json:"bay,omitempty"`         // Bay identifier

	// Port information
	PortNumber int    `json:"port_number,omitempty"` // Controller port number
	PhyID      int    `json:"phy_id,omitempty"`      // PHY identifier
	SASAddress string `json:"sas_address,omitempty"` // SAS address

	// NVMe-specific
	NVMeNamespace int `json:"nvme_namespace,omitempty"` // NVMe namespace ID

	// Fault domain hints
	PowerDomain string `json:"power_domain,omitempty"` // Power domain identifier (user-configured)
}

// DiskInventory represents a collection of discovered disks
type DiskInventory struct {
	Disks     map[string]*PhysicalDisk `json:"disks"`      // Keyed by DeviceID
	UpdatedAt time.Time                `json:"updated_at"` // Last inventory update
	Count     int                      `json:"count"`      // Total disk count
}

// DiskFilter represents criteria for filtering disks
type DiskFilter struct {
	States     []DiskState      `json:"states,omitempty"`
	HealthOnly []HealthStatus   `json:"health_only,omitempty"`
	Types      []DeviceType     `json:"types,omitempty"`
	Interfaces []InterfaceType  `json:"interfaces,omitempty"`
	PoolName   string           `json:"pool_name,omitempty"`   // Filter by pool membership
	Available  *bool            `json:"available,omitempty"`   // Filter available disks only
	MinSize    uint64           `json:"min_size,omitempty"`    // Minimum size in bytes
	MaxSize    uint64           `json:"max_size,omitempty"`    // Maximum size in bytes
	Tags       map[string]string `json:"tags,omitempty"`       // Filter by tags
}

// NewPhysicalDisk creates a new PhysicalDisk with defaults
func NewPhysicalDisk(deviceID, devicePath string) *PhysicalDisk {
	now := time.Now()
	return &PhysicalDisk{
		DeviceID:     deviceID,
		DevicePath:   devicePath,
		Type:         DeviceTypeUnknown,
		Interface:    InterfaceUnknown,
		State:        DiskStateDiscovered,
		Health:       HealthUnknown,
		DiscoveredAt: now,
		LastSeenAt:   now,
		UpdatedAt:    now,
		Tags:         make(map[string]string),
		Metadata:     make(map[string]string),
	}
}

// IsAvailable returns true if disk is available for use
func (d *PhysicalDisk) IsAvailable() bool {
	return d.State == DiskStateAvailable && d.Health != HealthFailed && d.Health != HealthCritical
}

// IsInUse returns true if disk is currently in use by a pool
func (d *PhysicalDisk) IsInUse() bool {
	return d.State == DiskStateInUse && d.PoolName != ""
}

// IsFaulted returns true if disk has hardware faults
func (d *PhysicalDisk) IsFaulted() bool {
	return d.State == DiskStateFaulted || d.Health == HealthFailed
}

// NeedsMaintenance returns true if disk requires attention
func (d *PhysicalDisk) NeedsMaintenance() bool {
	return d.Health == HealthWarning || d.Health == HealthCritical || d.State == DiskStateDegraded
}

// GetPreferredPath returns the preferred device path based on naming strategy
func (d *PhysicalDisk) GetPreferredPath(strategy NamingStrategy) string {
	switch strategy {
	case NamingByID:
		if d.ByIDPath != "" {
			return d.ByIDPath
		}
	case NamingByPath:
		if d.ByPathPath != "" {
			return d.ByPathPath
		}
	case NamingByVdev:
		if d.ByVdevPath != "" {
			return d.ByVdevPath
		}
	}
	// Fallback to primary device path
	return d.DevicePath
}

// MarkRemoved marks the disk as removed
func (d *PhysicalDisk) MarkRemoved() {
	now := time.Now()
	d.State = DiskStateOffline
	d.RemovedAt = &now
	d.UpdatedAt = now
}

// UpdateLastSeen updates the last seen timestamp
func (d *PhysicalDisk) UpdateLastSeen() {
	now := time.Now()
	d.LastSeenAt = now
	d.UpdatedAt = now
}

// SetState updates the disk state
func (d *PhysicalDisk) SetState(state DiskState) {
	d.State = state
	d.UpdatedAt = time.Now()
}

// SetHealth updates the disk health status
func (d *PhysicalDisk) SetHealth(health HealthStatus, reason string) {
	d.Health = health
	d.HealthReason = reason
	d.UpdatedAt = time.Now()
}

// MatchesFilter returns true if disk matches the filter criteria
func (d *PhysicalDisk) MatchesFilter(filter *DiskFilter) bool {
	if filter == nil {
		return true
	}

	// Check state filter
	if len(filter.States) > 0 {
		matched := false
		for _, state := range filter.States {
			if d.State == state {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check health filter
	if len(filter.HealthOnly) > 0 {
		matched := false
		for _, health := range filter.HealthOnly {
			if d.Health == health {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check type filter
	if len(filter.Types) > 0 {
		matched := false
		for _, t := range filter.Types {
			if d.Type == t {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check interface filter
	if len(filter.Interfaces) > 0 {
		matched := false
		for _, iface := range filter.Interfaces {
			if d.Interface == iface {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check pool name filter
	if filter.PoolName != "" && d.PoolName != filter.PoolName {
		return false
	}

	// Check availability filter
	if filter.Available != nil {
		if *filter.Available && !d.IsAvailable() {
			return false
		}
	}

	// Check size filters
	if filter.MinSize > 0 && d.SizeBytes < filter.MinSize {
		return false
	}
	if filter.MaxSize > 0 && d.SizeBytes > filter.MaxSize {
		return false
	}

	// Check tags filter
	if len(filter.Tags) > 0 {
		for key, value := range filter.Tags {
			if d.Tags[key] != value {
				return false
			}
		}
	}

	return true
}
