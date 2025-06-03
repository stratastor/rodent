// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/netmage"
	"github.com/stratastor/rodent/pkg/netmage/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommandExecutorIntegration tests direct command execution to validate
// internal parsing and type structures without HTTP/gRPC overhead
func TestCommandExecutorIntegration(t *testing.T) {
	if os.Getenv("SKIP_NETMAGE_TESTS") == "true" {
		t.Skip("Netmage tests skipped via SKIP_NETMAGE_TESTS environment variable")
	}

	// Create logger
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test.netmage.executor")
	require.NoError(t, err, "Failed to create logger")

	// Create netmage manager
	ctx := context.Background()
	manager, err := netmage.NewManager(ctx, log, types.RendererNetworkd)
	require.NoError(t, err, "Failed to create netmage manager")

	t.Run("ValidateNetplanStatusParsing", func(t *testing.T) {
		// Test netplan status parsing directly
		status, err := manager.GetNetplanStatus(ctx, "")
		require.NoError(t, err, "Failed to get netplan status")
		assert.NotNil(t, status, "Status should not be nil")

		// Validate structure
		if status.NetplanGlobalState != nil {
			t.Logf("Global state online: %v", status.NetplanGlobalState.Online)
			if status.NetplanGlobalState.Nameservers != nil {
				t.Logf("Global DNS addresses: %v", status.NetplanGlobalState.Nameservers.Addresses)
			}
		}

		// Validate interfaces parsing
		assert.NotNil(t, status.Interfaces, "Interfaces map should not be nil")
		for name, iface := range status.Interfaces {
			t.Logf("Interface %s: index=%d, type=%s, admin=%s, oper=%s",
				name, iface.Index, iface.Type, iface.AdminState, iface.OperState)

			// Validate addresses structure - should be []map[string]*AddressStatus
			for i, addrMap := range iface.Addresses {
				for addrStr, addrStatus := range addrMap {
					t.Logf("  Address %d: %s (prefix: %d, flags: %v)",
						i, addrStr, addrStatus.Prefix, addrStatus.Flags)
					assert.True(t, addrStatus.Prefix > 0, "Prefix should be positive")
				}
			}

			// Validate routes
			for i, route := range iface.Routes {
				t.Logf("  Route %d: to=%s, via=%s, table=%s",
					i, route.To, route.Via, route.Table)
			}
		}
	})

	t.Run("ValidateNetplanConfigParsing", func(t *testing.T) {
		// Test netplan config parsing
		config, err := manager.GetNetplanConfig(ctx)
		require.NoError(t, err, "Failed to get netplan config")
		assert.NotNil(t, config, "Config should not be nil")
		assert.NotNil(t, config.Network, "Network section should not be nil")

		// Validate basic structure
		assert.Equal(t, types.DefaultNetplanConfigVersion, config.Network.Version,
			"Version should match expected")

		t.Logf("Netplan version: %d", config.Network.Version)
		t.Logf("Renderer: %s", config.Network.Renderer)

		// Log ethernet configurations if present
		if config.Network.Ethernets != nil {
			for name, eth := range config.Network.Ethernets {
				t.Logf("Ethernet %s: DHCP4=%v, addresses=%v",
					name, eth.DHCPv4, eth.Addresses)
			}
		}
	})

	t.Run("ValidateInterfaceConversion", func(t *testing.T) {
		// Test interface listing and conversion
		interfaces, err := manager.ListInterfaces(ctx)
		require.NoError(t, err, "Failed to list interfaces")
		assert.NotEmpty(t, interfaces, "Should have at least one interface")

		// Validate first interface conversion
		iface := interfaces[0]
		t.Logf("First interface: %s (type: %s, index: %d)",
			iface.Name, iface.Type, iface.Index)

		assert.NotEmpty(t, iface.Name, "Interface name should not be empty")
		assert.True(t, iface.Index > 0, "Interface index should be positive")

		// Validate IP addresses conversion
		for i, addr := range iface.IPAddresses {
			t.Logf("  IP %d: %s/%d (family: %d)",
				i, addr.Address, addr.PrefixLength, addr.Family)
			assert.NotEmpty(t, addr.Address, "Address should not be empty")
			assert.True(t, addr.PrefixLength > 0, "Prefix length should be positive")
			assert.True(t, addr.Family == types.FamilyIPv4 || addr.Family == types.FamilyIPv6,
				"Family should be IPv4 or IPv6")
		}
	})

	t.Run("ValidateJSONSerialization", func(t *testing.T) {
		// Test that types serialize/deserialize correctly for API usage
		interfaces, err := manager.ListInterfaces(ctx)
		require.NoError(t, err, "Failed to list interfaces")

		// Serialize to JSON
		jsonData, err := json.Marshal(interfaces)
		require.NoError(t, err, "Failed to marshal interfaces")

		// Deserialize back
		var deserializedInterfaces []*types.NetworkInterface
		err = json.Unmarshal(jsonData, &deserializedInterfaces)
		require.NoError(t, err, "Failed to unmarshal interfaces")

		// Compare basic properties
		assert.Equal(t, len(interfaces), len(deserializedInterfaces),
			"Should have same number of interfaces")

		if len(interfaces) > 0 && len(deserializedInterfaces) > 0 {
			original := interfaces[0]
			deserialized := deserializedInterfaces[0]

			assert.Equal(t, original.Name, deserialized.Name, "Names should match")
			assert.Equal(t, original.Index, deserialized.Index, "Indices should match")
			assert.Equal(t, original.Type, deserialized.Type, "Types should match")

			t.Logf("JSON serialization validation passed for interface %s", original.Name)
		}
	})

	t.Run("ValidateIPValidation", func(t *testing.T) {
		// Test IP validation logic directly
		validIPs := []string{
			"192.168.1.1",
			"192.168.1.1/24",
			"10.0.0.1/8",
			"2001:db8::1",
			"2001:db8::/64",
		}

		invalidIPs := []string{
			"invalid.ip",
			"192.168.1.1/33", // Invalid CIDR
			"256.256.256.256",
			"192.168.1", // Incomplete
		}

		for _, ip := range validIPs {
			err := manager.ValidateIPAddress(ip)
			assert.NoError(t, err, "IP %s should be valid", ip)
		}

		for _, ip := range invalidIPs {
			err := manager.ValidateIPAddress(ip)
			assert.Error(t, err, "IP %s should be invalid", ip)
		}
	})

	t.Run("ValidateInterfaceNameValidation", func(t *testing.T) {
		// Test interface name validation
		validNames := []string{
			"eth0",
			"ens3",
			"enX0",
			"br0",
			"bond0",
			"vlan.100",
		}

		invalidNames := []string{
			"", // Empty
			"verylonginterfacenamethatexceedslimit", // Too long
			"eth@0", // Invalid character
			"eth 0", // Space
		}

		for _, name := range validNames {
			err := manager.ValidateInterfaceName(name)
			assert.NoError(t, err, "Interface name %s should be valid", name)
		}

		for _, name := range invalidNames {
			err := manager.ValidateInterfaceName(name)
			assert.Error(t, err, "Interface name %s should be invalid", name)
		}
	})

	t.Run("ValidateSystemInfo", func(t *testing.T) {
		// Test system network info gathering
		info, err := manager.GetSystemNetworkInfo(ctx)
		require.NoError(t, err, "Failed to get system network info")
		assert.NotNil(t, info, "System info should not be nil")

		t.Logf("System hostname: %s", info.Hostname)
		t.Logf("Interface count: %d", info.InterfaceCount)
		t.Logf("Renderer: %s", info.Renderer)
		t.Logf("DNS servers: %v", info.DNSServers)
		t.Logf("Search domains: %v", info.SearchDomains)

		assert.True(t, info.InterfaceCount > 0, "Should have at least one interface")
		assert.NotEmpty(t, info.Renderer, "Renderer should be set")
		assert.Equal(t, len(info.Interfaces), info.InterfaceCount,
			"Interface count should match interfaces slice length")
	})
}

