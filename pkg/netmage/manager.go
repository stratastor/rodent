// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package netmage

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/netmage/types"
)

// manager implements the networking management functionality
type manager struct {
	logger      logger.Logger
	executor    *CommandExecutor
	renderer    types.Renderer
	ipCmd       *IPCommand
	netplanCmd  *NetplanCommand
}

// NewManager creates a new networking manager instance
func NewManager(ctx context.Context, logger logger.Logger, renderer types.Renderer) (types.Manager, error) {
	if logger == nil {
		return nil, errors.New(errors.NetworkOperationFailed, "logger cannot be nil")
	}

	// Create command executor with sudo support for network operations
	executor := NewCommandExecutor(true)

	// Validate renderer
	if renderer != types.RendererNetworkd && renderer != types.RendererNetworkManager {
		renderer = types.RendererNetworkd // Default to networkd
	}

	// Initialize command wrappers
	ipCmd := NewIPCommand(executor)
	netplanCmd := NewNetplanCommand(executor)

	m := &manager{
		logger:     logger,
		executor:   executor,
		renderer:   renderer,
		ipCmd:      ipCmd,
		netplanCmd: netplanCmd,
	}

	// Validate netplan availability (simplified check)
	if err := m.validateNetplanAvailability(ctx); err != nil {
		return nil, errors.Wrap(err, errors.NetplanCommandFailed)
	}

	m.logger.Info("Network manager initialized", 
		"renderer", renderer,
		"netplan_major_version", "1.x")

	return m, nil
}

// validateNetplanAvailability checks if netplan is available
func (m *manager) validateNetplanAvailability(ctx context.Context) error {
	// Check if netplan is available
	result, err := m.executor.ExecuteCommand(ctx, "which", "netplan")
	if err != nil || result.ExitCode != 0 {
		return errors.New(errors.NetplanCommandFailed, "netplan not found in system PATH")
	}

	// Simple functionality check
	result, err = m.executor.ExecuteCommand(ctx, "netplan", "--help")
	if err != nil || result.ExitCode != 0 {
		return errors.New(errors.NetplanCommandFailed, "netplan command not functional")
	}

	return nil
}

// GetInterface retrieves information about a specific network interface
func (m *manager) GetInterface(ctx context.Context, name string) (*types.NetworkInterface, error) {
	if name == "" {
		return nil, errors.New(errors.NetworkInterfaceNotFound, "interface name cannot be empty")
	}

	// Get basic interface info
	interfaces, err := m.ipCmd.ShowInterface(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, errors.NetworkInterfaceNotFound)
	}

	if len(interfaces) == 0 {
		return nil, errors.New(errors.NetworkInterfaceNotFound, 
			fmt.Sprintf("interface %s not found", name))
	}

	iface := interfaces[0]

	// Get IP addresses
	addresses, err := m.ipCmd.GetAddresses(ctx, name)
	if err != nil {
		m.logger.Warn("Failed to get IP addresses for interface", 
			"interface", name, "error", err)
		// Don't fail completely, just log the warning
	} else {
		iface.IPAddresses = addresses
	}

	// Get routes for this interface
	routes, err := m.ipCmd.GetRoutes(ctx, "", name)
	if err != nil {
		m.logger.Warn("Failed to get routes for interface", 
			"interface", name, "error", err)
	} else {
		iface.Routes = routes
	}

	// Get netplan status if available
	status, err := m.netplanCmd.GetStatus(ctx, name)
	if err != nil {
		m.logger.Debug("Failed to get netplan status for interface", 
			"interface", name, "error", err)
	} else if ifaceStatus, exists := status.Interfaces[name]; exists {
		// Merge netplan status information
		iface.Backend = ifaceStatus.Backend
		iface.Vendor = ifaceStatus.Vendor
		iface.DNSAddresses = ifaceStatus.DNSAddresses
		iface.DNSSearch = ifaceStatus.DNSSearch
	}

	return iface, nil
}

