// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk/tools"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// Mapper handles physical topology discovery and mapping
type Mapper struct {
	logger      logger.Logger
	lsscsi      *tools.LsscsiExecutor
	sgses       *tools.SgSesExecutor
	toolChecker *tools.ToolChecker
	mu          sync.RWMutex
	topology    *types.PhysicalTopology
	lastScan    time.Time
}

// NewMapper creates a new topology mapper
func NewMapper(
	l logger.Logger,
	lsscsi *tools.LsscsiExecutor,
	sgses *tools.SgSesExecutor,
	toolChecker *tools.ToolChecker,
) *Mapper {
	return &Mapper{
		logger:      l,
		lsscsi:      lsscsi,
		sgses:       sgses,
		toolChecker: toolChecker,
		topology:    types.NewPhysicalTopology(),
	}
}

// DiscoverTopology discovers the complete physical topology
func (m *Mapper) DiscoverTopology(ctx context.Context, disks []*types.PhysicalDisk) (*types.PhysicalTopology, error) {
	m.logger.Info("starting topology discovery")
	startTime := time.Now()

	topo := types.NewPhysicalTopology()

	// Discover controllers and SCSI topology
	if m.toolChecker.IsAvailable("lsscsi") {
		if err := m.discoverSCSITopology(ctx, topo, disks); err != nil {
			m.logger.Warn("failed to discover SCSI topology", "error", err)
		}
	} else {
		m.logger.Warn("lsscsi not available, skipping SCSI topology discovery")
	}

	// Discover enclosures
	if m.toolChecker.IsAvailable("sg_ses") && m.toolChecker.IsAvailable("lsscsi") {
		if err := m.discoverEnclosures(ctx, topo); err != nil {
			m.logger.Warn("failed to discover enclosures", "error", err)
		}
	} else {
		m.logger.Debug("sg_ses or lsscsi not available, skipping enclosure discovery")
	}

	// Discover NVMe topology
	m.discoverNVMeTopology(ctx, topo, disks)

	// Update topology in disks
	m.assignTopologyToDisks(topo, disks)

	// Cache topology
	m.mu.Lock()
	m.topology = topo
	m.lastScan = time.Now()
	m.mu.Unlock()

	m.logger.Info("topology discovery completed",
		"controllers", len(topo.Controllers),
		"enclosures", len(topo.Enclosures),
		"duration", time.Since(startTime))

	return topo, nil
}

// discoverSCSITopology discovers SCSI/SAS topology using lsscsi
func (m *Mapper) discoverSCSITopology(ctx context.Context, topo *types.PhysicalTopology, disks []*types.PhysicalDisk) error {
	output, err := m.lsscsi.List(ctx)
	if err != nil {
		return errors.Wrap(err, errors.DiskTopologyFailed).
			WithMetadata("tool", "lsscsi")
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse lsscsi output
		// Format: [H:C:T:L]    type    vendor   model            device
		// Example: [0:0:0:0]    disk    ATA      Samsung SSD 860  /dev/sda
		scsiInfo := parseLsscsiLine(line)
		if scsiInfo == nil {
			continue
		}

		// Extract controller (host) information
		controllerID := "scsi-" + strconv.Itoa(scsiInfo.Host)
		if _, exists := topo.Controllers[controllerID]; !exists {
			ctrl := types.NewController(controllerID, "")
			ctrl.Type = "SCSI/SAS HBA"
			topo.AddController(ctrl)
		}

		// Find matching disk and assign topology
		for _, disk := range disks {
			if disk.DevicePath == scsiInfo.DevicePath {
				disk.Topology = &types.DiskTopology{
					ControllerID: controllerID,
					PortNumber:   scsiInfo.Channel,
				}
			}
		}
	}

	return nil
}

// discoverEnclosures discovers enclosures using sg_ses
func (m *Mapper) discoverEnclosures(ctx context.Context, topo *types.PhysicalTopology) error {
	// First, find enclosure devices using lsscsi
	output, err := m.lsscsi.ListEnclosures(ctx)
	if err != nil {
		return errors.Wrap(err, errors.DiskTopologyFailed).
			WithMetadata("tool", "lsscsi").
			WithMetadata("operation", "list_enclosures")
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "enclosu") {
			continue
		}

		// Extract sg device path (e.g., /dev/sg1)
		sgDevice := extractSGDevice(line)
		if sgDevice == "" {
			continue
		}

		// Get enclosure configuration
		encOutput, err := m.sgses.GetConfiguration(ctx, sgDevice)
		if err != nil {
			m.logger.Warn("failed to get enclosure config",
				"device", sgDevice,
				"error", err)
			continue
		}

		// Parse enclosure information (simplified - real parsing would be more complex)
		enc := m.parseEnclosureInfo(string(encOutput), sgDevice)
		if enc != nil {
			topo.AddEnclosure(enc)
		}
	}

	return nil
}

