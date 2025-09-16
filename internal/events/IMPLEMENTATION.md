# Event System Implementation Guide

This guide provides comprehensive instructions for implementing and contributing to the Rodent event system with structured schema definitions and proper categorization.

## Overview

The event system has been redesigned with a centralized schema architecture that prevents dissonance between Rodent and Toggle services. All event definitions, categories, and payload structures are managed in the shared `toggle-rodent-proto` repository.

## Architecture

### Repository Structure

```text
toggle-rodent-proto/
├── proto/
│   ├── base.proto              # Core Event message and categories
│   └── events/
│       └── events.proto        # Event type enums and payload structures
├── go/events/
│   ├── constants.go            # Go constants for event types
│   └── helpers.go              # Utility functions and builders
└── Makefile                    # Proto generation commands

rodent/
└── internal/events/
    ├── schema.go               # Typed event emission functions
    ├── types.go                # Category definitions
    ├── integration.go          # Backward compatibility layer
    └── migration_examples.go   # Migration patterns
```

### Event Categories (8 total)

| Category | ID | Purpose | Modules |
|----------|----|---------|---------|
| `SYSTEM` | 1 | OS, hardware, local system operations | pkg/system |
| `STORAGE` | 2 | ZFS pools, datasets, transfers | pkg/zfs |
| `NETWORK` | 3 | Interfaces, routing, connectivity | pkg/netmage |
| `SECURITY` | 4 | SSH keys, certificates, auth | pkg/keys |
| `SERVICE` | 5 | Service lifecycle management | internal/services |
| `IDENTITY` | 6 | AD/LDAP user/group/computer operations | pkg/ad |
| `ACCESS` | 7 | ACL, permissions, access control | pkg/facl |
| `SHARING` | 8 | SMB/NFS shares, connections | pkg/shares |

## Implementation Guidelines

### 1. Adding New Event Types

#### Step 1: Define in Proto

Add new event types to `toggle-rodent-proto/proto/events/events.proto`:

```protobuf
// Add to appropriate EventType enum
enum StorageEventType {
  // ... existing types
  STORAGE_EVENT_TYPE_POOL_RESILVERED = 29;  // New event type
}

// Add corresponding payload message
message StoragePoolResilverPayload {
  string pool_name = 1;
  int64 duration_seconds = 2;
  int64 bytes_resilvered = 3;
  bool success = 4;
}
```

#### Step 2: Add Go Constants

Update `toggle-rodent-proto/go/events/constants.go`:

```go
// Storage Events
const (
  // ... existing constants
  StoragePoolResilvered = "storage.pool.resilvered"
)
```

#### Step 3: Add Rodent Integration

Update `rodent/internal/events/schema.go`:

```go
func EmitStoragePoolResilver(payload *eventspb.StoragePoolResilverPayload, metadata map[string]string) {
  emitTypedEvent(eventsconstants.StoragePoolResilvered, LevelInfo, CategoryStorage, "zfs-pool-manager", payload, metadata)
}
```

### 2. Module Event Integration

#### For Existing Modules (Migration)

#### Pattern: Replace scattered events with structured events

❌ **Old Approach:**

```go
events.EmitStorageEvent("storage.dataset.created", events.LevelInfo, "zfs-manager",
  map[string]interface{}{
    "dataset": datasetName,
    "pool": poolName,
  },
  map[string]string{
    "component": "zfs",
  })
```

✅ **New Approach:**

```go
import (
  eventsconstants "github.com/stratastor/toggle-rodent-proto/go/events"
  eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
)

payload := &eventspb.StorageDatasetPayload{
  DatasetName: datasetName,
  PoolName:    poolName,
  Type:        "filesystem",
  Mountpoint:  mountpoint,
}

metadata := map[string]string{
  eventsconstants.MetaComponent:    "zfs-dataset-manager",
  eventsconstants.MetaAction:       "create",
  eventsconstants.MetaResourceType: "dataset",
  eventsconstants.MetaResourceName: datasetName,
}

events.EmitStorageDataset(eventsconstants.StorageDatasetCreated, events.LevelInfo, payload, metadata)
```

#### For New Modules (Fresh Implementation)

#### Step 1: Identify Events

List all significant operations that should generate events:

- Resource lifecycle (create, update, delete)
- State changes (start, stop, enable, disable)
- Error conditions (failures, violations, thresholds)
- Access events (granted, denied)

#### Step 2: Choose Appropriate Category

- `IDENTITY` - AD/LDAP operations
- `ACCESS` - Permission/ACL changes
- `SHARING` - File sharing operations
- etc.

#### Step 3: Implement Event Emissions

Add events at key points in your module:

```go
// Example: pkg/ad/user_manager.go
func (am *ADManager) CreateUser(ctx context.Context, req CreateUserRequest) error {
  // ... user creation logic

  if err := am.ldapClient.CreateUser(user); err != nil {
    return err
  }

  // Emit structured event
  payload := &eventspb.IdentityUserPayload{
    Username:    req.Username,
    DisplayName: req.DisplayName,
    Email:       req.Email,
    Groups:      req.Groups,
    Domain:      am.domain,
    Enabled:     true,
  }

  metadata := map[string]string{
    eventsconstants.MetaComponent: "ad-manager",
    eventsconstants.MetaAction:    "create",
    eventsconstants.MetaUser:      req.Username,
    eventsconstants.MetaDomain:    am.domain,
  }

  events.EmitIdentityUser(eventsconstants.IdentityUserCreated, events.LevelInfo, payload, metadata)
  return nil
}
```

### 3. Standardized Metadata Keys

