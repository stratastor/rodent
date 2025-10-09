// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"encoding/json"

	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// LsblkJSON represents the JSON output structure from lsblk
type LsblkJSON struct {
	BlockDevices []BlockDevice `json:"blockdevices"`
}

// BlockDevice represents a single block device from lsblk output
type BlockDevice struct {
	Name       string         `json:"name"`
	Path       string         `json:"path"`
	Type       string         `json:"type"`
	Size       uint64         `json:"size"`
	Vendor     *string        `json:"vendor"`
	Model      *string        `json:"model"`
	Serial     *string        `json:"serial"`
	WWN        *string        `json:"wwn"`
	State      *string        `json:"state"`
	Mountpoint *string        `json:"mountpoint"`
	Fstype     *string        `json:"fstype"`
	PhySec     int            `json:"phy-sec"`
	LogSec     int            `json:"log-sec"`
	Rota       bool           `json:"rota"`
	DiscGran   *int           `json:"disc-gran"`
	DiscMax    *uint64        `json:"disc-max"`
	Tran       *string        `json:"tran"` // Transport type (sata, sas, nvme, usb)
	HCTL       *string        `json:"hctl"` // Host:Channel:Target:LUN for SCSI devices
	Children   []BlockDevice  `json:"children,omitempty"`
}

// ParseLsblkJSON parses lsblk JSON output
func ParseLsblkJSON(jsonData []byte) ([]*BlockDevice, error) {
	var lsblk LsblkJSON
	if err := json.Unmarshal(jsonData, &lsblk); err != nil {
		return nil, errors.Wrap(err, errors.DiskDiscoveryFailed).
			WithMetadata("operation", "unmarshal_lsblk_json")
	}

	// Convert []BlockDevice to []*BlockDevice
	devices := make([]*BlockDevice, len(lsblk.BlockDevices))
	for i := range lsblk.BlockDevices {
		devices[i] = &lsblk.BlockDevices[i]
	}

	return devices, nil
}

// IsPhysicalDisk returns true if this is a physical disk (not partition, loop, etc.)
func (bd *BlockDevice) IsPhysicalDisk() bool {
	return bd.Type == "disk"
}

// IsPartition returns true if this is a partition
func (bd *BlockDevice) IsPartition() bool {
	return bd.Type == "part"
}

// IsLoop returns true if this is a loop device
func (bd *BlockDevice) IsLoop() bool {
	return bd.Type == "loop"
}

// IsZFSVolumeDevice returns true if this is a ZFS zvol device
func (bd *BlockDevice) IsZFSVolumeDevice() bool {
	// ZFS zvols appear as /dev/zd*
	if len(bd.Path) > 7 && bd.Path[:7] == "/dev/zd" {
		return true
	}
	return false
}

// DetermineInterfaceType determines the interface type from lsblk data
func (bd *BlockDevice) DetermineInterfaceType() types.InterfaceType {
	// Check transport if available
	if bd.Tran != nil {
		switch *bd.Tran {
		case "sata":
			return types.InterfaceSATA
		case "sas":
			return types.InterfaceSAS
		case "nvme":
			return types.InterfaceNVMe
		case "usb":
			return types.InterfaceUSB
		}
	}

	// Fallback: detect from device path
	if len(bd.Path) >= 11 && bd.Path[:11] == "/dev/nvme" {
		return types.InterfaceNVMe
	}
	if len(bd.Path) >= 8 && bd.Path[:8] == "/dev/sd" {
		// Could be SATA or SAS, need more info
		return types.InterfaceSATA // Default assumption
	}
	if bd.Model != nil && *bd.Model == "virtio" {
		return types.InterfaceVirtIO
	}

	return types.InterfaceUnknown
}

// DetermineDeviceType determines if device is HDD, SSD, or NVMe based on rota flag
func (bd *BlockDevice) DetermineDeviceType() types.DeviceType {
	iface := bd.DetermineInterfaceType()

	// NVMe is always SSD
	if iface == types.InterfaceNVMe {
		return types.DeviceTypeNVMe
	}

	// Use rotation flag to distinguish HDD from SSD
	if bd.Rota {
		return types.DeviceTypeHDD
	}

	return types.DeviceTypeSSD
}

// GetVendorString returns vendor as string (handles nil)
func (bd *BlockDevice) GetVendorString() string {
	if bd.Vendor != nil {
		return *bd.Vendor
	}
	return ""
}

// GetModelString returns model as string (handles nil)
func (bd *BlockDevice) GetModelString() string {
	if bd.Model != nil {
		return *bd.Model
	}
	return ""
}

// GetSerialString returns serial as string (handles nil)
func (bd *BlockDevice) GetSerialString() string {
	if bd.Serial != nil {
		return *bd.Serial
	}
	return ""
}

// GetWWNString returns WWN as string (handles nil)
func (bd *BlockDevice) GetWWNString() string {
	if bd.WWN != nil {
		return *bd.WWN
	}
	return ""
}

// GetStateString returns state as string (handles nil)
func (bd *BlockDevice) GetStateString() string {
	if bd.State != nil {
		return *bd.State
	}
	return ""
}

// ToPhysicalDisk converts BlockDevice to PhysicalDisk type
func (bd *BlockDevice) ToPhysicalDisk() *types.PhysicalDisk {
	disk := types.NewPhysicalDisk(bd.Path, bd.Path)

	disk.Serial = bd.GetSerialString()
	disk.Model = bd.GetModelString()
	disk.Vendor = bd.GetVendorString()
	disk.WWN = bd.GetWWNString()
	disk.SizeBytes = bd.Size
	disk.Type = bd.DetermineDeviceType()
	disk.Interface = bd.DetermineInterfaceType()

	// Set device path variations
	disk.DevicePath = bd.Path
	// By-id will be filled by udevadm
	// By-path will be filled by udevadm

	return disk
}

// FilterPhysicalDisks filters out only physical disks from lsblk output
func FilterPhysicalDisks(devices []*BlockDevice) []*BlockDevice {
	var disks []*BlockDevice

	for _, dev := range devices {
		// Skip loop devices, partitions, and ZFS zvols
		if dev.IsPhysicalDisk() && !dev.IsZFSVolumeDevice() {
			disks = append(disks, dev)
		}
	}

	return disks
}
