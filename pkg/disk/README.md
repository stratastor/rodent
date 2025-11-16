# Disk Management System

Production-ready disk management for ZFS storage servers supporting 1-100+ disks across cloud, VM, and physical platforms.

## Quick Start

```go
// Initialize disk manager
manager, err := disk.NewManager(logger, executor, eventBus)
if err != nil {
    return err
}

// Start manager
ctx := context.Background()
if err := manager.Start(ctx); err != nil {
    return err
}
defer manager.Stop(ctx)

// Get available disks for pool creation
filter := &types.DiskFilter{
    States: []types.DiskState{types.DiskStateAvailable},
}
disks := manager.GetInventory(filter)
```

## Core Features

- **Auto Discovery**: Automatic device enumeration with udev correlation
- **Health Monitoring**: SMART/NVMe health tracking with configurable intervals
- **SMART Probing**: Scheduled and on-demand self-tests with ZFS-aware conflict detection
- **Hotplug Detection**: Real-time device add/remove via netlink (~1ms latency)
- **Topology Mapping**: Controller, enclosure, and fault domain detection
- **State Persistence**: JSON state file with automatic debounced saves
- **Dual API**: REST and gRPC endpoints with 1:1 parity

## Architecture

```text
┌─────────────────────────────────────────────┐
│          API Layer (REST/gRPC)              │
│    Base: /api/v1/rodent/disk/               │
└────────────────┬────────────────────────────┘
                 ↓
┌────────────────────────────────────────────┐
│         Disk Manager (manager.go)           │
│  • Device inventory & state machine         │
│  • Event coordination & periodic tasks      │
│  • Config & state persistence               │
└──┬─────┬──────┬────────┬────────┬────┬─────┘
   ↓     ↓      ↓        ↓        ↓    ↓
Discovery Health Hotplug Probing  State Config
   ↓     ↓      ↓        ↓        ↓    ↓
Tool Executors (lsblk, smartctl, udevadm, etc)
   ↓                                   ↓
YAML Config ← ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ → JSON State
```

### Package Structure

```text
pkg/disk/
├── manager.go              # Core orchestrator
├── types/                  # Core types (device, topology, SMART, config, state)
├── config/                 # YAML config management
├── state/                  # JSON state persistence
├── discovery/              # Device enumeration with udev
├── topology/               # Physical topology detection
├── health/                 # SMART/NVMe health monitoring
├── hotplug/                # Real-time netlink monitoring
├── probing/                # SMART probe scheduling & execution
├── tools/                  # Command executors (lsblk, smartctl, etc)
└── api/                    # REST & gRPC handlers
```

## Configuration

**Location**: `~/.rodent/disk/disk-manager.yaml` (user) or `/etc/rodent/disk/disk-manager.yaml` (system)

```yaml
version: "1.0"

discovery:
  enabled: true
  scanInterval: 30s

health:
  enabled: true
  smart:
    enabled: true
    monitoring:
      interval: 10m              # Read SMART attributes
    probing:
      enabled: true
      quick:
        schedule: "0 2 * * *"    # Daily at 2 AM
        concurrentProbes: 2
        timeout: 10m
      extensive:
        schedule: "0 3 1 * *"    # Monthly on 1st at 3 AM
        concurrentProbes: 1
        timeout: 8h
      offPeakHours:
        start: "22:00"
        end: "06:00"
        enforceForExtensive: true

hotplug:
  enabled: true
  udevMonitoring: true
  reconciliationInterval: 30s

state:
  enabled: true
  autoSave: true
  saveInterval: 30s
  retainHistory: 10              # Probe results per device
```

## State File

**Location**: `~/.rodent/disk/disk-manager.state.json`

Automatically tracks:

- Device states and health history
- Active probe operations and progress
- Probe execution history (last 10 per device)
- Scheduled probe timing
- Discovery statistics

Auto-saves every 30s with debouncing.

## API Overview

See [API.md](API.md) for complete reference.

### REST Base Path

`/api/v1/rodent/disk`

### Common Operations

