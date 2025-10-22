# Disk Management API - Implementation Plan

**Status**: Near Complete (Production Ready)
**Last Updated**: 2025-10-22

## Overview

This document tracks the implementation status of all disk management API endpoints (REST and gRPC). The API layer exposes disk manager functionality through both REST HTTP endpoints and gRPC commands for Toggle integration.

**Key Principle**: One-to-one correspondence between gRPC commands and REST endpoints.

## Current Status Summary

| Category | Total | Implemented | Remaining | Progress |
|----------|-------|-------------|-----------|----------|
| **gRPC Commands** | 42 | 37 | 5 | 88% |
| **REST Endpoints** | 42 | 37 | 5 | 88% |

**Note**: Each gRPC command has a corresponding REST endpoint for one-to-one mapping. The 5 remaining commands require Phase 8 (Naming Strategy) and Phase 9 (Fault Domain Analysis) implementation.

## Proto Command Constants

**Location**: `/Users/raam/lab/code/tshack/strata/toggle-rodent-proto/proto/disk_command_types.go`

**Status**: ✅ Complete (43 constants defined, lines 8-72)

All command type constants are defined and ready to use. Commands follow the pattern: `disk.{category}.{action}` or `disk.{category}.{subcategory}.{action}`.

### Command Breakdown

- Inventory & Discovery: 4 commands (list, get, discover, refresh)
- Health & SMART: 3 commands (health.get, smart.get, smart.refresh)
- Probe Operations: 5 commands (start, cancel, get, list, history)
- Probe Schedules: 7 commands (list, get, create, update, delete, enable, disable)
- Topology: 4 commands (get, refresh, controllers, enclosures)
- Fault Domains: 2 commands (analyze, get)
- State Management: 4 commands (state.get, state.set, validate, quarantine)
- Naming Strategy: 3 commands (get, set, vdev-conf.generate)
- Metadata: 3 commands (tags.set, tags.delete, notes.set)
- Statistics & Monitoring: 4 commands (stats.get, stats.global, monitoring.get, monitoring.set)
- Configuration: 3 commands (config.get, config.update, config.reload)

## gRPC Command Handlers

**Base File**: `pkg/disk/api/routes_grpc.go` and `pkg/disk/api/handler_grpc.go`

### Implemented (37/42) ✅

#### Inventory Operations (4/4) ✅

- ✅ `disk.list` - List all disks with optional filter
- ✅ `disk.get` - Get disk details by device_id
- ✅ `disk.discover` - Trigger discovery scan
- ✅ `disk.refresh` - Refresh disk information

#### Health & SMART Operations (4/4) ✅

- ✅ `disk.health.get` - Get disk health status
- ✅ `disk.smart.get` - Get SMART data
- ✅ `disk.smart.refresh` - Trigger health check
- ✅ `disk.health.check` - Trigger health check (alias)

#### Probe Operations (5/5) ✅

- ✅ `disk.probe.start` - Start SMART probe (quick/extensive)
- ✅ `disk.probe.cancel` - Cancel running probe
- ✅ `disk.probe.get` - Get probe execution details
- ✅ `disk.probe.list` - List active probes
- ✅ `disk.probe.history` - Get probe history for device

#### Probe Schedule Operations (7/7) ✅

- ✅ `disk.probe.schedule.list` - List all schedules
- ✅ `disk.probe.schedule.get` - Get schedule details
- ✅ `disk.probe.schedule.create` - Create new schedule
- ✅ `disk.probe.schedule.update` - Update schedule
- ✅ `disk.probe.schedule.delete` - Delete schedule
- ✅ `disk.probe.schedule.enable` - Enable schedule
- ✅ `disk.probe.schedule.disable` - Disable schedule

#### Topology Operations (4/4) ✅

- ✅ `disk.topology.get` - Get complete topology
- ✅ `disk.topology.refresh` - Refresh topology
- ✅ `disk.topology.controllers` - List controllers
- ✅ `disk.topology.enclosures` - List enclosures

#### State Management Operations (4/4) ✅

- ✅ `disk.state.get` - Get complete state
- ✅ `disk.state.set` - Set disk state
- ✅ `disk.validate` - Validate a disk
- ✅ `disk.quarantine` - Quarantine a disk

#### Configuration Operations (3/3) ✅

- ✅ `disk.config.get` - Get current configuration
- ✅ `disk.config.update` - Update configuration
- ✅ `disk.config.reload` - Reload config from file

#### Metadata Operations (3/3) ✅

- ✅ `disk.tags.set` - Set disk tags
- ✅ `disk.tags.delete` - Delete disk tags
- ✅ `disk.notes.set` - Set disk notes

#### Statistics & Monitoring Operations (4/4) ✅

- ✅ `disk.stats.get` - Get device statistics
- ✅ `disk.stats.global` - Get global statistics
- ✅ `disk.monitoring.get` - Get monitoring config
- ✅ `disk.monitoring.set` - Set monitoring config

### Not Implemented (5/42) ⚠️

