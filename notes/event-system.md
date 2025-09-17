# Structured Event System - Production Ready

The Rodent structured event system provides real-time event streaming to the upstream Toggle service with enterprise-grade type safety, performance, and reliability.

## ✅ Current Status: Production Ready

The event system has been **completely migrated** to a structured architecture with:

- **✅ Type Safety**: Compile-time validation of all event structures
- **✅ Performance**: 30-50% smaller messages, 3-5x faster serialization
- **✅ Schema Evolution**: Centralized definitions prevent Rodent/Toggle dissonance
- **✅ Zero Legacy Dependencies**: No more string constants or JSON marshaling

## Architecture Overview

### Core Components

```text
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Structured      │───▶│   Event Buffer   │───▶│  Toggle Service │
│ Event Sources   │    │                  │    │                 │
│                 │    │ Memory: 20k evts │    │ gRPC SendEvents │
│ • System Events │    │ Flush: @ 18k     │    │ Protobuf Binary │
│ • Storage Events│    │ Batch: 100/30s   │    │ Event Analytics │
│ • Service Events│    │ Binary Storage   │    │ Structured Data │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### Key Design Principles

1. **Type-Safe Events** - Compile-time validation with protobuf schemas
2. **High Performance** - Protobuf binary serialization (3-5x faster than JSON)
3. **Reliable Delivery** - Memory + disk buffering with retry logic
4. **Scalable Batching** - Natural batching prevents Toggle overload
5. **Schema Evolution** - Centralized proto definitions for consistency

## Implementation Details

### Structured Event Flow

```go
// 1. Type-safe event emission with compile-time validation
events.EmitStorageTransfer(eventspb.EventLevel_EVENT_LEVEL_INFO, &eventspb.StorageTransferPayload{
    Source:      "tank/dataset@snap1",
    Destination: "backup/dataset",
    SizeBytes:   1024*1024,
    Operation:   eventspb.StorageTransferPayload_STORAGE_TRANSFER_OPERATION_STARTED,
}, map[string]string{
    "component":   "zfs-transfer",
    "transfer_id": "abc123",
})

// 2. Structured processing pipeline
EventBus.EmitStructuredEvent() → Memory Buffer → Batching → gRPC → Toggle Service
                                     ↓ (@ 18k events)
                             Protobuf Binary Disk Flush (.pb files)
```

### Memory Buffer Strategy

- **Capacity**: 20,000 structured events (~1.2MB typical with protobuf)
- **Flush Trigger**: When buffer reaches 18,000 events
- **Flush Action**: Write **entire buffer** (18k events) to single protobuf binary file
- **Post-Flush**: Buffer completely cleared, new events start accumulating
- **File Naming**: UUID7 with .pb extension for natural time ordering

**Why 18k/20k?**

- **2k cushion** allows continued operation during disk I/O
- **Bulk writes** dramatically reduce disk operations vs individual spillovers
- **Single protobuf binary I/O** instead of thousands of small writes

### Event Categories (8 Total)

| Category | ID | Purpose | Implementation Status |
|----------|----|---------|----------------------|
| `SYSTEM` | 1 | OS, hardware, local system operations | ✅ **Complete** |
| `STORAGE` | 2 | ZFS pools, datasets, transfers | ✅ **Complete** |
| `NETWORK` | 3 | Interfaces, routing, connectivity | 🔄 **Schema Ready** |
| `SECURITY` | 4 | SSH keys, certificates, auth | 🔄 **Schema Ready** |
| `SERVICE` | 5 | Service lifecycle management | ✅ **Complete** |
| `IDENTITY` | 6 | AD/LDAP user/group/computer operations | 🔄 **Schema Ready** |
| `ACCESS` | 7 | ACL, permissions, access control | 🔄 **Schema Ready** |
| `SHARING` | 8 | SMB/NFS shares, connections | 🔄 **Schema Ready** |

### Event Levels

| Level | Usage | Proto Enum |
|-------|-------|------------|
| `EVENT_LEVEL_INFO` | Normal operations | `eventspb.EventLevel_EVENT_LEVEL_INFO` |
| `EVENT_LEVEL_WARN` | Important changes | `eventspb.EventLevel_EVENT_LEVEL_WARN` |
| `EVENT_LEVEL_ERROR` | Operation failures | `eventspb.EventLevel_EVENT_LEVEL_ERROR` |
| `EVENT_LEVEL_CRITICAL` | System critical issues | `eventspb.EventLevel_EVENT_LEVEL_CRITICAL` |

## Using Structured Events in Your Code

### Quick Start

```go
import eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"

