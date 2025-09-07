# Event Notification System

The Rodent event notification system provides real-time event streaming to the upstream Toggle service for monitoring, auditing, and operational intelligence.

## Architecture Overview

### Core Components

```text
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Event Sources │───▶│   Event Buffer   │───▶│  Toggle Service │
│                 │    │                  │    │                 │
│ • ZFS Transfer  │    │ Memory: 20k evts │    │ gRPC SendEvents │
│ • User Mgmt     │    │ Flush: @ 18k     │    │ Batch Processing│
│ • System Ops    │    │ Batch: 100/30s   │    │ Event Analytics │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### Key Design Principles

1. **Non-Blocking Performance** - Event emission adds ~1-5μs overhead to operations
2. **Reliable Delivery** - Memory + disk buffering with retry logic
3. **Scalable Batching** - Natural batching prevents Toggle overload
4. **Resource Efficient** - Pre-allocated buffers and async processing

## Implementation Details

### Event Flow

```go
// 1. Non-blocking event emission
events.EmitStorageEvent("storage.transfer.started", events.LevelInfo, "zfs-manager", payload, metadata)

// 2. Async processing pipeline
EventBus.Emit() → Memory Buffer → Batching → gRPC → Toggle Service
                     ↓ (@ 18k events)
                Complete Buffer Flush (UUID7 files)
```

### Memory Buffer Strategy

- **Capacity**: 20,000 events (~1.6MB typical)
- **Flush Trigger**: When buffer reaches 18,000 events
- **Flush Action**: Write **entire buffer** (18k events) to single JSON file
- **Post-Flush**: Buffer completely cleared, new events start accumulating
- **File Naming**: UUID7 for natural time ordering

**Why 18k/20k?**

- **2k cushion** allows continued operation during disk I/O
- **Bulk writes** dramatically reduce disk operations vs individual spillovers
- **Single I/O** instead of thousands of small writes

### Event Categories

| Category | Purpose | Examples |
|----------|---------|----------|
| `CategorySystem` | System-level operations | Startup, configuration changes |
| `CategoryStorage` | ZFS operations | Dataset creation, transfers, snapshots |
| `CategorySecurity` | Authentication/authorization | User creation/deletion, permission changes |
| `CategoryNetwork` | Network configuration | Interface changes, routing updates |
| `CategoryService` | Service lifecycle | Server startup/shutdown, health status |

### Event Levels

| Level | Usage | Color Coding |
|-------|-------|--------------|
| `LevelInfo` | Normal operations | 🟢 Green |
| `LevelWarn` | Important changes | 🟡 Yellow |
| `LevelError` | Operation failures | 🔴 Red |
| `LevelCritical` | System critical issues | 🟣 Purple |

## Adding Events to Your Code

### Quick Start

```go
import "github.com/stratastor/rodent/internal/events"

// System event
events.EmitSystemEvent("system.config.updated", events.LevelInfo, payload, metadata)

// Storage event with source
events.EmitStorageEvent("storage.dataset.created", events.LevelInfo, "zfs-manager", payload, metadata)

// Security event
events.EmitSecurityEvent("security.login.failed", events.LevelWarn, "auth-service", payload, metadata)
```

### Event Structure

```go
type Event struct {
    ID        string                 // Auto-generated UUID7
    Type      string                 // e.g., "storage.transfer.completed"
    Level     EventLevel             // Info, Warn, Error, Critical
    Category  EventCategory          // System, Storage, Security, Network, Service
    Source    string                 // Component/module name
    Timestamp time.Time              // Auto-generated
    Payload   []byte                 // JSON-encoded event data
    Metadata  map[string]string      // Additional context
}
```

### Payload Best Practices

**✅ Good Payload:**

```go
payload := map[string]interface{}{
    "transfer_id": "01234567-89ab-cdef",
    "operation":   "send-receive",
    "snapshot":    "pool1/dataset@snapshot1",
    "duration_seconds": 42.5,
    "bytes_transferred": 1048576,
}
```

**❌ Avoid:**

```go
// Don't include sensitive data
payload := map[string]interface{}{
    "password": "secret123",  // ❌ Never include credentials
    "api_key":  "abc123xyz",  // ❌ Never include secrets
}

