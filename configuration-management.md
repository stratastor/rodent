# Configuration Management in Rodent

## Overview

Rodent implements a singleton configuration pattern using Go's sync.Once mechanism, with support for both initial loading and runtime reloading through multiple interfaces.

## Core Components

1. Global State

    ```go
    var (
        instance   *Config     // Singleton config instance
        once       sync.Once   // Ensures single initialization
        configPath string      // Tracks loaded config location
    )
    ```

2. Configuration Structure

The Config struct defines all configurable parameters:

```go
type Config struct {
    Server struct {
        Port      int    
        LogLevel  string
        Daemonize bool   
    }
    Health struct {
        Interval string
        Endpoint string 
    }
    Logs struct {
        Path      string
        Retention string
        Output    string
    }
    Logger struct {
        LogLevel     string
        EnableSentry bool  
        SentryDSN    string
    }
    Environment string
}
```

## Loading Mechanism

### Initial Load

The LoadConfig function uses sync.Once to ensure single initialization:

```go
func LoadConfig(configFilePath string) *Config {
    once.Do(func() {
        // Initialize defaults
        // Load from file
        // Apply environment variables
        instance = &cfg
    })
    return instance
}
```

### Configuration Sources (Priority Order)

1. Command line argument
2. RODENT_CONFIG environment variable
3. Current working directory (./rodent.yml)
4. User config (~/.rodent/rodent.yml)
5. System config (/etc/rodent/rodent.yml)

## Hot Reload Support

### Current Implementation

The current use of sync.Once creates a challenge for hot reloading since it prevents re-execution of the initialization block. This affects:

1. rodent config load command
2. SIGHUP signal handling
3. Runtime configuration updates

### Proposed Solutions

1. Replace sync.Once:

    ```go
    var (
        instance   *Config
        mu         sync.RWMutex
    )

    func LoadConfig(path string) *Config {
        mu.Lock()
        defer mu.Unlock()
        // Load configuration
        return instance
    }
    ```

2. Maintain State Separately:

    ```go
    type ConfigManager struct {
        config     *Config
        mu         sync.RWMutex
        onReload   []func(*Config)
    }
    ```

## Future Development

1. Safe Reloading

    Implement atomic configuration updates
    Add validation before applying changes
    Support partial updates

2. Change Notification

    ```go
    type ConfigManager struct {
        // ...
        subscribers []chan<- ConfigChange
    }

    type ConfigChange struct {
        Path  string
        Old   interface{}
        New   interface{}
    }
    ```

## Design Implications

### Advantages

1. Thread Safety: Current implementation guarantees safe initialization
2. Predictable Loading: Clear precedence rules for config sources
3. Default Values: Built-in fallback for missing configurations

### Limitations

1. Hot Reload: sync.Once prevents proper reloading
2. Validation: Limited validation of configuration values
3. Change Tracking: No built-in way to track configuration changes

### Recommendations

1. Replace sync.Once with mutex-based synchronization
2. Implement proper validation framework
3. Add configuration versioning
4. Support atomic updates
5. Add change notification system

## Code Example: Improved Implementation

```go
type ConfigManager struct {
    config     atomic.Value
    mu         sync.RWMutex
    validators []func(*Config) error
    listeners  []chan<- ConfigChange
}

func (cm *ConfigManager) Load(path string) error {
    cm.mu.Lock()
    defer cm.mu.Unlock()

    newCfg, err := loadFromFile(path)
    if err != nil {
        return err
    }

    if err := cm.validate(newCfg); err != nil {
        return err
    }

    oldCfg := cm.config.Load().(*Config)
    cm.config.Store(newCfg)
    cm.notifyListeners(oldCfg, newCfg)
    return nil
}
```

This improved design would better support features like hot reloading while maintaining thread safety and adding new capabilities for configuration management.
