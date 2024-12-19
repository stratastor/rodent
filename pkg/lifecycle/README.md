# Rodent Lifecycle Management Implementation

## Core Components

### 1. Shutdown Hooks Registry

```go
var shutdownHooks []func()

func RegisterShutdownHook(hook func()) {
    shutdownHooks = append(shutdownHooks, hook)
}
```

Purpose:

- Maintains list of cleanup functions
- Called during graceful shutdown
- Order: Last registered, first executed

### 2. Signal Handler

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

Signal Handling:

- SIGTERM/SIGINT: Graceful shutdown
- SIGHUP: Configuration reload
- Context cancellation: Clean exit

### 3. Integration Points

Server Integration:

```go
func Start(ctx context.Context, port int) error {
    // Register server shutdown
    lifecycle.RegisterShutdownHook(func() {
        if err := srv.Shutdown(ctx); err != nil {
            log.Error("Server shutdown error: %v", err)
        }
    })

    // Start signal handler
    go lifecycle.HandleSignals(ctx)
    
    // ...server startup code...
}
```

PID File Management:

```go
func EnsureSingleInstance(pidPath string) error {
    // ...PID file checks...

    // Register PID cleanup
    RegisterShutdownHook(func() {
        os.Remove(pidPath)
    })
    
    return nil
}
```

### 4. Shutdown Sequence

```go
func shutdown() {
    for i := len(shutdownHooks) - 1; i >= 0; i-- {
        shutdownHooks[i]()
    }
}
```

Shutdown Flow:

- Signal received (SIGTERM/SIGINT)
- shutdown() called
- Hooks executed in reverse order
- Resources cleaned up
- Process exits

### 5. Context Integration

```go
func startServer() error {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    lifecycle.RegisterShutdownHook(func() {
        cancel() // Triggers context cancellation
    })

    return server.Start(ctx, cfg.Server.Port)
}
```

Context Propagation:

- Root context created in serve command
- Passed to server component
- Cancellation triggers cleanup
- Propagates through component tree

## Critical Paths

1. **Startup:**
    - Load configuration
    - Check single instance
    - Register hooks
    - Start signal handler
    - Initialize server

2. **Running:**
    - Monitor signals
    - Handle SIGHUP for reload
    - Maintain PID file
    - Serve requests

3. **Shutdown:**
    - Receive signal
    - Cancel context
    - Execute hooks
    - Clean resources
    - Exit process

This implementation ensures:

- Clean resource cleanup
- Graceful shutdown
- Signal handling
- Single instance enforcement
- Configuration reloading