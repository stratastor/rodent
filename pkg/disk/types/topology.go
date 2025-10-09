// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package types

import "time"

// PhysicalTopology represents the complete physical topology of the storage system
type PhysicalTopology struct {
	Controllers map[string]*Controller `json:"controllers"` // Keyed by controller ID
	Enclosures  map[string]*Enclosure  `json:"enclosures"`  // Keyed by enclosure ID
	UpdatedAt   time.Time              `json:"updated_at"`  // Last topology update
}

// Controller represents a storage controller (HBA)
type Controller struct {
	ID           string            `json:"id"`            // Controller identifier
	PCIAddress   string            `json:"pci_address"`   // PCI bus address (e.g., 0000:05:00.0)
	Type         string            `json:"type"`          // Controller type (HBA, RAID, etc.)
	Vendor       string            `json:"vendor"`        // Vendor name
	Model        string            `json:"model"`         // Controller model
	Driver       string            `json:"driver"`        // Kernel driver
	Firmware     string            `json:"firmware"`      // Firmware version
	PortCount    int               `json:"port_count"`    // Number of ports
	Ports        map[int]*Port     `json:"ports"`         // Ports keyed by port number
	Capabilities []string          `json:"capabilities"`  // Controller capabilities
	Metadata     map[string]string `json:"metadata"`      // Additional metadata
	UpdatedAt    time.Time         `json:"updated_at"`    // Last update timestamp
}

// Port represents a controller port
type Port struct {
	Number      int                 `json:"number"`       // Port number
	State       string              `json:"state"`        // Port state (up, down, etc.)
	Speed       string              `json:"speed"`        // Link speed
	ConnectedTo string              `json:"connected_to"` // What's connected (enclosure, disk, etc.)
	Devices     []string            `json:"devices"`      // Device IDs connected to this port
	Metadata    map[string]string   `json:"metadata"`     // Additional metadata
}

// Enclosure represents a disk enclosure/JBOD
type Enclosure struct {
	ID           string             `json:"id"`            // Enclosure identifier (WWN or serial)
	Vendor       string             `json:"vendor"`        // Enclosure vendor
	Model        string             `json:"model"`         // Enclosure model
	Serial       string             `json:"serial"`        // Enclosure serial number
	Firmware     string             `json:"firmware"`      // Enclosure firmware version
	SlotCount    int                `json:"slot_count"`    // Number of disk slots
	Slots        map[int]*Slot      `json:"slots"`         // Slots keyed by slot number
	ControllerID string             `json:"controller_id"` // Connected controller ID
	Status       EnclosureStatus    `json:"status"`        // Overall enclosure status
	Elements     *EnclosureElements `json:"elements"`      // SES elements (fans, PSUs, etc.)
	Metadata     map[string]string  `json:"metadata"`      // Additional metadata
	UpdatedAt    time.Time          `json:"updated_at"`    // Last update timestamp
}

// EnclosureStatus represents enclosure health status
type EnclosureStatus struct {
	Overall     string    `json:"overall"`      // Overall status (OK, Warning, Critical)
	Temperature float64   `json:"temperature"`  // Enclosure temperature (if available)
	Fans        int       `json:"fans"`         // Number of functioning fans
	PowerSupply int       `json:"power_supply"` // Number of functioning PSUs
	Alarms      []string  `json:"alarms"`       // Active alarms
	UpdatedAt   time.Time `json:"updated_at"`   // Status update timestamp
}

// EnclosureElements represents SES (SCSI Enclosure Services) elements
type EnclosureElements struct {
	Fans         []*FanElement        `json:"fans,omitempty"`
	PowerSupplies []*PowerSupplyElement `json:"power_supplies,omitempty"`
	TempSensors  []*TempSensorElement `json:"temp_sensors,omitempty"`
	VoltageSensors []*VoltageSensorElement `json:"voltage_sensors,omitempty"`
}

// FanElement represents a cooling fan
type FanElement struct {
	Index     int     `json:"index"`
	Status    string  `json:"status"`    // OK, Warning, Failed
	Speed     int     `json:"speed"`     // RPM (if available)
	SpeedPct  float64 `json:"speed_pct"` // Percentage of max speed
}

// PowerSupplyElement represents a power supply unit
type PowerSupplyElement struct {
	Index  int    `json:"index"`
	Status string `json:"status"` // OK, Warning, Failed
	ACFail bool   `json:"ac_fail"`
	DCFail bool   `json:"dc_fail"`
}

// TempSensorElement represents a temperature sensor
type TempSensorElement struct {
	Index       int     `json:"index"`
	Location    string  `json:"location"`    // Sensor location description
	Temperature float64 `json:"temperature"` // Temperature in Celsius
	Status      string  `json:"status"`      // OK, Warning, Critical
	Threshold   float64 `json:"threshold"`   // Warning threshold
}

// VoltageSensorElement represents a voltage sensor
type VoltageSensorElement struct {
	Index   int     `json:"index"`
	Voltage float64 `json:"voltage"` // Voltage reading
	Status  string  `json:"status"`  // OK, Warning, Critical
}

// Slot represents a disk slot in an enclosure
type Slot struct {
	Number      int       `json:"number"`                 // Slot number
	Occupied    bool      `json:"occupied"`               // Whether slot has a disk
	DeviceID    string    `json:"device_id,omitempty"`    // Device ID if occupied
	Status      string    `json:"status"`                 // Slot status (empty, ok, fault, etc.)
	LED         *SlotLED  `json:"led,omitempty"`          // LED status
	Metadata    map[string]string `json:"metadata,omitempty"` // Additional metadata
}

