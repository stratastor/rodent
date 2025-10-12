# Enterprise Disk Management System - Implementation Guide

**Status**: In Development
**Version**: 1.0.0
**Last Updated**: 2025-10-10

## Table of Contents
1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Configuration Management](#configuration-management)
4. [State Management](#state-management)
5. [SMART Probe Scheduling](#smart-probe-scheduling)
6. [Package Structure](#package-structure)
7. [Implementation Status](#implementation-status)
8. [Design Decisions](#design-decisions)
9. [API Specification](#api-specification)
10. [Event Schema](#event-schema)
11. [Integration Points](#integration-points)

---

## Overview

Enterprise-grade disk management service for ZFS-based storage servers, supporting:
- **Scale**: Few directly attached disks (<20) to 100+ disks via JBODs
- **Platform Agnostic**: Cloud instances, VMs, physical servers
- **Vendor Agnostic**: Commodity hardware, no vendor lock-in
- **Production Ready**: Robust error handling, observability, graceful degradation

### Key Capabilities
- ✅ Automatic device discovery and physical topology mapping
- ✅ Multi-protocol health monitoring (SMART, NVMe, iostat)
- ✅ Scheduled & on-demand SMART probing with conflict prevention
- ✅ Real-time hotplug detection with reconciliation
- ✅ Intelligent device naming strategies for ZFS integration
- ✅ Fault domain analysis and redundancy planning
- ✅ Event-driven architecture with structured logging
- ✅ Runtime configuration via REST/gRPC APIs
- ✅ Persistent YAML configuration + JSON state tracking
- ✅ Operation tracking with progress monitoring

---

## Architecture

### System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        API Layer (pkg/disk/api/)                │
│                      Base: /api/v1/rodent/storage/              │
│  ┌──────────────────┐              ┌──────────────────┐        │
│  │  REST Handlers   │              │  gRPC Handlers   │        │
│  │  (handler.go)    │              │ (handler_grpc.go)│        │
│  └────────┬─────────┘              └────────┬─────────┘        │
│           │                                  │                   │
│           └──────────────┬───────────────────┘                  │
└───────────────────────────┼──────────────────────────────────────┘
                            │
┌───────────────────────────▼──────────────────────────────────────┐
│                  Disk Manager (pkg/disk/manager.go)              │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ • Device inventory management                             │  │
│  │ • State machine orchestration                             │  │
│  │ • Event coordination                                      │  │
│  │ • Periodic reconciliation                                 │  │
│  │ • Configuration management                                │  │
│  │ • State persistence                                       │  │
│  └──────────────────────────────────────────────────────────┘  │
└──────┬────────┬──────────┬──────────┬──────────┬──────┬────────┘
       │        │          │          │          │      │
   ┌───▼───┐┌──▼────┐┌────▼────┐┌───▼────┐┌────▼──┐┌──▼─────┐
   │Discov-││Topo-  ││Health   ││Hotplug ││Naming ││SMART   │
   │ery    ││logy   ││Monitor  ││Monitor ││Strat. ││Probe   │
   │       ││Parser ││         ││        ││       ││Sched.  │
   └───┬───┘└──┬────┘└────┬────┘└───┬────┘└───┬───┘└───┬────┘
       │       │          │          │         │        │
   ┌───▼───────▼──────────▼──────────▼─────────▼────────▼─────┐
   │         Tool Executors (pkg/disk/executor/)              │
   │  lsblk | smartctl | nvme | lsscsi | sg_ses | udev       │
   └──────────────────────────────────────────────────────────┘
              │                                    │
      ┌───────▼────────┐                  ┌───────▼────────┐
      │ Config File    │                  │ State File     │
      │ (YAML)         │                  │ (JSON)         │
      │ Settings/      │                  │ Runtime/       │
      │ Preferences    │                  │ Operations     │
      └────────────────┘                  └────────────────┘
```

### Data Flow

```
[Hardware Event] → [udev] → [Hotplug Monitor] → [Manager] → [Event Bus] → [Toggle]
                                    ↓
                            [Discovery System]
                                    ↓
                            [Topology Parser]
                                    ↓
                            [Health Monitor]
                                    ↓
                             [State Update]
                                    ↓
                       [State Persistence (JSON)]
                                    ↓
                            [API Response]

[Scheduler/API] → [SMART Probe Trigger]
                         ↓
                  [Conflict Check]
                         ↓
                  [Probe Execution]
                         ↓
                  [State Update]
                         ↓
                  [Event Emission]
```

---

## Configuration Management

### Configuration File Structure

**Location**: Managed via `config/references.go`
- System: `/etc/rodent/disk-manager/config.yaml` (if running as root)
- User: `~/.rodent/disk-manager/config.yaml` (non-root)

**Full Config Schema** (`DiskManagerConfig`):
```yaml
# Disk Manager Configuration
version: "1.0"

# Discovery settings
discovery:
  enabled: true
  scanInterval: 30s              # Full discovery scan interval
  cacheTimeout: 5s               # Cache results for API calls
  maxConcurrentScans: 1          # Prevent scan storms

# Health monitoring settings
health:
  enabled: true

  smart:
    enabled: true

    # Passive monitoring (read SMART attributes)
    monitoring:
      interval: 10m              # Read SMART attributes every 10 min
      concurrentChecks: 5        # Max concurrent SMART reads

    # Active probing (run SMART self-tests)
    probing:
      enabled: true

      quick:
        enabled: true
        schedule: "0 2 * * *"    # Daily at 2 AM (cron format)
        concurrentProbes: 2      # Max 2 drives probed simultaneously
        timeout: 10m             # Max duration per probe

      extensive:
        enabled: true
        schedule: "0 3 1 * *"    # Monthly on 1st at 3 AM
        concurrentProbes: 1      # Only 1 extensive probe at a time
        timeout: 8h              # Extensive probes can take hours
        stagger: 1h              # Delay between starting probes on different drives

      offPeakHours:              # Define off-peak hours
        start: "22:00"           # 10 PM
        end: "06:00"             # 6 AM
        enforceForExtensive: true   # Only extensive probes respect off-peak
        enforceForQuick: false      # Quick probes can run anytime

  nvme:
    enabled: true
    interval: 5m                 # NVMe health check interval

  iostat:
    enabled: true
    interval: 30s                # Performance metrics interval

# Hotplug monitoring settings
hotplug:
  enabled: true
  udevMonitoring: true
  reconciliationInterval: 30s    # Periodic reconciliation
  eventBufferSize: 100           # Max buffered udev events

# Naming strategy settings
naming:
  strategy: "auto"               # auto, by-id, by-path, by-vdev
  autoThresholds:
    byIdMax: 11                  # Switch to by-path at 12 disks
    byPathMax: 24                # Switch to by-vdev at 25 disks
  vdevConfig:
    enabled: false
    path: "/etc/zfs/vdev_id.conf"
    template: "{enclosure}-{slot}"

# Performance and resource limits
performance:
  maxDeviceCache: 1000           # Max devices in memory
  discoveryTimeout: 5s           # Timeout per discovery operation
  smartTimeout: 2s               # Timeout per SMART attribute read
  topologyTimeout: 3s            # Timeout for topology parsing

# State file configuration
state:
  enabled: true
  path: ""                       # Auto-determined by config/references.go
  autoSave: true
  saveInterval: 30s              # Auto-save interval
  retainHistory: 10              # Keep last N probe results per device
  maxOperations: 100             # Max operations in state file

# Logging and events
logging:
  level: "info"                  # debug, info, warn, error
  emitEvents: true               # Emit to event bus
  eventLevels:
    - "ESSENTIAL"
    - "INFO"
    - "WARN"
    - "ERROR"
    - "CRITICAL"
    - "DEBUG"
```

### Configuration APIs

**REST Endpoints** (Base: `/api/v1/rodent/storage/config`):
```
GET    /config                        # Get current configuration
PUT    /config                        # Update configuration (full replace)
PATCH  /config                        # Partial configuration update
POST   /config/reload                 # Reload from file
GET    /config/defaults               # Get default configuration
POST   /config/reset                  # Reset to defaults
POST   /config/validate               # Validate configuration
GET    /config/path                   # Get config file path
```

---

## State Management

### State File Architecture

**Purpose**: Track runtime state, operations, and probe history (separate from config)

**Location**: Managed via `config/references.go`
- System: `/etc/rodent/disk-manager/state.json` (if running as root)
- User: `~/.rodent/disk-manager/state.json` (non-root)

**State Schema** (`DiskManagerState`):
```json
{
  "version": "1.0",
  "last_updated": "2025-10-08T14:32:15Z",

  "devices": {
    "sda": {
      "device_name": "sda",
      "last_seen": "2025-10-08T14:32:00Z",
      "state": "healthy",
      "health_status": "healthy",
      "last_quick_probe": "2025-10-08T02:05:23Z",
      "last_extensive_probe": "2025-10-01T03:15:42Z",
      "probe_results": [
        {
          "probe_type": "quick",
          "status": "completed_without_error",
          "timestamp": "2025-10-08T02:05:23Z",
          "duration_seconds": 120,
          "completion_percentage": 100
        }
      ],
      "error_count": 0
    }
  },

  "operations": {
    "probe-quick-20251008-020000": {
      "operation_id": "probe-quick-20251008-020000",
      "type": "smart_quick_probe",
      "status": "completed",
      "started_at": "2025-10-08T02:00:00Z",
      "completed_at": "2025-10-08T02:25:45Z",
      "progress": 100,
      "devices_total": 12,
      "devices_done": 12
    }
  },

  "probe_schedule": {
    "next_quick_probe": "2025-10-09T02:00:00Z",
    "next_extensive_probe": "2025-11-01T03:00:00Z",
    "quick_probe_cron": "0 2 * * *",
    "extensive_probe_cron": "0 3 1 * *",
    "last_quick_probe": "2025-10-08T02:00:00Z",
    "last_extensive_probe": "2025-10-01T03:00:00Z"
  },

  "last_discovery": "2025-10-08T14:30:00Z",
  "discovery_count": 142
}
```

### State APIs

**REST Endpoints** (Base: `/api/v1/rodent/storage/state`):
```
GET    /state                         # Get entire state
GET    /state/devices                 # All device states
GET    /state/devices/:device         # Specific device state
GET    /state/devices/:device/probes  # Probe history for device
GET    /state/operations              # All operations
GET    /state/operations/active       # Active operations only
GET    /state/operations/:opId        # Specific operation status
GET    /state/schedule                # Probe schedule state
POST   /state/save                    # Force save state file
```

---

## SMART Probe Scheduling

### Probe Strategy

**Terminology**:
- **Quick Probe**: Previously "short test" - quick electrical/component check (2-10 min)
- **Extensive Probe**: Previously "long test" - full surface scan (2-8 hours)

**Recommended Default Schedule** (Enterprise Best Practices):

| Probe Type | Schedule | Frequency | Duration | Purpose |
|------------|----------|-----------|----------|---------|
| **Quick Probe** | `0 2 * * *` | Daily at 2 AM | 2-10 min | Quick electrical/component check |
| **Extensive Probe** | `0 3 1 * *` | Monthly on 1st at 3 AM | 2-8 hours | Full surface scan |

**Alternative Schedules**:
```yaml
# Conservative (weekly quick probes)
quick:
  schedule: "0 2 * * 0"    # Weekly on Sunday at 2 AM

# Quarterly extensive probes (very conservative)
extensive:
  schedule: "0 3 1 */3 *"  # Every 3 months on 1st at 3 AM
```

### Conflict Prevention

**Conflict Types**:
```go
type ConflictReason int
const (
    ConflictNone ConflictReason = iota
    ConflictProbeRunning        // Device already has active SMART probe
    ConflictZFSResilver         // Device under ZFS resilver
    ConflictZFSScrub            // Device under ZFS scrub
    ConflictConcurrencyLimit    // Max concurrent probes reached
    ConflictOffPeakRequired     // Off-peak hours enforced
    ConflictDeviceNotFound      // Device not found
    ConflictDeviceRemoving      // Device being removed
)
```

### Probe Execution Flow

```
┌─────────────────┐
│ Trigger Source  │ (Scheduler / API / CLI)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Create Operation│ (Generate operation ID)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Conflict Check  │ (For each device)
└────────┬────────┘
         │
         ├─── Can Run ───┐
         │               ▼
         │       ┌───────────────┐
         │       │ Queue Probe   │
         │       └───────┬───────┘
         │               │
         │               ▼
         │       ┌───────────────┐
         │       │Start smartctl │ (smartctl -t short /dev/sda)
         │       │  -t <type>    │
         │       └───────┬───────┘
         │               │
         │               ▼
         │       ┌───────────────┐
         │       │ Poll Progress │ (Every 30s: smartctl -a /dev/sda)
         │       └───────┬───────┘
         │               │
         │               ▼
         │       ┌───────────────┐
         │       │Update State   │ (Progress, ETA)
         │       └───────┬───────┘
         │               │
         │               ▼
         │       ┌───────────────┐
         │       │Probe Completes│
         │       └───────┬───────┘
         │               │
         │               ▼
         │       ┌───────────────┐
         │       │ Parse Results │ (smartctl -l selftest)
         │       └───────┬───────┘
         │               │
         │               ▼
         │       ┌───────────────┐
         │       │ Evaluate      │ (PASS/FAIL)
         │       └───────┬───────┘
         │               │
         │               ▼
         │       ┌───────────────┐
         │       │Record to State│ (Probe history)
         │       └───────┬───────┘
         │               │
         │               ▼
         │       ┌───────────────┐
         │       │ Emit Event    │ (If failed/warning)
         │       └───────┬───────┘
         │               │
         └───────────────┴───────┐
                         ▼
                 ┌───────────────┐
                 │Mark Complete  │ (Update operation)
                 └───────┬───────┘
                         │
                         ▼
                 ┌───────────────┐
                 │  Save State   │
                 └───────────────┘
```

### Manual Probe Trigger APIs

**REST Endpoints** (Base: `/api/v1/rodent/storage/probes`):
```
POST   /probes/quick                  # Trigger quick probe (all devices)
POST   /probes/extensive              # Trigger extensive probe (all devices)
POST   /probes/:device/quick          # Probe specific device (quick)
POST   /probes/:device/extensive      # Probe specific device (extensive)
GET    /probes/status                 # All active probes
GET    /probes/:device/status         # Active probe for device
GET    /probes/:device/history        # Probe history for device
DELETE /probes/:device                # Cancel running probe
GET    /probes/schedule               # Get probe schedule
PUT    /probes/schedule               # Update probe schedule (updates config)
```

**Request/Response Examples**:

**Trigger Quick Probe (Batch)**:
```bash
POST /api/v1/rodent/storage/probes/quick
Content-Type: application/json

{
  "devices": ["sda", "sdb", "sdc"],  # Optional: specific devices, else all
  "force": false                      # Skip conflict checks if true
}

Response 202 Accepted:
{
  "success": true,
  "result": {
    "operation_id": "probe-quick-manual-20251008-143052",
    "status": "pending",
    "devices_total": 12,
    "devices_queued": 10,
    "devices_skipped": 2,
    "skipped_details": {
      "sde": {
        "reason": "probe_already_running",
        "message": "Device has active SMART probe"
      },
      "sdf": {
        "reason": "zfs_resilver",
        "message": "Device under ZFS resilver"
      }
    },
    "estimated_completion": "2025-10-08T14:45:00Z"
  }
}
```

---

## Package Structure

### Complete Package Hierarchy

```
pkg/
├── disk/
│   ├── types/                    # Core types (avoid cyclic imports)
│   │   ├── device.go            # PhysicalDisk, DiskState, HealthStatus
│   │   ├── topology.go          # PhysicalTopology, Controller, Enclosure
│   │   ├── smart.go             # SMARTInfo, SMARTAttribute, NVMeHealth
│   │   ├── config.go            # DiskManagerConfig, MonitoringConfig
│   │   ├── state.go             # State types
│   │   ├── probing.go           # Probe-related types
│   │   └── constants.go         # Enums, defaults, thresholds
│   │
│   ├── config/                   # Configuration management
│   │   ├── manager.go           # Config loading/saving/validation
│   │   ├── defaults.go          # Default configuration values
│   │   ├── validation.go        # Configuration validation logic
│   │   └── persistence.go       # YAML serialization/deserialization
│   │
│   ├── state/                    # State file management
│   │   ├── manager.go           # State manager (load/save/update)
│   │   ├── persistence.go       # JSON serialization (atomic writes)
│   │   ├── migration.go         # State version migration
│   │   └── query.go             # State query helpers
│   │
│   ├── operations/               # Operation tracking
│   │   ├── manager.go           # Operation lifecycle management
│   │   ├── tracker.go           # Active operation tracking
│   │   └── progress.go          # Progress calculation/updates
│   │
│   ├── probing/                  # SMART probe scheduling & execution
│   │   ├── scheduler.go         # Cron-based probe scheduler
│   │   ├── executor.go          # Probe execution logic
│   │   ├── tracker.go           # Active probe tracking
│   │   ├── conflict.go          # Conflict detection & prevention
│   │   ├── parser.go            # SMART probe result parsing
│   │   └── stagger.go           # Probe staggering logic
│   │
│   ├── manager.go               # Core manager (NO API handlers)
│   │
│   ├── discovery/               # Device enumeration & correlation
│   │   ├── discoverer.go        # Main discovery orchestrator
│   │   ├── lsblk.go            # lsblk JSON parser
│   │   ├── sysfs.go            # Direct /sys parsing (fallback)
│   │   ├── correlator.go       # Path correlation (/dev/disk/by-*)
│   │   └── cache.go            # Discovery result caching
│   │
│   ├── topology/                # Physical topology detection
│   │   ├── parser.go           # Main topology orchestrator
│   │   ├── sas.go              # SAS enclosure via lsscsi/sg_ses
│   │   ├── pci.go              # PCI topology via sysfs
│   │   ├── nvme.go             # NVMe namespace topology
│   │   ├── scsi.go             # SCSI HCTL parsing
│   │   ├── enclosure.go        # Enclosure abstraction
│   │   └── raid.go             # RAID controller detection (warn if found)
│   │
│   ├── health/                  # Multi-protocol health monitoring
│   │   ├── monitor.go          # Periodic health checker
│   │   ├── smart.go            # SMART attribute monitoring (passive)
│   │   ├── nvme_health.go      # NVMe health log parsing
│   │   ├── iostat.go           # Performance metrics via iostat
│   │   ├── evaluator.go        # Health status evaluation
│   │   └── thresholds.go       # Configurable thresholds
│   │
│   ├── hotplug/                 # Real-time device monitoring
│   │   ├── monitor.go          # udev event monitor
│   │   ├── reconciler.go       # Periodic reconciliation loop
│   │   ├── state_machine.go    # Device state transitions
│   │   └── handler.go          # Event handler logic
│   │
│   ├── naming/                  # Device naming strategies
│   │   ├── strategy.go         # Strategy selector (by-id/by-path/by-vdev)
│   │   ├── vdev_conf.go        # vdev_id.conf generator
│   │   ├── resolver.go         # Name resolution & validation
│   │   └── templates.go        # Naming templates for enclosures
│   │
│   ├── executor/                # Command execution wrappers
│   │   ├── smartctl.go         # smartctl wrapper (monitor + probe)
│   │   ├── nvme_cli.go         # nvme-cli wrapper
│   │   ├── lsblk.go            # lsblk wrapper
│   │   ├── lsscsi.go           # lsscsi wrapper
│   │   ├── sg_ses.go           # sg_ses wrapper (LED control)
│   │   ├── iostat.go           # iostat wrapper
│   │   └── base.go             # Common executor logic
│   │
│   ├── zfs/                     # ZFS integration helpers
│   │   ├── integration.go      # ZFS pool manager integration
│   │   ├── validation.go       # Device validation for pools
│   │   ├── layout.go           # Pool layout suggestions
│   │   └── redundancy.go       # Redundancy planning
│   │
│   └── api/                     # REST & gRPC handlers (SEPARATE)
│       ├── handler.go           # REST HTTP handlers (devices, health)
│       ├── handler_grpc.go      # gRPC command handlers
│       ├── routes.go            # REST route registration
│       ├── routes_grpc.go       # gRPC command registration
│       ├── middleware.go        # API middleware
│       └── types.go             # API request/response types
│
├── errors/
│   └── disk.go                  # Disk-specific error codes (2300-2399)
│
└── system/                      # Existing system package (reuse)
    └── ...

internal/
└── disk/
    └── tools/
        ├── availability.go      # Tool availability checks
        ├── version.go           # Tool version detection
        └── fallback.go          # Graceful degradation logic

config/
└── references.go                # Add GetDiskManagerConfigDir() + state paths
```

---

## Implementation Status

### Phase 1: Foundation ✅
- [x] Architecture documentation
- [x] Core types package (`pkg/disk/types/`) - 8 files
- [x] Configuration types and defaults
- [x] State types
- [x] Error codes (`pkg/errors/disk.go`) - 2300-2399 range
- [x] Tool availability checker (`pkg/disk/tools/tools.go`)
- [x] Base command executors (smartctl, lsblk, lsscsi, udevadm, sg_ses)
- [x] Config manager with YAML persistence
- [x] State manager with JSON persistence with debouncing

### Phase 2: Discovery ✅
- [x] lsblk JSON wrapper and parser
- [x] smartctl JSON parser
- [x] udevadm integration for device correlation
- [x] Discovery system with caching (`pkg/disk/discovery/`)
- [x] Device enrichment (udev + SMART)

### Phase 3: Topology ✅
- [x] SCSI/SAS topology via lsscsi
- [x] NVMe topology extraction
- [x] Enclosure detection via sg_ses (basic)
- [x] Controller mapping
- [ ] Complete SES parsing (fans, PSUs, slot mapping) - TODO in code

### Phase 4: Health Monitoring ✅
- [x] smartctl executor for attribute reading (JSON format)
- [x] SMART attribute parser (ATA)
- [x] NVMe health log parser
- [x] Health evaluator with configurable thresholds
- [x] Concurrent health checking (`pkg/disk/health/`)
- [x] Health status caching

### Phase 5: SMART Probing (Active) ✅
- [x] smartctl probe executor (quick/extensive) - `pkg/disk/probing/executor.go`
- [x] Probe result parser - Integrated in executor
- [x] Probe tracker (active probes) - In scheduler with activeProbes map
- [x] Conflict checker (ZFS scrub/resilver awareness) - `pkg/disk/probing/conflicts.go`
- [x] ZFS pool manager integration - `pkg/zfs/pool/pool.go` (scrub/resilver detection, vdev tree parsing)
- [x] Probe scheduler with gocron - `pkg/disk/probing/scheduler.go`
- [x] Device resolver interface - Clean separation for filter→device resolution
- [x] Manual probe trigger API - `TriggerProbe()` with concurrency + conflict handling
- [x] Scheduled probe execution - `executeScheduledProbe()` with filter resolution
- [x] Cron expression validation - Config validation with gocron parser
- [ ] Probe staggering logic - Infrastructure ready, not yet implemented
- [ ] Off-peak hour enforcement - Infrastructure ready, not yet implemented

**Key Implementations**:
- ZFS-aware conflict detection (prevents probes during scrub/resilver)
- DeviceResolver pattern for clean Manager→Scheduler communication
- Recursive vdev tree traversal for pool membership detection
- Graceful conflict handling (continue with non-conflicting devices)
- Comprehensive event emission (started, progress, completed, conflict)

### Phase 6: State Management ✅
- [x] State file manager with JSON persistence
- [x] Device state tracking
- [x] Operation tracking
- [x] Probe history management
- [x] Debounced auto-save mechanism
- [x] ProbeExecution helper methods (Start, Complete, Fail, Cancel, Timeout)
- [ ] State query APIs (pending API implementation)

### Phase 7: Hotplug
- [ ] udev monitor integration
- [ ] State machine implementation
- [ ] Configurable reconciliation loop
- [ ] Event correlation logic

### Phase 8: Naming Strategy
- [ ] Strategy selector with config override
- [ ] vdev_id.conf generator
- [ ] Name resolver
- [ ] ZFS pool manager integration helpers

### Phase 9: Manager Core ✅
- [x] Manager initialization with config + state - `pkg/disk/manager.go`
- [x] Inventory management - Device cache with GetInventory/GetDisk
- [x] Periodic tasks orchestration - Discovery + health checks via gocron
- [x] Event emission - Disk events via event bus
- [x] Graceful shutdown - Context cancellation + WaitGroup
- [x] Device resolution for probe scheduling - ResolveDevices() implementation
- [ ] Runtime reconfiguration - Config reload not yet implemented

### Phase 10: Configuration Management
- [ ] Config manager implementation
- [ ] YAML persistence
- [ ] Validation logic
- [ ] Config API handlers (REST + gRPC)
- [ ] Default config generation
- [ ] Integration with config/references.go

### Phase 11: API Layer (In Progress) 🔨
- [x] Proto command type constants - `toggle-rodent-proto/proto/disk_command_types.go`
- [x] Proto event definitions - `toggle-rodent-proto/proto/events/event_messages.proto` (StorageDisk*)
- [x] Handler structure - `pkg/disk/api/handler_grpc.go`
- [x] gRPC routes registration - `pkg/disk/api/routes_grpc.go`
- [x] REST handler base - `pkg/disk/api/handler.go`
- [x] Server integration - `pkg/server/routes.go` registerDiskRoutes()
- [x] Event bus graceful handling - nil-safe emission in `pkg/disk/events/emitter.go`
- [x] API base path constant - `constants.APIDisk = "/api/v1/rodent/disk"`
- [ ] Complete all gRPC command handlers (7/63 implemented)
- [ ] Complete all REST endpoints (6/30+ implemented)
- [ ] API documentation

**Implemented Commands** (7):
- Inventory: list, get, discover, refresh
- Health: health.get, smart.get, smart.refresh

**TODO Commands** (56):
- Probe operations (7): start, cancel, get, list, history
- Probe schedules (7): list, get, create, update, delete, enable, disable
- Topology (5): get, refresh, controllers, enclosures, fault-domains
- State management (3): get, set, quarantine
- Configuration (3): get, update, reload
- Naming strategy (2): get, set
- Metadata (3): tags.set, tags.delete, notes.set
- Statistics (2): stats.get, stats.global
- Advanced (18+): validation, ZFS helpers, vdev suggestions, etc.

### Phase 12: ZFS Integration
- [ ] Pool creation helpers
- [ ] Device suggestion API
- [ ] Redundancy planner
- [ ] vdev layout optimizer
- [ ] Naming strategy integration
- [ ] ZFS operation awareness (resilver/scrub)

---

## Design Decisions

### 1. Health Monitoring: Passive vs Active

**Decision**: Separate passive monitoring from active probing

**Passive Monitoring** (Read SMART Attributes):
- Interval: 10 minutes (default)
- Impact: Minimal I/O overhead
- Purpose: Track attribute trends (reallocated sectors, temperature, etc.)

**Active Probing** (Run SMART Self-Tests):
- Quick: Daily at 2 AM (default)
- Extensive: Monthly at 3 AM (default)
- Impact: Moderate I/O, delays user operations
- Purpose: Proactive failure detection via surface scans

**Terminology**: Renamed "short/long tests" to "quick/extensive probes" to avoid confusion with software testing terminology.

### 2. HBA Mode Assumptions (NOT Hardware RAID)

**Decision**: Assume HBA in IT mode (passthrough), detect and warn if RAID detected

**Rationale**:
- ZFS requires direct disk access for checksumming and redundancy
- Modern ZFS deployments use:
  - **HBA IT Mode**: LSI 9300/9400 series in IT firmware
  - **Direct PCIe**: NVMe drives
  - **Software HBA**: virtio-scsi for VMs

**Fault Domains** (Physical, no RAID controllers):
- Controller PCI slot
- SAS enclosure ID
- Power domain (if detectable via IPMI)

### 3. Hotplug Detection: Hybrid Approach

**Decision**: udev events + periodic reconciliation (configurable interval)

**Rationale**:
- **udev events**: Real-time but can be missed during high load
- **Reconciliation**: Catches missed events, handles race conditions
- **Best of both**: <1s latency for events, 30s worst-case (configurable)

**State Machine**:
```
UNKNOWN → DISCOVERED → IDENTIFYING → HEALTHY ⇄ DEGRADED → FAILED → REMOVED
                          ↓
                    [SMART Check]
                          ↓
                    [Health Evaluation]
```

### 4. Device Naming Strategy

**Decision**: Auto-select based on disk count + manual override via API/config

**Strategy Selection**:
| Disk Count | Default Strategy | Reasoning |
|------------|------------------|-----------|
| 1-11       | `/dev/disk/by-id/` | Simple, persistent, human-readable |
| 12-24      | `/dev/disk/by-path/` | Topology-aware for redundancy |
| 25+        | `/dev/disk/by-vdev/` | Custom naming with `vdev_id.conf` |

### 5. Configuration vs State: Separation of Concerns

**Decision**: Separate configuration (preferences) from state (runtime data)

| Aspect | Configuration (YAML) | State (JSON) |
|--------|---------------------|--------------|
| **Purpose** | User preferences, settings | Runtime data, operation tracking |
| **Persistence** | Manual save on updates | Auto-save every 30s |
| **Examples** | Probe schedules, intervals, thresholds | Active probes, probe history, device states |
| **Modification** | Via API or manual file edit | Automatically by system |
| **Version Control** | Suitable for git | Not suitable (frequently changing) |

### 6. Event System Integration

**Decision**: Use existing event bus via `SendEvents` RPC, no new gRPC services needed

**Pattern** (from internal/events/schema.go):
1. Create structured event with `eventspb.Event`
2. Set appropriate `EventPayload` (oneof: system_event, storage_event, etc.)
3. Emit via `emitStructuredEvent(event)` which calls `globalEventBus.EmitStructuredEvent(event)`
4. Event bus batches and sends via `SendEvents` RPC

**Event Levels**: Support ESSENTIAL and DEBUG in addition to INFO/WARN/ERROR/CRITICAL

---

## API Specification

### Base Path
All disk management APIs use base path: `/api/v1/rodent/storage/`

### REST Endpoints

#### Disk Management
```
GET    /disks                         # List all disks
GET    /disks/:device                 # Get disk details
GET    /disks/:device/smart           # Get SMART attributes (passive)
GET    /disks/:device/topology        # Get physical topology
POST   /disks/discover                # Trigger discovery
GET    /health                        # Health summary
GET    /topology/enclosures           # List enclosures
GET    /topology/controllers          # List controllers
```

#### Configuration Management
```
GET    /config                        # Get current configuration
PUT    /config                        # Update configuration (full replace)
PATCH  /config                        # Partial configuration update
POST   /config/reload                 # Reload from file
GET    /config/defaults               # Get default configuration
POST   /config/reset                  # Reset to defaults
POST   /config/validate               # Validate configuration
GET    /config/path                   # Get config file path
```

#### State Management
```
GET    /state                         # Get entire state
GET    /state/devices                 # All device states
GET    /state/devices/:device         # Specific device state
GET    /state/devices/:device/probes  # Probe history for device
GET    /state/operations              # All operations
GET    /state/operations/active       # Active operations only
GET    /state/operations/:opId        # Specific operation status
GET    /state/schedule                # Probe schedule state
POST   /state/save                    # Force save state file
```

#### SMART Probe Management
```
POST   /probes/quick                  # Trigger quick probe (all devices)
POST   /probes/extensive              # Trigger extensive probe (all devices)
POST   /probes/:device/quick          # Probe specific device (quick)
POST   /probes/:device/extensive      # Probe specific device (extensive)
GET    /probes/status                 # All active probes
GET    /probes/:device/status         # Active probe for device
GET    /probes/:device/history        # Probe history for device
DELETE /probes/:device                # Cancel running probe
GET    /probes/schedule               # Get probe schedule
PUT    /probes/schedule               # Update probe schedule (updates config)
```

#### Naming Strategy
```
GET    /naming/strategies             # Available naming strategies
GET    /naming/current                # Current strategy in use
POST   /naming/strategy               # Change naming strategy
POST   /naming/vdev-config            # Generate vdev_id.conf
GET    /naming/vdev-config            # Get current vdev_id.conf
```

### gRPC Command Handlers

**Pattern** (from pkg/system/api/routes_grpc.go):
```go
// Register command handlers with Toggle client
func RegisterDiskGRPCHandlers(diskHandler *DiskHandler) {
    // Disk management operations
    client.RegisterCommandHandler(proto.CmdDiskList, handleDiskList(diskHandler))
    client.RegisterCommandHandler(proto.CmdDiskGet, handleDiskGet(diskHandler))
    client.RegisterCommandHandler(proto.CmdDiskDiscover, handleDiskDiscover(diskHandler))

    // Probe operations
    client.RegisterCommandHandler(proto.CmdDiskProbeQuick, handleProbeQuick(diskHandler))
    client.RegisterCommandHandler(proto.CmdDiskProbeExtensive, handleProbeExtensive(diskHandler))
    client.RegisterCommandHandler(proto.CmdDiskProbeStatus, handleProbeStatus(diskHandler))
    client.RegisterCommandHandler(proto.CmdDiskProbeCancel, handleProbeCancel(diskHandler))

    // Configuration operations
    client.RegisterCommandHandler(proto.CmdDiskConfigGet, handleConfigGet(diskHandler))
    client.RegisterCommandHandler(proto.CmdDiskConfigUpdate, handleConfigUpdate(diskHandler))

    // State operations
    client.RegisterCommandHandler(proto.CmdDiskStateGet, handleStateGet(diskHandler))
    client.RegisterCommandHandler(proto.CmdDiskStateDeviceGet, handleStateDeviceGet(diskHandler))
}
```

**Command Type Constants** (to add to proto/commands.go):
```go
// Disk Management Commands
const (
    CmdDiskList                  = "disk.list"
    CmdDiskGet                   = "disk.get"
    CmdDiskDiscover              = "disk.discover"
    CmdDiskHealthGet             = "disk.health.get"

    // Probe Commands
    CmdDiskProbeQuick            = "disk.probe.quick"
    CmdDiskProbeExtensive        = "disk.probe.extensive"
    CmdDiskProbeStatus           = "disk.probe.status"
    CmdDiskProbeHistory          = "disk.probe.history"
    CmdDiskProbeCancel           = "disk.probe.cancel"

    // Config Commands
    CmdDiskConfigGet             = "disk.config.get"
    CmdDiskConfigUpdate          = "disk.config.update"
    CmdDiskConfigValidate        = "disk.config.validate"

    // State Commands
    CmdDiskStateGet              = "disk.state.get"
    CmdDiskStateDeviceGet        = "disk.state.device.get"
    CmdDiskStateOperationsGet    = "disk.state.operations.get"
)
```

---

## Event Schema

### Proto Definitions (to add to event_messages.proto)

```protobuf
// Add to StorageEvent wrapper
message StorageEvent {
  oneof event_type {
    StoragePoolPayload pool_event = 1;
    StorageDatasetPayload dataset_event = 2;
    StorageSnapshotPayload snapshot_event = 4;
    StorageDiskPayload disk_event = 5;        // NEW
    StorageDiskProbePayload disk_probe_event = 6;  // NEW
  }
}

// Disk event payload
message StorageDiskPayload {
  // Identity
  string device_name = 1;              // sda, nvme0n1
  string device_path = 2;              // /dev/sda
  repeated string by_id = 3;           // /dev/disk/by-id/* symlinks
  repeated string by_path = 4;         // /dev/disk/by-path/* symlinks
  string by_vdev = 5;                  // Optional custom vdev name
  string serial = 6;
  string wwn = 7;
  string model = 8;
  string vendor = 9;

  // Capacity
  int64 capacity_bytes = 10;
  int32 block_size = 11;
  string transport = 12;               // sata, sas, nvme, usb

  // Physical location
  PhysicalLocation location = 13;

  // Health
  string health_status = 14;           // healthy, warning, degraded, failing
  int32 smart_status = 15;             // 0=PASSED, 1=FAILED
  int32 temperature_celsius = 16;

  // State
  string state = 17;                   // discovered, healthy, degraded, failed, removed
  StorageDiskOperation operation = 18;

  message PhysicalLocation {
    string controller = 1;             // pci-0000:05:00.0
    string enclosure_id = 2;           // SAS address or enclosure serial
    int32 slot = 3;                    // Physical slot number
    string bay_label = 4;              // "Front Bay 12"
    string scsi_host = 5;              // host3
    int32 scsi_channel = 6;
    int32 scsi_target = 7;
    int32 scsi_lun = 8;
  }

  enum StorageDiskOperation {
    STORAGE_DISK_OPERATION_UNSPECIFIED = 0;
    STORAGE_DISK_OPERATION_DISCOVERED = 1;
    STORAGE_DISK_OPERATION_ADDED = 2;
    STORAGE_DISK_OPERATION_REMOVED = 3;
    STORAGE_DISK_OPERATION_HEALTH_CHANGED = 4;
    STORAGE_DISK_OPERATION_FAILED = 5;
    STORAGE_DISK_OPERATION_SMART_CHECK = 6;
  }
}

// SMART probe event payload
message StorageDiskProbePayload {
  string device_name = 1;
  string probe_type = 2;               // quick, extensive
  string status = 3;                   // started, running, completed, failed, cancelled
  string result = 4;                   // completed_without_error, aborted_by_host, etc.
  int32 progress = 5;                  // 0-100
  int64 eta_seconds = 6;
  int64 duration_seconds = 7;
  string operation_id = 8;

  StorageDiskProbeOperation operation = 9;

  enum StorageDiskProbeOperation {
    STORAGE_DISK_PROBE_OPERATION_UNSPECIFIED = 0;
    STORAGE_DISK_PROBE_OPERATION_STARTED = 1;
    STORAGE_DISK_PROBE_OPERATION_PROGRESS = 2;   // Optional: emit progress updates
    STORAGE_DISK_PROBE_OPERATION_COMPLETED = 3;
    STORAGE_DISK_PROBE_OPERATION_FAILED = 4;
    STORAGE_DISK_PROBE_OPERATION_CANCELLED = 5;
  }
}
```

### Event Emission Examples

**Disk Discovery Event** (in pkg/disk/manager.go):
```go
func (m *Manager) emitDiskEvent(
    level eventspb.EventLevel,
    device *types.PhysicalDisk,
    operation StorageDiskOperation,
) {
    if !m.config.Logging.EmitEvents {
        return
    }

    event := &eventspb.Event{
        EventId:   generateEventID(),
        Level:     level,
        Category:  eventspb.EventCategory_EVENT_CATEGORY_STORAGE,
        Source:    "disk-manager",
        Timestamp: time.Now().UnixMilli(),
        Metadata:  make(map[string]string),
        EventPayload: &eventspb.Event_StorageEvent{
            StorageEvent: &eventspb.StorageEvent{
                EventType: &eventspb.StorageEvent_DiskEvent{
                    DiskEvent: m.toDiskPayload(device, operation),
                },
            },
        },
    }

    // Use internal/events pattern
    emitStructuredEvent(event)  // Calls globalEventBus.EmitStructuredEvent(event)
}
```

**SMART Probe Event**:
```go
func (m *Manager) emitProbeEvent(
    level eventspb.EventLevel,
    device string,
    probeType ProbeType,
    status string,
    result *ProbeResult,
    operation StorageDiskProbeOperation,
) {
    // Similar pattern to above
}
```

---

## Integration Points

### 1. Configuration System Integration

**Update config/references.go**:
```go
var (
    diskManagerConfigDir string
    diskManagerStateDir  string
)

func init() {
    // ... existing code ...
    diskManagerConfigDir = filepath.Join(configDir, "disk-manager")
    diskManagerStateDir = filepath.Join(configDir, "disk-manager")
}

func GetDiskManagerConfigDir() string {
    return diskManagerConfigDir
}

func GetDiskManagerStateDir() string {
    return diskManagerStateDir
}

func EnsureDirectories() error {
    dirs := []string{
        // ... existing dirs ...
        diskManagerConfigDir,
        diskManagerStateDir,
    }
    // ... rest of function ...
}
```

**File Paths**:
- Config: `/etc/rodent/disk-manager/config.yaml`
- State: `/etc/rodent/disk-manager/state.json`

### 2. ZFS Pool Manager Integration

**Helper Functions** (`pkg/disk/zfs/integration.go`):
```go
// SuggestPoolLayout suggests optimal device layout for pool creation
func (m *Manager) SuggestPoolLayout(
    ctx context.Context,
    diskCount int,
    redundancy string, // mirror, raidz1, raidz2, raidz3
) ([]VDevGroup, error)

// ValidateVDevDevices checks if devices are suitable for vdev
func (m *Manager) ValidateVDevDevices(
    ctx context.Context,
    devices []string,
) ([]ValidationIssue, error)

// GetDevicesByNamingStrategy returns device paths using active strategy
func (m *Manager) GetDevicesByNamingStrategy(
    ctx context.Context,
) (map[string]string, error) // device_name -> path

// IsDeviceResilivering checks if device is under ZFS resilver
func (m *Manager) IsDeviceResilivering(device string) (bool, error)

// IsDeviceScrubbing checks if device is under ZFS scrub
func (m *Manager) IsDeviceScrubbing(device string) (bool, error)
```

### 3. Event System Integration

**Pattern** (following internal/events/schema.go):
- Use `emitStructuredEvent(event)` helper
- Relies on global event bus initialized in `internal/events/integration.go`
- Events batched and sent via existing `SendEvents` RPC
- No new gRPC service definitions needed

### 4. System Package Reuse

**Use existing `pkg/system/` for**:
- System information collection (OS, kernel version)
- Platform detection (cloud, VM, physical)
- Privilege checking (sudo requirements)

---

## Security Considerations

1. **Command Injection Prevention**: Use existing `pkg/zfs/command` pattern
2. **Privilege Escalation**: Minimal sudo usage, validate all paths
3. **Path Traversal**: Validate all device paths against `/dev/` and `/sys/`
4. **Resource Limits**: Bounded goroutines, memory limits (configurable)
5. **Input Validation**: Sanitize all API inputs (config updates, device names, cron expressions)
6. **Config File Permissions**: 0644 for config/state files, 0755 for directories
7. **API Authentication**: Leverage existing middleware for auth checks
8. **Process Isolation**: Run SMART probes as separate processes (killable)
9. **Force Override Protection**: Log all forced operations for audit trail

---

## Performance Targets

- **Discovery**: <5s for 100 disks (cached: <100ms)
- **SMART Attribute Read**: <2s per disk (staggered, max 5 concurrent)
- **SMART Quick Probe**: 2-10 minutes per disk (vendor-specific)
- **SMART Extensive Probe**: 2-8 hours per disk (vendor-specific)
- **Hotplug Detection**: <1s event latency
- **API Response**: <200ms for list operations (from cache)
- **Config Reload**: <500ms
- **State Save**: <100ms (atomic write)
- **State Load**: <200ms on startup
- **Memory**: <10MB baseline + 100KB per disk + 1KB per operation
- **CPU**: <5% average on idle system, <15% during full probe batch

---

## Operational Considerations

### Disk Replacement Workflow
1. User detects failing disk (via SMART probe failure event)
2. User physically replaces disk (hotplug detected)
3. Disk manager emits `STORAGE_DISK_OPERATION_REMOVED` + `STORAGE_DISK_OPERATION_ADDED`
4. User creates new vdev or replaces in existing pool (via ZFS API)
5. ZFS resilver starts (disk manager detects, avoids SMART probes)
6. Resilver completes, disk manager resumes SMART probing

### Schedule Adjustment Best Practices
- **Quick probes**: Can run during business hours if needed (minimal impact)
- **Extensive probes**: Always schedule during off-peak hours (2-8 hour duration)
- **Staggering**: For 50+ drives, consider 2-4 hour stagger intervals
- **Cloud instances**: Be aware of IOPS limits, may need longer stagger

---

---

**Document Maintainers**: Rodent Development Team
**Last Updated**: 2025-10-10
**Review Cycle**: After each phase completion
**Feedback**: Submit issues to project tracker
