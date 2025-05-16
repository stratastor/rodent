/*
 * Copyright 2024-2025 Raamsri Kumar <raam@tinkershack.in>
 * Copyright 2024-2025 The StrataSTOR Authors and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package errors

import "net/http"

const (
	DomainConfig    Domain = "CONFIG"
	DomainServer    Domain = "SERVER"
	DomainZFS       Domain = "ZFS"
	DomainCommand   Domain = "CMD"
	DomainHealth    Domain = "HEALTH"
	DomainLifecycle Domain = "LIFECYCLE"
	DomainAD        Domain = "AD"
	DomainShares    Domain = "SHARES"
	DomainSSH       Domain = "SSH"
	DomainMisc      Domain = "MISC"
	DomainSystem    Domain = "SYSTEM"
	DomainService   Domain = "SERVICE"
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
// 1200-1299: Active Directory errors
// 1300-1399: Command execution
// 1400-1499: Health check
// 1500-1599: Lifecycle management
// 1600-1699: Rodent errors
// 1700-1799: Shares + ACL errors
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
	ConfigReadError                        // Error reading config
	ConfigWriteError                       // Error writing config
	ConfigParseError                       // Error parsing config
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
	ServerInternalError
	ServerBadRequest // Bad request error
)

const (
	// Active Directory Errors (1200-1299)
	ADConnectFailed      = 1200 + iota // Failed to connect to AD
	ADSearchFailed                     // Failed to search AD
	ADUserNotFound                     // User not found
	ADGroupNotFound                    // Group not found
	ADPermissionDenied                 // Permission denied
	ADInvalidCredentials               // Invalid credentials
	ADInvalidFilter                    // Invalid search filter
	ADInvalidBaseDN                    // Invalid base DN
	ADInvalidAttribute                 // Invalid attribute
	ADInvalidGroup                     // Invalid group
	ADInvalidUser                      // Invalid user

	ADCreateUserFailed     // Failed to add user
	ADUpdateUserFailed     // Failed to update user
	ADDeleteUserFailed     // Failed to delete user
	ADCreateGroupFailed    // Failed to add group
	ADUpdateGroupFailed    // Failed to update group
	ADDeleteGroupFailed    // Failed to delete group
	ADCreateComputerFailed // Failed to add computer
	ADUpdateComputerFailed // Failed to update computer
	ADDeleteComputerFailed // Failed to delete computer
	ADEncodePasswordFailed // Failed to encode password
	ADSetPasswordFailed    // Failed to set password
	ADEnableAccountFailed  // Failed to enable account
	ADCreateOUFailed       // Failed to create OU
)

const (
	// TODO: Remove redundant error codes
	// ZFS Operations (2000-2999)
	ZFSCommandFailed    = 2000 + iota // ZFS command execution failed
	ZFSPoolNotFound                   // Pool not found
	ZFSPermissionDenied               // Permission denied
	ZFSPropertyError                  // Property operation failed
	SchedulerError                    // Scheduler error
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
	ZFSRequestValidationError

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
	ZFSDatasetOperation

	ZFSDatasetSend
	ZFSDatasetReceive
	ZFSDatasetNoReceiveToken

	ZFSSnapshotList
	ZFSSnapshotDestroy
	ZFSSnapshotRollback
	ZFSSnapshotFailed // Snapshot operation failed
	ZFSSnapshotInvalidName
	ZFSSnapshotInvalidProperty
	ZFSSnapshotPolicyError // Auto-snapshot policy operation failed

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
	RodentMisc = 1600 + iota // Miscellaneous program error
	FSError
	NotFoundError // Not found error
	LoggerError   // Logger error
)

const (
	// Shares + ACL Errors (1700-1799)
	// ACL errors
	FACLInvalidInput = 1700 + iota // Invalid ACL input
	FACLReadError
	FACLWriteError
	FACLParseError
	FACLPathNotFound
	FACLInvalidPrincipal
	FACLUnsupportedFS

	// Shares service errors
	SharesInvalidInput    // Invalid share input or parameters
	SharesOperationFailed // Generic share operation failure
	SharesNotFound        // Share not found
	SharesAlreadyExists   // Share already exists
	SharesServiceFailed   // Share service operation failed
	SharesInternalError   // Internal shares subsystem error
	SharesAccessDenied    // Access denied for share operation
	SharesPathInvalid     // Invalid path for share
	SharesConfigInvalid   // Invalid share configuration
	SharesNetworkError    // Network error during share operation
	SharesLimitExceeded   // Exceeded share limits
	SharesProtocolError   // Protocol-specific share error
)

const (
	// System Errors (1750-1799)
	OperationFailed  = 1750 + iota // Generic operation failed
	PermissionDenied               // Permission denied

	// SSH Key Errors (1800-1849)
	SSHKeyPairGenerationFailed     = 1800 + iota // Failed to generate SSH key pair
	SSHKeyPairWriteFailed                        // Failed to write SSH key pair
	SSHKeyPairReadFailed                         // Failed to read SSH key pair
	SSHKeyPairDeleteFailed                       // Failed to delete SSH key pair
	SSHKeyPairNotFound                           // SSH key pair not found
	SSHKeyPairAlreadyExists                      // SSH key pair already exists
	SSHKeyPairInvalidType                        // Invalid SSH key type
	SSHKeyPairInvalidPeeringID                   // Invalid peering ID
	SSHKeyPairInvalidPublicKey                   // Invalid public key format
	SSHKeyPairInvalidHostname                    // Invalid hostname/IP
	SSHKeyPairPermissionDenied                   // Permission denied for SSH key operation
	SSHKnownHostAddFailed                        // Failed to add to known hosts
	SSHKnownHostRemoveFailed                     // Failed to remove from known hosts
	SSHKnownHostEntryNotFound                    // Known host entry not found
	SSHKnownHostEntryAlreadyExists               // Known host entry already exists
)

const (
	// Service Errors (1850-1899)
	ServiceNotFound      = 1850 + iota // Service not found
	ServiceUpdateFailed                // Service update failed
	ServiceStartFailed                 // Service start failed
	ServiceStopFailed                  // Service stop failed
	ServiceRestartFailed               // Service restart failed
	ServiceStatusFailed                // Service status check failed
)

var errorDefinitions = map[ErrorCode]struct {
	message    string
	domain     Domain
	httpStatus int
}{
	// System error definitions
	OperationFailed: {
		"Operation failed",
		DomainSystem,
		http.StatusInternalServerError,
	},
	PermissionDenied: {
		"Permission denied",
		DomainSystem,
		http.StatusForbidden,
	},
	// SSH Key error definitions
	SSHKeyPairGenerationFailed: {
		"Failed to generate SSH key pair",
		DomainSSH,
		http.StatusInternalServerError,
	},
	SSHKeyPairWriteFailed: {
		"Failed to write SSH key pair",
		DomainSSH,
		http.StatusInternalServerError,
	},
	SSHKeyPairReadFailed: {
		"Failed to read SSH key pair",
		DomainSSH,
		http.StatusInternalServerError,
	},
	SSHKeyPairDeleteFailed: {
		"Failed to delete SSH key pair",
		DomainSSH,
		http.StatusInternalServerError,
	},
	SSHKeyPairNotFound: {
		"SSH key pair not found",
		DomainSSH,
		http.StatusNotFound,
	},
	SSHKeyPairAlreadyExists: {
		"SSH key pair already exists",
		DomainSSH,
		http.StatusConflict,
	},
	SSHKeyPairInvalidType: {
		"Invalid SSH key type",
		DomainSSH,
		http.StatusBadRequest,
	},
	SSHKeyPairInvalidPeeringID: {
		"Invalid peering ID",
		DomainSSH,
		http.StatusBadRequest,
	},
	SSHKeyPairInvalidPublicKey: {
		"Invalid public key format",
		DomainSSH,
		http.StatusBadRequest,
	},
	SSHKeyPairInvalidHostname: {
		"Invalid hostname or IP address",
		DomainSSH,
		http.StatusBadRequest,
	},
	SSHKeyPairPermissionDenied: {
		"Permission denied for SSH key operation",
		DomainSSH,
		http.StatusForbidden,
	},
	SSHKnownHostAddFailed: {
		"Failed to add to known hosts",
		DomainSSH,
		http.StatusInternalServerError,
	},
	SSHKnownHostRemoveFailed: {
		"Failed to remove from known hosts",
		DomainSSH,
		http.StatusInternalServerError,
	},
	SSHKnownHostEntryNotFound: {
		"Known host entry not found",
		DomainSSH,
		http.StatusNotFound,
	},
	SSHKnownHostEntryAlreadyExists: {
		"Known host entry already exists",
		DomainSSH,
		http.StatusConflict,
	},
	// Configuration errors
	ConfigNotFound: {"Configuration file not found", DomainConfig, http.StatusNotFound},
	ConfigInvalid:  {"Invalid configuration format", DomainConfig, http.StatusBadRequest},
	ConfigLoadFailed: {
		"Failed to load configuration",
		DomainConfig,
		http.StatusInternalServerError,
	},
	ConfigWriteFailed: {
		"Failed to write configuration",
		DomainConfig,
		http.StatusInternalServerError,
	},
	ConfigPermissionDenied: {
		"Permission denied accessing config",
		DomainConfig,
		http.StatusForbidden,
	},
	ConfigDirectoryError: {
		"Config directory error",
		DomainConfig,
		http.StatusInternalServerError,
	},
	ConfigValidationFailed: {
		"Configuration validation failed",
		DomainConfig,
		http.StatusBadRequest,
	},
	ConfigMarshalFailed: {
		"Failed to serialize configuration",
		DomainConfig,
		http.StatusInternalServerError,
	},
	ConfigUnmarshalFailed: {
		"Failed to deserialize configuration",
		DomainConfig,
		http.StatusInternalServerError,
	},
	ConfigHomeDirectoryError: {
		"Failed to get home directory",
		DomainConfig,
		http.StatusInternalServerError,
	},
	ConfigReadError: {
		"Error reading configuration",
		DomainConfig,
		http.StatusInternalServerError,
	},
	ConfigWriteError: {
		"Error writing configuration",
		DomainConfig,
		http.StatusInternalServerError,
	},
	ConfigParseError: {
		"Error parsing configuration",
		DomainConfig,
		http.StatusInternalServerError,
	},

	// Server errors
	ServerStart: {
		"Failed to start server",
		DomainServer,
		http.StatusInternalServerError,
	},
	ServerShutdown: {
		"Error during server shutdown",
		DomainServer,
		http.StatusInternalServerError,
	},
	ServerBind: {
		"Failed to bind server port",
		DomainServer,
		http.StatusInternalServerError,
	},
	ServerTimeout: {
		"Server operation timed out",
		DomainServer,
		http.StatusGatewayTimeout,
	},
	ServerMiddleware: {
		"Middleware execution failed",
		DomainServer,
		http.StatusInternalServerError,
	},
	ServerRouting:           {"Route handling error", DomainServer, http.StatusInternalServerError},
	ServerRequestValidation: {"Request validation failed", DomainServer, http.StatusBadRequest},
	ServerResponseError: {
		"Error generating response",
		DomainServer,
		http.StatusInternalServerError,
	},
	ServerContextCancelled: {
		"Server context cancelled",
		DomainServer,
		http.StatusServiceUnavailable,
	},
	ServerTLSError: {
		"TLS configuration error",
		DomainServer,
		http.StatusInternalServerError,
	},
	ServerBadRequest: {
		"Bad request error",
		DomainServer,
		http.StatusBadRequest,
	},

	// Active Directory errors
	ADConnectFailed: {
		"Failed to connect to Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADSearchFailed: {
		"Failed to search Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADUserNotFound: {
		"User not found in Active Directory",
		DomainAD,
		http.StatusNotFound,
	},
	ADGroupNotFound: {
		"Group not found in Active Directory",
		DomainAD,
		http.StatusNotFound,
	},
	ADPermissionDenied: {
		"Permission denied accessing Active Directory",
		DomainAD,
		http.StatusForbidden,
	},
	ADInvalidCredentials: {
		"Invalid credentials for Active Directory",
		DomainAD,
		http.StatusUnauthorized,
	},
	ADInvalidFilter: {
		"Invalid search filter",
		DomainAD,
		http.StatusBadRequest,
	},
	ADInvalidBaseDN: {
		"Invalid base DN",
		DomainAD,
		http.StatusBadRequest,
	},
	ADInvalidAttribute: {
		"Invalid attribute",
		DomainAD,
		http.StatusBadRequest,
	},
	ADInvalidGroup: {
		"Invalid group",
		DomainAD,
		http.StatusBadRequest,
	},
	ADInvalidUser: {
		"Invalid user",
		DomainAD,
		http.StatusBadRequest,
	},
	ADCreateUserFailed: {
		"Failed to create user in Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADUpdateUserFailed: {
		"Failed to update user in Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADDeleteUserFailed: {
		"Failed to delete user from Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADCreateGroupFailed: {
		"Failed to create group in Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADUpdateGroupFailed: {
		"Failed to update group in Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADDeleteGroupFailed: {
		"Failed to delete group from Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADCreateComputerFailed: {
		"Failed to create computer in Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADUpdateComputerFailed: {
		"Failed to update computer in Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADDeleteComputerFailed: {
		"Failed to delete computer from Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADEncodePasswordFailed: {
		"Failed to encode password for Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADSetPasswordFailed: {
		"Failed to set password in Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADEnableAccountFailed: {
		"Failed to enable account in Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},
	ADCreateOUFailed: {
		"Failed to create Organizational Unit in Active Directory",
		DomainAD,
		http.StatusInternalServerError,
	},

	// ZFS errors
	ZFSCommandFailed: {
		"ZFS command execution failed",
		DomainZFS,
		http.StatusInternalServerError,
	},
	ZFSPermissionDenied: {
		"Permission denied for ZFS operation",
		DomainZFS,
		http.StatusForbidden,
	},
	ZFSPropertyError: {
		"ZFS property operation failed",
		DomainZFS,
		http.StatusInternalServerError,
	},
	SchedulerError: {
		"Scheduler operation failed",
		DomainZFS,
		http.StatusInternalServerError,
	},
	ZFSPropertyValueTooLong: {"ZFS property value too long ", DomainZFS, http.StatusBadRequest},
	ZFSInvalidPropertyValue: {"ZFS invalid property value", DomainZFS, http.StatusBadRequest},
	ZFSMountError: {
		"ZFS mount operation failed",
		DomainZFS,
		http.StatusInternalServerError,
	},
	ZFSIOError: {
		"ZFS I/O operation failed",
		DomainZFS,
		http.StatusInternalServerError,
	},
	ZFSPermissionError: {
		"Permission denied for ZFS operation",
		DomainZFS,
		http.StatusForbidden,
	},
	ZFSInvalidSize: {"Invalid size specified", DomainZFS, http.StatusBadRequest},
	ZFSRequestValidationError: {
		"ZFS validation error",
		DomainZFS,
		http.StatusBadRequest,
	},

	ZFSBookmarkFailed: {
		"Failed to create/list bookmark",
		DomainZFS,
		http.StatusInternalServerError,
	},

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

	ZFSDatasetCreate:   {"Failed to create ZFS dataset", DomainZFS, http.StatusBadRequest},
	ZFSDatasetNotFound: {"ZFS dataset not found", DomainZFS, http.StatusNotFound},
	ZFSDatasetList:     {"Failed to list ZFS datasets", DomainZFS, http.StatusBadRequest},
	ZFSDatasetDestroy:  {"Failed to destroy ZFS dataset", DomainZFS, http.StatusBadRequest},
	ZFSDatasetGetProperty: {
		"Failed to get dataset property",
		DomainZFS,
		http.StatusBadRequest,
	},
	ZFSDatasetSetProperty: {
		"Failed to set dataset property",
		DomainZFS,
		http.StatusBadRequest,
	},
	ZFSDatasetPropertyNotFound: {"Dataset property not found", DomainZFS, http.StatusNotFound},
	ZFSDatasetClone:            {"Failed to clone dataset", DomainZFS, http.StatusBadRequest},
	ZFSDatasetInvalidName:      {"Invalid dataset name", DomainZFS, http.StatusBadRequest},
	ZFSDatasetInvalidProperty:  {"Invalid property", DomainZFS, http.StatusBadRequest},
	ZFSDatasetRename:           {"Failed to rename dataset", DomainZFS, http.StatusBadRequest},
	ZFSDatasetSnapshot:         {"Failed to create snapshot", DomainZFS, http.StatusBadRequest},
	ZFSDatasetOperation: {
		"Failed to perform dataset operation",
		DomainZFS,
		http.StatusBadRequest,
	},

	ZFSDatasetSend:           {"Failed to send dataset", DomainZFS, http.StatusBadRequest},
	ZFSDatasetReceive:        {"Failed to receive dataset", DomainZFS, http.StatusBadRequest},
	ZFSDatasetNoReceiveToken: {"No _receive_ token", DomainZFS, http.StatusNotFound},

	ZFSSnapshotList:     {"Failed to list snapshots", DomainZFS, http.StatusBadRequest},
	ZFSSnapshotDestroy:  {"Failed to destroy snapshot", DomainZFS, http.StatusBadRequest},
	ZFSSnapshotRollback: {"Failed to rollback snapshot", DomainZFS, http.StatusBadRequest},
	ZFSSnapshotFailed: {
		"Failed to create/manage snapshot",
		DomainZFS,
		http.StatusBadRequest,
	},
	ZFSSnapshotInvalidName: {"Invalid snapshot name", DomainZFS, http.StatusBadRequest},
	ZFSSnapshotInvalidProperty: {
		"Invalid snapshot property value",
		DomainZFS,
		http.StatusBadRequest,
	},
	ZFSSnapshotPolicyError: {
		"Auto-snapshot policy operation failed",
		DomainZFS,
		http.StatusInternalServerError,
	},

	ZFSBookmarkInvalidName: {"Invalid bookmark name", DomainZFS, http.StatusBadRequest},
	ZFSBookmarkInvalidProperty: {
		"Invalid bookmark property value",
		DomainZFS,
		http.StatusBadRequest,
	},

	ZFSPoolNotFound:         {"ZFS pool not found", DomainZFS, http.StatusNotFound},
	ZFSPoolCreate:           {"Failed to create ZFS pool", DomainZFS, http.StatusBadRequest},
	ZFSPoolImport:           {"Failed to import ZFS pool", DomainZFS, http.StatusBadRequest},
	ZFSPoolExport:           {"Failed to export ZFS pool", DomainZFS, http.StatusBadRequest},
	ZFSPoolStatus:           {"Failed to get pool status", DomainZFS, http.StatusBadRequest},
	ZFSPoolList:             {"Failed to get pool list", DomainZFS, http.StatusBadRequest},
	ZFSPoolDestroy:          {"Failed to destroy pool", DomainZFS, http.StatusBadRequest},
	ZFSPoolGetProperty:      {"Failed to get pool property", DomainZFS, http.StatusBadRequest},
	ZFSPoolSetProperty:      {"Failed to set pool property", DomainZFS, http.StatusBadRequest},
	ZFSPoolPropertyNotFound: {"Pool property not found", DomainZFS, http.StatusNotFound},
	ZFSPoolInvalidName:      {"Invalid pool name", DomainZFS, http.StatusBadRequest},
	ZFSPoolInvalidDevice:    {"Invalid device", DomainZFS, http.StatusBadRequest},
	ZFSPoolDeviceOperation: {
		"Failed to perform zpool device operation",
		DomainZFS,
		http.StatusBadRequest,
	},
	ZFSPoolRestrictedDevice: {"ZFS device not allowed", DomainZFS, http.StatusForbidden},
	ZFSPoolTooManyDevices:   {"ZFS too many devices", DomainZFS, http.StatusForbidden},

	// Command execution errors
	CommandNotFound:  {"Command not found", DomainCommand, http.StatusNotFound},
	CommandExecution: {"Command execution failed", DomainCommand, http.StatusBadRequest},
	CommandTimeout:   {"Command execution timed out", DomainCommand, http.StatusGatewayTimeout},
	CommandPermission: {
		"Permission denied executing command",
		DomainCommand,
		http.StatusForbidden,
	},
	CommandInvalidInput: {"Invalid command input", DomainCommand, http.StatusBadRequest},
	CommandOutputParse: {
		"Failed to parse command output",
		DomainCommand,
		http.StatusInternalServerError,
	},
	CommandSignal: {
		"Command signal handling failed",
		DomainCommand,
		http.StatusInternalServerError,
	},
	CommandContext: {"Command context error", DomainCommand, http.StatusInternalServerError},
	CommandPipe: {
		"Command pipe operation failed",
		DomainCommand,
		http.StatusInternalServerError,
	},
	CommandWorkDir: {"Working directory error", DomainCommand, http.StatusInternalServerError},

	// Health check errors
	HealthCheckFailed:  {"Health check failed", DomainHealth, http.StatusServiceUnavailable},
	HealthCheckTimeout: {"Health check timed out", DomainHealth, http.StatusGatewayTimeout},
	HealthCheckComponent: {
		"Component health check failed",
		DomainHealth,
		http.StatusServiceUnavailable,
	},
	HealthCheckConfig: {
		"Health check configuration error",
		DomainHealth,
		http.StatusInternalServerError,
	},
	HealthCheckEndpoint: {
		"Health check endpoint error",
		DomainHealth,
		http.StatusServiceUnavailable,
	},
	HealthCheckClient: {
		"Health check client error",
		DomainHealth,
		http.StatusInternalServerError,
	},
	HealthCheckValidation: {"Health check validation failed", DomainHealth, http.StatusBadRequest},
	HealthCheckThreshold: {
		"Health check threshold exceeded",
		DomainHealth,
		http.StatusServiceUnavailable,
	},
	HealthCheckState: {
		"Health check state error",
		DomainHealth,
		http.StatusInternalServerError,
	},
	HealthCheckRecovery: {
		"Health check recovery failed",
		DomainHealth,
		http.StatusInternalServerError,
	},

	// Lifecycle errors
	LifecyclePID: {
		"PID file operation failed",
		DomainLifecycle,
		http.StatusInternalServerError,
	},
	LifecycleShutdown: {
		"Error during shutdown process",
		DomainLifecycle,
		http.StatusInternalServerError,
	},
	LifecycleSignal: {"Signal handling error", DomainLifecycle, http.StatusInternalServerError},
	LifecycleReload: {
		"Configuration reload failed",
		DomainLifecycle,
		http.StatusInternalServerError,
	},
	LifecycleHook: {
		"Lifecycle hook execution failed",
		DomainLifecycle,
		http.StatusInternalServerError,
	},
	LifecycleState: {
		"Invalid lifecycle state transition",
		DomainLifecycle,
		http.StatusInternalServerError,
	},
	LifecycleLock: {
		"Failed to acquire lifecycle lock",
		DomainLifecycle,
		http.StatusInternalServerError,
	},
	LifecycleCleanup: {
		"Lifecycle cleanup failed",
		DomainLifecycle,
		http.StatusInternalServerError,
	},
	LifecycleDaemon: {"Daemon operation failed", DomainLifecycle, http.StatusInternalServerError},
	LifecycleResource: {
		"Resource management error",
		DomainLifecycle,
		http.StatusInternalServerError,
	},

	// Rodent errors
	RodentMisc:    {"Miscellaneous program error", DomainLifecycle, http.StatusInternalServerError},
	FSError:       {"Filesystem error", DomainMisc, http.StatusInternalServerError},
	NotFoundError: {"Not found", DomainMisc, http.StatusNotFound},
	LoggerError: {
		"Logger error",
		DomainMisc,
		http.StatusInternalServerError,
	},

	// ACL errors
	FACLInvalidInput: {
		"Invalid ACL input",
		DomainShares,
		http.StatusBadRequest,
	},
	FACLReadError: {
		"Failed to read filesystem ACLs",
		DomainShares,
		http.StatusInternalServerError,
	},
	FACLWriteError: {
		"Failed to modify filesystem ACLs",
		DomainShares,
		http.StatusInternalServerError,
	},
	FACLParseError: {
		"Failed to parse ACL data",
		DomainShares,
		http.StatusInternalServerError,
	},
	FACLPathNotFound: {
		"Path not found",
		DomainShares,
		http.StatusNotFound,
	},
	FACLInvalidPrincipal: {
		"Invalid user or group specified in ACL",
		DomainShares,
		http.StatusBadRequest,
	},
	FACLUnsupportedFS: {
		"Unsupported filesystem for ACL operations",
		DomainShares,
		http.StatusBadRequest,
	},

	SharesInvalidInput: {
		"Invalid share input or parameters",
		DomainShares,
		http.StatusBadRequest,
	},
	SharesOperationFailed: {
		"Share operation failed",
		DomainShares,
		http.StatusInternalServerError,
	},
	SharesNotFound: {
		"Share not found",
		DomainShares,
		http.StatusNotFound,
	},
	SharesAlreadyExists: {
		"Share already exists",
		DomainShares,
		http.StatusConflict,
	},
	SharesServiceFailed: {
		"Share service operation failed",
		DomainShares,
		http.StatusInternalServerError,
	},
	SharesInternalError: {
		"Internal shares subsystem error",
		DomainShares,
		http.StatusInternalServerError,
	},
	SharesAccessDenied: {
		"Access denied for share operation",
		DomainShares,
		http.StatusForbidden,
	},
	SharesPathInvalid: {
		"Invalid path for share",
		DomainShares,
		http.StatusBadRequest,
	},
	SharesConfigInvalid: {
		"Invalid share configuration",
		DomainShares,
		http.StatusBadRequest,
	},
	SharesNetworkError: {
		"Network error during share operation",
		DomainShares,
		http.StatusInternalServerError,
	},
	SharesLimitExceeded: {
		"Exceeded share limits",
		DomainShares,
		http.StatusTooManyRequests,
	},
	SharesProtocolError: {
		"Protocol-specific share error",
		DomainShares,
		http.StatusBadRequest,
	},
	ServerInternalError: {
		"Internal server error",
		DomainServer,
		http.StatusInternalServerError,
	},

	ServiceNotFound: {
		"Service not found",
		DomainService,
		http.StatusNotFound,
	},
	ServiceUpdateFailed: {
		"Service update failed",
		DomainService,
		http.StatusInternalServerError,
	},
	ServiceStartFailed: {
		"Service start failed",
		DomainService,
		http.StatusInternalServerError,
	},
	ServiceStopFailed: {
		"Service stop failed",
		DomainService,
		http.StatusInternalServerError,
	},
	ServiceRestartFailed: {
		"Service restart failed",
		DomainService,
		http.StatusInternalServerError,
	},
	ServiceStatusFailed: {
		"Service status check failed",
		DomainService,
		http.StatusInternalServerError,
	},
}
