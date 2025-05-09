# Path Handling Updates for Rodent User

## Overview

To make Rodent run as a non-root `rodent` user, we've enhanced the path handling capabilities within the codebase. This document outlines the changes needed to ensure proper path resolution throughout the application.

## New Path Handling Functions

We've added several utility functions in `internal/common/path.go`:

1. `ExpandPath(path string) (string, error)`: Expands paths with a tilde (`~`) to the user's home directory
2. `GetConfigDir() (string, error)`: Returns the appropriate configuration directory based on user permissions
3. `EnsureDir(path string, perm os.FileMode) error`: Ensures a directory exists, creating it if necessary

## Code Updates Required

### 1. Update Constants

Current hardcoded paths in `internal/constants/constants.go` should be updated to use relative paths with tildes:

```go
// Before:
SystemConfigDir = "/etc/rodent"
UserConfigDir   = "~/.rodent"

// After:
SystemConfigDir = "~/.rodent"  // When running as rodent user
UserConfigDir   = "~/.rodent"  // Keeps consistency
```

### 2. Update SMB Configuration

In `pkg/shares/smb/manager.go`, the paths should be updated:

```go
// Before:
DefaultSMBConfigPath = "/etc/samba/smb.conf"
SharesConfigDir      = "/etc/samba/shares.d"
TemplateDir          = "/etc/rodent/templates/smb"

// After:
// These should be updated to reflect the rodent user paths
DefaultSMBConfigPath = "/etc/samba/smb.conf"  // This remains the same as it's a system file
SharesConfigDir      = "~/.rodent/shares/smb"  // User-specific share configs
TemplateDir          = "~/.rodent/templates/smb"  // User-specific templates
```

### 3. Update Code to Use Path Utilities

For any code that works with these paths:

```go
// Before:
configPath := filepath.Join(constants.SystemConfigDir, constants.ConfigFileName)

// After:
baseDir, err := common.GetConfigDir()
if err != nil {
    return err
}
configPath := filepath.Join(baseDir, constants.ConfigFileName)
```

### 4. Update Directory Creation

Any code that creates directories should use the new utility:

```go
// Before:
if err := os.MkdirAll(constants.SystemConfigDir, 0755); err != nil {
    return fmt.Errorf("failed to create config directory: %w", err)
}

// After:
if err := common.EnsureDir("~/.rodent", 0755); err != nil {
    return fmt.Errorf("failed to create config directory: %w", err)
}
```

## Implementation Strategy

1. First, add the new utility functions to `internal/common/path.go` ✅
2. Modify the setup script to create the correct directory structure ✅
3. Update each file that needs path handling:
   - `config/config.go`
   - `pkg/shares/smb/manager.go`
   - `pkg/keys/ssh/manager.go`
   - Any other modules that use hardcoded paths

## Migration Considerations

For existing installations:
- The setup script should detect existing configurations in `/etc/rodent` and copy them to `~/.rodent` ✅ 
- During system startup, check for and migrate configuration if needed