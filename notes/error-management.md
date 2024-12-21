# Rodent's Error Management System

## Overview

`errors.go` implements a comprehensive error management system for Rodent, designed to handle errors across different subsystems while providing structured error information for both API responses and logging.

## Core Components

### 1. Error Structure

The `RodentError` type is the foundation:

```go
type RodentError struct {
    Code       ErrorCode           `json:"code"`
    Domain     Domain             `json:"domain"`
    Message    string             `json:"message"`
    Details    string             `json:"details,omitempty"`
    HTTPStatus int               `json:"-"`
    Metadata   map[string]string `json:"metadata,omitempty"`
}
```

Key features:
    - Domain-based categorization
    - Unique error codes
    - HTTP status code mapping
    - Extensible metadata
    - JSON serialization support

### 2. Domain Classification

Domains help identify error sources:

```go
const (
    DomainConfig    Domain = "CONFIG"
    DomainServer    Domain = "SERVER"
    DomainZFS       Domain = "ZFS"
    DomainCommand   Domain = "CMD"
    DomainHealth    Domain = "HEALTH"
    DomainLifecycle Domain = "LIFECYCLE"
)
```

### 3. Error Code Ranges

Well-defined ranges for each subsystem:
    - 1000-1099: Configuration errors
    - 1100-1199: Server errors
    - 1200-1299: ZFS operations
    - 1300-1399: Command execution
    - 1400-1499: Health checks
    - 1500-1599: Lifecycle management

## Integration Points

### 1. Middleware Integration

`middleware.go` demonstrates structured error logging:

```go
if re, ok := err.Err.(*errors.RodentError); ok {
    attrs = append(attrs,
        slog.Int("error_code", int(re.Code)),
        slog.String("error_domain", string(re.Domain)),
        slog.String("error_message", re.Message),
        slog.String("error_details", re.Details),
    )
    // Add metadata as fields
    for k, v := range re.Metadata {
        attrs = append(attrs, slog.String("error_metadata_"+k, v))
    }
}
```

### 2. API Response Handling

Error information is automatically included in JSON responses:

```go
{
    "code": 1200,
    "domain": "ZFS",
    "message": "ZFS command execution failed",
    "details": "dataset not found",
    "metadata": {
        "dataset": "tank/test",
        "command": "zfs list",
        "exit_code": "1"
    },
    "timestamp": "2024-03-19T10:30:45Z"
}
```

## Usage Examples

### 1. Creating Errors

```go
// Simple error
err := errors.New(errors.ZFSCommandFailed, "dataset creation failed")

// With metadata
err := errors.New(errors.ZFSCommandFailed, "dataset creation failed").
    WithMetadata("dataset", "tank/test").
    WithMetadata("command", "zfs create")
```

### 2. Wrapping Errors

```go
if err := cmd.Run(); err != nil {
    return errors.Wrap(err, errors.CommandExecution)
}
```

### 3. Error Handling in Handlers

```go
func handleZFSOperation(c *gin.Context) {
    if err := zfs.CreateDataset(dataset); err != nil {
        if re, ok := err.(*errors.RodentError); ok {
            c.JSON(re.HTTPStatus, re)
            return
        }
        c.JSON(http.StatusInternalServerError, 
            errors.New(errors.ServerError, "internal error"))
    }
}
```

## Best Practices

1. Domain Usage:
    - Keep errors within their domain
    - Use appropriate error codes
    - Include relevant metadata
2. Error Creation:
    - Use `New()` for fresh errors
    - Use `Wrap()` for existing errors
    - Add context with metadata
3. Error Handling:
    - Check error types with `IsRodentError()`
    - Log all error fields
    - Preserve error context

## What Could Go Wrong

1. Error Wrapping Depth:
    - Multiple wraps can lose original context
    - Need to consider error unwrapping
2. Metadata Growth:
    - Unconstrained metadata can bloat logs
    - Need metadata validation/limits
3. Error Code Collisions:
    - Manual management of error codes
    - Need automated checks
4. Performance Impact:
    - Metadata map allocations
    - JSON marshaling overhead

## Future Improvements

### 1. Error Recovery

```go
type RecoveryHint struct {
    Suggestion string
    Action     func() error
}
```

### 2. Error Aggregation

```go
type ErrorAggregator interface {
    Add(error)
    GetErrors() []error
    HasErrors() bool
}
```

### 3. Stack Traces

```go
type RodentError struct {
    // ...existing fields
    Stack    []string `json:"stack,omitempty"`
}
```

### 4. Error Categories

```go
type ErrorCategory int

const (
    CategoryValidation ErrorCategory = iota
    CategorySystem
    CategoryBusiness
)
```

## Monitoring Integration

The error system provides data for monitoring:

1. Error Metrics:
    - Count by domain
    - Error code frequency
    - Response times
2. Alert Triggers:
    - Error rate thresholds
    - Critical error codes
    - System health impact

## Current Limitations

1. No built-in stack traces
2. Limited error recovery hints
3. No error aggregation
4. No error pattern detection

## Conclusion

Rodent's error management system provides a solid foundation for handling errors across the application. While there's room for improvement, the current implementation offers:
    - Structured error information
    - Clear domain separation
    - Rich contextual metadata
    - Integrated logging
    - API-friendly responses

Future work should focus on adding advanced features like stack traces, error recovery, and monitoring integration while maintaining the system's simplicity and usability.