These 5 commands require Phase 8 (Naming Strategy) and Phase 9 (Fault Domain Analysis) to be implemented first:

#### Fault Domain Operations (2/2) - Requires Phase 9

- ❌ `disk.fault-domains.analyze` - Analyze fault domains
  - **Blocker**: Phase 9 not implemented
  - **Status**: Planned for future release
- ❌ `disk.fault-domains.get` - Get fault domain info
  - **Blocker**: Phase 9 not implemented
  - **Status**: Planned for future release

#### Naming Strategy Operations (3/3) - Requires Phase 8

- ❌ `disk.naming-strategy.get` - Get current naming strategy
  - **Blocker**: Phase 8 - Naming Strategy not implemented
  - **Status**: Planned for future release
- ❌ `disk.naming-strategy.set` - Set naming strategy
  - **Blocker**: Phase 8 - Naming Strategy not implemented
  - **Status**: Planned for future release
- ❌ `disk.vdev-conf.generate` - Generate vdev_id.conf
  - **Blocker**: Phase 8 - Naming Strategy not implemented
  - **Status**: Planned for future release

---

### Future Enhancements

#### Advanced Operations (Not in proto constants yet)

These are planned but don't have proto constants yet:

- Pool suggestion API (suggest disks for new pool)
- Redundancy planning (analyze fault domains)
- Device compatibility check (for mixed pools)
- Wear leveling analysis (for SSD pools)

## REST Endpoints

**Base File**: `pkg/disk/api/handler.go`
**Base Path**: `/api/v1/rodent/disk`

### Implemented (6/35+) ✅

#### Inventory

- ✅ `GET /inventory` - List all disks
- ✅ `GET /disks/:device_id` - Get disk details
- ✅ `POST /discovery/trigger` - Trigger discovery

#### Health

- ✅ `POST /health/check` - Trigger health check
- ✅ `GET /disks/:device_id/health` - Get disk health
- ✅ `GET /disks/:device_id/smart` - Get SMART data

### Not Implemented (29+/35+) ⚠️

#### Probe Operations

- ❌ `POST /disks/:device_id/probes` - Trigger probe (quick/extensive)
- ❌ `GET /disks/:device_id/probes` - List probes for device
- ❌ `GET /probes` - List all active probes
- ❌ `GET /probes/:probe_id` - Get probe details
- ❌ `POST /probes/:probe_id/cancel` - Cancel probe
- ❌ `GET /disks/:device_id/probe-history` - Get probe history

#### Probe Schedules

- ❌ `GET /probe-schedules` - List all schedules
- ❌ `POST /probe-schedules` - Create schedule
- ❌ `GET /probe-schedules/:schedule_id` - Get schedule
- ❌ `PUT /probe-schedules/:schedule_id` - Update schedule
- ❌ `DELETE /probe-schedules/:schedule_id` - Delete schedule
- ❌ `POST /probe-schedules/:schedule_id/enable` - Enable schedule
- ❌ `POST /probe-schedules/:schedule_id/disable` - Disable schedule

#### Topology

- ❌ `GET /topology` - Get complete topology
- ❌ `POST /topology/refresh` - Refresh topology
- ❌ `GET /topology/controllers` - List controllers
- ❌ `GET /topology/enclosures` - List enclosures
- ❌ `GET /topology/fault-domains` - Analyze fault domains

#### Configuration

- ❌ `GET /config` - Get configuration
- ❌ `PUT /config` - Update configuration
- ❌ `POST /config/reload` - Reload from file
- ❌ `GET /config/monitoring` - Get monitoring config
- ❌ `PUT /config/monitoring` - Update monitoring config

#### State & Statistics

- ❌ `GET /state` - Get manager state
- ❌ `GET /stats` - Get global statistics
- ❌ `GET /disks/:device_id/stats` - Get device statistics

#### Metadata

- ❌ `PUT /disks/:device_id/tags` - Set tags
- ❌ `DELETE /disks/:device_id/tags` - Delete tags
- ❌ `PUT /disks/:device_id/notes` - Set notes
- ❌ `POST /disks/:device_id/quarantine` - Quarantine disk

#### Naming Strategy

- ❌ `GET /naming-strategy` - Get current strategy
- ❌ `PUT /naming-strategy` - Set strategy
- ❌ `POST /vdev-conf/generate` - Generate vdev_id.conf

## Implementation Priority

### Priority 1: Core Management API (Enable basic operations)

**Goal**: Expose existing Manager functionality through API

1. **Probe Management** (High business value)
   - Add Manager wrapper methods for probe operations
   - Implement probe trigger, list, get, cancel handlers
   - Add probe history query to StateManager

2. **Configuration Management** (Essential for operations)
   - Expose ConfigManager through Manager
   - Implement config get/update/reload handlers
   - Add validation to config updates

3. **State Management** (Essential for monitoring)
   - Expose StateManager read methods
   - Implement state query handlers
   - Add disk state modification methods

### Priority 2: Scheduling & Automation

**Goal**: Enable scheduled probes and automation