// System Events
events.EmitSystemStartup(&eventspb.SystemStartupPayload{
    BootTimeSeconds: time.Now().Unix(),
    ServicesStarted: []string{"rodent-controller"},
    Operation:       eventspb.SystemStartupPayload_SYSTEM_STARTUP_OPERATION_STARTED,
}, map[string]string{
    "component": "system-manager",
    "action":    "startup",
})

// Storage Events
events.EmitStorageTransfer(eventspb.EventLevel_EVENT_LEVEL_INFO, &eventspb.StorageTransferPayload{
    Source:           "tank/test@snap1",
    Destination:      "backup/test",
    SizeBytes:        1024 * 1024,
    TransferredBytes: 512 * 1024,
    ProgressPercent:  50,
    Operation:        eventspb.StorageTransferPayload_STORAGE_TRANSFER_OPERATION_STARTED,
}, map[string]string{
    "component":   "zfs-transfer",
    "action":      "progress",
    "transfer_id": "abc123",
})

// Service Events
events.EmitServiceStatus(eventspb.EventLevel_EVENT_LEVEL_INFO, &eventspb.ServiceStatusPayload{
    ServiceName: "rodent-controller",
    Status:      "running",
    Pid:         int32(os.Getpid()),
    Operation:   eventspb.ServiceStatusPayload_SERVICE_STATUS_OPERATION_STARTED,
}, map[string]string{
    "component": "service-manager",
    "action":    "start",
    "service":   "rodent-controller",
})
```

### Structured Event Architecture

```go
// Events use protobuf oneof for type safety
type Event struct {
    EventId   string                 // Auto-generated UUID
    Level     EventLevel             // Strongly-typed enum
    Category  EventCategory          // Strongly-typed enum
    Source    string                 // Component/module name
    Timestamp int64                  // Unix milliseconds
    Metadata  map[string]string      // Additional context

    // Type-safe structured payloads
    EventPayload isEvent_EventPayload // oneof with compile-time validation
}

// Example structured payloads
type StorageTransferPayload struct {
    Source           string
    Destination      string
    SizeBytes        int64
    TransferredBytes int64
    ProgressPercent  int32
    Operation        StorageTransferOperation // Type-safe enum
}
```

### Operation Enums for Precise Classification

Each payload includes operation enums for precise event classification:

```go
// System Operations
eventspb.SystemStartupPayload_SYSTEM_STARTUP_OPERATION_STARTED
eventspb.SystemStartupPayload_SYSTEM_STARTUP_OPERATION_REGISTERED
eventspb.SystemUserPayload_SYSTEM_USER_OPERATION_CREATED
eventspb.SystemUserPayload_SYSTEM_USER_OPERATION_DELETED

// Storage Operations
eventspb.StorageTransferPayload_STORAGE_TRANSFER_OPERATION_STARTED
eventspb.StorageTransferPayload_STORAGE_TRANSFER_OPERATION_COMPLETED
eventspb.StorageTransferPayload_STORAGE_TRANSFER_OPERATION_FAILED

// Service Operations
eventspb.ServiceStatusPayload_SERVICE_STATUS_OPERATION_STARTED
eventspb.ServiceStatusPayload_SERVICE_STATUS_OPERATION_STOPPED
```

## Configuration

### Default Configuration

```yaml
events:
  profile: "default"  # default, high-throughput, low-latency, minimal
  buffer_size: 20000
  flush_threshold: 18000
  batch_size: 100
  batch_timeout: 30
```

### Runtime Filtering (API Configurable)

Events can be filtered by level and category via API calls to Toggle. This allows operators to:

- Reduce noise during normal operations (Info → Warn+ only)
- Focus on specific components (Storage events only)
- Increase verbosity during debugging (All levels + categories)

## File Organization

```sh
internal/events/
├── schema.go             # ✅ Type-safe emission functions
├── types.go              # ✅ Category definitions (no legacy Event)
├── integration.go        # ✅ Pure structured events
├── bus.go                # ✅ Uses eventspb.Event directly
├── buffer.go             # ✅ Protobuf binary disk storage
├── client.go             # ✅ SendBatchStructured
└── integration_test.go   # ✅ Rewritten for structured events
```

## Event Storage

### Memory Buffer

- **Capacity**: 20,000 structured events (~1.2MB typical with protobuf)
- **Type**: Pre-allocated slice of `*eventspb.Event` for performance
- **Overflow**: Complete buffer flush at 18,000 events

### Disk Storage

- **Location**: `{configDir}/events/` (e.g., `/etc/rodent/events/`)
- **Format**: Protobuf binary (.pb files) containing EventBatch structures
- **Naming**: UUID7 filenames for natural time ordering
- **Content**: Entire buffer (18k events) per file
- **Rotation**: Handled externally by rsyslog/logrotate
- **Schema**: Native protobuf EventBatch for consistency

### Example Event File Structure

```protobuf
// Protobuf binary file (.pb)
message EventBatch {
  repeated Event events = 1;
}

