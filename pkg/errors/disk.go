// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"maps"
	"net/http"
)

// Disk Management Error Codes (2300-2399)
const (
	// Discovery Errors (2300-2309)
	DiskDiscoveryFailed = 2300 + iota // Failed to discover disks
	DiskDiscoveryTimeout               // Discovery operation timed out
	DiskCorrelationFailed              // Failed to correlate disk paths
	DiskCacheError                     // Disk cache operation error
	DiskNotFound                       // Disk not found
	DiskAlreadyExists                  // Disk already exists in inventory

	// Topology Errors (2310-2319)
	DiskTopologyParseFailed = 2310 + iota // Failed to parse disk topology
	DiskTopologyFailed                    // Topology discovery failed
	DiskEnclosureNotFound                 // Enclosure not found
	DiskControllerNotFound                // Controller not found
	DiskTopologyInvalid                   // Invalid topology data
	DiskRAIDDetected                      // RAID controller detected (warning)

	// Health Monitoring Errors (2320-2329)
	DiskHealthCheckFailed = 2320 + iota // Health check failed
	DiskSMARTReadFailed                 // Failed to read SMART attributes
	DiskSMARTNotAvailable               // SMART not available on device
	DiskSMARTRefreshFailed              // Failed to refresh SMART data
	DiskSMARTParseFailed                // Failed to parse SMART data
	DiskNVMeHealthFailed                // Failed to read NVMe health
	DiskIOStatFailed                    // Failed to get iostat metrics
	DiskHealthEvalFailed                // Health evaluation failed
	DiskThresholdExceeded               // Health threshold exceeded

	// Probe Errors (2330-2349)
	DiskProbeScheduleFailed = 2330 + iota // Failed to schedule probe
	DiskProbeStartFailed                  // Failed to start probe
	DiskProbeParseFailed                  // Failed to parse probe results
	DiskProbeTimeout                      // Probe operation timed out
	DiskProbeNotFound                     // Probe not found
	DiskProbeAlreadyRunning               // Probe already running on device
	DiskProbeCancelled                    // Probe cancelled by user
	DiskProbeFailed                       // Probe execution failed
	DiskProbeConflict                     // Probe conflict detected
	DiskProbeConcurrencyLimit             // Max concurrent probes reached

	// Hotplug Errors (2350-2359)
	DiskHotplugMonitorFailed = 2350 + iota // Hotplug monitor failed
	DiskHotplugEventFailed                 // Failed to process hotplug event
	DiskUdevError                          // udev operation error
	DiskReconciliationFailed               // Reconciliation failed
	DiskStateTransitionFailed              // State transition failed

	// Naming Strategy Errors (2360-2369)
	DiskNamingStrategyInvalid = 2360 + iota // Invalid naming strategy
	DiskVdevConfGenFailed                   // vdev_id.conf generation failed
	DiskNameResolutionFailed                // Name resolution failed
	DiskDevicePathInvalid                   // Invalid device path

	// Configuration Errors (2370-2379)
	DiskConfigInvalid = 2370 + iota // Invalid disk manager configuration
	DiskConfigValidationFailed      // Configuration validation failed
	DiskConfigLoadFailed            // Failed to load configuration
	DiskConfigSaveFailed            // Failed to save configuration
	DiskConfigCronInvalid           // Invalid cron expression

	// State Management Errors (2380-2389)
	DiskStateLoadFailed = 2380 + iota // Failed to load state file
	DiskStateSaveFailed               // Failed to save state file
	DiskStateCorrupted                // State file corrupted
	DiskStateMigrationFailed          // State migration failed
	DiskOperationNotFound             // Operation not found

	// Tool Errors (2390-2399)
	DiskToolNotFound = 2390 + iota // Required tool not found
	DiskToolVersionMismatch        // Tool version mismatch
	DiskToolExecutionFailed        // Tool execution failed
	DiskToolOutputParseFailed      // Failed to parse tool output
	DiskToolTimeout                // Tool execution timed out
)

