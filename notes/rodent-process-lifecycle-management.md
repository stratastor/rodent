# Rodent Process Lifecycle Management

## Overview

Rodent implements a robust process lifecycle management system using multiple components:

- Single instance enforcement via PID files
- Signal handling for graceful shutdown
- Context-based cancellation
- Daemon mode support
- Configuration management
- Graceful HTTP server shutdown

## Architecture

### 1. Process Instance Management

Located in lifecycle.EnsureSingleInstance:

```go
func EnsureSingleInstance(pidPath string) error {
    // Check existing PID file
    if _, err := os.Stat(pidPath); err == nil {
        pidBytes, err := os.ReadFile(pidPath)
        // ...
        if err := process.Signal(syscall.Signal(0)); err == nil {
            return fmt.Errorf("another instance is already running (PID: %d)", pid)
        }
        // Remove stale PID file if process doesn't exist
    }
    // Write current PID
    currentPid := os.Getpid()
    if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", currentPid)), 0644); err != nil {
        return fmt.Errorf("failed to write PID file: %w", err)
    }
    // Clean up PID file on shutdown
    RegisterShutdownHook(func() {
        os.Remove(pidPath)
    })
    return nil
}
```

### 2. Signal Handling

lifecycle.HandleSignals manages process signals:

```go
func HandleSignals(ctx context.Context) {
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    for {
        select {
        case sig := <-stop:
            switch sig {
            case syscall.SIGTERM, syscall.SIGINT:
                shutdown()
                return
            case syscall.SIGHUP:
                reload()
            }
        case <-ctx.Done():
            return
        }
    }
}
```

### 3. Server Lifecycle

server.go implements graceful server management.

Gist of what's happening:

We're using Gin's Engine (gin.Default()) which provides:

- A router with middleware support
- Default middleware (Logger and Recovery)
- HTTP handler implementation (ServeHTTP)

When assigned to http.Server.Handler, we're using Gin's ServeHTTP method
since gin.Engine implements http.Handler interface

This gives us several benefits:

- Graceful Shutdown: Using http.Server gives us control over graceful shutdown through the Shutdown() method
- Context Integration: We can properly integrate with the application's context for lifecycle management
- Timeouts: We can set various timeouts (read, write, idle) on the server
- Error Handling: Better control over startup errors and shutdown process
- Middleware: Still have access to all of Gin's middleware and routing features
- Customization: Can configure additional http.Server options like TLS, custom error handlers, etc.

The main tradeoff is slightly more complex(strange?) code compared to gin.Run(), but the benefits of proper lifecycle management and graceful shutdown make it worthwhile for a production service.
This setup integrates well with our lifecycle package for signal handling and graceful shutdown.

```go
func Start(ctx context.Context, port int) error {
    router := gin.Default()
    srv = &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: router,
    }

    errChan := make(chan error, 1)
    go func() {
        if err := srv.ListenAndServe(); err != nil {
            if err != http.ErrServerClosed {
                errChan <- err
            }
        }
    }()

    select {
    case err := <-errChan:
        return fmt.Errorf("server startup failed: %w", err)
    case <-ctx.Done():
        return Shutdown(ctx)
    }
}
```

## Command Flow

**1. serve.go initiates the process:**

- Loads configuration
- Checks for existing instance
- Optionally daemonizes
- Sets up signal handlers
- Starts server

```go
func runServe(cmd *cobra.Command, args []string) {
    // Check single instance
    if err := lifecycle.EnsureSingleInstance(pidFile); err != nil {
        log.Error("Failed to start: %v", err)
        os.Exit(1)
    }

    if detached {
        // Daemonize if requested
        ctx := &daemon.Context{...}
        d, err := ctx.Reborn()
        // ...
    }

    startServer()
}
```

**2. Server startup sequence:**

- Creates context for lifecycle management
- Registers shutdown hooks
- Starts signal handler
- Initializes HTTP server

## Rationale

**1. PID File Management**

- Prevents multiple instances
- Handles stale PID files
- Cleanup on shutdown
**2. Signal Handling**

- Graceful shutdown on SIGTERM/SIGINT
- Configuration reload on SIGHUP
- Context-based cancellation
**3. HTTP Server**

- Uses Gin for routing but http.Server for lifecycle control
- Graceful shutdown support
- Error propagation during startup

## Potential Failure Points

**1. PID File Issues**

- Permission problems in run
- Race conditions during startup
- Stale PID files after crash

```go
// Mitigation in lifecycle.EnsureSingleInstance
if _, err := os.Stat(pidPath); err == nil {
    // Handles stale PID files
    if err := process.Signal(syscall.Signal(0)); err != nil {
        os.Remove(pidPath)
    }
}
```

**2. Server Startup**

- Port already in use
- Permission issues for privileged ports
- Network interface unavailability

```go
// Error handling in server.Start
case err := <-errChan:
    return fmt.Errorf("server startup failed: %w", err)
```

**3. Shutdown Race Conditions**

- Long-running requests during shutdown
- Multiple shutdown signals
- Context cancellation timing

```go
// Graceful shutdown in server.Shutdown
func Shutdown(ctx context.Context) error {
    if srv == nil {
        return nil
    }
    return srv.Shutdown(ctx)
}
```

## Best Practices

1. Always use lifecycle.RegisterShutdownHook for cleanup
2. Handle context cancellation in long-running operations
3. Use config.GetConfig() for configuration
4. Implement health checks for monitoring
5. Log important lifecycle events

This architecture ensures robust process management while maintaining clean shutdown capabilities and proper resource cleanup.