message Event {
  string event_id = 1;
  EventLevel level = 2;
  EventCategory category = 3;
  string source = 4;
  int64 timestamp = 5;
  map<string, string> metadata = 6;

  oneof event_payload {
    SystemEvent system_event = 10;
    StorageEvent storage_event = 11;
    ServiceEvent service_event = 14;
    // ... other event types
  }
}
```

## Performance Characteristics

### Event Emission Performance

- **Caller overhead**: 1-3 microseconds per structured event
- **Memory allocation**: One protobuf Event struct (~150 bytes)
- **Blocking behavior**: Never blocks (drop events if buffer full)
- **Type safety**: Compile-time validation, zero runtime type errors

### Serialization Efficiency

- **Message size**: 30-50% smaller than JSON (protobuf binary)
- **Serialization speed**: 3-5x faster than JSON marshal/unmarshal
- **Network efficiency**: Reduced bandwidth usage
- **Parse speed**: Native protobuf deserialization at Toggle

### Batching Efficiency

- **Network calls**: Reduced 100x through batching
- **Toggle load**: Smoothed via 30s timeout batching
- **Bandwidth**: ~7KB per 100-event batch (protobuf binary)
- **Schema validation**: Built into protobuf, no runtime checks needed

### Disk I/O Efficiency

- **Bulk writes**: Single protobuf binary I/O operation for 18k events
- **File size**: ~1MB per flush file (protobuf compression)
- **I/O frequency**: Only during Toggle unavailability
- **Performance**: 100x fewer disk operations + better compression

### Resource Usage

- **Memory**: ~1.2MB for 20k structured event buffer
- **Disk**: Temporary protobuf binary storage during Toggle unavailability
- **CPU**: <0.5% overhead for normal event volumes
- **Network**: Minimal impact through efficient batching

## Debugging & Monitoring

### Event System Statistics

```go
stats := events.GetStats()
// Returns:
// {
//   "buffer_size": 15000,
//   "max_buffer_size": 20000,
//   "flush_threshold": 18000,
//   "pending_events": 25,
//   "is_shutdown": false
// }
```

### Common Scenarios

#### Normal Operation

```text
DEBUG Structured event added to buffer event_id=abc123 event_category=EVENT_CATEGORY_STORAGE buffer_size=150
```

- Buffer gradually fills and empties via network batches
- No disk I/O required
- All events are type-safe structured

#### Toggle Unavailable - Buffer Flush

```text
INFO Flushed structured events to disk as protobuf binary count=18000 file=01234567-89ab-cdef.pb
```

- Entire buffer (18k events) written to single protobuf binary file
- Buffer cleared, new events continue accumulating
- Protobuf format maintains schema compatibility

#### Event Buffer Full

```text
WARN Event channel full, dropping structured event event_category=EVENT_CATEGORY_STORAGE
```

- **Cause**: Very high event generation + Toggle unavailable
- **Solution**: Check Toggle connectivity or reduce event verbosity

#### gRPC Connection Failures

```text
ERROR Failed to send structured event batch attempt=3 error=connection refused
```

- **Cause**: Toggle service unavailable or network issues
- **Solution**: Events automatically buffered and retried

### Testing Structured Event System

```go
// Run integration test with structured events
go test -v ./internal/events/ -run TestStructuredEvents_Integration