// Don't include massive objects
payload := map[string]interface{}{
    "entire_dataset": hugeObject,  // ❌ Keep payloads small
}
```

## Configuration

### Default Configuration

```go
&EventConfig{
    BufferSize:        20000,              // Max events in memory
    FlushThreshold:    18000,              // Complete buffer flush trigger
    BatchSize:         100,                // Events per network batch  
    BatchTimeout:      30 * time.Second,   // Max batch wait time
    EnabledLevels:     []EventLevel{LevelInfo, LevelWarn, LevelError, LevelCritical},
    EnabledCategories: []EventCategory{CategorySystem, CategoryStorage, CategoryNetwork, CategorySecurity, CategoryService},
    MaxRetryAttempts:  3,                  // gRPC retry attempts
    RetryBackoffBase:  1 * time.Second,    // Exponential backoff base
}
```

### Runtime Filtering (API Configurable)

Events can be filtered by level and category via API calls to Toggle. This allows operators to:

- Reduce noise during normal operations (Info → Warn+ only)
- Focus on specific components (Storage events only)
- Increase verbosity during debugging (All levels + categories)

## File Organization

```sh
internal/events/
├── types.go           # Event structures and configuration
├── buffer.go          # Memory buffer with bulk disk flush
├── bus.go            # Event coordinator and processing
├── client.go         # gRPC client with retry logic
├── integration.go    # Global API and helper functions
├── init.go           # Initialization with Toggle client
└── test_integration.go # Testing utilities
```

## Event Storage

### Memory Buffer

- **Capacity**: 20,000 events (~1.6MB typical)
- **Type**: Pre-allocated Go slice for performance
- **Overflow**: Complete buffer flush at 18,000 events

### Disk Storage

- **Location**: `{configDir}/events/` (e.g., `/etc/rodent/events/`)
- **Format**: JSON arrays of proto.Event structures
- **Naming**: UUID7 filenames for natural time ordering
- **Content**: Entire buffer (18k events) per file
- **Rotation**: Handled externally by rsyslog/logrotate
- **Schema**: Same as gRPC proto for consistency

### Example Event File

```json
[
  {
    "event_id": "01234567-89ab-cdef-0123-456789abcdef",
    "event_type": "storage.transfer.completed",
    "level": 1,
    "category": 2, 
    "source": "zfs-transfer-manager",
    "timestamp": 1694088000000,
    "payload": "{\"transfer_id\":\"abc123\",\"duration_seconds\":42.5}",
    "metadata": {
      "component": "zfs-transfer",
      "action": "completed"
    }
  }
  // ... 17,999 more events in same file
]
```

## Performance Characteristics

### Event Emission Performance

- **Caller overhead**: 1-5 microseconds per event
- **Memory allocation**: One Event struct (~200 bytes)
- **Blocking behavior**: Never blocks (drop events if buffer full)

### Batching Efficiency

- **Network calls**: Reduced 100x through batching
- **Toggle load**: Smoothed via 30s timeout batching
- **Bandwidth**: ~10KB per 100-event batch (JSON)

### Disk I/O Efficiency

- **Bulk writes**: Single I/O operation for 18k events
- **File size**: ~1-2MB per flush file
- **I/O frequency**: Only during Toggle unavailability
- **Performance**: 100x fewer disk operations than individual writes

### Resource Usage

- **Memory**: ~1.6MB for 20k event buffer
- **Disk**: Temporary storage during Toggle unavailability
- **CPU**: <1% overhead for normal event volumes
- **Network**: Minimal impact through batching

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
DEBUG Event added to buffer event_id=abc123 buffer_size=150
```

- Buffer gradually fills and empties via network batches
- No disk I/O required

#### Toggle Unavailable - Buffer Flush

```text
INFO Flushed events to disk count=18000 file=01234567-89ab-cdef.json
```

- Entire buffer (18k events) written to single file
- Buffer cleared, new events continue accumulating

#### Event Buffer Full

```text
WARN Event channel full, dropping event event_type=storage.test
```

- **Cause**: Very high event generation + Toggle unavailable
- **Solution**: Check Toggle connectivity or reduce event verbosity

#### gRPC Connection Failures

```text
ERROR Failed to send event batch attempt=3 error=connection refused
```

- **Cause**: Toggle service unavailable or network issues  
- **Solution**: Events automatically buffered and retried

### Testing Event System

```go
import "github.com/stratastor/rodent/internal/events"

// Run integration test
events.TestEventSystem(logger)

// Run load test  
events.TestEventSystemWithContext(ctx, logger)
```

## Integration Points

### Automatic Initialization

Event system initializes automatically during Toggle registration:

```go
// internal/toggle/register.go:159
if err := events.InitializeWithClient(ctx, toggleClient, l); err != nil {
    l.Warn("Failed to initialize event system", "error", err)
}
```

### Graceful Shutdown

Events are flushed during server shutdown:

```go  
// pkg/lifecycle integration
lifecycle.RegisterShutdownHook(func() {
    if err := events.Shutdown(ctx); err != nil {
        l.Error("Failed to shutdown event system", "error", err)
    }
})
```

## Examples by Component

### ZFS Transfer Manager

```go
// Transfer started
events.EmitStorageEvent("storage.transfer.started", events.LevelInfo, "zfs-transfer-manager",
    map[string]interface{}{
        "transfer_id": transferID,
        "operation":   "send-receive", 
        "snapshot":    "pool1/dataset@snap1",
        "target":      "remote-host",
    },
    map[string]string{
        "component": "zfs-transfer",
        "action":    "start",
    })

// Transfer completed with metrics
events.EmitStorageEvent("storage.transfer.completed", events.LevelInfo, "zfs-transfer-manager", 
    map[string]interface{}{
        "transfer_id": info.ID,
        "duration_seconds": 42.5,
        "bytes_transferred": 1048576,
        "status": "completed",
    },
    map[string]string{
        "component": "zfs-transfer", 
        "action":    "completed",
    })
```