// SlotLED represents slot LED indicators
type SlotLED struct {
	Activity bool `json:"activity"` // Activity LED
	Locate   bool `json:"locate"`   // Locate LED
	Fault    bool `json:"fault"`    // Fault LED
}

// FaultDomain represents a failure domain grouping
type FaultDomain struct {
	Type      FaultDomainType `json:"type"`       // Controller, Enclosure, PowerDomain
	ID        string          `json:"id"`         // Domain identifier
	DeviceIDs []string        `json:"device_ids"` // Devices in this domain
	Metadata  map[string]string `json:"metadata"` // Additional metadata
}

// FaultDomainType represents the type of fault domain
type FaultDomainType string

const (
	FaultDomainController FaultDomainType = "CONTROLLER" // Controller-based domain
	FaultDomainEnclosure  FaultDomainType = "ENCLOSURE"  // Enclosure-based domain
	FaultDomainPower      FaultDomainType = "POWER"      // Power domain (user-configured)
)

// FaultDomainAnalysis represents fault domain analysis results
type FaultDomainAnalysis struct {
	Domains     map[string]*FaultDomain `json:"domains"`      // Keyed by domain ID
	Violations  []string                `json:"violations"`   // Detected violations
	Warnings    []string                `json:"warnings"`     // Warnings
	AnalyzedAt  time.Time               `json:"analyzed_at"`  // Analysis timestamp
}

// NewPhysicalTopology creates a new empty topology
func NewPhysicalTopology() *PhysicalTopology {
	return &PhysicalTopology{
		Controllers: make(map[string]*Controller),
		Enclosures:  make(map[string]*Enclosure),
		UpdatedAt:   time.Now(),
	}
}

// NewController creates a new controller
func NewController(id, pciAddress string) *Controller {
	return &Controller{
		ID:         id,
		PCIAddress: pciAddress,
		Ports:      make(map[int]*Port),
		Metadata:   make(map[string]string),
		UpdatedAt:  time.Now(),
	}
}

// NewEnclosure creates a new enclosure
func NewEnclosure(id string) *Enclosure {
	return &Enclosure{
		ID:       id,
		Slots:    make(map[int]*Slot),
		Metadata: make(map[string]string),
		Status: EnclosureStatus{
			Overall: "UNKNOWN",
		},
		UpdatedAt: time.Now(),
	}
}

// AddController adds a controller to the topology
func (t *PhysicalTopology) AddController(ctrl *Controller) {
	t.Controllers[ctrl.ID] = ctrl
	t.UpdatedAt = time.Now()
}

// AddEnclosure adds an enclosure to the topology
func (t *PhysicalTopology) AddEnclosure(enc *Enclosure) {
	t.Enclosures[enc.ID] = enc
	t.UpdatedAt = time.Now()
}

// GetController returns a controller by ID
func (t *PhysicalTopology) GetController(id string) (*Controller, bool) {
	ctrl, ok := t.Controllers[id]
	return ctrl, ok
}

// GetEnclosure returns an enclosure by ID
func (t *PhysicalTopology) GetEnclosure(id string) (*Enclosure, bool) {
	enc, ok := t.Enclosures[id]
	return enc, ok
}

// GetControllerCount returns the number of controllers
func (t *PhysicalTopology) GetControllerCount() int {
	return len(t.Controllers)
}

// GetEnclosureCount returns the number of enclosures
func (t *PhysicalTopology) GetEnclosureCount() int {
	return len(t.Enclosures)
}

// AnalyzeFaultDomains analyzes fault domains based on topology
func (t *PhysicalTopology) AnalyzeFaultDomains(disks []*PhysicalDisk) *FaultDomainAnalysis {
	analysis := &FaultDomainAnalysis{
		Domains:    make(map[string]*FaultDomain),
		Violations: []string{},
		Warnings:   []string{},
		AnalyzedAt: time.Now(),
	}

	// Group disks by controller
	for _, disk := range disks {
		if disk.Topology == nil {
			continue
		}

		// Controller domain
		if disk.Topology.ControllerID != "" {
			domainID := "ctrl-" + disk.Topology.ControllerID
			if _, exists := analysis.Domains[domainID]; !exists {
				analysis.Domains[domainID] = &FaultDomain{
					Type:      FaultDomainController,
					ID:        disk.Topology.ControllerID,
					DeviceIDs: []string{},
					Metadata:  make(map[string]string),
				}
			}
			analysis.Domains[domainID].DeviceIDs = append(analysis.Domains[domainID].DeviceIDs, disk.DeviceID)
		}

		// Enclosure domain
		if disk.Topology.EnclosureID != "" {
			domainID := "enc-" + disk.Topology.EnclosureID
			if _, exists := analysis.Domains[domainID]; !exists {
				analysis.Domains[domainID] = &FaultDomain{
					Type:      FaultDomainEnclosure,
					ID:        disk.Topology.EnclosureID,
					DeviceIDs: []string{},
					Metadata:  make(map[string]string),
				}
			}
			analysis.Domains[domainID].DeviceIDs = append(analysis.Domains[domainID].DeviceIDs, disk.DeviceID)
		}

		// Power domain (if configured)
		if disk.Topology.PowerDomain != "" {
			domainID := "pwr-" + disk.Topology.PowerDomain
			if _, exists := analysis.Domains[domainID]; !exists {
				analysis.Domains[domainID] = &FaultDomain{
					Type:      FaultDomainPower,
					ID:        disk.Topology.PowerDomain,
					DeviceIDs: []string{},
					Metadata:  make(map[string]string),
				}
			}
			analysis.Domains[domainID].DeviceIDs = append(analysis.Domains[domainID].DeviceIDs, disk.DeviceID)
		}
	}

	return analysis
}
