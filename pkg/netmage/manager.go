// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package netmage

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/system/privilege"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/netmage/types"
)

// manager implements the networking management functionality
type manager struct {
	logger     logger.Logger
	executor   *CommandExecutor
	renderer   types.Renderer
	sudoOps    *privilege.SudoFileOperations
	netplanCmd *NetplanCommand
}

// NewManager creates a new networking manager instance
func NewManager(
	ctx context.Context,
	logger logger.Logger,
	renderer types.Renderer,
) (types.Manager, error) {
	if logger == nil {
		return nil, errors.New(errors.NetworkOperationFailed, "logger cannot be nil")
	}

	// Create command executor with sudo support for network operations
	executor := NewCommandExecutor(true)

	// Validate renderer
	if renderer != types.RendererNetworkd && renderer != types.RendererNetworkManager {
		renderer = types.RendererNetworkd // Default to networkd
	}

	// Create sudo file operations for privileged file access
	allowedPaths := []string{"/etc/netplan", "/etc/systemd/resolved.conf.d"}
	sudoOps := privilege.NewSudoFileOperations(
		logger,
		command.NewCommandExecutor(true),
		allowedPaths,
	)

	// Initialize netplan command wrapper
	netplanCmd := NewNetplanCommand(executor, sudoOps)

	m := &manager{
		logger:     logger,
		executor:   executor,
		renderer:   renderer,
		sudoOps:    sudoOps,
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
	m.logger.Debug("Checking netplan availability",
		"result", result.Stdout,
		"error", err)
	if err != nil || result.ExitCode != 0 {
		return errors.Wrap(err, errors.NetplanCommandNotFound)
	}

	return nil
}

// GetInterface retrieves information about a specific network interface
func (m *manager) GetInterface(ctx context.Context, name string) (*types.NetworkInterface, error) {
	if name == "" {
		return nil, errors.New(errors.NetworkInterfaceNotFound, "interface name cannot be empty")
	}

	// Get netplan status for the specific interface
	status, err := m.netplanCmd.GetStatus(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, errors.NetworkInterfaceNotFound)
	}

	// Check if interface exists in netplan status
	ifaceStatus, exists := status.Interfaces[name]
	if !exists {
		return nil, errors.New(errors.NetworkInterfaceNotFound,
			fmt.Sprintf("interface %s not found", name))
	}

	// Convert netplan status to NetworkInterface
	iface := m.convertInterfaceStatus(name, ifaceStatus)

	return iface, nil
}