Always use constants from `eventsconstants` package:

```go
// Standard keys
eventsconstants.MetaComponent      // Source component
eventsconstants.MetaAction         // Operation type
eventsconstants.MetaUser           // Username involved
eventsconstants.MetaResourceType   // Type of resource
eventsconstants.MetaResourceName   // Name/ID of resource

// Domain-specific keys
eventsconstants.MetaPool           // ZFS pool name
eventsconstants.MetaDataset        // ZFS dataset name
eventsconstants.MetaShareName      // Share name
eventsconstants.MetaDomain         // AD domain
eventsconstants.MetaInterface      // Network interface
```

### 4. Event Levels

Use appropriate levels for different event types:

```go
events.LevelInfo      // Normal operations (create, update, start)
events.LevelWarn      // Warning conditions (delete, stop, threshold)
events.LevelError     // Error conditions (failures, denied access)
events.LevelCritical  // Critical issues (security violations, data loss)
```

### 5. Testing Events

Add event verification to your tests:

```go
func TestUserCreation(t *testing.T) {
  // Test setup...

  err := userManager.CreateUser(ctx, request)
  require.NoError(t, err)

  // Verify event was emitted
  stats := events.GetStats()
  assert.True(t, stats["initialized"].(bool))

  // For integration tests, verify event content reaches Toggle
}
```

## Module-Specific Implementation Status

### ✅ Completed Modules

| Module | Status | Events Implemented |
|--------|--------|--------------------|
| **pkg/zfs/dataset** | ✅ Migrated | Transfer lifecycle events |
| **pkg/system** | ✅ Migrated | Local user management |
| **pkg/server** | ✅ Migrated | Server lifecycle |
| **internal/toggle** | ✅ Migrated | System startup |

### 🔄 Pending Modules

| Module | Category | Priority | Key Events Needed |
|--------|----------|----------|-------------------|
| **pkg/ad** | IDENTITY | High | User/group/computer CRUD, domain sync |
| **pkg/facl** | ACCESS | High | ACL changes, permission grants/denials |
| **pkg/shares** | SHARING | High | Share lifecycle, connections, access |
| **pkg/netmage** | NETWORK | Medium | Interface changes, connectivity |
| **pkg/keys/ssh** | SECURITY | Medium | Key generation, peering |
| **internal/services** | SERVICE | Medium | Service management operations |
| **pkg/zfs/pool** | STORAGE | Low | Pool operations, scrubs |
| **pkg/zfs/snapshot** | STORAGE | Low | Snapshot operations |

## Contributing Guidelines

### 1. Before Adding Events

#### Check existing definitions

1. Review `toggle-rodent-proto/proto/events/events.proto` for similar events
2. Check `toggle-rodent-proto/go/events/constants.go` for naming patterns
3. Ensure your event fits an existing category

#### Design considerations

- Use descriptive, hierarchical event types (`storage.pool.scrub.completed`)
- Include all relevant context in payload structures
- Follow existing naming conventions
- Consider Toggle's filtering and monitoring needs

### 2. Development Process

1. **Proto First**: Define event types and payloads in proto files
2. **Generate Bindings**: Run `make generate` in `toggle-rodent-proto` repository
3. **Add Constants**: Update constants file with new event types
4. **Implement Integration**: Add typed emission functions
5. **Update Module**: Add event emissions to module code
6. **Test**: Verify events are emitted correctly
7. **Document**: Update implementation guide

### 3. Code Review Checklist

- [ ] Event type defined in appropriate proto enum
- [ ] Payload structure includes all relevant context
- [ ] Go constants follow naming conventions
- [ ] Metadata uses standardized keys
- [ ] Appropriate event level used
- [ ] Proper category assigned
- [ ] Tests include event verification
- [ ] Documentation updated

### 4. Proto Generation

After making changes to proto files, regenerate Go bindings:

```bash
cd toggle-rodent-proto
make generate
```

Then update Rodent dependencies:

```bash
cd rodent
go mod vendor
```

## Event Filtering Configuration

Events can be filtered by level and category:

```yaml
# rodent.yml
events:
  profile: "default"  # default, high-throughput, low-latency, minimal
  enabled_levels: ["info", "warn", "error", "critical"]
  enabled_categories: ["system", "storage", "network", "security", "service", "identity", "access", "sharing"]
```

## Troubleshooting

### Common Issues

#### 1. Import Errors

```text
could not import github.com/stratastor/toggle-rodent-proto/proto/events
```

**Solution:** Run `make generate` in `toggle-rodent-proto`, then `go mod vendor` in rodent

#### 2. Event Not Appearing in Toggle

- Check if event system is initialized (`events.IsInitialized()`)
- Verify JWT configuration
- Check event filtering configuration
- Monitor event buffer stats (`events.GetStats()`)

#### 3. Proto Compilation Errors

- Ensure protoc is installed and updated
- Check proto syntax and field numbering
- Verify import paths
- Use `make generate` instead of manual protoc commands

---

## Quick Reference

### Import Pattern

```go
import (
  eventsconstants "github.com/stratastor/toggle-rodent-proto/go/events"
  eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
)
```

### Event Emission Pattern

```go
payload := &eventspb.CategoryPayload{
  // Structured fields
}

metadata := map[string]string{
  eventsconstants.MetaComponent: "module-name",
  eventsconstants.MetaAction:    "operation",
}

events.EmitCategory(eventsconstants.EventType, events.LevelInfo, payload, metadata)
```

### Backward Compatibility

All existing `events.EmitCategoryEvent()` functions continue to work during migration period.
