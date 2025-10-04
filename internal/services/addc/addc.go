// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package addc

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/stratastor/logger"
	rodentCfg "github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/internal/events"
	"github.com/stratastor/rodent/internal/services"
	"github.com/stratastor/rodent/internal/services/config"
	"github.com/stratastor/rodent/internal/services/docker"
	"github.com/stratastor/rodent/internal/templates"
	"github.com/stratastor/rodent/pkg/netmage"
	"github.com/stratastor/rodent/pkg/netmage/types"
	"github.com/stratastor/rodent/pkg/system"
)

var (
	// Runtime paths for files
	servicesDir            string
	defaultAdDcComposePath string

	// Template file names (no paths needed as they are embedded)
	adDcComposeHostTemplate    = "dc-addc.yml.tmpl"
	adDcComposeMacvlanTemplate = "dc-addc-macvlan.yml.tmpl"
)

func init() {
	servicesDir = rodentCfg.GetServicesDir()
	defaultAdDcComposePath = servicesDir + "/addc/dc-addc.yml"
}

// AdDcConfig contains configuration data for AD DC
type AdDcConfig struct {
	ContainerName   string
	Hostname        string
	Realm           string
	Domain          string
	AdminPassword   string
	DnsForwarder    string
	EtcVolume       string
	PrivateVolume   string
	VarVolume       string
	// Network configuration
	NetworkMode     string // "host" or "macvlan"
	ParentInterface string // Parent interface for macvlan
	IPAddress       string // IP address (with CIDR for macvlan, without for host mode)
	Gateway         string // Gateway for macvlan
	Subnet          string // Subnet for macvlan
}

// Client handles interactions with AD DC
type Client struct {
	logger        logger.Logger
	dockerSvc     *docker.Client
	composeFile   string
	configManager *config.ServiceConfigManager
	executor      *command.CommandExecutor
}