// TestAPIRequestResponseTypes validates that request/response types work correctly
func TestAPIRequestResponseTypes(t *testing.T) {
	t.Run("SafeConfigRequest", func(t *testing.T) {
		// Test SafeConfigRequest serialization
		request := types.SafeConfigRequest{
			Config: &types.NetplanConfig{
				Network: &types.NetworkConfig{
					Version:  2,
					Renderer: types.RendererNetworkd,
				},
			},
			Options: &types.SafeConfigOptions{
				AutoBackup:           true,
				AutoRollback:         true,
				ValidateConnectivity: false,
				SkipPreValidation:    true,
			},
		}

		// Serialize
		jsonData, err := json.Marshal(request)
		require.NoError(t, err, "Failed to marshal SafeConfigRequest")

		// Deserialize
		var parsed types.SafeConfigRequest
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err, "Failed to unmarshal SafeConfigRequest")

		// Validate
		assert.Equal(t, request.Config.Network.Version, parsed.Config.Network.Version)
		assert.Equal(t, request.Config.Network.Renderer, parsed.Config.Network.Renderer)
		assert.Equal(t, request.Options.AutoBackup, parsed.Options.AutoBackup)
		assert.Equal(t, request.Options.ValidateConnectivity, parsed.Options.ValidateConnectivity)

		t.Log("SafeConfigRequest serialization validation passed")
	})

	t.Run("GlobalDNSRequest", func(t *testing.T) {
		// Test GlobalDNSRequest serialization
		request := types.GlobalDNSRequest{
			Addresses: []string{"8.8.8.8", "1.1.1.1"},
			Search:    []string{"example.com", "test.local"},
		}

		// Serialize
		jsonData, err := json.Marshal(request)
		require.NoError(t, err, "Failed to marshal GlobalDNSRequest")

		// Deserialize
		var parsed types.GlobalDNSRequest
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err, "Failed to unmarshal GlobalDNSRequest")

		// Validate
		assert.Equal(t, request.Addresses, parsed.Addresses)
		assert.Equal(t, request.Search, parsed.Search)

		t.Log("GlobalDNSRequest serialization validation passed")
	})

	t.Run("APIResponse", func(t *testing.T) {
		// Test APIResponse structure
		response := APIResponse{
			Success: true,
			Result: map[string]interface{}{
				"test":    "value",
				"number":  42,
				"boolean": true,
			},
		}

		// Serialize
		jsonData, err := json.Marshal(response)
		require.NoError(t, err, "Failed to marshal APIResponse")

		// Deserialize
		var parsed APIResponse
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err, "Failed to unmarshal APIResponse")

		// Validate
		assert.Equal(t, response.Success, parsed.Success)
		assert.NotNil(t, parsed.Result)

		result, ok := parsed.Result.(map[string]interface{})
		require.True(t, ok, "Result should be a map")
		assert.Equal(t, "value", result["test"])
		assert.Equal(t, float64(42), result["number"]) // JSON numbers become float64
		assert.Equal(t, true, result["boolean"])

		t.Log("APIResponse serialization validation passed")
	})
}