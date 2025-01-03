package errors

import "net/http"

const (
	DomainConfig    Domain = "CONFIG"
	DomainServer    Domain = "SERVER"
	DomainZFS       Domain = "ZFS"
	DomainCommand   Domain = "CMD"
	DomainHealth    Domain = "HEALTH"
	DomainLifecycle Domain = "LIFECYCLE"
)

// ErrorCode represents unique error identifiers
type ErrorCode int

// Domain represents the subsystem where the error originated
type Domain string

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
	// TODO: perhaps use a map[string]interface{} for flexibility?
	// TODO: Consider map[string][]string for multiple values to accommodate error chains?
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Error code ranges:
// 1000-1099: Configuration errors
// 1100-1199: Server errors
// 1300-1399: Command execution
// 1400-1499: Health check
// 1500-1599: Lifecycle management
// 1600-1699: Rodent errors
// 2000-2999: ZFS operations
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
	// ZFS Operations (2000-2999)
	ZFSCommandFailed    = 2000 + iota // ZFS command execution failed
	ZFSPoolNotFound                   // Pool not found
	ZFSPermissionDenied               // Permission denied
	ZFSPropertyError                  // Property operation failed
	ZFSPropertyValueTooLong
	ZFSInvalidPropertyValue
	ZFSMountError // Mount operation failed
	ZFSInvalidMountPoint
	ZFSRestrictedMountPoint
	ZFSCloneError // Clone operation failed
	ZFSQuotaError // Quota operation failed
	ZFSIOError    // I/O error during operation
	ZFSInvalidSize
	ZFSQuotaExceeded
	ZFSQuotaInvalid
	ZFSPermissionError

	ZFSNameLeadingSlash
	ZFSNameEmptyComponent
	ZFSNameTrailingSlash
	ZFSNameInvalidChar
	ZFSNameMultipleDelimiters // multiple '@'/'#' delimiters found
	ZFSNameNoLetter           // pool doesn't begin with a letter
	ZFSNameReserved
	ZFSNameDiskLike
	ZFSNameTooLong
	ZFSNameSelfRef   // "."
	ZFSNameParentRef // ".."
	ZFSNameNoAtSign  // Missing "@" in snapshot
	ZFSNameNoPound   // Missing "#" in bookmark
	ZFSNameInvalid

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

	ZFSDatasetSend
	ZFSDatasetReceive
	ZFSDatasetNoReceiveToken

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
	ZFSPoolDeviceOperation
	ZFSPoolTooManyDevices
	ZFSPoolRestrictedDevice
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