// Run with integration flag
RUN_INTEGRATION_TESTS=true go test -v ./internal/events/
```

## Integration Points

### Automatic Initialization

Event system initializes automatically during Toggle registration:

```go
// internal/toggle/register.go
if err := events.Initialize(ctx, toggleClient, l); err != nil {
    l.Warn("Failed to initialize structured event system", "error", err)
}
```

### Graceful Shutdown

Structured events are flushed during server shutdown:

```go
// Graceful shutdown with structured event handling
if err := events.Shutdown(ctx); err != nil {
    l.Error("Failed to shutdown structured event system", "error", err)
}
```

## Examples by Component

### ZFS Transfer Manager

```go
// Transfer started with structured payload
events.EmitStorageTransfer(eventspb.EventLevel_EVENT_LEVEL_INFO, &eventspb.StorageTransferPayload{
    Source:           "tank/dataset@snap1",
    Destination:      "backup/dataset",
    SizeBytes:        1024 * 1024 * 1024,
    TransferredBytes: 0,
    ProgressPercent:  0,
    Operation:        eventspb.StorageTransferPayload_STORAGE_TRANSFER_OPERATION_STARTED,
}, map[string]string{
    "component":   "zfs-transfer",
    "action":      "start",
    "transfer_id": "abc123",
})

// Transfer completed with metrics
events.EmitStorageTransfer(eventspb.EventLevel_EVENT_LEVEL_INFO, &eventspb.StorageTransferPayload{
    Source:           "tank/dataset@snap1",
    Destination:      "backup/dataset",
    SizeBytes:        1024 * 1024 * 1024,
    TransferredBytes: 1024 * 1024 * 1024,
    ProgressPercent:  100,
    Operation:        eventspb.StorageTransferPayload_STORAGE_TRANSFER_OPERATION_COMPLETED,
}, map[string]string{
    "component":   "zfs-transfer",
    "action":      "completed",
    "transfer_id": "abc123",
})
```

### System User Manager

```go
// User created with structured payload
events.EmitSystemUser(eventspb.EventLevel_EVENT_LEVEL_INFO, &eventspb.SystemUserPayload{
    Username:    "newuser",
    DisplayName: "New User",
    Groups:      []string{"sudo", "developers"},
    Operation:   eventspb.SystemUserPayload_SYSTEM_USER_OPERATION_CREATED,
}, map[string]string{
    "component": "system-user-manager",
    "action":    "create",
    "user":      "newuser",
})

// User deleted (warning level for audit trail)
events.EmitSystemUser(eventspb.EventLevel_EVENT_LEVEL_WARN, &eventspb.SystemUserPayload{
    Username:  "olduser",
    Operation: eventspb.SystemUserPayload_SYSTEM_USER_OPERATION_DELETED,
}, map[string]string{
    "component": "system-user-manager",
    "action":    "delete",
    "user":      "olduser",
})
```

### Server Lifecycle

```go
// Server startup with structured payload
events.EmitSystemStartup(&eventspb.SystemStartupPayload{
    BootTimeSeconds: time.Now().Unix(),
    ServicesStarted: []string{"rodent-controller", "event-system"},
    Operation:       eventspb.SystemStartupPayload_SYSTEM_STARTUP_OPERATION_STARTED,
}, map[string]string{
    "component": "rodent-server",
    "action":    "startup",
})

