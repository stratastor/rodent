// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"maps"
	"net/http"
)

const (
	DomainNetwork Domain = "NETWORK"
)

// Network error codes (1900-1999)
const (
	// General networking errors (1900-1919)
	NetworkOperationFailed          = 1900 + iota // Generic network operation failed
	NetworkPermissionDenied                       // Permission denied for network operation
	NetworkConfigurationInvalid                   // Invalid network configuration
	NetworkInterfaceNotFound                      // Network interface not found
	NetworkInterfaceOperationFailed               // Network interface operation failed
	NetworkAddressInvalid                         // Invalid network address
	NetworkRouteOperationFailed                   // Network route operation failed
	NetworkRouteNotFound                          // Network route not found
	NetworkDNSConfigurationFailed                 // DNS configuration failed
	NetworkValidationFailed                       // Network configuration validation failed
	NetworkStateInconsistent                      // Network state is inconsistent
	NetworkResourceBusy                           // Network resource is busy
	NetworkTimeout                                // Network operation timed out
	NetworkConnectivityFailed                     // Network connectivity test failed
	NetworkFeatureUnsupported                     // Network feature not supported
	NetworkPolicyViolation                        // Network policy violation
	NetworkBackendError                           // Network backend error
)

const (
	// Netplan-specific errors (1920-1949)
	NetplanCommandFailed       = 1920 + iota // Netplan command failed
	NetplanCommandNotFound                   // Netplan command not found
	NetplanConfigInvalid                     // Invalid Netplan configuration
	NetplanApplyFailed                       // Netplan apply failed
	NetplanGenerateFailed                    // Netplan generate failed
	NetplanTryFailed                         // Netplan try failed
	NetplanGetFailed                         // Netplan get failed
	NetplanSetFailed                         // Netplan set failed
	NetplanStatusFailed                      // Netplan status failed
	NetplanVersionUnsupported                // Netplan version not supported
	NetplanYAMLParseError                    // Netplan YAML parsing error
	NetplanYAMLValidationError               // Netplan YAML validation error
	NetplanRendererInvalid                   // Invalid Netplan renderer
	NetplanFileOperationFailed               // Netplan file operation failed
	NetplanBackupFailed                      // Netplan backup operation failed
	NetplanRestoreFailed                     // Netplan restore operation failed
	NetplanDiffFailed                        // Netplan diff operation failed
	NetplanConfigFileNotFound                // Netplan config file not found
	NetplanConfigFileLocked                  // Netplan config file locked
	NetplanRollbackFailed                    // Netplan rollback failed
	NetplanTryTimeout                        // Netplan try timeout
	NetplanTryCancelled                      // Netplan try cancelled
)

const (
	// IP command errors (1950-1979)
	IPCommandFailed             = 1950 + iota // IP command failed
	IPLinkOperationFailed                     // IP link operation failed
	IPAddressOperationFailed                  // IP address operation failed
	IPRouteOperationFailed                    // IP route operation failed
	IPRuleOperationFailed                     // IP rule operation failed
	IPNeighborOperationFailed                 // IP neighbor operation failed
	IPTunnelOperationFailed                   // IP tunnel operation failed
	IPNamespaceOperationFailed                // IP namespace operation failed
	IPJSONParseError                          // IP command JSON parsing error
	IPInterfaceStateError                     // IP interface state error
	IPBridgeOperationFailed                   // IP bridge operation failed
	IPVLANOperationFailed                     // IP VLAN operation failed
	IPBondOperationFailed                     // IP bond operation failed
	IPMTUOperationFailed                      // IP MTU operation failed
	IPMACAddressOperationFailed               // IP MAC address operation failed
)

const (
	// Network validation errors (1980-1999)
	NetworkIPAddressInvalid     = 1980 + iota // Invalid IP address format
	NetworkCIDRInvalid                        // Invalid CIDR notation
	NetworkMACAddressInvalid                  // Invalid MAC address format
	NetworkPortInvalid                        // Invalid port number
	NetworkHostnameInvalid                    // Invalid hostname format
	NetworkVLANIDInvalid                      // Invalid VLAN ID
	NetworkMTUInvalid                         // Invalid MTU value
	NetworkGatewayInvalid                     // Invalid gateway address
	NetworkDNSServerInvalid                   // Invalid DNS server address
	NetworkSearchDomainInvalid                // Invalid search domain
	NetworkInterfaceNameInvalid               // Invalid interface name
	NetworkBondModeInvalid                    // Invalid bond mode
	NetworkBridgeConfigInvalid                // Invalid bridge configuration
	NetworkTunnelConfigInvalid                // Invalid tunnel configuration
	NetworkRoutingTableInvalid                // Invalid routing table
	NetworkMetricInvalid                      // Invalid route metric
	NetworkPrefixLengthInvalid                // Invalid prefix length
	NetworkProtocolInvalid                    // Invalid network protocol
	NetworkScopeInvalid                       // Invalid route scope
	NetworkTypeInvalid                        // Invalid network type
	NetworkBondConfigInvalid                  // Invalid bond configuration
)