// NewClient creates a new AD DC client
func NewClient(logger logger.Logger) (*Client, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Create Docker client
	dockerSvc, err := docker.NewClient(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Create config manager
	configManager := config.NewServiceConfigManager(logger)

	// Load both templates (host and macvlan)
	hostTemplate, err := templates.GetAddcTemplate(adDcComposeHostTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to load host compose template: %w", err)
	}

	macvlanTemplate, err := templates.GetAddcTemplate(adDcComposeMacvlanTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to load macvlan compose template: %w", err)
	}

	// Output paths
	composePath := defaultAdDcComposePath

	// Register both templates
	// The actual template used will be determined in UpdateConfig based on network mode
	configManager.RegisterTemplate("addc-compose-host", &config.ConfigTemplate{
		Name:        "addc-compose-host",
		Content:     hostTemplate,
		OutputPath:  composePath,
		Permissions: 0644,
		BackupPath:  composePath + ".bak",
	})

	configManager.RegisterTemplate("addc-compose-macvlan", &config.ConfigTemplate{
		Name:        "addc-compose-macvlan",
		Content:     macvlanTemplate,
		OutputPath:  composePath,
		Permissions: 0644,
		BackupPath:  composePath + ".bak",
	})

	// Create command executor for system commands
	executor := command.NewCommandExecutor(true) // Use sudo for network operations

	// Create client
	client := &Client{
		logger:        logger,
		dockerSvc:     dockerSvc,
		composeFile:   defaultAdDcComposePath,
		configManager: configManager,
		executor:      executor,
	}

	// Register state callback for event-based reporting
	configManager.RegisterStateCallback(client.reportConfigChange)

	return client, nil
}

// Name returns the name of the service
func (c *Client) Name() string {
	return "addc"
}

// Status returns the current status of AD DC with detailed information
func (c *Client) Status(ctx context.Context) ([]services.ServiceStatus, error) {
	// Get detailed container information if running
	containers, err := c.dockerSvc.ComposeStatus(ctx, c.composeFile, "addc")
	if err != nil {
		c.logger.Warn("Failed to get detailed container status", "err", err)
		return []services.ServiceStatus{}, nil
	}

	return containers, nil
}

// Start starts the AD DC service
func (c *Client) Start(ctx context.Context) error {
	// Before starting, ensure config is up to date
	if err := c.UpdateConfig(ctx, nil); err != nil {
		return fmt.Errorf("failed to update configuration before starting: %w", err)
	}

	return c.dockerSvc.ComposeUp(ctx, c.composeFile, true)
}

// Stop stops the AD DC service
func (c *Client) Stop(ctx context.Context) error {
	return c.dockerSvc.ComposeDown(ctx, c.composeFile, false)
}

// Restart restarts the AD DC service
func (c *Client) Restart(ctx context.Context) error {
	return c.dockerSvc.ComposeRestart(ctx, c.composeFile)
}

// UpdateConfig updates the configuration files for AD DC
// If config is nil, it will use the values from the global config
func (c *Client) UpdateConfig(ctx context.Context, config *AdDcConfig) error {
	var adDcConfig AdDcConfig

	if config == nil {
		// Use config from the global config if none provided
		cfg := rodentCfg.GetConfig()
		adDcConfig = AdDcConfig{
			ContainerName:   cfg.AD.DC.ContainerName,
			Hostname:        cfg.AD.DC.Hostname,
			Realm:           cfg.AD.DC.Realm,
			Domain:          cfg.AD.DC.Domain,
			AdminPassword:   cfg.AD.AdminPassword,
			DnsForwarder:    cfg.AD.DC.DnsForwarder,
			EtcVolume:       cfg.AD.DC.EtcVolume,
			PrivateVolume:   cfg.AD.DC.PrivateVolume,
			VarVolume:       cfg.AD.DC.VarVolume,
			NetworkMode:     cfg.AD.DC.NetworkMode,
			ParentInterface: cfg.AD.DC.ParentInterface,
			IPAddress:       cfg.AD.DC.IPAddress,
			Gateway:         cfg.AD.DC.Gateway,
			Subnet:          cfg.AD.DC.Subnet,
		}
	} else {
		// Use the provided config
		adDcConfig = *config
	}

	// Determine network mode if set to "auto"
	networkMode := adDcConfig.NetworkMode
	if networkMode == "" || strings.ToLower(networkMode) == "auto" {
		detectedMode, err := c.detectNetworkMode(ctx)
		if err != nil {
			c.logger.Warn("Failed to auto-detect network mode, defaulting to host", "error", err)
			networkMode = "host"
		} else {
			networkMode = detectedMode
			c.logger.Info("Auto-detected network mode", "mode", networkMode)
		}

		// Auto-fill missing parameters for auto mode
		if err := c.autoFillNetworkParams(ctx, &adDcConfig, networkMode); err != nil {
			c.logger.Warn("Failed to auto-fill network parameters", "error", err)
		}
	}
	adDcConfig.NetworkMode = networkMode

	// Validate configuration based on network mode
	if err := c.validateNetworkConfig(ctx, &adDcConfig); err != nil {
		return fmt.Errorf("invalid network configuration: %w", err)
	}

	// Configure netplan for AD DC networking (host mode only, macvlan is handled by Docker)
	if err := c.configureNetplanForADDC(ctx, &adDcConfig); err != nil {
		c.logger.Warn("Failed to configure netplan for AD DC", "error", err)
		// Don't fail completely - netplan config is best-effort
		// User may have manual configuration or different setup
	}

	// Ensure directory exists
	if err := common.EnsureDir(filepath.Dir(c.composeFile), 0755); err != nil {
		return fmt.Errorf("failed to create directory for AD DC: %w", err)
	}

	// Select the appropriate template based on network mode
	templateName := "addc-compose-host"
	if networkMode == "macvlan" {
		templateName = "addc-compose-macvlan"
	}

	// Update docker-compose with the configuration
	if err := c.configManager.UpdateConfig(ctx, templateName, adDcConfig); err != nil {
		return fmt.Errorf("failed to update docker-compose configuration: %w", err)
	}

	c.logger.Info("Successfully updated AD DC configuration",
		"realm", adDcConfig.Realm,
		"domain", adDcConfig.Domain,
		"container", adDcConfig.ContainerName,
		"network_mode", networkMode)

	return nil
}

// detectNetworkMode auto-detects the appropriate network mode based on environment
func (c *Client) detectNetworkMode(ctx context.Context) (string, error) {
	detector := system.NewEnvironmentDetector(c.logger)
	envInfo, err := detector.DetectEnvironment(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to detect environment: %w", err)
	}

	return envInfo.RecommendedNetworkMode, nil
}

// validateNetworkConfig validates the network configuration based on mode
func (c *Client) validateNetworkConfig(ctx context.Context, cfg *AdDcConfig) error {
	switch cfg.NetworkMode {
	case "host":
		// For host mode, validate interface and IP if provided
		if cfg.ParentInterface != "" {
			// Validate the interface exists and is UP
			if err := c.validateInterface(ctx, cfg.ParentInterface); err != nil {
				return fmt.Errorf("parent interface validation failed: %w", err)
			}

			// If IP is specified, validate it's assigned to the interface
			if cfg.IPAddress != "" {
				if err := c.validateIPOnInterface(ctx, cfg.ParentInterface, cfg.IPAddress); err != nil {
					c.logger.Warn("IP address may not be configured on interface",
						"interface", cfg.ParentInterface,
						"ip", cfg.IPAddress,
						"error", err)
					// Don't fail - just warn, as the IP might be configured via netplan later
				}
			}
		} else if cfg.IPAddress == "" {
			c.logger.Warn("No interface or IP specified for host mode - AD DC will use host's primary IP")
		}
		return nil

	case "macvlan":
		// For MACVLAN mode, we need all network parameters
		if cfg.ParentInterface == "" {
			return fmt.Errorf("parent interface is required for MACVLAN mode")
		}
		if cfg.IPAddress == "" {
			return fmt.Errorf("IP address is required for MACVLAN mode")
		}
		if cfg.Gateway == "" {
			return fmt.Errorf("gateway is required for MACVLAN mode")
		}
		if cfg.Subnet == "" {
			return fmt.Errorf("subnet is required for MACVLAN mode")
		}

		// Validate the parent interface exists and is UP
		if err := c.validateInterface(ctx, cfg.ParentInterface); err != nil {
			return fmt.Errorf("parent interface validation failed: %w", err)
		}

		// Validate MACVLAN support
		if err := c.validateMACVLANSupport(ctx); err != nil {
			return fmt.Errorf("MACVLAN not supported in this environment: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("unsupported network mode: %s (must be 'host' or 'macvlan')", cfg.NetworkMode)
	}
}

// validateMACVLANSupport checks if MACVLAN is supported in the current environment
func (c *Client) validateMACVLANSupport(ctx context.Context) error {
	detector := system.NewEnvironmentDetector(c.logger)
	envInfo, err := detector.DetectEnvironment(ctx)
	if err != nil {
		c.logger.Warn("Could not fully validate MACVLAN support", "error", err)
		// Don't fail - allow the user to proceed if they explicitly requested MACVLAN
		return nil
	}

	if !envInfo.SupportsMACVLAN {
		return fmt.Errorf(
			"MACVLAN is not supported in this environment (type: %s, cloud: %s). "+
				"Consider using 'host' network mode or upgrading kernel (requires >= 3.9, recommended >= 4.0)",
			envInfo.Type,
			envInfo.CloudProvider,
		)
	}

	c.logger.Info("MACVLAN support validated",
		"kernel_version", envInfo.KernelVersion,
		"environment_type", envInfo.Type)

	return nil
}

// autoFillNetworkParams intelligently fills in missing network parameters for auto mode
func (c *Client) autoFillNetworkParams(ctx context.Context, cfg *AdDcConfig, networkMode string) error {
	switch networkMode {
	case "host":
		// For host mode, try to detect the primary interface if not specified
		if cfg.ParentInterface == "" || cfg.IPAddress == "" {
			iface, ip, err := c.detectPrimaryInterface(ctx)
			if err != nil {
				c.logger.Warn("Could not auto-detect primary interface", "error", err)
				return err
			}

			if cfg.ParentInterface == "" {
				cfg.ParentInterface = iface
				c.logger.Info("Auto-detected parent interface", "interface", iface)
			}

			if cfg.IPAddress == "" {
				cfg.IPAddress = ip
				c.logger.Info("Auto-detected IP address", "ip", ip)
			}
		}
		return nil

	case "macvlan":
		// For MACVLAN, we cannot auto-fill safely - user must provide parameters
		// We can try to detect the primary interface as a suggestion
		if cfg.ParentInterface == "" {
			iface, _, err := c.detectPrimaryInterface(ctx)
			if err == nil {
				c.logger.Info("Suggested parent interface for MACVLAN", "interface", iface,
					"note", "please configure ipAddress, gateway, and subnet manually")
				cfg.ParentInterface = iface
			}
		}
		return nil

	default:
		return fmt.Errorf("unknown network mode: %s", networkMode)
	}
}

// detectPrimaryInterface finds the primary network interface and its IP
func (c *Client) detectPrimaryInterface(ctx context.Context) (string, string, error) {
	// Find the interface with the default route
	output, err := c.executor.ExecuteWithCombinedOutput(ctx, "ip", "-o", "-4", "route", "show", "to", "default")
	if err != nil {
		return "", "", fmt.Errorf("failed to get default route: %w", err)
	}

	// Parse output like: "default via 172.31.0.1 dev enX0 proto dhcp src 172.31.2.47 metric 100"
	parts := strings.Fields(string(output))
	for i, part := range parts {
		if part == "dev" && i+1 < len(parts) {
			ifaceName := parts[i+1]

			// Get the IP address for this interface
			ipOutput, err := c.executor.ExecuteWithCombinedOutput(ctx, "ip", "-o", "-4", "addr", "show", "dev", ifaceName)
			if err != nil {
				continue
			}

			// Parse: "2: enX0    inet 172.31.2.47/20 brd ..."
			ipParts := strings.Fields(string(ipOutput))
			for j, ipPart := range ipParts {
				if ipPart == "inet" && j+1 < len(ipParts) {
					ipWithCIDR := ipParts[j+1]
					// Remove CIDR notation for host mode
					ip := strings.Split(ipWithCIDR, "/")[0]
					return ifaceName, ip, nil
				}
			}
		}
	}

	return "", "", fmt.Errorf("could not detect primary interface")
}

// validateInterface checks if a network interface exists and is UP
func (c *Client) validateInterface(ctx context.Context, ifaceName string) error {
	// Check if interface exists and get its state
	output, err := c.executor.ExecuteWithCombinedOutput(ctx, "ip", "link", "show", "dev", ifaceName)
	if err != nil {
		return fmt.Errorf("interface %s does not exist: %w", ifaceName, err)
	}

	// Check if interface is UP
	if !strings.Contains(string(output), "UP") {
		return fmt.Errorf("interface %s is not UP", ifaceName)
	}

	c.logger.Debug("Interface validation passed", "interface", ifaceName)
	return nil
}

// validateIPOnInterface checks if an IP address is configured on the specified interface
func (c *Client) validateIPOnInterface(ctx context.Context, ifaceName, ipAddr string) error {
	// Get IP addresses for the interface
	output, err := c.executor.ExecuteWithCombinedOutput(ctx, "ip", "-o", "-4", "addr", "show", "dev", ifaceName)
	if err != nil {
		return fmt.Errorf("failed to get IP addresses for %s: %w", ifaceName, err)
	}

	// Remove CIDR notation from ipAddr if present for comparison
	compareIP := strings.Split(ipAddr, "/")[0]

	// Check if the IP is in the output
	if !strings.Contains(string(output), compareIP) {
		return fmt.Errorf("IP address %s is not configured on interface %s", compareIP, ifaceName)
	}

	c.logger.Debug("IP validation passed", "interface", ifaceName, "ip", compareIP)
	return nil
}

// configureNetplanForADDC configures netplan for AD DC networking based on the mode
func (c *Client) configureNetplanForADDC(ctx context.Context, cfg *AdDcConfig) error {
	switch cfg.NetworkMode {
	case "host":
		return c.configureNetplanHostMode(ctx, cfg)
	case "macvlan":
		// MACVLAN doesn't need netplan config - Docker handles everything
		c.logger.Info("MACVLAN mode - skipping netplan configuration (Docker manages networking)")
		return nil
	default:
		return fmt.Errorf("unsupported network mode for netplan config: %s", cfg.NetworkMode)
	}
}

// configureNetplanHostMode configures netplan for host mode with dedicated interface
func (c *Client) configureNetplanHostMode(ctx context.Context, cfg *AdDcConfig) error {
	// Only configure netplan if we have the necessary parameters
	if cfg.ParentInterface == "" || cfg.IPAddress == "" {
		c.logger.Warn("Skipping netplan configuration - insufficient parameters",
			"interface", cfg.ParentInterface,
			"ip", cfg.IPAddress)
		return nil
	}

	c.logger.Info("Configuring netplan for AD DC host mode",
		"interface", cfg.ParentInterface,
		"ip", cfg.IPAddress)

	// Create netmage manager (using netmage primitives)
	netmgr, err := netmage.NewManager(ctx, c.logger, types.RendererNetworkd)
	if err != nil {
		return fmt.Errorf("failed to create netmage manager: %w", err)
	}

	// Get current netplan configuration
	netplanConfig, err := netmgr.GetNetplanConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get netplan config: %w", err)
	}

	// Ensure network structure exists
	if netplanConfig.Network == nil {
		netplanConfig.Network = &types.NetworkConfig{
			Version:  types.DefaultNetplanConfigVersion,
			Renderer: types.RendererNetworkd,
		}
	}
	if netplanConfig.Network.Ethernets == nil {
		netplanConfig.Network.Ethernets = make(map[string]*types.EthernetConfig)
	}

	// Get or create ethernet config for the AD DC interface
	ethConfig, exists := netplanConfig.Network.Ethernets[cfg.ParentInterface]
	if !exists {
		ethConfig = &types.EthernetConfig{}
		netplanConfig.Network.Ethernets[cfg.ParentInterface] = ethConfig
	}

	// Use gateway and subnet from config if provided, otherwise auto-detect
	gateway := cfg.Gateway
	subnet := cfg.Subnet

	if gateway == "" || subnet == "" {
		c.logger.Debug("Auto-detecting gateway/subnet for interface", "interface", cfg.ParentInterface)
		detectedGateway, detectedSubnet, err := c.detectNetworkParams(ctx, cfg.ParentInterface)
		if err != nil {
			c.logger.Warn("Could not auto-detect gateway/subnet", "error", err)
			// Keep whatever was provided in config (might be empty)
		} else {
			// Use detected values only if not provided in config
			if gateway == "" {
				gateway = detectedGateway
				c.logger.Info("Auto-detected gateway", "gateway", gateway)
			}
			if subnet == "" {
				subnet = detectedSubnet
				c.logger.Info("Auto-detected subnet", "subnet", subnet)
			}
		}
	} else {
		c.logger.Info("Using gateway and subnet from config",
			"gateway", gateway,
			"subnet", subnet)
	}

	// Configure the interface for AD DC
	c.configureADDCInterface(ethConfig, cfg, gateway, subnet)

	// Apply the configuration using SafeApplyConfig for safety
	c.logger.Info("Applying netplan configuration with safety checks")
	safeOpts := netmage.DefaultSafeConfigOptions()
	safeOpts.BackupDescription = fmt.Sprintf("AD DC configuration for %s", cfg.ParentInterface)

	result, err := netmgr.SafeApplyConfig(ctx, netplanConfig, safeOpts)
	if err != nil {
		c.logger.Error("Failed to apply netplan configuration",
			"error", err,
			"backup_id", result.BackupID,
			"rolled_back", result.RolledBack)
		return fmt.Errorf("failed to apply netplan config: %w (backup: %s)", err, result.BackupID)
	}

	c.logger.Info("Netplan configuration applied successfully",
		"backup_id", result.BackupID,
		"duration", result.TotalDuration)

	return nil
}

// configureADDCInterface configures an ethernet interface for AD DC
func (c *Client) configureADDCInterface(
	ethConfig *types.EthernetConfig,
	cfg *AdDcConfig,
	gateway, subnet string,
) {
	// Disable DHCP since we're using static IP
	dhcp4False := false
	dhcp6False := false
	ethConfig.DHCPv4 = &dhcp4False
	ethConfig.DHCPv6 = &dhcp6False

	// Set static IP address with CIDR notation
	ipWithCIDR := cfg.IPAddress
	if !strings.Contains(ipWithCIDR, "/") {
		// Add default CIDR if not present
		if subnet != "" {
			_, ipNet, err := net.ParseCIDR(subnet)
			if err == nil {
				ones, _ := ipNet.Mask.Size()
				ipWithCIDR = fmt.Sprintf("%s/%d", cfg.IPAddress, ones)
			}
		} else {
			// Default to /20 (matching AWS subnet)
			ipWithCIDR = cfg.IPAddress + "/20"
			c.logger.Warn("No subnet detected, using default /20 CIDR")
		}
	}

	// Add IP address if not already present
	addressExists := false
	for _, addr := range ethConfig.Addresses {
		if strings.HasPrefix(addr, strings.Split(cfg.IPAddress, "/")[0]) {
			addressExists = true
			break
		}
	}
	if !addressExists {
		ethConfig.Addresses = append(ethConfig.Addresses, ipWithCIDR)
	}

	// Configure DNS to point to AD DC itself (critical for AD functionality)
	if ethConfig.Nameservers == nil {
		ethConfig.Nameservers = &types.NameserverConfig{}
	}
	ethConfig.Nameservers.Addresses = []string{
		cfg.IPAddress, // Primary: AD DC itself
		"1.1.1.1",     // Fallback: Cloudflare DNS
	}
	ethConfig.Nameservers.Search = []string{
		strings.ToLower(cfg.Realm), // AD domain search
	}

	// Set optional to true (interface can fail without blocking boot)
	optional := true
	ethConfig.Optional = &optional

	// Configure routing table (table 101 for AD DC interface)
	routingTable := 101
	if gateway != "" {
		// Add default route via this gateway in table 101
		defaultRoute := &types.RouteConfig{
			To:    "0.0.0.0/0",
			Via:   gateway,
			Table: &routingTable,
		}

		// Check if route already exists
		routeExists := false
		for _, route := range ethConfig.Routes {
			if route.To == "0.0.0.0/0" && route.Table != nil && *route.Table == routingTable {
				routeExists = true
				break
			}
		}
		if !routeExists {
			ethConfig.Routes = append(ethConfig.Routes, defaultRoute)
		}
	}

	// Configure policy-based routing (route from this IP via table 101)
	routingPolicy := &types.RoutingPolicyConfig{
		From:     cfg.IPAddress,
		Table:    &routingTable,
		Priority: intPtr(100),
	}

	// Check if policy already exists
	policyExists := false
	for _, policy := range ethConfig.RoutingPolicy {
		if policy.From == cfg.IPAddress && policy.Table != nil && *policy.Table == routingTable {
			policyExists = true
			break
		}
	}
	if !policyExists {
		ethConfig.RoutingPolicy = append(ethConfig.RoutingPolicy, routingPolicy)
	}

	c.logger.Debug("Configured AD DC interface",
		"interface", cfg.ParentInterface,
		"ip", ipWithCIDR,
		"gateway", gateway,
		"table", routingTable)
}

// detectNetworkParams detects gateway and subnet from an existing interface
func (c *Client) detectNetworkParams(ctx context.Context, ifaceName string) (gateway, subnet string, err error) {
	// Try to get default gateway for this specific interface
	output, err := c.executor.ExecuteWithCombinedOutput(ctx, "ip", "-4", "route", "show", "dev", ifaceName, "default")
	if err == nil && len(output) > 0 {
		// Parse: "default via 172.31.0.1 ..."
		parts := strings.Fields(string(output))
		for i, part := range parts {
			if part == "via" && i+1 < len(parts) {
				gateway = parts[i+1]
				break
			}
		}
	}

	// If no default route found for this interface, try the main routing table
	// This handles cases where the interface is in the same subnet but doesn't have its own default route
	if gateway == "" {
		output, err = c.executor.ExecuteWithCombinedOutput(ctx, "ip", "-4", "route", "show", "default")
		if err == nil && len(output) > 0 {
			// Parse: "default via 172.31.0.1 dev enX0 ..."
			parts := strings.Fields(string(output))
			for i, part := range parts {
				if part == "via" && i+1 < len(parts) {
					gateway = parts[i+1]
					c.logger.Debug("Using main table gateway for interface",
						"interface", ifaceName,
						"gateway", gateway)
					break
				}
			}
		}
	}

	// Get subnet from interface IP
	output, err = c.executor.ExecuteWithCombinedOutput(ctx, "ip", "-o", "-4", "addr", "show", "dev", ifaceName)
	if err == nil && len(output) > 0 {
		// Parse: "2: enX0    inet 172.31.2.47/20 ..."
		parts := strings.Fields(string(output))
		for i, part := range parts {
			if part == "inet" && i+1 < len(parts) {
				ipWithCIDR := parts[i+1]
				// Extract subnet
				if ip, ipNet, err := net.ParseCIDR(ipWithCIDR); err == nil {
					ipNet.IP = ipNet.IP.Mask(ipNet.Mask) // Get network address
					subnet = ipNet.String()
					c.logger.Debug("Detected network params",
						"interface", ifaceName,
						"gateway", gateway,
						"subnet", subnet,
						"source_ip", ip.String())
					return gateway, subnet, nil
				}
			}
		}
	}

	if gateway == "" {
		return "", "", fmt.Errorf("could not detect gateway for %s", ifaceName)
	}

	return gateway, subnet, nil
}

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}

// reportConfigChange reports configuration changes via the event system
func (c *Client) reportConfigChange(
	ctx context.Context,
	serviceName string,
	state config.ServiceState,
) error {
	// Emit configuration change event
	metadata := map[string]string{
		"updated_at": state.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	events.EmitServiceConfigChange(serviceName, state.ConfigPath, state.Status, metadata)

	c.logger.Debug("Reported service configuration change via events",
		"service", serviceName,
		"config_path", state.ConfigPath,
		"status", state.Status)

	return nil
}
