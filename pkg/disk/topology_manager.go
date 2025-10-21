// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"context"
)

// TopologyInfo represents aggregated topology information
type TopologyInfo struct {
	Controllers []*ControllerInfo `json:"controllers"`
	Enclosures  []*EnclosureInfo  `json:"enclosures"`
	TotalDisks  int               `json:"total_disks"`
}

// ControllerInfo represents a controller with its disks
type ControllerInfo struct {
	PCIAddress string   `json:"pci_address"`
	Model      string   `json:"model"`
	DiskCount  int      `json:"disk_count"`
	Disks      []string `json:"disks"`
}

// EnclosureInfo represents an enclosure with its disks
type EnclosureInfo struct {
	EnclosureID string   `json:"enclosure_id"`
	Model       string   `json:"model,omitempty"`
	SlotCount   int      `json:"slot_count"`
	Disks       []string `json:"disks"`
}

// GetTopology returns complete topology information
func (m *Manager) GetTopology() (*TopologyInfo, error) {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	info := &TopologyInfo{
		Controllers: make([]*ControllerInfo, 0),
		Enclosures:  make([]*EnclosureInfo, 0),
		TotalDisks:  len(m.deviceCache),
	}

	// Aggregate by controller
	controllerMap := make(map[string]*ControllerInfo)
	enclosureMap := make(map[string]*EnclosureInfo)

	for _, disk := range m.deviceCache {
		if disk.Topology != nil {
			// Aggregate by controller using ControllerPCI
			if disk.Topology.ControllerPCI != "" {
				if ctrl, exists := controllerMap[disk.Topology.ControllerPCI]; exists {
					ctrl.DiskCount++
					ctrl.Disks = append(ctrl.Disks, disk.DeviceID)
				} else {
					controllerMap[disk.Topology.ControllerPCI] = &ControllerInfo{
						PCIAddress: disk.Topology.ControllerPCI,
						Model:      disk.Topology.ControllerType,
						DiskCount:  1,
						Disks:      []string{disk.DeviceID},
					}
				}
			}

			// Aggregate by enclosure
			if disk.Topology.EnclosureID != "" {
				if enc, exists := enclosureMap[disk.Topology.EnclosureID]; exists {
					enc.SlotCount++
					enc.Disks = append(enc.Disks, disk.DeviceID)
				} else {
					enclosureMap[disk.Topology.EnclosureID] = &EnclosureInfo{
						EnclosureID: disk.Topology.EnclosureID,
						Model:       disk.Topology.EnclosureModel,
						SlotCount:   1,
						Disks:       []string{disk.DeviceID},
					}
				}
			}
		}
	}

	// Convert maps to slices
	for _, ctrl := range controllerMap {
		info.Controllers = append(info.Controllers, ctrl)
	}
	for _, enc := range enclosureMap {
		info.Enclosures = append(info.Enclosures, enc)
	}

	return info, nil
}

// RefreshTopology refreshes topology information by triggering discovery
func (m *Manager) RefreshTopology(ctx context.Context) error {
	return m.TriggerDiscovery(ctx)
}

// GetControllers returns all unique controllers
func (m *Manager) GetControllers() ([]*ControllerInfo, error) {
	topology, err := m.GetTopology()
	if err != nil {
		return nil, err
	}
	return topology.Controllers, nil
}

// GetEnclosures returns all unique enclosures
func (m *Manager) GetEnclosures() ([]*EnclosureInfo, error) {
	topology, err := m.GetTopology()
	if err != nil {
		return nil, err
	}
	return topology.Enclosures, nil
}