// ListInterfaces retrieves information about all network interfaces
func (m *manager) ListInterfaces(ctx context.Context) ([]*types.NetworkInterface, error) {
	interfaces, err := m.ipCmd.ShowInterface(ctx, "")
	if err != nil {
		return nil, errors.Wrap(err, errors.NetworkOperationFailed)
	}

	// Enhance each interface with additional information
	for _, iface := range interfaces {
		// Get IP addresses
		addresses, err := m.ipCmd.GetAddresses(ctx, iface.Name)
		if err != nil {
			m.logger.Warn("Failed to get IP addresses for interface", 
				"interface", iface.Name, "error", err)
		} else {
			iface.IPAddresses = addresses
		}

		// Get routes for this interface
		routes, err := m.ipCmd.GetRoutes(ctx, "", iface.Name)
		if err != nil {
			m.logger.Warn("Failed to get routes for interface", 
				"interface", iface.Name, "error", err)
		} else {
			iface.Routes = routes
		}
	}

	// Get netplan status for all interfaces
	status, err := m.netplanCmd.GetStatus(ctx, "")
	if err != nil {
		m.logger.Debug("Failed to get netplan status", "error", err)
	} else {
		// Merge netplan status information
		for _, iface := range interfaces {
			if ifaceStatus, exists := status.Interfaces[iface.Name]; exists {
				iface.Backend = ifaceStatus.Backend
				iface.Vendor = ifaceStatus.Vendor
				iface.DNSAddresses = ifaceStatus.DNSAddresses
				iface.DNSSearch = ifaceStatus.DNSSearch
			}
		}
	}

	return interfaces, nil
}

// SetInterfaceState sets the administrative state of an interface
func (m *manager) SetInterfaceState(ctx context.Context, name string, state types.InterfaceState) error {
	if name == "" {
		return errors.New(errors.NetworkInterfaceNotFound, "interface name cannot be empty")
	}

	switch state {
	case types.InterfaceStateUp:
		return m.ipCmd.SetInterfaceUp(ctx, name)
	case types.InterfaceStateDown:
		return m.ipCmd.SetInterfaceDown(ctx, name)
	default:
		return errors.New(errors.NetworkOperationFailed, 
			fmt.Sprintf("invalid interface state: %s", state))
	}
}

// GetInterfaceStatistics retrieves statistics for a network interface
func (m *manager) GetInterfaceStatistics(ctx context.Context, name string) (*types.InterfaceStatistics, error) {
	if name == "" {
		return nil, errors.New(errors.NetworkInterfaceNotFound, "interface name cannot be empty")
	}

	return m.ipCmd.GetStatistics(ctx, name)
}

// AddIPAddress adds an IP address to an interface
func (m *manager) AddIPAddress(ctx context.Context, iface string, address string) error {
	if iface == "" || address == "" {
		return errors.New(errors.NetworkAddressInvalid, "interface and address cannot be empty")
	}

	if err := m.ValidateIPAddress(address); err != nil {
		return err
	}

	return m.ipCmd.AddAddress(ctx, iface, address)
}

// RemoveIPAddress removes an IP address from an interface
func (m *manager) RemoveIPAddress(ctx context.Context, iface string, address string) error {
	if iface == "" || address == "" {
		return errors.New(errors.NetworkAddressInvalid, "interface and address cannot be empty")
	}

	return m.ipCmd.RemoveAddress(ctx, iface, address)
}

// GetIPAddresses retrieves all IP addresses for an interface
func (m *manager) GetIPAddresses(ctx context.Context, iface string) ([]*types.IPAddress, error) {
	if iface == "" {
		return nil, errors.New(errors.NetworkInterfaceNotFound, "interface name cannot be empty")
	}

	return m.ipCmd.GetAddresses(ctx, iface)
}

// AddRoute adds a network route
func (m *manager) AddRoute(ctx context.Context, route *types.Route) error {
	if route == nil {
		return errors.New(errors.NetworkRouteOperationFailed, "route cannot be nil")
	}

	if route.To == "" {
		return errors.New(errors.NetworkRouteOperationFailed, "route destination cannot be empty")
	}

	return m.ipCmd.AddRoute(ctx, route)
}

// RemoveRoute removes a network route
func (m *manager) RemoveRoute(ctx context.Context, route *types.Route) error {
	if route == nil {
		return errors.New(errors.NetworkRouteOperationFailed, "route cannot be nil")
	}

	if route.To == "" {
		return errors.New(errors.NetworkRouteOperationFailed, "route destination cannot be empty")
	}

	return m.ipCmd.RemoveRoute(ctx, route)
}

// GetRoutes retrieves network routes
func (m *manager) GetRoutes(ctx context.Context, table string) ([]*types.Route, error) {
	return m.ipCmd.GetRoutes(ctx, table, "")
}

// GetNetplanConfig retrieves the current netplan configuration
func (m *manager) GetNetplanConfig(ctx context.Context) (*types.NetplanConfig, error) {
	return m.netplanCmd.GetConfig(ctx)
}

// SetNetplanConfig updates the netplan configuration
func (m *manager) SetNetplanConfig(ctx context.Context, config *types.NetplanConfig) error {
	if config == nil {
		return errors.New(errors.NetplanConfigInvalid, "configuration cannot be nil")
	}

	// Validate the configuration
	if err := m.ValidateNetplanConfig(ctx, config); err != nil {
		return err
	}

	return m.netplanCmd.SetConfig(ctx, config)
}

