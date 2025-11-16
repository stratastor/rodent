# Event System Implementation

The Rodent event system provides real-time event streaming to the upstream Toggle service using Protocol Buffers for type safety and efficient serialization.

## Architecture

Events flow from emission points through an in-memory buffer to Toggle via gRPC. When Toggle is unavailable, events are persisted to disk as protobuf binary files.

```text
Event Sources → Memory Buffer → Batching → Toggle Service
                     ↓ (overflow/unavailable)
                Disk (.pb files)
```

### Repository Structure

```text
toggle-rodent-proto/
├── proto/
│   ├── base.proto                    # EventBatch definitions
│   └── events/
│       └── event_messages.proto      # Event payload definitions

rodent/
└── internal/events/
    ├── schema.go                     # Type-safe emission functions
    ├── types.go                      # Category definitions
    ├── integration.go                # Toggle integration
    ├── bus.go                        # Event bus implementation
    ├── buffer.go                     # Disk-backed buffering
    └── client.go                     # HTTP client for Toggle
```

### Core Components

- **Event Bus** - In-memory channel-based event distribution
- **Buffer** - 20k event capacity with disk spillover at 18k
- **Client** - gRPC client for Toggle communication
- **Schema** - Type-safe emission functions in [schema.go](schema.go)

### Event Categories

Events are organized into 8 categories defined in `event_messages.proto`:

| Category | ID | Purpose |
|----------|----|---------|
| SYSTEM | 1 | OS, hardware, local operations |
| STORAGE | 2 | ZFS pools, datasets, transfers |
| NETWORK | 3 | Interfaces, routing, connectivity |
| SECURITY | 4 | SSH keys, certificates, auth |
| SERVICE | 5 | Service lifecycle management |
| IDENTITY | 6 | AD/LDAP user/group/computer operations |
| ACCESS | 7 | ACL, permissions, access control |
| SHARING | 8 | SMB/NFS shares, connections |

### Event Levels

| Level | Usage |
|-------|-------|
| INFO | Normal operations |
| WARN | Important changes |
| ERROR | Operation failures |
| CRITICAL | System critical issues |

## Emitting Events

Events are emitted using type-safe functions defined in [schema.go](schema.go). Each function accepts a strongly-typed payload and optional metadata.

### Basic Usage

```go
import (
    eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
    "github.com/stratastor/rodent/internal/events"
)

// System event
events.EmitSystemStartup(&eventspb.SystemStartupPayload{
    BootTimeSeconds: time.Now().Unix(),
    ServicesStarted: []string{"rodent-controller"},
    Operation:       eventspb.SystemStartupPayload_SYSTEM_STARTUP_OPERATION_REGISTERED,
}, map[string]string{
    "component": "toggle-registration",
})

// Storage event
events.EmitStorageTransfer(eventspb.EventLevel_EVENT_LEVEL_INFO, &eventspb.StorageTransferPayload{
    Source:      "tank/test@snap1",
    Destination: "backup/test",
    SizeBytes:   1024 * 1024,
    Operation:   eventspb.StorageTransferPayload_STORAGE_TRANSFER_OPERATION_STARTED,
}, map[string]string{
    "transfer_id": transferID,
})

// Service event
events.EmitServiceStatus(eventspb.EventLevel_EVENT_LEVEL_INFO, &eventspb.ServiceStatusPayload{
    ServiceName: "rodent-controller",
    Status:      "running",
    Pid:         int32(os.Getpid()),
    Operation:   eventspb.ServiceStatusPayload_SERVICE_STATUS_OPERATION_STARTED,
}, nil)
```

### Available Functions

Emission functions are implemented in [schema.go](schema.go):

**System:**

- `EmitSystemStartup(payload, metadata)`
- `EmitSystemShutdown(payload, metadata)`
- `EmitSystemConfigChange(payload, metadata)`
- `EmitSystemUser(level, payload, metadata)`

**Storage:**

- `EmitStoragePool(level, payload, metadata)`
- `EmitStorageDataset(level, payload, metadata)`
- `EmitStorageTransfer(level, payload, metadata)`

**Service:**

- `EmitServiceStatus(level, payload, metadata)`

Functions for other categories need to be added as they're implemented.

## Buffering and Storage

### Memory Buffer

- Capacity: 20,000 events
- Flush trigger: 18,000 events
- Flush action: Write entire buffer to single protobuf file
- Post-flush: Buffer cleared, continues accumulating
- Type: Pre-allocated slice of `*eventspb.Event`

The 2k cushion allows continued operation during disk I/O without blocking event emission.

### Disk Storage

- Location: `{configDir}/events/` (e.g., `/etc/rodent/events/`)
- Format: Protobuf binary (.pb files) containing EventBatch structures
- Naming: UUID7 for natural time ordering
- Content: EventBatch containing ~18k events per file
- Trigger: Toggle unavailability or buffer overflow

Disk files are only created when Toggle is unavailable. Once Toggle recovers, new events are sent directly via gRPC.