func init() {
	// Disk error definitions
	diskErrorDefinitions := map[ErrorCode]struct {
		message    string
		domain     Domain
		httpStatus int
	}{
		// Discovery Errors
		DiskDiscoveryFailed: {
			"Failed to discover disks",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskDiscoveryTimeout: {
			"Disk discovery operation timed out",
			DomainSystem,
			http.StatusGatewayTimeout,
		},
		DiskCorrelationFailed: {
			"Failed to correlate disk device paths",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskCacheError: {
			"Disk cache operation error",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskNotFound: {
			"Disk not found",
			DomainSystem,
			http.StatusNotFound,
		},
		DiskAlreadyExists: {
			"Disk already exists in inventory",
			DomainSystem,
			http.StatusConflict,
		},

		// Topology Errors
		DiskTopologyParseFailed: {
			"Failed to parse disk physical topology",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskTopologyFailed: {
			"Disk topology discovery failed",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskEnclosureNotFound: {
			"Disk enclosure not found",
			DomainSystem,
			http.StatusNotFound,
		},
		DiskControllerNotFound: {
			"Disk controller not found",
			DomainSystem,
			http.StatusNotFound,
		},
		DiskTopologyInvalid: {
			"Invalid disk topology data",
			DomainSystem,
			http.StatusBadRequest,
		},
		DiskRAIDDetected: {
			"RAID controller detected - ZFS requires direct disk access",
			DomainSystem,
			http.StatusBadRequest,
		},

		// Health Monitoring Errors
		DiskHealthCheckFailed: {
			"Disk health check failed",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskSMARTReadFailed: {
			"Failed to read SMART attributes",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskSMARTNotAvailable: {
			"SMART not available on device",
			DomainSystem,
			http.StatusServiceUnavailable,
		},
		DiskSMARTRefreshFailed: {
			"Failed to refresh SMART data",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskSMARTParseFailed: {
			"Failed to parse SMART data",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskNVMeHealthFailed: {
			"Failed to read NVMe health data",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskIOStatFailed: {
			"Failed to get disk I/O statistics",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskHealthEvalFailed: {
			"Disk health evaluation failed",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskThresholdExceeded: {
			"Disk health threshold exceeded",
			DomainSystem,
			http.StatusServiceUnavailable,
		},

		// Probe Errors
		DiskProbeScheduleFailed: {
			"Failed to schedule SMART probe",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskProbeStartFailed: {
			"Failed to start SMART probe",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskProbeParseFailed: {
			"Failed to parse SMART probe results",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskProbeTimeout: {
			"SMART probe operation timed out",
			DomainSystem,
			http.StatusGatewayTimeout,
		},
		DiskProbeNotFound: {
			"SMART probe not found",
			DomainSystem,
			http.StatusNotFound,
		},
		DiskProbeAlreadyRunning: {
			"SMART probe already running on device",
			DomainSystem,
			http.StatusConflict,
		},
		DiskProbeCancelled: {
			"SMART probe cancelled",
			DomainSystem,
			http.StatusOK,
		},
		DiskProbeFailed: {
			"SMART probe execution failed",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskProbeConflict: {
			"SMART probe conflict detected",
			DomainSystem,
			http.StatusConflict,
		},
		DiskProbeConcurrencyLimit: {
			"Maximum concurrent SMART probes reached",
			DomainSystem,
			http.StatusTooManyRequests,
		},

		// Hotplug Errors
		DiskHotplugMonitorFailed: {
			"Disk hotplug monitor failed",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskHotplugEventFailed: {
			"Failed to process disk hotplug event",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskUdevError: {
			"udev operation error",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskReconciliationFailed: {
			"Disk reconciliation failed",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskStateTransitionFailed: {
			"Disk state transition failed",
			DomainSystem,
			http.StatusInternalServerError,
		},

		// Naming Strategy Errors
		DiskNamingStrategyInvalid: {
			"Invalid disk naming strategy",
			DomainSystem,
			http.StatusBadRequest,
		},
		DiskVdevConfGenFailed: {
			"Failed to generate vdev_id.conf",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskNameResolutionFailed: {
			"Disk name resolution failed",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskDevicePathInvalid: {
			"Invalid disk device path",
			DomainSystem,
			http.StatusBadRequest,
		},

		// Configuration Errors
		DiskConfigInvalid: {
			"Invalid disk manager configuration",
			DomainSystem,
			http.StatusBadRequest,
		},
		DiskConfigValidationFailed: {
			"Disk configuration validation failed",
			DomainSystem,
			http.StatusBadRequest,
		},
		DiskConfigLoadFailed: {
			"Failed to load disk manager configuration",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskConfigSaveFailed: {
			"Failed to save disk manager configuration",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskConfigCronInvalid: {
			"Invalid cron expression in disk configuration",
			DomainSystem,
			http.StatusBadRequest,
		},

		// State Management Errors
		DiskStateLoadFailed: {
			"Failed to load disk manager state",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskStateSaveFailed: {
			"Failed to save disk manager state",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskStateCorrupted: {
			"Disk manager state file corrupted",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskStateMigrationFailed: {
			"Disk state migration failed",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskOperationNotFound: {
			"Disk operation not found",
			DomainSystem,
			http.StatusNotFound,
		},

		// Tool Errors
		DiskToolNotFound: {
			"Required disk management tool not found",
			DomainSystem,
			http.StatusServiceUnavailable,
		},
		DiskToolVersionMismatch: {
			"Disk tool version mismatch",
			DomainSystem,
			http.StatusServiceUnavailable,
		},
		DiskToolExecutionFailed: {
			"Disk tool execution failed",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskToolOutputParseFailed: {
			"Failed to parse disk tool output",
			DomainSystem,
			http.StatusInternalServerError,
		},
		DiskToolTimeout: {
			"Disk tool execution timed out",
			DomainSystem,
			http.StatusGatewayTimeout,
		},
	}

	// Add disk error definitions to the main error definitions map
	maps.Copy(errorDefinitions, diskErrorDefinitions)
}