// ApplyNetplanConfig applies the current netplan configuration
func (m *manager) ApplyNetplanConfig(ctx context.Context) error {
	return m.netplanCmd.Apply(ctx)
}

// TryNetplanConfig tries a netplan configuration with automatic rollback
func (m *manager) TryNetplanConfig(ctx context.Context, timeout time.Duration) (*types.NetplanTryResult, error) {
	if timeout <= 0 {
		timeout = 120 * time.Second // Default timeout
	}

	return m.netplanCmd.Try(ctx, timeout)
}

// GetNetplanStatus retrieves netplan status
func (m *manager) GetNetplanStatus(ctx context.Context, iface string) (*types.NetplanStatus, error) {
	return m.netplanCmd.GetStatus(ctx, iface)
}

// GetNetplanDiff retrieves differences between current state and netplan config
func (m *manager) GetNetplanDiff(ctx context.Context) (*types.NetplanDiff, error) {
	return m.netplanCmd.GetDiff(ctx)
}

// BackupNetplanConfig creates a backup of the current netplan configuration
func (m *manager) BackupNetplanConfig(ctx context.Context) (string, error) {
	return m.netplanCmd.Backup(ctx)
}

// RestoreNetplanConfig restores netplan configuration from a backup
func (m *manager) RestoreNetplanConfig(ctx context.Context, backupID string) error {
	if backupID == "" {
		return errors.New(errors.NetplanRestoreFailed, "backup ID cannot be empty")
	}

	return m.netplanCmd.Restore(ctx, backupID)
}

// ListBackups lists available netplan configuration backups
func (m *manager) ListBackups(ctx context.Context) ([]*types.ConfigBackup, error) {
	return m.netplanCmd.ListBackups(ctx)
}

// ValidateNetplanConfig validates a netplan configuration
func (m *manager) ValidateNetplanConfig(ctx context.Context, config *types.NetplanConfig) error {
	if config == nil {
		return errors.New(errors.NetplanConfigInvalid, "configuration cannot be nil")
	}

	if config.Network == nil {
		return errors.New(errors.NetplanConfigInvalid, "network section cannot be nil")
	}

	// Validate version
	if config.Network.Version != types.DefaultNetplanConfigVersion {
		return errors.New(errors.NetplanConfigInvalid, 
			fmt.Sprintf("unsupported network version: %d", config.Network.Version))
	}

	// Validate renderer
	if config.Network.Renderer != "" {
		if config.Network.Renderer != types.RendererNetworkd && 
		   config.Network.Renderer != types.RendererNetworkManager {
			return errors.New(errors.NetplanRendererInvalid, 
				fmt.Sprintf("invalid renderer: %s", config.Network.Renderer))
		}
	}

	// Validate interface configurations
	if err := m.validateInterfaceConfigs(config.Network); err != nil {
		return err
	}

	return nil
}

// validateInterfaceConfigs validates interface configurations
func (m *manager) validateInterfaceConfigs(network *types.NetworkConfig) error {
	// Validate ethernet interfaces
	for name, eth := range network.Ethernets {
		if err := m.validateEthernetConfig(name, eth); err != nil {
			return err
		}
	}

	// Validate bridge interfaces
	for name, bridge := range network.Bridges {
		if err := m.validateBridgeConfig(name, bridge); err != nil {
			return err
		}
	}

	// Validate bond interfaces
	for name, bond := range network.Bonds {
		if err := m.validateBondConfig(name, bond); err != nil {
			return err
		}
	}

	// Validate VLAN interfaces
	for name, vlan := range network.VLANs {
		if err := m.validateVLANConfig(name, vlan); err != nil {
			return err
		}
	}

	return nil
}

// validateEthernetConfig validates ethernet interface configuration
func (m *manager) validateEthernetConfig(name string, eth *types.EthernetConfig) error {
	if err := m.ValidateInterfaceName(name); err != nil {
		return err
	}

	// Validate MAC address if present
	if eth.MACAddress != "" {
		if err := m.validateMACAddress(eth.MACAddress); err != nil {
			return err
		}
	}

	// Validate match configuration
	if eth.Match != nil && eth.Match.MACAddress != "" {
		if err := m.validateMACAddress(eth.Match.MACAddress); err != nil {
			return err
		}
	}

	// Validate addresses
	for _, addr := range eth.Addresses {
		if err := m.ValidateIPAddress(addr); err != nil {
			return err
		}
	}

	return nil
}

// validateBridgeConfig validates bridge interface configuration
func (m *manager) validateBridgeConfig(name string, bridge *types.BridgeConfig) error {
	if err := m.ValidateInterfaceName(name); err != nil {
		return err
	}

	// Validate member interfaces
	if len(bridge.Interfaces) == 0 {
		return errors.New(errors.NetworkBridgeConfigInvalid, 
			fmt.Sprintf("bridge %s must have at least one member interface", name))
	}

	for _, iface := range bridge.Interfaces {
		if err := m.ValidateInterfaceName(iface); err != nil {
			return errors.Wrap(err, errors.NetworkBridgeConfigInvalid)
		}
	}

	return nil
}