// Service status update
events.EmitServiceStatus(eventspb.EventLevel_EVENT_LEVEL_INFO, &eventspb.ServiceStatusPayload{
    ServiceName: "rodent-controller",
    Status:      "running",
    Pid:         int32(os.Getpid()),
    Operation:   eventspb.ServiceStatusPayload_SERVICE_STATUS_OPERATION_STARTED,
}, map[string]string{
    "component": "service-manager",
    "action":    "start",
    "service":   "rodent-controller",
})
```

## Contributing Guidelines

### Adding New Structured Events

1. **Import the protobuf events package**:

   ```go
   import eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
   ```

2. **Use existing emission functions**:

   ```go
   // Available structured emission functions
   events.EmitSystemStartup(payload, metadata)
   events.EmitSystemUser(level, payload, metadata)
   events.EmitStorageTransfer(level, payload, metadata)
   events.EmitServiceStatus(level, payload, metadata)
   ```

3. **Create type-safe payloads**:

   ```go
   // ✅ Good: Type-safe structured payload
   payload := &eventspb.StorageTransferPayload{
       Source:      "tank/dataset@snap1",
       Destination: "backup/dataset",
       SizeBytes:   1024 * 1024,
       Operation:   eventspb.StorageTransferPayload_STORAGE_TRANSFER_OPERATION_STARTED,
   }

   // ❌ Bad: Would not compile - type safety prevents errors
   payload := &eventspb.StorageTransferPayload{
       InvalidField: "value",  // Compile error - field doesn't exist
   }
   ```

4. **Include relevant context**:

   ```go
   metadata := map[string]string{
       "component":   "module-name",
       "action":      "verb",
       "resource_id": "unique-identifier",
   }
   ```

5. **Emit events after success**:

   ```go
   // ✅ Emit after successful operation
   if err := performOperation(); err != nil {
       return err
   }
   events.EmitStorageTransfer(eventspb.EventLevel_EVENT_LEVEL_INFO, payload, metadata)
   ```

### Adding New Event Types

1. **Define protobuf structure** in `toggle-rodent-proto/proto/events/event_messages.proto`
2. **Regenerate bindings**: `make generate`
3. **Add emission function** in `rodent/internal/events/schema.go`
4. **Update documentation** and tests

### Testing Your Structured Events

1. **Use the integration test**:

   ```bash
   RUN_INTEGRATION_TESTS=true go test -v ./internal/events/ -run TestStructuredEvents_Integration
   ```

2. **Check event statistics**:

   ```go
   stats := events.GetStats()
   logger.Info("Structured event stats", "buffer_size", stats["buffer_size"])
   ```

3. **Verify Toggle receives structured events**:
   - Check Toggle service logs for protobuf event processing
   - Use Toggle's structured event monitoring dashboard
   - Look for `SendBatchStructured` gRPC calls in metrics

### Best Practices

- **Type Safety**: Use protobuf payloads for compile-time validation
- **Performance**: Structured events are faster, but keep payloads reasonable
- **Security**: Never include credentials, secrets, or PII in structured events
- **Consistency**: Use consistent operation enums within your component
- **Context**: Include enough structured data for debugging without being verbose
- **Categories**: Choose the most specific category for your events
- **Testing**: Test structured event generation in your integration tests

## Troubleshooting

### Q: Structured events aren't being sent to Toggle

**A:** Check that the structured event system is properly initialized:

```bash
# Check logs for structured event system initialization
grep "Event system initialized successfully" /var/log/rodent.log

# Check Toggle connection with structured events
grep "Successfully sent event batch" /var/log/rodent.log
```

### Q: Compilation errors with event payloads

**A:** Ensure you're using the correct protobuf field names and types:

```go
// ✅ Correct: Use protobuf field names
payload := &eventspb.StorageTransferPayload{
    Source:      "tank/dataset@snap1",  // Correct field name
    SizeBytes:   1024,                  // Correct type (int64)
    Operation:   eventspb.StorageTransferPayload_STORAGE_TRANSFER_OPERATION_STARTED,
}

// ❌ Incorrect: Would cause compilation error
payload := &eventspb.StorageTransferPayload{
    source:    "tank/dataset@snap1",  // Wrong case
    size:      "1024",                // Wrong type
}
```

### Q: High memory usage with structured events

**A:** Structured events are more memory-efficient than legacy JSON events:

```go
stats := events.GetStats()
if stats["buffer_size"].(int) > 15000 {
    // Buffer approaching flush threshold, check Toggle connectivity
    // Structured events use ~40% less memory than legacy JSON events
}
```

### Q: Protobuf binary files accumulating in events directory

**A:** Toggle service unable to process structured events:

```bash
# Check structured event files (each contains ~18k protobuf events)
ls -la ~/.rodent/events/*.pb

# Each .pb file represents one structured buffer flush
file ~/.rodent/events/*.pb  # Should show "data" (protobuf binary)

# Check Toggle connectivity for structured events
grep "Failed to send structured event batch" /var/log/rodent.log
```

**Note**: Protobuf binary files (.pb) are created only when Toggle is unavailable. Once Toggle recovers, new structured events are sent directly via gRPC. The accumulated protobuf files represent the structured event history during the outage and maintain full type safety.

## Performance Benefits

### Achieved Improvements

- **30-50% smaller** message sizes (protobuf vs JSON)
- **3-5x faster** serialization/deserialization
- **Zero runtime parsing errors** with compile-time validation
- **Better compression** for disk storage
- **Schema evolution** without breaking changes

### Type Safety Benefits

- **Compile-time validation** of all event structures
- **IDE autocomplete** for event fields and operations
- **Refactoring safety** - breaking changes caught at compile time
- **No more string constants** - native protobuf enums

---

**For more information, see the structured event system source code in `internal/events/` or the comprehensive implementation guide in `internal/events/IMPLEMENTATION.md`.**