### System User Manager  

```go
// User created
events.EmitSecurityEvent("security.user.created", events.LevelInfo, "system-user-manager",
    map[string]interface{}{
        "username":     "newuser",
        "groups":       []string{"sudo", "developers"},
        "shell":        "/bin/bash",
        "home_dir":     "/home/newuser",
        "system_user":  false,
    },
    map[string]string{
        "component": "user-management",
        "action":    "create", 
    })

// User deleted (warning level for audit trail)
events.EmitSecurityEvent("security.user.deleted", events.LevelWarn, "system-user-manager",
    map[string]interface{}{
        "username": "olduser",
    },
    map[string]string{
        "component": "user-management",
        "action":    "delete",
    })
```

### Server Lifecycle

```go
// Server shutdown
events.EmitServiceEvent("service.server.shutdown", events.LevelInfo, "rodent-server",
    map[string]interface{}{
        "message": "Rodent server shutting down gracefully",
        "uptime_seconds": uptimeSeconds,
    },
    map[string]string{
        "component": "server",
        "action":    "shutdown",
    })
```

## Contributing Guidelines

### Adding New Event Sources

1. **Import the events package**:

   ```go
   import "github.com/stratastor/rodent/internal/events"
   ```

2. **Choose appropriate category and level**:
   - Use `LevelInfo` for normal operations
   - Use `LevelWarn` for important changes
   - Use `LevelError` for failures
   - Use `LevelCritical` sparingly for system-critical issues

3. **Use descriptive event types**:

   ```go
   // ✅ Good: Hierarchical and specific
   "storage.dataset.created"
   "security.user.password.changed" 
   "network.interface.configured"
   
   // ❌ Bad: Vague or flat
   "created"
   "event"
   "something_happened"
   ```

4. **Include relevant context**:

   ```go
   payload := map[string]interface{}{
       "resource_id": "unique-identifier",
       "operation":   "specific-action",  
       // ... other relevant data
   }
   
   metadata := map[string]string{
       "component": "module-name",
       "action":    "verb",
   }
   ```

5. **Emit events after success**:

   ```go
   // ✅ Emit after successful operation
   if err := performOperation(); err != nil {
       return err
   }
   events.EmitStorageEvent("storage.operation.completed", ...)
   
   // ❌ Don't emit before operation completes
   events.EmitStorageEvent("storage.operation.completed", ...)
   return performOperation() // Might fail!
   ```

### Testing Your Events

1. **Use the integration test**:

   ```go
   events.TestEventSystem(logger)
   ```

2. **Check event statistics**:

   ```go
   stats := events.GetStats()
   logger.Info("Event stats", "buffer_size", stats["buffer_size"])
   ```

3. **Verify Toggle receives events**:
   - Check Toggle service logs for event processing
   - Use Toggle's event monitoring dashboard
   - Look for `SendEvents` gRPC calls in metrics

### Best Practices

- **Performance**: Event emission is async, but keep payloads reasonable (<1KB)
- **Security**: Never include credentials, secrets, or PII in events
- **Consistency**: Use consistent naming patterns within your component
- **Context**: Include enough information for debugging without being verbose
- **Categories**: Choose the most specific category for your events
- **Testing**: Test event generation in your integration tests

## Troubleshooting

### Q: Events aren't being sent to Toggle

**A:** Check that the Toggle client is gRPC-based and properly initialized:

```bash
# Check logs for event system initialization
grep "Event system initialized" /var/log/rodent.log

# Check Toggle connection
grep "Connected to Toggle via streaming gRPC" /var/log/rodent.log  
```

### Q: High memory usage

**A:** Event buffer may be growing due to Toggle unavailability:

```go
stats := events.GetStats()
if stats["buffer_size"].(int) > 15000 {
    // Buffer approaching flush threshold, check Toggle connectivity
}
```

### Q: Events being dropped

**A:** Event generation exceeding buffer capacity:

```bash
# Check for dropped events
grep "Event channel full" /var/log/rodent.log

# Solutions:
# 1. Reduce event verbosity via Toggle API
# 2. Increase BufferSize in configuration
# 3. Check Toggle service health
```

### Q: Disk files accumulating in events directory

**A:** Toggle service unable to process events:

```bash
# Check event files (each contains ~18k events)
ls -la ~/.rodent/events/ # or /etc/rodent/events/

# Each file represents one buffer flush
wc -l ~/.rodent/events/*.json

# Check Toggle connectivity
grep "Failed to send event batch" /var/log/rodent.log
```

**Note**: Disk files are created only when Toggle is unavailable. Once Toggle recovers, new events are sent directly via gRPC. The accumulated disk files represent the event history during the outage and can be processed separately if needed.

---

**For more information, see the event system source code in `internal/events/` or contact the development team.**