const (
	// Rodent Errors (1600-1699)
	RodentMisc = 1600 + iota // Miscelaneous program error
)

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
	ZFSCommandFailed:        {"ZFS command execution failed", DomainZFS, http.StatusInternalServerError},
	ZFSPermissionDenied:     {"Permission denied for ZFS operation", DomainZFS, http.StatusForbidden},
	ZFSPropertyError:        {"ZFS property operation failed", DomainZFS, http.StatusInternalServerError},
	ZFSPropertyValueTooLong: {"ZFS property value too long ", DomainZFS, http.StatusBadRequest},
	ZFSInvalidPropertyValue: {"ZFS invalid property value", DomainZFS, http.StatusBadRequest},
	ZFSMountError:           {"ZFS mount operation failed", DomainZFS, http.StatusInternalServerError},
	ZFSIOError:              {"ZFS I/O operation failed", DomainZFS, http.StatusInternalServerError},
	ZFSPermissionError:      {"Permission denied for ZFS operation", DomainZFS, http.StatusForbidden},
	ZFSInvalidSize:          {"Invalid size specified", DomainZFS, http.StatusBadRequest},

	ZFSBookmarkFailed: {"Failed to create/list bookmark", DomainZFS, http.StatusInternalServerError},

	ZFSQuotaExceeded: {"Dataset quota exceeded", DomainZFS, http.StatusForbidden},
	ZFSQuotaError:    {"ZFS quota operation failed", DomainZFS, http.StatusInternalServerError},
	ZFSQuotaInvalid:  {"ZFS invalid quota", DomainZFS, http.StatusBadRequest},

	ZFSCloneError: {"ZFS clone operation failed", DomainZFS, http.StatusInternalServerError},

	ZFSNameLeadingSlash:       {"Leading slash in name", DomainZFS, http.StatusBadRequest},
	ZFSNameEmptyComponent:     {"Empty component in name", DomainZFS, http.StatusBadRequest},
	ZFSNameTrailingSlash:      {"Trailing slash in name", DomainZFS, http.StatusBadRequest},
	ZFSNameInvalidChar:        {"Invalid character in name", DomainZFS, http.StatusBadRequest},
	ZFSNameMultipleDelimiters: {"Multiple delimiters in name", DomainZFS, http.StatusBadRequest},
	ZFSNameNoLetter:           {"Name must begin with a letter", DomainZFS, http.StatusBadRequest},
	ZFSNameReserved:           {"Name is reserved", DomainZFS, http.StatusBadRequest},
	ZFSNameDiskLike:           {"Reserved disk name (c[0-9].*)", DomainZFS, http.StatusBadRequest},
	ZFSNameTooLong:            {"Name is too long", DomainZFS, http.StatusBadRequest},
	ZFSNameSelfRef:            {"Name is self reference", DomainZFS, http.StatusBadRequest},
	ZFSNameParentRef:          {"Name is parent reference", DomainZFS, http.StatusBadRequest},
	ZFSNameNoAtSign:           {"Missing '@' in snapshot name", DomainZFS, http.StatusBadRequest},
	ZFSNameNoPound:            {"Missing '#' in bookmark name", DomainZFS, http.StatusBadRequest},
	ZFSNameInvalid:            {"Invalid name", DomainZFS, http.StatusBadRequest},

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

	ZFSDatasetSend:           {"Failed to send dataset", DomainZFS, http.StatusInternalServerError},
	ZFSDatasetReceive:        {"Failed to receive dataset", DomainZFS, http.StatusInternalServerError},
	ZFSDatasetNoReceiveToken: {"No _receive_ token", DomainZFS, http.StatusNotFound},

	ZFSSnapshotList:            {"Failed to list snapshots", DomainZFS, http.StatusInternalServerError},
	ZFSSnapshotDestroy:         {"Failed to destroy snapshot", DomainZFS, http.StatusInternalServerError},
	ZFSSnapshotRollback:        {"Failed to rollback snapshot", DomainZFS, http.StatusInternalServerError},
	ZFSSnapshotFailed:          {"Failed to create/manage snapshot", DomainZFS, http.StatusInternalServerError},
	ZFSSnapshotInvalidName:     {"Invalid snapshot name", DomainZFS, http.StatusBadRequest},
	ZFSSnapshotInvalidProperty: {"Invalid snapshot property value", DomainZFS, http.StatusBadRequest},

	ZFSBookmarkInvalidName:     {"Invalid bookmark name", DomainZFS, http.StatusBadRequest},
	ZFSBookmarkInvalidProperty: {"Invalid bookmark property value", DomainZFS, http.StatusBadRequest},

	ZFSPoolNotFound:         {"ZFS pool not found", DomainZFS, http.StatusNotFound},
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
	ZFSPoolDeviceOperation:  {"Failed to perform zpool device operation", DomainZFS, http.StatusInternalServerError},
	ZFSPoolRestrictedDevice: {"ZFS device not allowed", DomainZFS, http.StatusForbidden},
	ZFSPoolTooManyDevices:   {"ZFS too many devices", DomainZFS, http.StatusForbidden},

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

	// Rodent errors
	RodentMisc: {"Miscellaneous program error", DomainLifecycle, http.StatusInternalServerError},
}
