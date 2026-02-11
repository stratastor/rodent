# HTTP Tunnel Over gRPC Stream

Generic HTTP proxy command (`tunnel.http`) that forwards HTTP requests to local services on Rodent through the existing gRPC bidirectional stream. No new tunnel — reuses the established connection, auth, session registry, and reconnection logic.

## Connection Model

Rodent dials out to Toggle (client-initiated). Toggle is the gRPC server.

- **Proto**: `RodentService.Connect(stream RodentRequest) returns (stream ToggleRequest)` — bidirectional stream
- **Auth**: JWT Bearer token in gRPC metadata (`"prv"` claim = private network)
- **Rodent entry**: `pkg/server/server.go:58` → `toggle.StartRegistrationProcess()` → `internal/toggle/register.go` → `establishStreamConnection()`
- **Toggle entry**: `internal/grpc/server/server.go` `Connect()` method

## Command Flow (Toggle → Rodent)

```
HTTP Client
  → Toggle Gin route (e.g., /api/v1/rodent-proxy/api/v1/rodent/disks)
    → middleware: UserAuth → RodentNodeMiddleware → RequireRodentPermission → AddContextMiddleware → Audit
      → handleRodentProxy() [internal/server/rodent_proxy.go:174]
        → GRPCProxyHandler.Handle() [internal/server/grpc_proxy.go:29]
          → isProxyAPIPath() → RodentGRPCServer.HandleGRPCProxy()
            → domain handler switch → createPayload() → extractCommandType()
            → handler.processRequest() → SendCommand() → stream.Send()
            → WaitForResponse() (Redis pub/sub) → TransformGRPCToRest()
  ← HTTP response
```

## Command Flow (Rodent side)

```
gRPC stream.Recv()
  → receiveLoop() → inboundChan
    → StartMessageHandler() → processToggleRequest()
      → handleCommandRequest()
        → commandHandlers[commandType] lookup (RegisterCommandHandler)
        → handler(toggleReq, cmd) → CommandResponse
        → stream.Send(RodentRequest{command_response})
```

## Key Proto Types

```protobuf
CommandRequest  { command_type: string, target: string, payload: bytes }
CommandResponse { request_id: string, success: bool, message: string, payload: bytes, error: RodentError }
```

## Tunnel Design

A single command type `tunnel.http` serializes a full HTTP request/response through the existing stream.

```
User HTTP request
  → Toggle: /api/v1/rodent-proxy/api/v1/rodent/tunnel/:service/*path
    → Auth (JWT + SpiceDB "manage") → Audit
    → Pack full HTTP request into CommandRequest.payload
    → SendCommand() over gRPC stream → Redis pub/sub correlation
  → Rodent: receives tunnel.http command
    → Validate service against config allowlist (name, path prefix, method)
    → http.Client call to local service address
    → Pack HTTP response into CommandResponse.payload
  ← Toggle: unpack response, write raw status/headers/body back to caller
```

### Wire Format

**Request** (Toggle → Rodent, `CommandRequest.payload`):
```json
{"service":"cubejs","method":"GET","path":"/cubejs-api/v1/load","query":"q=...","headers":{"Content-Type":"application/json"},"body":"<base64>"}
```

**Response** (Rodent → Toggle, `CommandResponse.payload`):
```json
{"status_code":200,"headers":{"Content-Type":"application/json"},"body":"<base64>"}
```

### Separate Code Path on Toggle

The tunnel handler bypasses the shared `processDomainRequest` flow via early return in `HandleGRPCProxy()`. Domain handlers use `createPayload()` (flattens path params + query + body into merged JSON) and `sendResponse()` / `TransformGRPCToRest()` (JSON-unwrap and re-wrap). The tunnel needs raw HTTP preserved in both directions, so it has its own pack/unpack logic and writes the proxied service's status code, headers, and body directly.

## Rodent Configuration

```yaml
tunnel:
  services:
    cubejs:
      address: "http://localhost:4000"
      allowedPaths: ["/cubejs-api/"]
      allowedMethods: ["GET", "POST"]
      timeout: "30s"
```

Only services in this allowlist are reachable. Path prefixes and methods are validated per-service. Handler registration is conditional on config having services defined.

## API Examples

```
GET /api/v1/rodent-proxy/api/v1/rodent/tunnel/cubejs/cubejs-api/v1/load?query=...
X-Rodent-Id: <rodent-id>
Authorization: Bearer <user-jwt>

POST /api/v1/rodent-proxy/api/v1/rodent/tunnel/clickhouse/query
X-Rodent-Id: <rodent-id>
Authorization: Bearer <user-jwt>
Content-Type: application/json
{"query": "SELECT count() FROM events"}
```

## Security

- **Rodent-side allowlist**: only config-listed services are reachable; path prefix and method validated per-service
- **Toggle-side AuthZ**: SpiceDB `manage` permission check on tunnel routes
- **Audit trail**: all tunnel requests logged via existing audit middleware
- **No arbitrary port access**: service → address mapping is config-driven, not user-controlled

## Files

**toggle-rodent-proto**: `proto/tunnel_command_types.go` — `CmdTunnelHTTP = "tunnel.http"`

**rodent**:
- `config/config.go` — `Tunnel` config section, `TunnelService` type
- `internal/tunnel/handler.go` — HTTP proxy logic (validate, forward, serialize)
- `internal/tunnel/handler_grpc.go` — `RegisterTunnelGRPCHandlers()`, command handler
- `internal/toggle/handlers.go` — calls `tunnel.RegisterTunnelGRPCHandlers()`

**toggle**:
- `internal/server/tunnel_routes.go` — Gin routes, `Any("/:service/*path")`
- `internal/grpc/server/tunnel_handlers.go` — `handleTunnelProxy()` on `RodentGRPCServer`
- `internal/server/routes.go` — wires `registerTunnelProxyRoutes()`
- `internal/server/grpc_proxy.go` — tunnel paths in `isProxyAPIPath()`
- `internal/grpc/server/server_handle_proxy.go` — early return for tunnel

## Limitations

- **Request-response only** — no WebSocket/SSE/long-polling without stream-level changes
- **16MB message limit** — gRPC max message size on both sides; large payloads need chunking
- **~10-50ms overhead** — gRPC serialization + Redis pub/sub correlation per request
- **Shared stream** — tunnel traffic shares the gRPC stream with domain commands (mitigated by per-command goroutines)