// discoverNVMeTopology discovers NVMe topology
func (m *Mapper) discoverNVMeTopology(ctx context.Context, topo *types.PhysicalTopology, disks []*types.PhysicalDisk) {
	// NVMe devices - extract controller and namespace info from device path
	nvmeRegex := regexp.MustCompile(`/dev/nvme(\d+)n(\d+)`)

	for _, disk := range disks {
		if disk.Interface != types.InterfaceNVMe {
			continue
		}

		matches := nvmeRegex.FindStringSubmatch(disk.DevicePath)
		if len(matches) != 3 {
			continue
		}

		ctrlNum, _ := strconv.Atoi(matches[1])
		nsNum, _ := strconv.Atoi(matches[2])

		controllerID := "nvme-" + strconv.Itoa(ctrlNum)

		// Create controller if not exists
		if _, exists := topo.Controllers[controllerID]; !exists {
			ctrl := types.NewController(controllerID, "")
			ctrl.Type = "NVMe"
			ctrl.Model = disk.Model
			topo.AddController(ctrl)
		}

		// Assign topology
		disk.Topology = &types.DiskTopology{
			ControllerID:  controllerID,
			NVMeNamespace: nsNum,
		}
	}
}

// assignTopologyToDisks assigns discovered topology to disks
func (m *Mapper) assignTopologyToDisks(topo *types.PhysicalTopology, disks []*types.PhysicalDisk) {
	// Topology assignment is already done during discovery
	// This function can be used for additional correlation logic if needed
}

// GetTopology returns the cached topology
func (m *Mapper) GetTopology() *types.PhysicalTopology {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy
	topo := &types.PhysicalTopology{
		Controllers: make(map[string]*types.Controller),
		Enclosures:  make(map[string]*types.Enclosure),
		UpdatedAt:   m.topology.UpdatedAt,
	}

	for k, v := range m.topology.Controllers {
		ctrlCopy := *v
		topo.Controllers[k] = &ctrlCopy
	}

	for k, v := range m.topology.Enclosures {
		encCopy := *v
		topo.Enclosures[k] = &encCopy
	}

	return topo
}

// SCSIInfo represents parsed lsscsi information
type SCSIInfo struct {
	Host       int
	Channel    int
	Target     int
	LUN        int
	Type       string
	Vendor     string
	Model      string
	DevicePath string
}

// parseLsscsiLine parses a single line from lsscsi output
func parseLsscsiLine(line string) *SCSIInfo {
	// Example: [0:0:0:0]    disk    ATA      Samsung SSD 860  /dev/sda
	hctlRegex := regexp.MustCompile(`\[(\d+):(\d+):(\d+):(\d+)\]`)
	matches := hctlRegex.FindStringSubmatch(line)
	if len(matches) != 5 {
		return nil
	}

	host, _ := strconv.Atoi(matches[1])
	channel, _ := strconv.Atoi(matches[2])
	target, _ := strconv.Atoi(matches[3])
	lun, _ := strconv.Atoi(matches[4])

	// Extract device path (last field)
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	devicePath := fields[len(fields)-1]

	return &SCSIInfo{
		Host:       host,
		Channel:    channel,
		Target:     target,
		LUN:        lun,
		DevicePath: devicePath,
	}
}

// extractSGDevice extracts sg device path from lsscsi line
func extractSGDevice(line string) string {
	// Look for /dev/sg* pattern
	sgRegex := regexp.MustCompile(`/dev/sg\d+`)
	match := sgRegex.FindString(line)
	return match
}

// parseEnclosureInfo parses enclosure information (simplified)
// TODO: Complete SES parsing implementation
// - Parse element descriptors for slot mapping
// - Parse enclosure status for fans, PSUs, temp sensors
// - Map SCSI devices to enclosure slots
// - Extract SAS addresses and PHY information
func (m *Mapper) parseEnclosureInfo(output, sgDevice string) *types.Enclosure {
	// This is a simplified parser - real implementation would parse
	// the complete SES structure using sg_ses output
	enc := types.NewEnclosure(sgDevice)
	enc.Slots = make(map[int]*types.Slot)

	// Extract basic info from output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "vendor:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				enc.Vendor = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "product:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				enc.Model = strings.TrimSpace(parts[1])
			}
		}
	}

	enc.Status.Overall = "OK"
	enc.Status.UpdatedAt = time.Now()

	return enc
}