## Configuration

Events are configured in `rodent.yml`:

```yaml
events:
    profile: "default"  # default, high-throughput, low-latency, minimal
    buffer_size: 20000
    flush_threshold: 18000
    batch_size: 100
    batch_timeout: 30
```

See [types.go](types.go) for profile definitions.

## Initialization and Shutdown

The event system initializes automatically during Toggle registration:

```go
// internal/toggle/register.go
if err := events.Initialize(ctx, toggleClient, logger); err != nil {
    logger.Warn("Failed to initialize event system", "error", err)
}
```

Shutdown flushes pending events:

```go
if err := events.Shutdown(ctx); err != nil {
    logger.Error("Failed to shutdown event system", "error", err)
}
```

## Monitoring

Get event system statistics:

```go
stats := events.GetStats()
// Returns buffer_size, max_buffer_size, flush_threshold, pending_events, is_shutdown
```

Check if initialized:

```go
if !events.IsInitialized() {
    log.Error("Event system not initialized")
}
```

## Adding New Events

### 1. Define the Protocol Buffer

Add the payload message to `toggle-rodent-proto/proto/events/event_messages.proto`:

```protobuf
message NetworkEvent {
    oneof event_type {
        NetworkConnectionPayload connection_event = 2;
    }
}

message NetworkConnectionPayload {
    string source_ip = 1;
    string destination_ip = 2;
    int32 source_port = 3;
    NetworkConnectionOperation operation = 4;

    enum NetworkConnectionOperation {
        NETWORK_CONNECTION_OPERATION_UNSPECIFIED = 0;
        NETWORK_CONNECTION_OPERATION_ESTABLISHED = 1;
        NETWORK_CONNECTION_OPERATION_FAILED = 2;
    }
}
```

### 2. Regenerate Go Code

```bash
cd toggle-rodent-proto
make generate
cd ../rodent
go mod vendor
go mod tidy
```

### 3. Add Emission Function

Add to [schema.go](schema.go):

```go
func EmitNetworkConnection(level eventspb.EventLevel, payload *eventspb.NetworkConnectionPayload, metadata map[string]string) {
    event := &eventspb.Event{
        EventId:   generateEventID(),
        Level:     level,
        Category:  eventspb.EventCategory_EVENT_CATEGORY_NETWORK,
        Source:    "network-manager",
        Timestamp: time.Now().UnixMilli(),
        Metadata:  metadata,
        EventPayload: &eventspb.Event_NetworkEvent{
            NetworkEvent: &eventspb.NetworkEvent{
                EventType: &eventspb.NetworkEvent_ConnectionEvent{
                    ConnectionEvent: payload,
                },
            },
        },
    }
    emitStructuredEvent(event)
}
```

### 4. Use in Code

```go
events.EmitNetworkConnection(eventspb.EventLevel_EVENT_LEVEL_ERROR, &eventspb.NetworkConnectionPayload{
    SourceIp:      "192.168.1.100",
    DestinationIp: "192.168.1.1",
    SourcePort:    12345,
    Operation:     eventspb.NetworkConnectionPayload_NETWORK_CONNECTION_OPERATION_FAILED,
}, map[string]string{
    "component": "netmage",
    "interface": "eth0",
})
```

## Testing

The event system uses a mock Toggle server for integration tests. See [integration_test.go](integration_test.go) for examples.

```go
func TestStructuredEvents(t *testing.T) {
    err := events.Initialize(ctx, toggleClient, logger)
    require.NoError(t, err)

    events.EmitSystemStartup(&eventspb.SystemStartupPayload{
        BootTimeSeconds: time.Now().Unix(),
        Operation:       eventspb.SystemStartupPayload_SYSTEM_STARTUP_OPERATION_STARTED,
    }, map[string]string{
        "component": "test-system",
    })

    // Validate event received by mock server
}
```

Run integration tests:

```bash
go test -v ./internal/events/ -run TestStructuredEvents_Integration
```

## Debugging

### Compilation errors

Ensure protobuf field names and types match schema. Run `go mod vendor` after proto regeneration.

## Current Implementations

Event emission is implemented in:

- [pkg/server/server.go](../../pkg/server/server.go) - Service status events
- [internal/toggle/register.go](../../internal/toggle/register.go) - System startup events
- [pkg/zfs/dataset/transfer_manager.go](../../pkg/zfs/dataset/transfer_manager.go) - Storage transfer events
- [pkg/system/user_manager.go](../../pkg/system/user_manager.go) - System user events

Additional modules (Network, Security, Identity, Access, Sharing) are defined in the proto schema but need emission functions and module integration.

## Best Practices

- Emit events after successful operations, not before
- Use appropriate event levels (INFO for normal ops, WARN/ERROR for issues)
- Include relevant context in metadata without being verbose
- Never include credentials, secrets, or PII in events
- Use operation enums consistently within your component
- Leverage compile-time type safety - the compiler catches errors
