# Disk Management API Reference

**Status**: 88% Complete (38/43 endpoints)
**Last Updated**: 2025-10-24

## Overview

All disk management APIs use base path `/api/v1/rodent/disks/` for REST and follow the pattern `disk.{category}.{action}` for gRPC commands. Each REST endpoint has a corresponding gRPC command for one-to-one parity.

## Implementation Status

| Category | Endpoints | Status |
|----------|-----------|--------|
| Inventory & Discovery | 4 | ✅ 100% |
| Health & SMART | 4 | ✅ 100% |
| Probe Operations | 5 | ✅ 100% |
| Probe Schedules | 7 | ✅ 100% |
| Topology | 4 | ✅ 100% |
| State Management | 4 | ✅ 100% |
| Configuration | 3 | ✅ 100% |
| Metadata | 3 | ✅ 100% |
| Statistics & Monitoring | 4 | ✅ 100% |
| **Naming Strategy** | 3 | ⏳ Pending Phase 8 |
| **Fault Domains** | 2 | ⏳ Pending Phase 9 |
| **Total** | **43** | **38 (88%)** |

## API Endpoints

### Inventory & Discovery

```text
GET    /                        List all disks
  ?states=AVAILABLE,ONLINE      Filter by state

GET    /available                List available disks for pool creation
GET    /:device_id               Get disk details
POST   /discovery/trigger        Trigger device discovery
POST   /refresh                  Refresh disk information

gRPC: disk.list, disk.list.available, disk.get, disk.discover, disk.refresh
```

### Health & SMART

```text
GET    /:device_id/health        Get disk health status
GET    /:device_id/smart         Get SMART data
POST   /health/check             Trigger health check for all disks
POST   /:device_id/smart/refresh Refresh SMART data for disk

gRPC: disk.health.get, disk.smart.get, disk.health.check, disk.smart.refresh
```

### Probe Operations

```text
POST   /probes/start             Start SMART probe (quick/extensive)
  Body: {
    "probe_type": "quick",      // or "extensive"
    "device_ids": ["vol123"],   // optional, defaults to all
    "force": false              // skip conflict checks
  }

GET    /probes                   List all active probes
GET    /probes/:probe_id         Get probe execution details
POST   /probes/:probe_id/cancel  Cancel running probe
GET    /:device_id/probe-history Get probe history for device
  ?limit=10                      Number of results

gRPC: disk.probe.start, disk.probe.list, disk.probe.get,
      disk.probe.cancel, disk.probe.history
```

### Probe Schedules

```text
GET    /probe-schedules          List all schedules
POST   /probe-schedules          Create new schedule
  Body: {
    "name": "daily-quick",
    "probe_type": "quick",
    "cron": "0 2 * * *",
    "enabled": true
  }

GET    /probe-schedules/:id      Get schedule details
PUT    /probe-schedules/:id      Update schedule
DELETE /probe-schedules/:id      Delete schedule
POST   /probe-schedules/:id/enable    Enable schedule
POST   /probe-schedules/:id/disable   Disable schedule

gRPC: disk.probe.schedule.list, disk.probe.schedule.get,
      disk.probe.schedule.create, disk.probe.schedule.update,
      disk.probe.schedule.delete, disk.probe.schedule.enable,
      disk.probe.schedule.disable
```

### Topology

```text
GET    /topology                 Get complete physical topology
POST   /topology/refresh         Refresh topology information
GET    /topology/controllers     List storage controllers
GET    /topology/enclosures      List disk enclosures

gRPC: disk.topology.get, disk.topology.refresh,
      disk.topology.controllers, disk.topology.enclosures
```

### State Management

```text
GET    /state                    Get complete manager state
GET    /state/devices            All device states
GET    /state/devices/:device    Specific device state
PUT    /:device_id/state         Set disk state
  Body: {
    "state": "AVAILABLE",        // or ONLINE, DEGRADED, etc.
    "reason": "Manual override"
  }

POST   /:device_id/validate      Validate disk for ZFS use
POST   /:device_id/quarantine    Quarantine disk
  Body: {
    "reason": "Suspected failure"
  }

gRPC: disk.state.get, disk.state.set, disk.validate, disk.quarantine
```

### Configuration

```text
GET    /config                   Get current configuration
PUT    /config                   Update configuration
  Body: {
    "discovery": { "scanInterval": "60s" },
    "health": { "smart": { "monitoring": { "interval": "15m" } } }
  }

POST   /config/reload            Reload config from file
GET    /config/monitoring        Get monitoring configuration
PUT    /config/monitoring        Update monitoring settings

gRPC: disk.config.get, disk.config.update, disk.config.reload,
      disk.monitoring.get, disk.monitoring.set
```

### Metadata

```text
PUT    /:device_id/tags          Set disk tags
  Body: {
    "tags": {
      "datacenter": "us-west-2",
      "tier": "performance"
    }
  }

DELETE /:device_id/tags          Delete disk tags
  Body: {
    "keys": ["datacenter", "tier"]
  }

PUT    /:device_id/notes         Set disk notes
  Body: {
    "notes": "Replaced 2025-10-24"
  }

gRPC: disk.tags.set, disk.tags.delete, disk.notes.set
```

### Statistics

```text
GET    /stats                    Get global statistics
GET    /:device_id/stats         Get device statistics

gRPC: disk.stats.global, disk.stats.get
```

### Naming Strategy (Pending Phase 8)