func init() {
	// Register network error definitions
	networkErrorDefinitions := map[ErrorCode]struct {
		message    string
		domain     Domain
		httpStatus int
	}{
		// General networking errors
		NetworkOperationFailed: {
			"Network operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetworkPermissionDenied: {
			"Permission denied for network operation",
			DomainNetwork,
			http.StatusForbidden,
		},
		NetworkConfigurationInvalid: {
			"Invalid network configuration",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkInterfaceNotFound: {
			"Network interface not found",
			DomainNetwork,
			http.StatusNotFound,
		},
		NetworkInterfaceOperationFailed: {
			"Network interface operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetworkAddressInvalid: {
			"Invalid network address",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkRouteOperationFailed: {
			"Network route operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetworkRouteNotFound: {
			"Network route not found",
			DomainNetwork,
			http.StatusNotFound,
		},
		NetworkDNSConfigurationFailed: {
			"DNS configuration failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetworkValidationFailed: {
			"Network configuration validation failed",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkStateInconsistent: {
			"Network state is inconsistent",
			DomainNetwork,
			http.StatusConflict,
		},
		NetworkResourceBusy: {
			"Network resource is busy",
			DomainNetwork,
			http.StatusConflict,
		},
		NetworkTimeout: {
			"Network operation timed out",
			DomainNetwork,
			http.StatusGatewayTimeout,
		},
		NetworkConnectivityFailed: {
			"Network connectivity test failed",
			DomainNetwork,
			http.StatusServiceUnavailable,
		},
		NetworkFeatureUnsupported: {
			"Network feature not supported",
			DomainNetwork,
			http.StatusNotImplemented,
		},
		NetworkPolicyViolation: {
			"Network policy violation",
			DomainNetwork,
			http.StatusForbidden,
		},
		NetworkBackendError: {
			"Network backend error",
			DomainNetwork,
			http.StatusInternalServerError,
		},

		// Netplan-specific errors
		NetplanCommandFailed: {
			"Netplan command failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanCommandNotFound: {
			"Netplan command not found",
			DomainNetwork,
			http.StatusNotFound,
		},
		NetplanConfigInvalid: {
			"Invalid Netplan configuration",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetplanApplyFailed: {
			"Netplan apply failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanGenerateFailed: {
			"Netplan generate failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanTryFailed: {
			"Netplan try failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanGetFailed: {
			"Netplan get failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanSetFailed: {
			"Netplan set failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanStatusFailed: {
			"Netplan status failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanVersionUnsupported: {
			"Netplan version not supported",
			DomainNetwork,
			http.StatusNotImplemented,
		},
		NetplanYAMLParseError: {
			"Netplan YAML parsing error",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetplanYAMLValidationError: {
			"Netplan YAML validation error",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetplanRendererInvalid: {
			"Invalid Netplan renderer",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetplanFileOperationFailed: {
			"Netplan file operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanBackupFailed: {
			"Netplan backup operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanRestoreFailed: {
			"Netplan restore operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanDiffFailed: {
			"Netplan diff operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanConfigFileNotFound: {
			"Netplan config file not found",
			DomainNetwork,
			http.StatusNotFound,
		},
		NetplanConfigFileLocked: {
			"Netplan config file locked",
			DomainNetwork,
			http.StatusLocked,
		},
		NetplanRollbackFailed: {
			"Netplan rollback failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		NetplanTryTimeout: {
			"Netplan try timeout",
			DomainNetwork,
			http.StatusGatewayTimeout,
		},
		NetplanTryCancelled: {
			"Netplan try cancelled",
			DomainNetwork,
			http.StatusConflict,
		},

		// IP command errors
		IPCommandFailed: {
			"IP command failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPLinkOperationFailed: {
			"IP link operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPAddressOperationFailed: {
			"IP address operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPRouteOperationFailed: {
			"IP route operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPRuleOperationFailed: {
			"IP rule operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPNeighborOperationFailed: {
			"IP neighbor operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPTunnelOperationFailed: {
			"IP tunnel operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPNamespaceOperationFailed: {
			"IP namespace operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPJSONParseError: {
			"IP command JSON parsing error",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPInterfaceStateError: {
			"IP interface state error",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPBridgeOperationFailed: {
			"IP bridge operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPVLANOperationFailed: {
			"IP VLAN operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPBondOperationFailed: {
			"IP bond operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPMTUOperationFailed: {
			"IP MTU operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},
		IPMACAddressOperationFailed: {
			"IP MAC address operation failed",
			DomainNetwork,
			http.StatusInternalServerError,
		},

		// Network validation errors
		NetworkIPAddressInvalid: {
			"Invalid IP address format",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkCIDRInvalid: {
			"Invalid CIDR notation",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkMACAddressInvalid: {
			"Invalid MAC address format",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkPortInvalid: {
			"Invalid port number",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkHostnameInvalid: {
			"Invalid hostname format",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkVLANIDInvalid: {
			"Invalid VLAN ID",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkMTUInvalid: {
			"Invalid MTU value",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkGatewayInvalid: {
			"Invalid gateway address",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkDNSServerInvalid: {
			"Invalid DNS server address",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkSearchDomainInvalid: {
			"Invalid search domain",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkInterfaceNameInvalid: {
			"Invalid interface name",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkBondModeInvalid: {
			"Invalid bond mode",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkBridgeConfigInvalid: {
			"Invalid bridge configuration",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkTunnelConfigInvalid: {
			"Invalid tunnel configuration",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkRoutingTableInvalid: {
			"Invalid routing table",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkMetricInvalid: {
			"Invalid route metric",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkPrefixLengthInvalid: {
			"Invalid prefix length",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkProtocolInvalid: {
			"Invalid network protocol",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkScopeInvalid: {
			"Invalid route scope",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkTypeInvalid: {
			"Invalid network type",
			DomainNetwork,
			http.StatusBadRequest,
		},
		NetworkBondConfigInvalid: {
			"Invalid bond configuration",
			DomainNetwork,
			http.StatusBadRequest,
		},
	}

	// Add to the global errorDefinitions map
	maps.Copy(errorDefinitions, networkErrorDefinitions)
}