// validateBondConfig validates bond interface configuration
func (m *manager) validateBondConfig(name string, bond *types.BondConfig) error {
	if err := m.ValidateInterfaceName(name); err != nil {
		return err
	}

	// Validate member interfaces
	if len(bond.Interfaces) < 2 {
		return errors.New(errors.NetworkBondConfigInvalid, 
			fmt.Sprintf("bond %s must have at least two member interfaces", name))
	}

	for _, iface := range bond.Interfaces {
		if err := m.ValidateInterfaceName(iface); err != nil {
			return errors.Wrap(err, errors.NetworkBondConfigInvalid)
		}
	}

	return nil
}

// validateVLANConfig validates VLAN interface configuration
func (m *manager) validateVLANConfig(name string, vlan *types.VLANConfig) error {
	if err := m.ValidateInterfaceName(name); err != nil {
		return err
	}

	// Validate VLAN ID
	if vlan.ID < 1 || vlan.ID > 4094 {
		return errors.New(errors.NetworkVLANIDInvalid, 
			fmt.Sprintf("invalid VLAN ID %d: must be between 1 and 4094", vlan.ID))
	}

	// Validate link interface
	if vlan.Link == "" {
		return errors.New(errors.NetworkConfigurationInvalid, 
			fmt.Sprintf("VLAN %s must specify a link interface", name))
	}

	if err := m.ValidateInterfaceName(vlan.Link); err != nil {
		return errors.Wrap(err, errors.NetworkConfigurationInvalid)
	}

	return nil
}

// ValidateIPAddress validates an IP address
func (m *manager) ValidateIPAddress(address string) error {
	// This will handle both single IPs and CIDR notation
	if err := validateIPAddressFormat(address); err != nil {
		return errors.Wrap(err, errors.NetworkIPAddressInvalid)
	}
	return nil
}

// ValidateInterfaceName validates an interface name
func (m *manager) ValidateInterfaceName(name string) error {
	if name == "" {
		return errors.New(errors.NetworkInterfaceNameInvalid, "interface name cannot be empty")
	}

	if len(name) > 15 {
		return errors.New(errors.NetworkInterfaceNameInvalid, 
			fmt.Sprintf("interface name '%s' is too long (max 15 characters)", name))
	}

	// Basic validation - interface names should not contain special characters
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || 
			 (char >= 'A' && char <= 'Z') || 
			 (char >= '0' && char <= '9') || 
			 char == '-' || char == '_' || char == '.') {
			return errors.New(errors.NetworkInterfaceNameInvalid, 
				fmt.Sprintf("interface name '%s' contains invalid character '%c'", name, char))
		}
	}

	return nil
}

// validateMACAddress validates a MAC address format
func (m *manager) validateMACAddress(mac string) error {
	if err := validateMACAddressFormat(mac); err != nil {
		return errors.Wrap(err, errors.NetworkMACAddressInvalid)
	}
	return nil
}

// GetSystemNetworkInfo retrieves overall system network information
func (m *manager) GetSystemNetworkInfo(ctx context.Context) (*types.SystemNetworkInfo, error) {
	info := &types.SystemNetworkInfo{
		Renderer:       m.renderer,
		NetplanVersion: "1.x", // Simplified version assumption
	}

	// Get hostname
	result, err := m.executor.ExecuteCommand(ctx, "hostname")
	if err == nil && result.ExitCode == 0 {
		info.Hostname = result.Stdout
	}

	// Get interfaces
	interfaces, err := m.ListInterfaces(ctx)
	if err != nil {
		return nil, errors.Wrap(err, errors.NetworkOperationFailed)
	}

	info.Interfaces = interfaces
	info.InterfaceCount = len(interfaces)

	// Get default gateway
	routes, err := m.GetRoutes(ctx, "main")
	if err == nil {
		for _, route := range routes {
			if route.To == "default" || route.To == "0.0.0.0/0" {
				if route.Via != "" {
					if ip := net.ParseIP(route.Via); ip != nil {
						info.DefaultGateway = &ip
						break
					}
				}
			}
		}
	}

	// Get DNS information from netplan status
	status, err := m.GetNetplanStatus(ctx, "")
	if err == nil && status.NetplanGlobalState != nil && status.NetplanGlobalState.Nameservers != nil {
		info.DNSServers = status.NetplanGlobalState.Nameservers.Addresses
		info.SearchDomains = status.NetplanGlobalState.Nameservers.Search
	}

	return info, nil
}