```text
GET    /naming-strategy          Get current naming strategy
PUT    /naming-strategy          Set naming strategy
  Body: {
    "strategy": "by-id"          // by-id, by-path, by-vdev, auto
  }

POST   /vdev-conf/generate       Generate vdev_id.conf file

gRPC: disk.naming-strategy.get, disk.naming-strategy.set,
      disk.vdev-conf.generate
```

### Fault Domains (Pending Phase 9)

```text
GET    /fault-domains/analyze    Analyze fault domains
GET    /fault-domains/:domain    Get fault domain info

gRPC: disk.fault-domains.analyze, disk.fault-domains.get
```

## Response Format

### Success Response

```json
{
  "success": true,
  "result": {
    // Response data
  },
  "timestamp": "2025-10-24T10:30:00Z"
}
```

### Error Response

```json
{
  "success": false,
  "error": {
    "code": "DISK_2301",
    "message": "Device not found",
    "details": {
      "device_id": "vol123"
    }
  },
  "timestamp": "2025-10-24T10:30:00Z"
}
```

## Common Filters

### Disk States

- `UNKNOWN` - Initial state
- `DISCOVERED` - Found but not yet identified
- `AVAILABLE` - Ready for pool creation
- `ONLINE` - Active in ZFS pool
- `DEGRADED` - Has issues but functional
- `FAULTED` - Failed, needs replacement
- `OFFLINE` - Intentionally offline
- `UNAVAILABLE` - Not accessible
- `REMOVED` - Physically removed
- `QUARANTINED` - Isolated due to suspicion
- `SYSTEM` - System disk (Boot disk and other non-pool disks in use)

### Health Status

- `HEALTHY` - All checks pass
- `WARNING` - Minor issues detected
- `DEGRADED` - Significant issues
- `FAILING` - Imminent failure predicted
- `FAILED` - Drive has failed
- `UNKNOWN` - Cannot determine

### Probe Types

- `quick` - Quick electrical check (2-10 minutes)
- `extensive` - Full surface scan (2-8 hours)

## Error Codes

Disk management uses error code range 2300-2399:

- `2300` - General disk error
- `2301` - Device not found
- `2302` - Discovery failed
- `2303` - Health check failed
- `2304` - SMART data unavailable
- `2305` - Probe failed
- `2306` - Invalid probe type
- `2307` - Probe already running
- `2308` - Probe not found
- `2309` - Config validation failed
- `2310` - State save failed
- `2320-2324` - Probe schedule errors

## Testing

All endpoints have integration tests in `api/handler_grpc_integration_test.go`.

Run tests:

```bash
RUN_INTEGRATION_TESTS=true \
RODENT_CONFIG=/path/to/config.yml \
go test -v -count=1 -timeout=5m ./pkg/disk/api
```

## Examples

### Get Available Disks for Pool Creation

```bash
# REST
curl http://localhost:8042/api/v1/rodent/disk/available

# gRPC (via Toggle protocol)
{
  "command_type": "disk.list.available",
  "payload": {}
}
```

Response:

```json
{
  "success": true,
  "result": {
    "disks": [
      {
        "device_id": "vol0fa56050d9a207d86",
        "device_path": "/dev/nvme4n1",
        "by_id_path": "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol0fa56050d9a207d86",
        "state": "AVAILABLE",
        "health": "HEALTHY",
        "size_bytes": 1073741824,
        "model": "Amazon Elastic Block Store",
        "interface": "NVME"
      }
    ],
    "count": 1
  }
}
```

### Start Quick Probe on Specific Devices

```bash
# REST
curl -X POST http://localhost:8042/api/v1/rodent/disk/probes/start \
  -H "Content-Type: application/json" \
  -d '{
    "probe_type": "quick",
    "device_ids": ["vol123", "vol456"]
  }'

# gRPC
{
  "command_type": "disk.probe.start",
  "payload": {
    "probe_type": "quick",
    "device_ids": ["vol123", "vol456"]
  }
}
```

Response:

```json
{
  "success": true,
  "result": {
    "operation_id": "probe-quick-20251024-103000",
    "status": "pending",
    "devices_queued": 2,
    "estimated_completion": "2025-10-24T10:40:00Z"
  }
}
```

### Update Configuration

```bash
# REST
curl -X PUT http://localhost:8042/api/v1/rodent/disk/config \
  -H "Content-Type: application/json" \
  -d '{
    "health": {
      "smart": {
        "monitoring": {
          "interval": "15m"
        }
      }
    }
  }'

# gRPC
{
  "command_type": "disk.config.update",
  "payload": {
    "health": {
      "smart": {
        "monitoring": {
          "interval": "15m"
        }
      }
    }
  }
}
```

## Proto Command Constants

All commands are defined in `toggle-rodent-proto/proto/disk_command_types.go`:

```go
const (
    CmdDiskList                    = "disk.list"
    CmdDiskListAvailable          = "disk.list.available"
    CmdDiskGet                    = "disk.get"
    CmdDiskDiscover               = "disk.discover"
    CmdDiskRefresh                = "disk.refresh"
    CmdDiskHealthGet              = "disk.health.get"
    CmdDiskHealthCheck            = "disk.health.check"
    CmdDiskSmartGet               = "disk.smart.get"
    CmdDiskSmartRefresh           = "disk.smart.refresh"
    CmdDiskProbeStart             = "disk.probe.start"
    CmdDiskProbeCancel            = "disk.probe.cancel"
    CmdDiskProbeGet               = "disk.probe.get"
    CmdDiskProbeList              = "disk.probe.list"
    CmdDiskProbeHistory           = "disk.probe.history"
    // ... 28 more commands
)
```

See file for complete list of 43 command constants.