1. **Probe Schedules** (High value for automation)
   - Expose ProbeScheduler through Manager
   - Implement schedule CRUD handlers
   - Add enable/disable functionality to schedules

2. **Statistics** (Good for observability)
   - Add statistics tracking to PhysicalDisk
   - Add global statistics to DiskManagerState
   - Implement stats query handlers

### Priority 3: Advanced Features

**Goal**: Enable topology-aware operations and ZFS integration

1. **Topology Operations**
   - Implement topology aggregation methods
   - Add controller/enclosure listing
   - Implement fault domain analysis (when Phase 3 complete)

2. **Metadata Operations**
   - Implement tags/notes setters in Manager
   - Add metadata update handlers
   - Add quarantine convenience method

3. **Naming Strategy** (Depends on Phase 8)
   - Wait for Phase 8 implementation
   - Add naming strategy handlers once module is ready

### Priority 4: ZFS Integration Helpers (Future)

**Goal**: Streamline ZFS pool operations

- Pool suggestion API
- Device compatibility checker
- Redundancy planner
- Wear leveling analyzer

## Required Manager API Extensions

To complete the API implementation, the Manager needs these additional public methods:

```go
// Probe operations
func (m *Manager) TriggerProbe(deviceID string, probeType types.ProbeType) (probeID string, error)
func (m *Manager) CancelProbe(probeID string) error
func (m *Manager) GetProbeExecution(probeID string) (*types.ProbeExecution, error)
func (m *Manager) GetActiveProbes() []*types.ProbeExecution
func (m *Manager) GetProbeHistory(deviceID string, limit int) []*types.ProbeExecution

// Probe schedule operations
func (m *Manager) GetProbeSchedules() []*types.ProbeSchedule
func (m *Manager) GetProbeSchedule(scheduleID string) (*types.ProbeSchedule, error)
func (m *Manager) CreateProbeSchedule(schedule *types.ProbeSchedule) error
func (m *Manager) UpdateProbeSchedule(scheduleID string, schedule *types.ProbeSchedule) error
func (m *Manager) DeleteProbeSchedule(scheduleID string) error
func (m *Manager) EnableProbeSchedule(scheduleID string) error
func (m *Manager) DisableProbeSchedule(scheduleID string) error

// Configuration operations
func (m *Manager) GetConfig() *types.DiskManagerConfig
func (m *Manager) UpdateConfig(config *types.DiskManagerConfig) error
func (m *Manager) ReloadConfig() error

// State operations
func (m *Manager) GetState() *types.DiskManagerState
func (m *Manager) SetDiskState(deviceID string, state types.DiskState, reason string) error
func (m *Manager) QuarantineDisk(deviceID string, reason string) error

// Metadata operations
func (m *Manager) SetDiskTags(deviceID string, tags map[string]string) error
func (m *Manager) DeleteDiskTags(deviceID string, tagKeys []string) error
func (m *Manager) SetDiskNotes(deviceID string, notes string) error

// Topology operations
func (m *Manager) GetTopology() (*types.TopologyInfo, error)
func (m *Manager) RefreshTopology(ctx context.Context) error
func (m *Manager) GetControllers() ([]*types.ControllerInfo, error)
func (m *Manager) GetEnclosures() ([]*types.EnclosureInfo, error)

// Statistics operations
func (m *Manager) GetDeviceStatistics(deviceID string) (*types.DeviceStatistics, error)
func (m *Manager) GetGlobalStatistics() *types.GlobalStatistics
```

## Required StateManager Extensions

```go
// Query methods
func (sm *StateManager) GetProbeSchedule(scheduleID string) (*types.ProbeSchedule, error)
func (sm *StateManager) GetProbeExecution(probeID string) (*types.ProbeExecution, error)
func (sm *StateManager) GetProbeHistory(deviceID string, limit int) []*types.ProbeExecution
func (sm *StateManager) GetActiveProbes() []*types.ProbeExecution

// Statistics methods
func (sm *StateManager) GetDeviceStatistics(deviceID string) *types.DeviceStatistics
func (sm *StateManager) GetGlobalStatistics() *types.GlobalStatistics
```

## Required ProbeSchedule Extensions

```go
type ProbeSchedule struct {
    // ... existing fields ...
    Enabled bool `json:"enabled"` // NEW: Enable/disable schedule
}
```

## Testing Strategy

### Unit Tests

- Test each handler with mock Manager
- Test request validation
- Test error handling and error response format
- Test success response format

### Integration Tests

- Test with real Manager instance
- Test end-to-end gRPC command flow
- Test end-to-end REST endpoint flow
- Test concurrent operations

### API Documentation

- Generate OpenAPI spec for REST endpoints
- Document gRPC command payloads
- Provide example requests/responses
- Document error codes

## Notes

- All gRPC handlers follow the pattern from `pkg/system/api/routes_grpc.go`
- REST handlers follow the pattern from `pkg/system/api/handler.go`
- Both use standardized `APIResponse` and `APIError` structures
- Error responses use `errorResponse()` helper which returns `(nil, error)`
- Success responses use `successResponse()` which wraps data in `APIResponse`