```bash
# Get available disks for pool creation
curl http://localhost:8042/api/v1/rodent/disk/available

# List all disks
curl http://localhost:8042/api/v1/rodent/disk/

# Get disk details
curl http://localhost:8042/api/v1/rodent/disk/vol123

# Trigger discovery
curl -X POST http://localhost:8042/api/v1/rodent/disk/discovery/trigger

# Start quick SMART probe
curl -X POST http://localhost:8042/api/v1/rodent/disk/probes/start \
  -H "Content-Type: application/json" \
  -d '{"probe_type": "quick", "device_ids": ["vol123"]}'

# Get health status
curl http://localhost:8042/api/v1/rodent/disk/vol123/health

# Get SMART data
curl http://localhost:8042/api/v1/rodent/disk/vol123/smart
```

### gRPC Commands

All REST endpoints have equivalent gRPC commands: `disk.{category}.{action}`

Examples: `disk.list`, `disk.list.available`, `disk.get`, `disk.probe.start`, `disk.health.get`

See [API.md](API.md) for complete list of 43 commands.

## Key Features Explained

### Device Path Selection

Discovery intelligently selects stable device paths for ZFS:

1. **Prefers serial-based by-id paths** (e.g., `/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol123`)
2. **Skips partition links** (no `-part1`, `_1` suffixes)
3. **Fallback chain**: serial path → non-partition by-id → first by-id

This ensures reliable device naming for ZFS pool creation.

### SMART Probing Strategy

**Passive Monitoring** (every 10 minutes):

- Reads SMART attributes without impacting I/O
- Tracks temperature, reallocated sectors, error counts
- Minimal system overhead

**Active Probing** (scheduled tests):

- **Quick probes**: Daily at 2 AM (2-10 min duration)
- **Extensive probes**: Monthly on 1st at 3 AM (2-8 hour duration)
- ZFS-aware: Automatically skips devices under scrub/resilver
- Conflict detection prevents concurrent operations

### Hotplug Detection

**Hybrid approach** for reliability:

- **Netlink monitor**: Real-time udev events (~1ms latency)
- **Reconciliation**: Periodic scans every 30s catch missed events
- **Result**: Sub-second detection with 30s worst-case guarantee

### Event System

Emits structured events via global event bus:

- **Disk Events**: discovered, added, removed, health_changed, failed
- **Probe Events**: started, progress, completed, failed, cancelled

Events automatically flow to Toggle via `SendEvents` RPC.

## Implementation Status

### Completed (88%)

- Core discovery with topology mapping
- Health monitoring (SMART/NVMe/iostat)
- SMART probe scheduling with ZFS-aware conflict detection
- Hotplug detection (netlink-based, <1ms latency)
- State and config management with persistence
- REST and gRPC APIs (38/43 endpoints, 88%)
- Integration test suite for all handlers
- Available disks endpoint for pool creation

### In Progress

- ZFS integration helpers (pool suggestions, redundancy planning)

### Planned

- Naming strategy (auto/by-id/by-path/by-vdev selection)
- Fault domain analysis
- Advanced wear leveling for SSDs

## Testing

### Integration Tests

Requires hardware or staging environment:

```bash
RUN_INTEGRATION_TESTS=true \
RODENT_CONFIG=/path/to/config.yml \
go test -v -count=1 -timeout=5m ./pkg/disk/api
```

Integration test covers:

- Inventory operations (list, list.available, get)
- Discovery and topology operations
- Health and SMART operations
- Probe operations (start, cancel, history)
- Configuration and state management

Location: [api/handler_grpc_integration_test.go](api/handler_grpc_integration_test.go)

## Design Decisions

### 1. Config vs State Separation

| Aspect | Config (YAML) | State (JSON) |
|--------|---------------|--------------|
| Purpose | User preferences | Runtime data |
| Persistence | Manual on updates | Auto-save 30s |
| Examples | Schedules, intervals | Active probes, history |
| Version Control | Yes | No |

### 3. ZFS-Aware Conflict Detection

Probes automatically detect and skip devices under:

- ZFS scrub operations
- ZFS resilver operations
- Existing SMART probes
- Off-peak hour violations (configurable)

### 4. Graceful Degradation

System continues operating when tools are unavailable:

- SMART unavailable → health status marked as UNKNOWN
- Topology tools missing → basic device info only
- Config file missing → uses defaults

## Documentation

- [API.md](API.md) - Complete API reference with examples
