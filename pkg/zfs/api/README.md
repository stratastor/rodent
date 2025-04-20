# ZFS API

This package provides both REST and gRPC APIs for ZFS dataset and pool operations.

## Overview

The ZFS API package provides a comprehensive interface for managing ZFS datasets and pools. It offers:

- REST API endpoints for HTTP-based interactions
- gRPC handlers for low-latency, binary protocol interactions
- Consistent error handling through the shared errors package
- Proper validation and security measures

## REST API

The REST API is implemented using the Gin web framework and provides traditional HTTP endpoints for:

- Dataset operations
- Pool operations
- Property management
- Snapshot and clone operations
- And more

See the dataset_api_doc.md and pool_api_doc.md files for detailed documentation of the REST API endpoints.

## gRPC API

The gRPC API provides the same functionality as the REST API but uses Google's Remote Procedure Call framework with Protocol Buffers for efficient serialization and communication.

All ZFS commands are mapped to specific gRPC command types and handlers in the base.proto definition file.

### Command Type Structure

Command requests follow this pattern in the proto definition:

```protobuf
message CommandRequest {
  string command_type = 1; // E.g., "zfs.dataset.create", "zfs.pool.status"
  string target = 2;       // Target resource (optional)
  bytes payload = 3;       // JSON-encoded command parameters
}
```

Where:
- `command_type` identifies the operation (e.g., "zfs.pool.list", "zfs.dataset.create")
- `payload` contains the necessary parameters as JSON

### Error Handling

gRPC responses include a structured error object when operations fail:

```protobuf
message CommandResponse {
  string request_id = 1;   // Matches the request
  bool success = 2;
  string message = 3;
  bytes payload = 4;       // JSON-encoded response data
  
  // Error information when success = false
  RodentError error = 5;   // Structured error information
}
```

This allows consumers to handle errors consistently across both REST and gRPC interfaces.

### Usage Example

To list all ZFS pools via gRPC:

```
// Command type: "zfs.pool.list"
// Payload: {} (empty JSON object)
```

To create a ZFS dataset:

```
// Command type: "zfs.dataset.create"
// Payload: {"name": "tank/myfs", "properties": {"mountpoint": "/mnt/myfs"}}
```

## Handler Implementation

Both REST and gRPC handlers follow consistent patterns:
1. Parse and validate input parameters
2. Call the appropriate manager method with context
3. Return structured responses with proper error handling

This ensures that both APIs behave similarly and provide consistent interfaces for consumers.