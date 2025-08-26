// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"maps"
	"net/http"
)

// System Management Error Codes (2100-2199)
const (
	// System Information Errors (2100-2119)
	SystemInfoCollectionFailed = 2100 + iota // Failed to collect system information
	SystemInfoParseError                     // Failed to parse system information
	SystemInfoInvalidData                    // Invalid system information data
	SystemInfoUnavailable                    // System information unavailable
	
	// Hostname Management Errors (2120-2129)
	SystemHostnameInvalid = 2120 + iota // Invalid hostname format
	SystemHostnameSetFailed             // Failed to set hostname
	SystemHostnameGetFailed             // Failed to get hostname
	SystemHostnameTooLong               // Hostname too long
	SystemHostnameReserved              // Hostname is reserved
	
	// User Management Errors (2130-2149)
	SystemUserNotFound = 2130 + iota // User not found
	SystemUserAlreadyExists          // User already exists
	SystemUserCreateFailed           // Failed to create user
	SystemUserDeleteFailed           // Failed to delete user
	SystemUserModifyFailed           // Failed to modify user
	SystemUserInvalidName            // Invalid username
	SystemUserInvalidPassword        // Invalid password
	SystemUserInvalidShell           // Invalid shell
	SystemUserInvalidHome            // Invalid home directory
	SystemUserProtected              // Protected user cannot be modified
	SystemUserPasswordEncryptFailed // Failed to encrypt password
	
	// Group Management Errors (2150-2169)
	SystemGroupNotFound = 2150 + iota // Group not found
	SystemGroupAlreadyExists          // Group already exists
	SystemGroupCreateFailed           // Failed to create group
	SystemGroupDeleteFailed           // Failed to delete group
	SystemGroupModifyFailed           // Failed to modify group
	SystemGroupInvalidName            // Invalid group name
	SystemGroupProtected              // Protected group cannot be modified
	SystemGroupMembershipFailed       // Failed to modify group membership
	
	// Power Management Errors (2170-2189)
	SystemPowerShutdownFailed = 2170 + iota // Failed to shutdown system
	SystemPowerRebootFailed                 // Failed to reboot system
	SystemPowerScheduleFailed               // Failed to schedule power operation
	SystemPowerCancelFailed                 // Failed to cancel scheduled operation
	SystemPowerInvalidDelay                 // Invalid power operation delay
	SystemPowerStatusFailed                 // Failed to get power status
	SystemPowerOperationDenied              // Power operation denied
	SystemPowerInvalidMessage               // Invalid power operation message
	
	// System Configuration Errors (2190-2199)
	SystemTimezoneInvalid = 2190 + iota // Invalid timezone
	SystemTimezoneSetFailed             // Failed to set timezone
	SystemLocaleInvalid                 // Invalid locale
	SystemLocaleSetFailed               // Failed to set locale
	SystemConfigValidationFailed        // System configuration validation failed
	SystemHealthCheckFailed             // System health check failed
	SystemOperationNotSupported         // System operation not supported
	SystemResourceUnavailable           // System resource unavailable
	SystemPermissionInsufficient        // Insufficient system permissions
)

func init() {
	// System error definitions
	systemErrorDefinitions := map[ErrorCode]struct {
		message    string
		domain     Domain
		httpStatus int
	}{
		// System Information Errors
		SystemInfoCollectionFailed: {
			"Failed to collect system information",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemInfoParseError: {
			"Failed to parse system information",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemInfoInvalidData: {
			"Invalid system information data",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemInfoUnavailable: {
			"System information unavailable",
			DomainSystem,
			http.StatusServiceUnavailable,
		},

		// Hostname Management Errors
		SystemHostnameInvalid: {
			"Invalid hostname format",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemHostnameSetFailed: {
			"Failed to set hostname",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemHostnameGetFailed: {
			"Failed to get hostname",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemHostnameTooLong: {
			"Hostname too long",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemHostnameReserved: {
			"Hostname is reserved",
			DomainSystem,
			http.StatusBadRequest,
		},

		// User Management Errors
		SystemUserNotFound: {
			"User not found",
			DomainSystem,
			http.StatusNotFound,
		},
		SystemUserAlreadyExists: {
			"User already exists",
			DomainSystem,
			http.StatusConflict,
		},
		SystemUserCreateFailed: {
			"Failed to create user",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemUserDeleteFailed: {
			"Failed to delete user",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemUserModifyFailed: {
			"Failed to modify user",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemUserInvalidName: {
			"Invalid username",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemUserInvalidPassword: {
			"Invalid password",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemUserInvalidShell: {
			"Invalid shell",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemUserInvalidHome: {
			"Invalid home directory",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemUserProtected: {
			"Protected user cannot be modified",
			DomainSystem,
			http.StatusForbidden,
		},
		SystemUserPasswordEncryptFailed: {
			"Failed to encrypt password",
			DomainSystem,
			http.StatusInternalServerError,
		},

		// Group Management Errors
		SystemGroupNotFound: {
			"Group not found",
			DomainSystem,
			http.StatusNotFound,
		},
		SystemGroupAlreadyExists: {
			"Group already exists",
			DomainSystem,
			http.StatusConflict,
		},
		SystemGroupCreateFailed: {
			"Failed to create group",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemGroupDeleteFailed: {
			"Failed to delete group",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemGroupModifyFailed: {
			"Failed to modify group",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemGroupInvalidName: {
			"Invalid group name",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemGroupProtected: {
			"Protected group cannot be modified",
			DomainSystem,
			http.StatusForbidden,
		},
		SystemGroupMembershipFailed: {
			"Failed to modify group membership",
			DomainSystem,
			http.StatusInternalServerError,
		},

		// Power Management Errors
		SystemPowerShutdownFailed: {
			"Failed to shutdown system",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemPowerRebootFailed: {
			"Failed to reboot system",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemPowerScheduleFailed: {
			"Failed to schedule power operation",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemPowerCancelFailed: {
			"Failed to cancel scheduled operation",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemPowerInvalidDelay: {
			"Invalid power operation delay",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemPowerStatusFailed: {
			"Failed to get power status",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemPowerOperationDenied: {
			"Power operation denied",
			DomainSystem,
			http.StatusForbidden,
		},
		SystemPowerInvalidMessage: {
			"Invalid power operation message",
			DomainSystem,
			http.StatusBadRequest,
		},

		// System Configuration Errors
		SystemTimezoneInvalid: {
			"Invalid timezone",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemTimezoneSetFailed: {
			"Failed to set timezone",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemLocaleInvalid: {
			"Invalid locale",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemLocaleSetFailed: {
			"Failed to set locale",
			DomainSystem,
			http.StatusInternalServerError,
		},
		SystemConfigValidationFailed: {
			"System configuration validation failed",
			DomainSystem,
			http.StatusBadRequest,
		},
		SystemHealthCheckFailed: {
			"System health check failed",
			DomainSystem,
			http.StatusServiceUnavailable,
		},
		SystemOperationNotSupported: {
			"System operation not supported",
			DomainSystem,
			http.StatusNotImplemented,
		},
		SystemResourceUnavailable: {
			"System resource unavailable",
			DomainSystem,
			http.StatusServiceUnavailable,
		},
		SystemPermissionInsufficient: {
			"Insufficient system permissions",
			DomainSystem,
			http.StatusForbidden,
		},
	}

	// Add system error definitions to the main error definitions map
	maps.Copy(errorDefinitions, systemErrorDefinitions)
}