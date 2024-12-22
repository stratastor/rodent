package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ErrorCode represents unique error identifiers
type ErrorCode int

// Domain represents the subsystem where the error originated
type Domain string

const (
	DomainConfig    Domain = "CONFIG"
	DomainServer    Domain = "SERVER"
	DomainZFS       Domain = "ZFS"
	DomainCommand   Domain = "CMD"
	DomainHealth    Domain = "HEALTH"
	DomainLifecycle Domain = "LIFECYCLE"
)

// Error code ranges:
// 1000-1099: Configuration errors
// 1100-1199: Server errors
// 1200-1299: ZFS operations
// 1300-1399: Command execution
// 1400-1499: Health check
// 1500-1599: Lifecycle management
// Domain-specific error code ranges:
const (
	// Configuration Errors (1000-1099)
	ConfigNotFound           = 1000 + iota // Config file not found
	ConfigInvalid                          // Invalid config format
	ConfigLoadFailed                       // Failed to load config
	ConfigWriteFailed                      // Failed to write config
	ConfigPermissionDenied                 // Permission denied accessing config
	ConfigDirectoryError                   // Config directory error
	ConfigValidationFailed                 // Config validation failed
	ConfigMarshalFailed                    // Config serialization failed
	ConfigUnmarshalFailed                  // Config deserialization failed
	ConfigHomeDirectoryError               // Error getting home directory
)
const (
	// Server Errors (1100-1199)
	ServerStart             = 1100 + iota // Failed to start server
	ServerShutdown                        // Error during shutdown
	ServerBind                            // Failed to bind port
	ServerTimeout                         // Operation timeout
	ServerMiddleware                      // Middleware error
	ServerRouting                         // Routing error
	ServerRequestValidation               // Request validation failed
	ServerResponseError                   // Response generation error
	ServerContextCancelled                // Context cancelled
	ServerTLSError                        // TLS configuration error
)

const (
	// TODO: Remove redundant error codes
	// ZFS Operations (1200-1299)
	ZFSCommandFailed    = 1200 + iota // ZFS command execution failed
	ZFSPoolNotFound                   // Pool not found
	ZFSPermissionDenied               // Permission denied
	ZFSPropertyError                  // Property operation failed
	ZFSMountError                     // Mount operation failed
	ZFSCloneError                     // Clone operation failed
	ZFSQuotaError                     // Quota operation failed
	ZFSIOError                        // I/O error during operation
	ZFSInvalidSize
	ZFSQuotaExceeded
	ZFSPermissionError

	ZFSDatasetNotFound // Dataset not found
	ZFSDatasetCreate
	ZFSDatasetList
	ZFSDatasetDestroy
	ZFSDatasetGetProperty
	ZFSDatasetSetProperty
	ZFSDatasetPropertyNotFound
	ZFSDatasetClone
	ZFSDatasetInvalidName
	ZFSDatasetInvalidProperty
	ZFSDatasetRename
	ZFSDatasetSnapshot

	ZFSSnapshotList
	ZFSSnapshotDestroy
	ZFSSnapshotRollback
	ZFSSnapshotFailed // Snapshot operation failed
	ZFSSnapshotInvalidName
	ZFSSnapshotInvalidProperty

	ZFSBookmarkFailed
	ZFSBookmarkInvalidName
	ZFSBookmarkInvalidProperty

	ZFSClonePromoteFailed
	ZFSMountOperationFailed
	ZFSUnmountOperationFailed
	ZFSPoolScrubFailed
	ZFSPoolResilverFailed

	ZFSVolumeOperationFailed

	ZFSPoolCreate
	ZFSPoolImport
	ZFSPoolExport
	ZFSPoolStatus
	ZFSPoolList
	ZFSPoolDestroy
	ZFSPoolGetProperty
	ZFSPoolSetProperty
	ZFSPoolPropertyNotFound
	ZFSPoolInvalidName
	ZFSPoolInvalidDevice
)

const (
	// Command Execution (1300-1399)
	CommandNotFound     = 1300 + iota // Command not found
	CommandExecution                  // Execution failed
	CommandTimeout                    // Command timed out
	CommandPermission                 // Permission denied
	CommandInvalidInput               // Invalid command input
	CommandOutputParse                // Output parsing failed
	CommandSignal                     // Signal handling failed
	CommandContext                    // Context handling error
	CommandPipe                       // Command pipe error
	CommandWorkDir                    // Working directory error
)