// ListInterfaces retrieves information about all network interfaces
func (m *manager) ListInterfaces(ctx context.Context) ([]*types.NetworkInterface, error) {
	// Get netplan status for all interfaces
	status, err := m.netplanCmd.GetStatus(ctx, "")
	if err != nil {
		return nil, errors.Wrap(err, errors.NetworkOperationFailed)
	}

	// Convert netplan status to NetworkInterface list
	var interfaces []*types.NetworkInterface
	for name, ifaceStatus := range status.Interfaces {
		iface := m.convertInterfaceStatus(name, ifaceStatus)
		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// SetInterfaceState sets the administrative state of an interface using networkctl
func (m *manager) SetInterfaceState(
	ctx context.Context,
	name string,
	state types.InterfaceState,
) error {
	if name == "" {
		return errors.New(errors.NetworkInterfaceNotFound, "interface name cannot be empty")
	}

	var cmd string
	switch state {
	case types.InterfaceStateUp:
		cmd = "up"
	case types.InterfaceStateDown:
		cmd = "down"
	default:
		return errors.New(errors.NetworkOperationFailed,
			fmt.Sprintf("invalid interface state: %s", state))
	}

	result, err := m.executor.ExecuteCommand(ctx, "networkctl", cmd, name)
	if err != nil {
		return errors.Wrap(err, errors.NetworkInterfaceOperationFailed).
			WithMetadata("interface", name).
			WithMetadata("state", string(state)).
			WithMetadata("output", result.Stderr)
	}

	return nil
}

// GetInterfaceStatistics retrieves statistics for a network interface
func (m *manager) GetInterfaceStatistics(
	ctx context.Context,
	name string,
) (*types.InterfaceStatistics, error) {
	if name == "" {
		return nil, errors.New(errors.NetworkInterfaceNotFound, "interface name cannot be empty")
	}

	// For now, return empty statistics as netplan doesn't provide interface statistics
	// This would need to be implemented using /proc/net/dev or similar
	return &types.InterfaceStatistics{}, nil
}

// AddIPAddress adds an IP address to an interface via netplan configuration
func (m *manager) AddIPAddress(ctx context.Context, iface string, address string) error {
	if iface == "" || address == "" {
		return errors.New(errors.NetworkAddressInvalid, "interface and address cannot be empty")
	}

	if err := m.ValidateIPAddress(address); err != nil {
		return err
	}

	// Get current netplan config
	config, err := m.netplanCmd.GetConfig(ctx)
	if err != nil {
		return errors.Wrap(err, errors.NetworkOperationFailed)
	}

	// Find and update the interface configuration
	if config.Network == nil {
		config.Network = &types.NetworkConfig{
			Version:   types.DefaultNetplanConfigVersion,
			Renderer:  m.renderer,
			Ethernets: make(map[string]*types.EthernetConfig),
		}
	}

	if config.Network.Ethernets == nil {
		config.Network.Ethernets = make(map[string]*types.EthernetConfig)
	}

	// Get or create ethernet config for interface
	ethConfig, exists := config.Network.Ethernets[iface]
	if !exists {
		ethConfig = &types.EthernetConfig{}
		config.Network.Ethernets[iface] = ethConfig
	}

	// Add address to the list if not already present
	for _, addr := range ethConfig.Addresses {
		if addr == address {
			return nil // Address already exists
		}
	}
	ethConfig.Addresses = append(ethConfig.Addresses, address)

	// Update netplan configuration
	if err := m.netplanCmd.SetConfig(ctx, config); err != nil {
		return errors.Wrap(err, errors.NetworkOperationFailed)
	}

	return nil
}

// RemoveIPAddress removes an IP address from an interface via netplan configuration
func (m *manager) RemoveIPAddress(ctx context.Context, iface string, address string) error {
	if iface == "" || address == "" {
		return errors.New(errors.NetworkAddressInvalid, "interface and address cannot be empty")
	}

	// Get current netplan config
	config, err := m.netplanCmd.GetConfig(ctx)
	if err != nil {
		return errors.Wrap(err, errors.NetworkOperationFailed)
	}

	if config.Network == nil || config.Network.Ethernets == nil {
		return errors.New(errors.NetworkInterfaceNotFound, "interface not found in configuration")
	}

	// Find interface configuration
	ethConfig, exists := config.Network.Ethernets[iface]
	if !exists {
		return errors.New(errors.NetworkInterfaceNotFound, "interface not found in configuration")
	}

	// Remove address from the list
	var newAddresses []string
	for _, addr := range ethConfig.Addresses {
		if addr != address {
			newAddresses = append(newAddresses, addr)
		}
	}
	ethConfig.Addresses = newAddresses

	// Update netplan configuration
	if err := m.netplanCmd.SetConfig(ctx, config); err != nil {
		return errors.Wrap(err, errors.NetworkOperationFailed)
	}

	return nil
}

// GetIPAddresses retrieves all IP addresses for an interface
func (m *manager) GetIPAddresses(ctx context.Context, iface string) ([]*types.IPAddress, error) {
	if iface == "" {
		return nil, errors.New(errors.NetworkInterfaceNotFound, "interface name cannot be empty")
	}

	// Get interface information which includes addresses
	networkInterface, err := m.GetInterface(ctx, iface)
	if err != nil {
		return nil, err
	}

	return networkInterface.IPAddresses, nil
}

// AddRoute adds a network route via netplan configuration
func (m *manager) AddRoute(ctx context.Context, route *types.Route) error {
	if route == nil {
		return errors.New(errors.NetworkRouteOperationFailed, "route cannot be nil")
	}

	if route.To == "" {
		return errors.New(errors.NetworkRouteOperationFailed, "route destination cannot be empty")
	}

	// Add route to appropriate interface configuration
	// This is a simplified implementation - would need to determine the correct interface
	// and update the netplan configuration accordingly
	return errors.New(
		errors.NetworkFeatureUnsupported,
		"route management via netplan not fully implemented",
	)
}

// RemoveRoute removes a network route via netplan configuration
func (m *manager) RemoveRoute(ctx context.Context, route *types.Route) error {
	if route == nil {
		return errors.New(errors.NetworkRouteOperationFailed, "route cannot be nil")
	}

	if route.To == "" {
		return errors.New(errors.NetworkRouteOperationFailed, "route destination cannot be empty")
	}

	// Remove route from appropriate interface configuration
	// This is a simplified implementation - would need to determine the correct interface
	// and update the netplan configuration accordingly
	return errors.New(
		errors.NetworkFeatureUnsupported,
		"route management via netplan not fully implemented",
	)
}

// GetRoutes retrieves network routes from netplan status
func (m *manager) GetRoutes(ctx context.Context, table string) ([]*types.Route, error) {
	// Get netplan status for all interfaces
	status, err := m.netplanCmd.GetStatus(ctx, "")
	if err != nil {
		return nil, errors.Wrap(err, errors.NetworkOperationFailed)
	}

	// Collect routes from all interfaces
	var routes []*types.Route
	for _, ifaceStatus := range status.Interfaces {
		for _, routeStatus := range ifaceStatus.Routes {
			// Filter by table if specified
			if table != "" && routeStatus.Table != table {
				continue
			}

			route := &types.Route{
				To:       routeStatus.To,
				From:     routeStatus.From,
				Via:      routeStatus.Via,
				Table:    routeStatus.Table,
				Metric:   routeStatus.Metric,
				Family:   types.Family(routeStatus.Family),
				Type:     types.RouteType(routeStatus.Type),
				Scope:    types.RouteScope(routeStatus.Scope),
				Protocol: types.RouteProtocol(routeStatus.Protocol),
			}
			routes = append(routes, route)
		}
	}

	return routes, nil
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
func (m *manager) TryNetplanConfig(
	ctx context.Context,
	timeout time.Duration,
) (*types.NetplanTryResult, error) {
	if timeout <= 0 {
		timeout = 120 * time.Second // Default timeout
	}

	return m.netplanCmd.Try(ctx, timeout)
}

// GetNetplanStatus retrieves netplan status
func (m *manager) GetNetplanStatus(
	ctx context.Context,
	iface string,
) (*types.NetplanStatus, error) {
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
	result, err := m.executor.ExecuteCommand(ctx, "hostnamectl hostname")
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

	// Get DNS information from resolvectl for accuracy
	if dns, err := m.GetGlobalDNS(ctx); err == nil {
		info.DNSServers = dns.Addresses
		info.SearchDomains = dns.Search
	}

	return info, nil
}

// GetGlobalDNS retrieves the current global DNS configuration with fallback
func (m *manager) GetGlobalDNS(ctx context.Context) (*types.NameserverConfig, error) {
	// First try resolvectl status for accurate DNS information
	result, err := m.executor.ExecuteCommand(ctx, "resolvectl", "status", "--no-pager")
	if err == nil {
		dns, parseErr := m.parseResolvectlStatus(result.Stdout)
		if parseErr == nil && len(dns.Addresses) > 0 && len(dns.Search) > 0 {
			// Only use resolvectl if we have both addresses and search domains
			m.logger.Debug("Retrieved global DNS configuration from resolvectl",
				"addresses", dns.Addresses,
				"search", dns.Search)
			return dns, nil
		}
	}

	// Fall back to netplan status if resolvectl doesn't have global DNS info
	status, err := m.GetNetplanStatus(ctx, "")
	if err != nil {
		return nil, errors.Wrap(err, errors.NetworkOperationFailed)
	}

	dns := &types.NameserverConfig{}

	// Extract DNS from netplan global state if available
	if status.NetplanGlobalState != nil && status.NetplanGlobalState.Nameservers != nil {
		dns.Addresses = status.NetplanGlobalState.Nameservers.Addresses
		dns.Search = status.NetplanGlobalState.Nameservers.Search
	}

	m.logger.Debug("Retrieved global DNS configuration from netplan fallback",
		"addresses", dns.Addresses,
		"search", dns.Search)

	return dns, nil
}

// SetGlobalDNS sets the global DNS configuration via systemd-resolved
func (m *manager) SetGlobalDNS(ctx context.Context, dns *types.NameserverConfig) error {
	if dns == nil {
		return errors.New(errors.NetworkConfigurationInvalid, "DNS configuration cannot be nil")
	}

	// Validate DNS addresses
	for _, addr := range dns.Addresses {
		if err := m.ValidateIPAddress(addr); err != nil {
			return errors.Wrap(err, errors.NetworkIPAddressInvalid).
				WithMetadata("dns_address", addr)
		}
	}

	m.logger.Info("Setting global DNS configuration",
		"addresses", dns.Addresses,
		"search", dns.Search)

	// Create systemd-resolved configuration file
	configPath := "/etc/systemd/resolved.conf.d/90-netmage.conf"
	configContent := "[Resolve]\n"

	if len(dns.Addresses) > 0 {
		configContent += "DNS=" + strings.Join(dns.Addresses, " ") + "\n"
	}

	if len(dns.Search) > 0 {
		configContent += "Domains=" + strings.Join(dns.Search, " ") + "\n"
	}

	// Ensure the directory exists
	result, err := m.executor.ExecuteCommand(ctx, "mkdir", "-p", "/etc/systemd/resolved.conf.d")
	if err != nil {
		return errors.Wrap(err, errors.NetworkOperationFailed).
			WithMetadata("output", result.Stderr)
	}

	// Write the configuration file
	if err := m.sudoOps.WriteFile(ctx, configPath, []byte(configContent), 0644); err != nil {
		return errors.Wrap(err, errors.NetworkOperationFailed).
			WithMetadata("config_path", configPath)
	}

	// Restart systemd-resolved to apply the changes
	result, err = m.executor.ExecuteCommand(ctx, "systemctl", "restart", "systemd-resolved")
	if err != nil {
		return errors.Wrap(err, errors.NetworkOperationFailed).
			WithMetadata("output", result.Stderr)
	}

	m.logger.Info("Global DNS configuration applied successfully",
		"config_path", configPath)

	return nil
}

// convertInterfaceStatus converts netplan InterfaceStatus to NetworkInterface
func (m *manager) convertInterfaceStatus(
	name string,
	ifaceStatus *types.InterfaceStatus,
) *types.NetworkInterface {
	iface := &types.NetworkInterface{
		Index:        ifaceStatus.Index,
		Name:         name,
		Type:         types.DeviceType(ifaceStatus.Type),
		MACAddress:   ifaceStatus.MACAddress,
		AdminState:   types.InterfaceState(ifaceStatus.AdminState),
		OperState:    types.InterfaceState(ifaceStatus.OperState),
		Backend:      ifaceStatus.Backend,
		Vendor:       ifaceStatus.Vendor,
		DNSAddresses: ifaceStatus.DNSAddresses,
		DNSSearch:    ifaceStatus.DNSSearch,
		Interfaces:   ifaceStatus.Interfaces,
		Bridge:       ifaceStatus.Bridge,
	}

	// Convert addresses - netplan format is []map[string]*AddressStatus
	var addresses []*types.IPAddress
	for _, addrMap := range ifaceStatus.Addresses {
		for addrStr, addrStatus := range addrMap {
			address := &types.IPAddress{
				Address:      addrStr,
				PrefixLength: addrStatus.Prefix,
				Flags:        addrStatus.Flags,
			}
			// Determine address family from format
			if strings.Contains(addrStr, ":") {
				address.Family = types.FamilyIPv6
			} else {
				address.Family = types.FamilyIPv4
			}
			addresses = append(addresses, address)
		}
	}
	iface.IPAddresses = addresses

	// Convert routes
	var routes []*types.Route
	for _, routeStatus := range ifaceStatus.Routes {
		route := &types.Route{
			To:       routeStatus.To,
			From:     routeStatus.From,
			Via:      routeStatus.Via,
			Table:    routeStatus.Table,
			Metric:   routeStatus.Metric,
			Family:   types.Family(routeStatus.Family),
			Type:     types.RouteType(routeStatus.Type),
			Scope:    types.RouteScope(routeStatus.Scope),
			Protocol: types.RouteProtocol(routeStatus.Protocol),
		}
		routes = append(routes, route)
	}
	iface.Routes = routes

	return iface
}

// parseResolvectlStatus parses the output of `resolvectl status` to extract global DNS configuration
func (m *manager) parseResolvectlStatus(output string) (*types.NameserverConfig, error) {
	dns := &types.NameserverConfig{}
	lines := strings.Split(output, "\n")
	
	inGlobalSection := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check if we're entering the Global section
		if trimmed == "Global" {
			inGlobalSection = true
			continue
		}
		
		// Check if we're leaving the Global section (new Link section starts)
		if strings.HasPrefix(trimmed, "Link ") {
			inGlobalSection = false
			continue
		}
		
		// Only process lines in the Global section
		if !inGlobalSection {
			continue
		}
		
		// Parse DNS Servers line
		if strings.HasPrefix(trimmed, "DNS Servers:") {
			dnsLine := strings.TrimPrefix(trimmed, "DNS Servers:")
			dnsLine = strings.TrimSpace(dnsLine)
			if dnsLine != "" {
				// Split by spaces and filter out empty strings
				servers := strings.Fields(dnsLine)
				dns.Addresses = servers
			}
		}
		
		// Parse DNS Domain line
		if strings.HasPrefix(trimmed, "DNS Domain:") {
			domainLine := strings.TrimPrefix(trimmed, "DNS Domain:")
			domainLine = strings.TrimSpace(domainLine)
			if domainLine != "" {
				// DNS domains are space-separated
				domains := strings.Fields(domainLine)
				dns.Search = domains
			}
		}
	}
	
	return dns, nil
}