const (
	// Health Check (1400-1499)
	HealthCheckFailed     = 1400 + iota // Health check failed
	HealthCheckTimeout                  // Health check timed out
	HealthCheckComponent                // Component check failed
	HealthCheckConfig                   // Health check config error
	HealthCheckEndpoint                 // Endpoint error
	HealthCheckClient                   // Client error
	HealthCheckValidation               // Validation error
	HealthCheckThreshold                // Threshold exceeded
	HealthCheckState                    // State transition error
	HealthCheckRecovery                 // Recovery failed
)

const (
	// Lifecycle Management (1500-1599)
	LifecyclePID      = 1500 + iota // PID file operation failed
	LifecycleShutdown               // Shutdown process error
	LifecycleSignal                 // Signal handling error
	LifecycleReload                 // Config reload failed
	LifecycleHook                   // Lifecycle hook error
	LifecycleState                  // State transition error
	LifecycleLock                   // Lock acquisition failed
	LifecycleCleanup                // Cleanup operation failed
	LifecycleDaemon                 // Daemon operation failed
	LifecycleResource               // Resource management error
)

type RodentError struct {
	Code    ErrorCode `json:"code"`
	Domain  Domain    `json:"domain"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	// Context    string           `json:"context,omitempty"`
	HTTPStatus int `json:"-"`

	// The Metadata field is designed for additional contextual information
	// that doesn't fit into the standard error fields but is valuable for
	// debugging and API responses. It's particularly useful for:
	// - API responses where JSON serialization includes the metadata
	// - Logging with structured details
	// - Debugging with command-specific information
	// - Error tracking/monitoring systems
	Metadata map[string]string `json:"metadata,omitempty"`
}

var errorDefinitions = map[ErrorCode]struct {
	message    string
	domain     Domain
	httpStatus int
}{
	// Configuration errors
	ConfigNotFound:           {"Configuration file not found", DomainConfig, http.StatusNotFound},
	ConfigInvalid:            {"Invalid configuration format", DomainConfig, http.StatusBadRequest},
	ConfigLoadFailed:         {"Failed to load configuration", DomainConfig, http.StatusInternalServerError},
	ConfigWriteFailed:        {"Failed to write configuration", DomainConfig, http.StatusInternalServerError},
	ConfigPermissionDenied:   {"Permission denied accessing config", DomainConfig, http.StatusForbidden},
	ConfigDirectoryError:     {"Config directory error", DomainConfig, http.StatusInternalServerError},
	ConfigValidationFailed:   {"Configuration validation failed", DomainConfig, http.StatusBadRequest},
	ConfigMarshalFailed:      {"Failed to serialize configuration", DomainConfig, http.StatusInternalServerError},
	ConfigUnmarshalFailed:    {"Failed to deserialize configuration", DomainConfig, http.StatusInternalServerError},
	ConfigHomeDirectoryError: {"Failed to get home directory", DomainConfig, http.StatusInternalServerError},

	// Server errors
	ServerStart:             {"Failed to start server", DomainServer, http.StatusInternalServerError},
	ServerShutdown:          {"Error during server shutdown", DomainServer, http.StatusInternalServerError},
	ServerBind:              {"Failed to bind server port", DomainServer, http.StatusInternalServerError},
	ServerTimeout:           {"Server operation timed out", DomainServer, http.StatusGatewayTimeout},
	ServerMiddleware:        {"Middleware execution failed", DomainServer, http.StatusInternalServerError},
	ServerRouting:           {"Route handling error", DomainServer, http.StatusInternalServerError},
	ServerRequestValidation: {"Request validation failed", DomainServer, http.StatusBadRequest},
	ServerResponseError:     {"Error generating response", DomainServer, http.StatusInternalServerError},
	ServerContextCancelled:  {"Server context cancelled", DomainServer, http.StatusServiceUnavailable},
	ServerTLSError:          {"TLS configuration error", DomainServer, http.StatusInternalServerError},

	// ZFS errors
	ZFSCommandFailed:    {"ZFS command execution failed", DomainZFS, http.StatusInternalServerError},
	ZFSPoolNotFound:     {"ZFS pool not found", DomainZFS, http.StatusNotFound},
	ZFSPermissionDenied: {"Permission denied for ZFS operation", DomainZFS, http.StatusForbidden},
	ZFSPropertyError:    {"ZFS property operation failed", DomainZFS, http.StatusInternalServerError},
	ZFSMountError:       {"ZFS mount operation failed", DomainZFS, http.StatusInternalServerError},
	ZFSQuotaError:       {"ZFS quota operation failed", DomainZFS, http.StatusInternalServerError},
	ZFSIOError:          {"ZFS I/O operation failed", DomainZFS, http.StatusInternalServerError},

	ZFSBookmarkFailed:  {"Failed to create/list bookmark", DomainZFS, http.StatusInternalServerError},
	ZFSQuotaExceeded:   {"Dataset quota exceeded", DomainZFS, http.StatusForbidden},
	ZFSPermissionError: {"Permission denied for ZFS operation", DomainZFS, http.StatusForbidden},
	ZFSInvalidSize:     {"Invalid volume size specified", DomainZFS, http.StatusBadRequest},

	ZFSCloneError: {"ZFS clone operation failed", DomainZFS, http.StatusInternalServerError},

	ZFSDatasetCreate:           {"Failed to create ZFS dataset", DomainZFS, http.StatusInternalServerError},
	ZFSDatasetNotFound:         {"ZFS dataset not found", DomainZFS, http.StatusNotFound},
	ZFSDatasetList:             {"Failed to list ZFS datasets", DomainZFS, http.StatusInternalServerError},
	ZFSDatasetDestroy:          {"Failed to destroy ZFS dataset", DomainZFS, http.StatusInternalServerError},
	ZFSDatasetGetProperty:      {"Failed to get dataset property", DomainZFS, http.StatusInternalServerError},
	ZFSDatasetSetProperty:      {"Failed to set dataset property", DomainZFS, http.StatusInternalServerError},
	ZFSDatasetPropertyNotFound: {"Dataset property not found", DomainZFS, http.StatusNotFound},
	ZFSDatasetClone:            {"Failed to clone dataset", DomainZFS, http.StatusInternalServerError},
	ZFSDatasetInvalidName:      {"Invalid dataset name", DomainZFS, http.StatusBadRequest},
	ZFSDatasetInvalidProperty:  {"Invalid property value", DomainZFS, http.StatusBadRequest},
	ZFSDatasetRename:           {"Failed to rename dataset", DomainZFS, http.StatusInternalServerError},
	ZFSDatasetSnapshot:         {"Failed to create snapshot", DomainZFS, http.StatusInternalServerError},

	ZFSSnapshotList:            {"Failed to list snapshots", DomainZFS, http.StatusInternalServerError},
	ZFSSnapshotDestroy:         {"Failed to destroy snapshot", DomainZFS, http.StatusInternalServerError},
	ZFSSnapshotRollback:        {"Failed to rollback snapshot", DomainZFS, http.StatusInternalServerError},
	ZFSSnapshotFailed:          {"Failed to create/manage snapshot", DomainZFS, http.StatusInternalServerError},
	ZFSSnapshotInvalidName:     {"Invalid snapshot name", DomainZFS, http.StatusBadRequest},
	ZFSSnapshotInvalidProperty: {"Invalid snapshot property value", DomainZFS, http.StatusBadRequest},

	ZFSBookmarkInvalidName:     {"Invalid bookmark name", DomainZFS, http.StatusBadRequest},
	ZFSBookmarkInvalidProperty: {"Invalid bookmark property value", DomainZFS, http.StatusBadRequest},

	ZFSPoolCreate:           {"Failed to create ZFS pool", DomainZFS, http.StatusInternalServerError},
	ZFSPoolImport:           {"Failed to import ZFS pool", DomainZFS, http.StatusInternalServerError},
	ZFSPoolExport:           {"Failed to export ZFS pool", DomainZFS, http.StatusInternalServerError},
	ZFSPoolStatus:           {"Failed to get pool status", DomainZFS, http.StatusInternalServerError},
	ZFSPoolList:             {"Failed to get pool list", DomainZFS, http.StatusInternalServerError},
	ZFSPoolDestroy:          {"Failed to destroy pool", DomainZFS, http.StatusInternalServerError},
	ZFSPoolGetProperty:      {"Failed to get pool property", DomainZFS, http.StatusInternalServerError},
	ZFSPoolSetProperty:      {"Failed to set pool property", DomainZFS, http.StatusInternalServerError},
	ZFSPoolPropertyNotFound: {"Pool property not found", DomainZFS, http.StatusNotFound},
	ZFSPoolInvalidName:      {"Invalid pool name", DomainZFS, http.StatusBadRequest},
	ZFSPoolInvalidDevice:    {"Invalid device", DomainZFS, http.StatusBadRequest},

	// Command execution errors
	CommandNotFound:     {"Command not found", DomainCommand, http.StatusNotFound},
	CommandExecution:    {"Command execution failed", DomainCommand, http.StatusInternalServerError},
	CommandTimeout:      {"Command execution timed out", DomainCommand, http.StatusGatewayTimeout},
	CommandPermission:   {"Permission denied executing command", DomainCommand, http.StatusForbidden},
	CommandInvalidInput: {"Invalid command input", DomainCommand, http.StatusBadRequest},
	CommandOutputParse:  {"Failed to parse command output", DomainCommand, http.StatusInternalServerError},
	CommandSignal:       {"Command signal handling failed", DomainCommand, http.StatusInternalServerError},
	CommandContext:      {"Command context error", DomainCommand, http.StatusInternalServerError},
	CommandPipe:         {"Command pipe operation failed", DomainCommand, http.StatusInternalServerError},
	CommandWorkDir:      {"Working directory error", DomainCommand, http.StatusInternalServerError},

	// Health check errors
	HealthCheckFailed:     {"Health check failed", DomainHealth, http.StatusServiceUnavailable},
	HealthCheckTimeout:    {"Health check timed out", DomainHealth, http.StatusGatewayTimeout},
	HealthCheckComponent:  {"Component health check failed", DomainHealth, http.StatusServiceUnavailable},
	HealthCheckConfig:     {"Health check configuration error", DomainHealth, http.StatusInternalServerError},
	HealthCheckEndpoint:   {"Health check endpoint error", DomainHealth, http.StatusServiceUnavailable},
	HealthCheckClient:     {"Health check client error", DomainHealth, http.StatusInternalServerError},
	HealthCheckValidation: {"Health check validation failed", DomainHealth, http.StatusBadRequest},
	HealthCheckThreshold:  {"Health check threshold exceeded", DomainHealth, http.StatusServiceUnavailable},
	HealthCheckState:      {"Health check state error", DomainHealth, http.StatusInternalServerError},
	HealthCheckRecovery:   {"Health check recovery failed", DomainHealth, http.StatusInternalServerError},

	// Lifecycle errors
	LifecyclePID:      {"PID file operation failed", DomainLifecycle, http.StatusInternalServerError},
	LifecycleShutdown: {"Error during shutdown process", DomainLifecycle, http.StatusInternalServerError},
	LifecycleSignal:   {"Signal handling error", DomainLifecycle, http.StatusInternalServerError},
	LifecycleReload:   {"Configuration reload failed", DomainLifecycle, http.StatusInternalServerError},
	LifecycleHook:     {"Lifecycle hook execution failed", DomainLifecycle, http.StatusInternalServerError},
	LifecycleState:    {"Invalid lifecycle state transition", DomainLifecycle, http.StatusInternalServerError},
	LifecycleLock:     {"Failed to acquire lifecycle lock", DomainLifecycle, http.StatusInternalServerError},
	LifecycleCleanup:  {"Lifecycle cleanup failed", DomainLifecycle, http.StatusInternalServerError},
	LifecycleDaemon:   {"Daemon operation failed", DomainLifecycle, http.StatusInternalServerError},
	LifecycleResource: {"Resource management error", DomainLifecycle, http.StatusInternalServerError},
}

func (e *RodentError) Error() string {
	// The reason Error() doesn't include metadata is that:
	// - It follows the standard error interface pattern for concise error messages
	// - Metadata is meant for structured data consumption (API responses, logging, monitoring)
	// - Including all metadata would make error messages too verbose for standard logging
	if e.Details != "" {
		return fmt.Sprintf("[%s-%d] %s - %s", e.Domain, e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s-%d] %s", e.Domain, e.Code, e.Message)
}

func (e *RodentError) WithMetadata(key, value string) *RodentError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]string)
	}
	e.Metadata[key] = value
	return e
}

// MarshalJSON customizes JSON serialization
func (e *RodentError) MarshalJSON() ([]byte, error) {
	type Alias RodentError
	return json.Marshal(&struct {
		*Alias
		Timestamp string `json:"timestamp"`
	}{
		Alias:     (*Alias)(e),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// New creates a new RodentError
func New(code ErrorCode, details string) *RodentError {
	def, ok := errorDefinitions[code]
	if !ok {
		return &RodentError{
			Code:       code,
			Domain:     "UNKNOWN",
			Message:    "Unknown error",
			Details:    details,
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	return &RodentError{
		Code:       code,
		Domain:     def.domain,
		Message:    def.message,
		Details:    details,
		HTTPStatus: def.httpStatus,
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code ErrorCode) *RodentError {
	re := New(code, err.Error())
	return re
}

// IsRodentError checks if an error is a RodentError
func IsRodentError(err error) bool {
	_, ok := err.(*RodentError)
	return ok
}

// CommandError helper for command execution errors
type CommandError struct {
	Command  string
	ExitCode int
	StdErr   string
}

func NewCommandError(cmd string, exitCode int, stderr string) *RodentError {
	return New(CommandExecution, "").
		WithMetadata("command", cmd).
		WithMetadata("exit_code", fmt.Sprintf("%d", exitCode)).
		WithMetadata("stderr", stderr)
}
