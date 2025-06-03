// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"context"
	"encoding/json"
	"net"
	"time"
)

// Supported Netplan version range
const (
	DefaultNetplanConfigVersion = 2 // YAML network.version field
)

// Renderer types
type Renderer string

const (
	RendererNetworkd       Renderer = "networkd"
	RendererNetworkManager Renderer = "NetworkManager"
)

// Interface state constants
type InterfaceState string

const (
	InterfaceStateUp      InterfaceState = "UP"
	InterfaceStateDown    InterfaceState = "DOWN"
	InterfaceStateUnknown InterfaceState = "UNKNOWN"
)

// Network family constants
type Family int

const (
	FamilyIPv4 Family = 2
	FamilyIPv6 Family = 10
)

// Route types
type RouteType string

const (
	RouteTypeUnicast   RouteType = "unicast"
	RouteTypeLocal     RouteType = "local"
	RouteTypeBroadcast RouteType = "broadcast"
	RouteTypeMulticast RouteType = "multicast"
)

// Route scopes
type RouteScope string

const (
	RouteScopeGlobal RouteScope = "global"
	RouteScopeLink   RouteScope = "link"
	RouteScopeHost   RouteScope = "host"
)

// Route protocols
type RouteProtocol string

const (
	RouteProtocolKernel RouteProtocol = "kernel"
	RouteProtocolStatic RouteProtocol = "static"
	RouteProtocolDHCP   RouteProtocol = "dhcp"
)

// Network device types
type DeviceType string

const (
	DeviceTypeEthernet        DeviceType = "ethernet"
	DeviceTypeBridge          DeviceType = "bridge"
	DeviceTypeVirtualEthernet DeviceType = "virtual-ethernet"
	DeviceTypeLoopback        DeviceType = "loopback"
	DeviceTypeWiFi            DeviceType = "wifi"
	DeviceTypeVLAN            DeviceType = "vlan"
	DeviceTypeBond            DeviceType = "bond"
	DeviceTypeTunnel          DeviceType = "tunnel"
	DeviceTypeVRF             DeviceType = "vrf"
	DeviceTypeModem           DeviceType = "modem"
	DeviceTypeDummy           DeviceType = "dummy"
)

// Manager interface defines the core networking management functionality
type Manager interface {
	// Interface management
	GetInterface(ctx context.Context, name string) (*NetworkInterface, error)
	ListInterfaces(ctx context.Context) ([]*NetworkInterface, error)
	SetInterfaceState(ctx context.Context, name string, state InterfaceState) error
	GetInterfaceStatistics(ctx context.Context, name string) (*InterfaceStatistics, error)

	// IP address management
	AddIPAddress(ctx context.Context, iface string, address string) error
	RemoveIPAddress(ctx context.Context, iface string, address string) error
	GetIPAddresses(ctx context.Context, iface string) ([]*IPAddress, error)

	// Route management
	AddRoute(ctx context.Context, route *Route) error
	RemoveRoute(ctx context.Context, route *Route) error
	GetRoutes(ctx context.Context, table string) ([]*Route, error)

	// Netplan configuration management
	GetNetplanConfig(ctx context.Context) (*NetplanConfig, error)
	SetNetplanConfig(ctx context.Context, config *NetplanConfig) error
	ApplyNetplanConfig(ctx context.Context) error
	TryNetplanConfig(ctx context.Context, timeout time.Duration) (*NetplanTryResult, error)
	GetNetplanStatus(ctx context.Context, iface string) (*NetplanStatus, error)
	GetNetplanDiff(ctx context.Context) (*NetplanDiff, error)

	// Safe configuration management (replacement for unreliable TryNetplanConfig)
	SafeApplyConfig(
		ctx context.Context,
		config *NetplanConfig,
		options *SafeConfigOptions,
	) (*SafeConfigResult, error)

	// Backup and restore
	BackupNetplanConfig(ctx context.Context) (string, error)
	RestoreNetplanConfig(ctx context.Context, backupID string) error
	ListBackups(ctx context.Context) ([]*ConfigBackup, error)

	// Validation
	ValidateNetplanConfig(ctx context.Context, config *NetplanConfig) error
	ValidateIPAddress(address string) error
	ValidateInterfaceName(name string) error

	// System information
	GetSystemNetworkInfo(ctx context.Context) (*SystemNetworkInfo, error)

	// Global DNS management
	GetGlobalDNS(ctx context.Context) (*NameserverConfig, error)
	SetGlobalDNS(ctx context.Context, dns *NameserverConfig) error
}

// NetworkInterface represents a network interface
type NetworkInterface struct {
	Index        int                  `json:"index"`
	Name         string               `json:"name"`
	Type         DeviceType           `json:"type"`
	MACAddress   string               `json:"mac_address"`
	MTU          int                  `json:"mtu"`
	AdminState   InterfaceState       `json:"admin_state"`
	OperState    InterfaceState       `json:"oper_state"`
	Flags        []string             `json:"flags"`
	IPAddresses  []*IPAddress         `json:"ip_addresses"`
	Routes       []*Route             `json:"routes"`
	Statistics   *InterfaceStatistics `json:"statistics,omitempty"`
	Master       string               `json:"master,omitempty"`
	LinkIndex    int                  `json:"link_index,omitempty"`
	LinkNetNSID  int                  `json:"link_netnsid,omitempty"`
	Backend      string               `json:"backend,omitempty"`
	Vendor       string               `json:"vendor,omitempty"`
	DNSAddresses []string             `json:"dns_addresses,omitempty"`
	DNSSearch    []string             `json:"dns_search,omitempty"`
	Interfaces   []string             `json:"interfaces,omitempty"` // For bridges
	Bridge       string               `json:"bridge,omitempty"`     // For bridge members
}

// IPAddress represents an IP address configuration
type IPAddress struct {
	Address           string   `json:"address"`
	PrefixLength      int      `json:"prefix_length"`
	Family            Family   `json:"family"`
	Scope             string   `json:"scope,omitempty"`
	Flags             []string `json:"flags,omitempty"`
	Label             string   `json:"label,omitempty"`
	Broadcast         string   `json:"broadcast,omitempty"`
	ValidLifetime     uint32   `json:"valid_lifetime,omitempty"`
	PreferredLifetime uint32   `json:"preferred_lifetime,omitempty"`
}

// Route represents a network route
type Route struct {
	To       string        `json:"to"`
	From     string        `json:"from,omitempty"`
	Via      string        `json:"via,omitempty"`
	Device   string        `json:"device,omitempty"`
	Table    string        `json:"table,omitempty"`
	Metric   int           `json:"metric,omitempty"`
	Family   Family        `json:"family"`
	Type     RouteType     `json:"type"`
	Scope    RouteScope    `json:"scope"`
	Protocol RouteProtocol `json:"protocol"`
}

// InterfaceStatistics represents network interface statistics
type InterfaceStatistics struct {
	RXBytes   uint64 `json:"rx_bytes"`
	TXBytes   uint64 `json:"tx_bytes"`
	RXPackets uint64 `json:"rx_packets"`
	TXPackets uint64 `json:"tx_packets"`
	RXErrors  uint64 `json:"rx_errors"`
	TXErrors  uint64 `json:"tx_errors"`
	RXDropped uint64 `json:"rx_dropped"`
	TXDropped uint64 `json:"tx_dropped"`
}

// NetplanConfig represents the complete Netplan configuration
type NetplanConfig struct {
	Network *NetworkConfig `yaml:"network" json:"network"`
}

// NetworkConfig represents the network section of Netplan configuration
type NetworkConfig struct {
	Version          int                          `yaml:"version"                     json:"version"`
	Renderer         Renderer                     `yaml:"renderer,omitempty"          json:"renderer,omitempty"`
	Ethernets        map[string]*EthernetConfig   `yaml:"ethernets,omitempty"         json:"ethernets,omitempty"`
	Bonds            map[string]*BondConfig       `yaml:"bonds,omitempty"             json:"bonds,omitempty"`
	Bridges          map[string]*BridgeConfig     `yaml:"bridges,omitempty"           json:"bridges,omitempty"`
	VLANs            map[string]*VLANConfig       `yaml:"vlans,omitempty"             json:"vlans,omitempty"`
	Tunnels          map[string]*TunnelConfig     `yaml:"tunnels,omitempty"           json:"tunnels,omitempty"`
	WiFis            map[string]*WiFiConfig       `yaml:"wifis,omitempty"             json:"wifis,omitempty"`
	Modems           map[string]*ModemConfig      `yaml:"modems,omitempty"            json:"modems,omitempty"`
	VRFs             map[string]*VRFConfig        `yaml:"vrfs,omitempty"              json:"vrfs,omitempty"`
	DummyDevices     map[string]*DummyConfig      `yaml:"dummy-devices,omitempty"     json:"dummy_devices,omitempty"`
	VirtualEthernets map[string]*VirtualEthConfig `yaml:"virtual-ethernets,omitempty" json:"virtual_ethernets,omitempty"`
	NMDevices        map[string]*NMDeviceConfig   `yaml:"nm-devices,omitempty"        json:"nm_devices,omitempty"`
}

// BaseDeviceConfig represents common configuration for all network devices
// Note: Prefer MAC address matching for interface identification over names,
// especially for bonding, virtual interfaces, and when setting MTU with networkd renderer
type BaseDeviceConfig struct {
	DHCPv4          *bool                  `yaml:"dhcp4,omitempty"           json:"dhcp4,omitempty"`
	DHCPv6          *bool                  `yaml:"dhcp6,omitempty"           json:"dhcp6,omitempty"`
	IPv6PrivacyMode *bool                  `yaml:"ipv6-privacy,omitempty"    json:"ipv6_privacy,omitempty"`
	LinkLocal       []string               `yaml:"link-local,omitempty"      json:"link_local,omitempty"`
	Critical        *bool                  `yaml:"critical,omitempty"        json:"critical,omitempty"`
	DHCPIdentifier  string                 `yaml:"dhcp-identifier,omitempty" json:"dhcp_identifier,omitempty"`
	DHCP4Overrides  *DHCPOverrides         `yaml:"dhcp4-overrides,omitempty" json:"dhcp4_overrides,omitempty"`
	DHCP6Overrides  *DHCPOverrides         `yaml:"dhcp6-overrides,omitempty" json:"dhcp6_overrides,omitempty"`
	AcceptRA        *bool                  `yaml:"accept-ra,omitempty"       json:"accept_ra,omitempty"`
	Addresses       []string               `yaml:"addresses,omitempty"       json:"addresses,omitempty"`
	IPv6MTU         *int                   `yaml:"ipv6-mtu,omitempty"        json:"ipv6_mtu,omitempty"`
	Gateway4        string                 `yaml:"gateway4,omitempty"        json:"gateway4,omitempty"`
	Gateway6        string                 `yaml:"gateway6,omitempty"        json:"gateway6,omitempty"`
	Nameservers     *NameserverConfig      `yaml:"nameservers,omitempty"     json:"nameservers,omitempty"`
	MACAddress      string                 `yaml:"macaddress,omitempty"      json:"macaddress,omitempty"`
	MTU             *int                   `yaml:"mtu,omitempty"             json:"mtu,omitempty"`
	Optional        *bool                  `yaml:"optional,omitempty"        json:"optional,omitempty"`
	ActivationMode  string                 `yaml:"activation-mode,omitempty" json:"activation_mode,omitempty"`
	Routes          []*RouteConfig         `yaml:"routes,omitempty"          json:"routes,omitempty"`
	RoutingPolicy   []*RoutingPolicyConfig `yaml:"routing-policy,omitempty"  json:"routing_policy,omitempty"`
	Neigh           []*NeighborConfig      `yaml:"neigh,omitempty"           json:"neigh,omitempty"`
}

// EthernetConfig represents Ethernet interface configuration
// Best practice: Use MAC address matching for reliable device identification
type EthernetConfig struct {
	BaseDeviceConfig `             yaml:",inline"`
	Match            *MatchConfig `yaml:"match,omitempty"        json:"match,omitempty"`
	SetName          string       `yaml:"set-name,omitempty"     json:"set_name,omitempty"`
	WakeOnLAN        *bool        `yaml:"wakeonlan,omitempty"    json:"wakeonlan,omitempty"`
	EmitLLDP         *bool        `yaml:"emit-lldp,omitempty"    json:"emit_lldp,omitempty"`
	ReceiveLLDP      *bool        `yaml:"receive-lldp,omitempty" json:"receive_lldp,omitempty"`
}

// BondConfig represents bond interface configuration
type BondConfig struct {
	BaseDeviceConfig `                yaml:",inline"`
	Interfaces       []string        `yaml:"interfaces"           json:"interfaces"`
	Parameters       *BondParameters `yaml:"parameters,omitempty" json:"parameters,omitempty"`
}

// BridgeConfig represents bridge interface configuration
type BridgeConfig struct {
	BaseDeviceConfig `                  yaml:",inline"`
	Interfaces       []string          `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
	Parameters       *BridgeParameters `yaml:"parameters,omitempty" json:"parameters,omitempty"`
}

// VLANConfig represents VLAN interface configuration
type VLANConfig struct {
	BaseDeviceConfig `       yaml:",inline"`
	ID               int    `yaml:"id"      json:"id"`
	Link             string `yaml:"link"    json:"link"`
}

// TunnelConfig represents tunnel interface configuration
type TunnelConfig struct {
	BaseDeviceConfig `            yaml:",inline"`
	Mode             string      `yaml:"mode"             json:"mode"`
	Local            string      `yaml:"local,omitempty"  json:"local,omitempty"`
	Remote           string      `yaml:"remote,omitempty" json:"remote,omitempty"`
	TTL              *int        `yaml:"ttl,omitempty"    json:"ttl,omitempty"`
	Key              string      `yaml:"key,omitempty"    json:"key,omitempty"`
	Keys             *TunnelKeys `yaml:"keys,omitempty"   json:"keys,omitempty"`
}

// WiFiConfig represents WiFi interface configuration
type WiFiConfig struct {
	BaseDeviceConfig `                     yaml:",inline"`
	Match            *MatchConfig         `yaml:"match,omitempty"         json:"match,omitempty"`
	SetName          string               `yaml:"set-name,omitempty"      json:"set_name,omitempty"`
	AccessPoints     map[string]*APConfig `yaml:"access-points,omitempty" json:"access_points,omitempty"`
}

// ModemConfig represents modem interface configuration
type ModemConfig struct {
	BaseDeviceConfig `       yaml:",inline"`
	APN              string `yaml:"apn,omitempty"        json:"apn,omitempty"`
	DeviceID         string `yaml:"device-id,omitempty"  json:"device_id,omitempty"`
	NetworkID        string `yaml:"network-id,omitempty" json:"network_id,omitempty"`
	Number           string `yaml:"number,omitempty"     json:"number,omitempty"`
	Password         string `yaml:"password,omitempty"   json:"password,omitempty"`
	PIN              string `yaml:"pin,omitempty"        json:"pin,omitempty"`
	SIMId            string `yaml:"sim-id,omitempty"     json:"sim_id,omitempty"`
	SIMPIN           string `yaml:"sim-pin,omitempty"    json:"sim_pin,omitempty"`
	Username         string `yaml:"username,omitempty"   json:"username,omitempty"`
}

// VRFConfig represents VRF interface configuration
type VRFConfig struct {
	BaseDeviceConfig `         yaml:",inline"`
	Table            int      `yaml:"table"                json:"table"`
	Interfaces       []string `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
}

// DummyConfig represents dummy interface configuration
type DummyConfig struct {
	BaseDeviceConfig `yaml:",inline"`
}

// VirtualEthConfig represents virtual ethernet configuration
type VirtualEthConfig struct {
	BaseDeviceConfig `       yaml:",inline"`
	Mode             string `yaml:"mode,omitempty" json:"mode,omitempty"`
	Peer             string `yaml:"peer,omitempty" json:"peer,omitempty"`
}

// NMDeviceConfig represents NetworkManager device configuration
type NMDeviceConfig struct {
	NetworkManagerSettings map[string]any `yaml:"networkmanager,omitempty" json:"networkmanager,omitempty"`
}

// Supporting configuration structures

// MatchConfig represents interface matching criteria
// Best practice: Use macaddress for reliable device identification,
// especially when setting MTU or working with virtual interfaces
type MatchConfig struct {
	Driver     []string `yaml:"driver,omitempty"     json:"driver,omitempty"`
	MACAddress string   `yaml:"macaddress,omitempty" json:"macaddress,omitempty"`
	Name       []string `yaml:"name,omitempty"       json:"name,omitempty"`
	Path       []string `yaml:"path,omitempty"       json:"path,omitempty"`
}

// DHCPOverrides represents DHCP override configuration
type DHCPOverrides struct {
	UseHostname  *bool  `yaml:"use-hostname,omitempty"  json:"use_hostname,omitempty"`
	UseDNS       *bool  `yaml:"use-dns,omitempty"       json:"use_dns,omitempty"`
	UseDomains   string `yaml:"use-domains,omitempty"   json:"use_domains,omitempty"`
	UseMTU       *bool  `yaml:"use-mtu,omitempty"       json:"use_mtu,omitempty"`
	UseNTP       *bool  `yaml:"use-ntp,omitempty"       json:"use_ntp,omitempty"`
	UseRoutes    *bool  `yaml:"use-routes,omitempty"    json:"use_routes,omitempty"`
	Hostname     string `yaml:"hostname,omitempty"      json:"hostname,omitempty"`
	RouteMetric  *int   `yaml:"route-metric,omitempty"  json:"route_metric,omitempty"`
	SendHostname *bool  `yaml:"send-hostname,omitempty" json:"send_hostname,omitempty"`
}

// NameserverConfig represents DNS configuration
type NameserverConfig struct {
	Search    []string `yaml:"search,omitempty"    json:"search,omitempty"`
	Addresses []string `yaml:"addresses,omitempty" json:"addresses,omitempty"`
}

// RouteConfig represents route configuration
type RouteConfig struct {
	To                      string `yaml:"to,omitempty"                        json:"to,omitempty"`
	Via                     string `yaml:"via,omitempty"                       json:"via,omitempty"`
	From                    string `yaml:"from,omitempty"                      json:"from,omitempty"`
	OnLink                  *bool  `yaml:"on-link,omitempty"                   json:"on_link,omitempty"`
	Metric                  *int   `yaml:"metric,omitempty"                    json:"metric,omitempty"`
	Type                    string `yaml:"type,omitempty"                      json:"type,omitempty"`
	Scope                   string `yaml:"scope,omitempty"                     json:"scope,omitempty"`
	Table                   *int   `yaml:"table,omitempty"                     json:"table,omitempty"`
	MTU                     *int   `yaml:"mtu,omitempty"                       json:"mtu,omitempty"`
	CongestionWindow        *int   `yaml:"congestion-window,omitempty"         json:"congestion_window,omitempty"`
	AdvertisedReceiveWindow *int   `yaml:"advertised-receive-window,omitempty" json:"advertised_receive_window,omitempty"`
}

// RoutingPolicyConfig represents routing policy configuration
type RoutingPolicyConfig struct {
	From          string `yaml:"from,omitempty"            json:"from,omitempty"`
	To            string `yaml:"to,omitempty"              json:"to,omitempty"`
	Table         *int   `yaml:"table,omitempty"           json:"table,omitempty"`
	Priority      *int   `yaml:"priority,omitempty"        json:"priority,omitempty"`
	Mark          *int   `yaml:"mark,omitempty"            json:"mark,omitempty"`
	TypeOfService *int   `yaml:"type-of-service,omitempty" json:"type_of_service,omitempty"`
}

// NeighborConfig represents neighbor/ARP configuration
type NeighborConfig struct {
	To         string `yaml:"to"         json:"to"`
	MACAddress string `yaml:"macaddress" json:"macaddress"`
}

// BondParameters represents bond-specific parameters
type BondParameters struct {
	Mode                  string   `yaml:"mode,omitempty"                    json:"mode,omitempty"`
	LACPRate              string   `yaml:"lacp-rate,omitempty"               json:"lacp_rate,omitempty"`
	MIIMonitorInterval    string   `yaml:"mii-monitor-interval,omitempty"    json:"mii_monitor_interval,omitempty"`
	MinLinks              *int     `yaml:"min-links,omitempty"               json:"min_links,omitempty"`
	TransmitHashPolicy    string   `yaml:"transmit-hash-policy,omitempty"    json:"transmit_hash_policy,omitempty"`
	ADSelect              string   `yaml:"ad-select,omitempty"               json:"ad_select,omitempty"`
	AllSlavesActive       *bool    `yaml:"all-slaves-active,omitempty"       json:"all_slaves_active,omitempty"`
	ARPInterval           string   `yaml:"arp-interval,omitempty"            json:"arp_interval,omitempty"`
	ARPIPTargets          []string `yaml:"arp-ip-targets,omitempty"          json:"arp_ip_targets,omitempty"`
	ARPValidate           string   `yaml:"arp-validate,omitempty"            json:"arp_validate,omitempty"`
	ARPAllTargets         string   `yaml:"arp-all-targets,omitempty"         json:"arp_all_targets,omitempty"`
	UpDelay               string   `yaml:"up-delay,omitempty"                json:"up_delay,omitempty"`
	DownDelay             string   `yaml:"down-delay,omitempty"              json:"down_delay,omitempty"`
	FailOverMACPolicy     string   `yaml:"fail-over-mac-policy,omitempty"    json:"fail_over_mac_policy,omitempty"`
	GratuitousARP         *int     `yaml:"gratuitous-arp,omitempty"          json:"gratuitous_arp,omitempty"`
	PacketsPerSlave       *int     `yaml:"packets-per-slave,omitempty"       json:"packets_per_slave,omitempty"`
	PrimaryReselectPolicy string   `yaml:"primary-reselect-policy,omitempty" json:"primary_reselect_policy,omitempty"`
	ResendIGMP            *int     `yaml:"resend-igmp,omitempty"             json:"resend_igmp,omitempty"`
	LearnPacketInterval   string   `yaml:"learn-packet-interval,omitempty"   json:"learn_packet_interval,omitempty"`
	Primary               string   `yaml:"primary,omitempty"                 json:"primary,omitempty"`
}

// BridgeParameters represents bridge-specific parameters
type BridgeParameters struct {
	AgeingTime   string `yaml:"ageing-time,omitempty"   json:"ageing_time,omitempty"`
	Priority     *int   `yaml:"priority,omitempty"      json:"priority,omitempty"`
	PortPriority *int   `yaml:"port-priority,omitempty" json:"port_priority,omitempty"`
	ForwardDelay string `yaml:"forward-delay,omitempty" json:"forward_delay,omitempty"`
	HelloTime    string `yaml:"hello-time,omitempty"    json:"hello_time,omitempty"`
	MaxAge       string `yaml:"max-age,omitempty"       json:"max_age,omitempty"`
	PathCost     *int   `yaml:"path-cost,omitempty"     json:"path_cost,omitempty"`
	STP          *bool  `yaml:"stp,omitempty"           json:"stp,omitempty"`
}

// TunnelKeys represents tunnel key configuration
type TunnelKeys struct {
	Input  string `yaml:"input,omitempty"  json:"input,omitempty"`
	Output string `yaml:"output,omitempty" json:"output,omitempty"`
}

// APConfig represents WiFi access point configuration
type APConfig struct {
	Password string          `yaml:"password,omitempty" json:"password,omitempty"`
	Mode     string          `yaml:"mode,omitempty"     json:"mode,omitempty"`
	Channel  *int            `yaml:"channel,omitempty"  json:"channel,omitempty"`
	Band     string          `yaml:"band,omitempty"     json:"band,omitempty"`
	BSSID    string          `yaml:"bssid,omitempty"    json:"bssid,omitempty"`
	Hidden   *bool           `yaml:"hidden,omitempty"   json:"hidden,omitempty"`
	Auth     *WiFiAuthConfig `yaml:"auth,omitempty"     json:"auth,omitempty"`
}

// WiFiAuthConfig represents WiFi authentication configuration
type WiFiAuthConfig struct {
	KeyManagement     string `yaml:"key-management,omitempty"      json:"key_management,omitempty"`
	Method            string `yaml:"method,omitempty"              json:"method,omitempty"`
	Identity          string `yaml:"identity,omitempty"            json:"identity,omitempty"`
	AnonymousIdentity string `yaml:"anonymous-identity,omitempty"  json:"anonymous_identity,omitempty"`
	Password          string `yaml:"password,omitempty"            json:"password,omitempty"`
	CACertificate     string `yaml:"ca-certificate,omitempty"      json:"ca_certificate,omitempty"`
	ClientCertificate string `yaml:"client-certificate,omitempty"  json:"client_certificate,omitempty"`
	ClientKey         string `yaml:"client-key,omitempty"          json:"client_key,omitempty"`
	ClientKeyPassword string `yaml:"client-key-password,omitempty" json:"client_key_password,omitempty"`
	Phase2Auth        string `yaml:"phase2-auth,omitempty"         json:"phase2_auth,omitempty"`
}

// NetplanStatus represents netplan status output
type NetplanStatus struct {
	NetplanGlobalState *NetplanGlobalState         `json:"netplan-global-state,omitempty"`
	Interfaces         map[string]*InterfaceStatus `json:"-"`
}

// UnmarshalJSON implements custom JSON unmarshaling for NetplanStatus
// The netplan status JSON has a flat structure where interfaces are at root level
func (ns *NetplanStatus) UnmarshalJSON(data []byte) error {
	// First, unmarshal into a generic map to handle the flat structure
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Initialize the interfaces map
	ns.Interfaces = make(map[string]*InterfaceStatus)

	// Process each key in the JSON
	for key, value := range raw {
		if key == "netplan-global-state" {
			// Unmarshal the global state
			var globalState NetplanGlobalState
			if err := json.Unmarshal(value, &globalState); err != nil {
				return err
			}
			ns.NetplanGlobalState = &globalState
		} else {
			// Everything else should be an interface
			var interfaceStatus InterfaceStatus
			if err := json.Unmarshal(value, &interfaceStatus); err != nil {
				// Skip non-interface entries
				continue
			}
			ns.Interfaces[key] = &interfaceStatus
		}
	}

	return nil
}

// NetplanGlobalState represents global netplan state
type NetplanGlobalState struct {
	Online      bool              `json:"online"`
	Nameservers *NameserverStatus `json:"nameservers,omitempty"`
}

// NameserverStatus represents DNS server status
type NameserverStatus struct {
	Addresses []string `json:"addresses,omitempty"`
	Search    []string `json:"search,omitempty"`
	Mode      string   `json:"mode,omitempty"`
}

// InterfaceStatus represents interface status from netplan
type InterfaceStatus struct {
	Index        int                         `json:"index"`
	AdminState   string                      `json:"adminstate"`
	OperState    string                      `json:"operstate"`
	Type         string                      `json:"type"`
	Backend      string                      `json:"backend,omitempty"`
	ID           string                      `json:"id,omitempty"`
	MACAddress   string                      `json:"macaddress,omitempty"`
	Vendor       string                      `json:"vendor,omitempty"`
	Addresses    []map[string]*AddressStatus `json:"addresses,omitempty"`
	DNSAddresses []string                    `json:"dns_addresses,omitempty"`
	DNSSearch    []string                    `json:"dns_search,omitempty"`
	Routes       []*RouteStatus              `json:"routes,omitempty"`
	Interfaces   []string                    `json:"interfaces,omitempty"`
	Bridge       string                      `json:"bridge,omitempty"`
}

// AddressStatus represents address status from netplan
type AddressStatus struct {
	Prefix int      `json:"prefix"`
	Flags  []string `json:"flags,omitempty"`
}

// RouteStatus represents route status
type RouteStatus struct {
	To       string `json:"to"`
	From     string `json:"from,omitempty"`
	Via      string `json:"via,omitempty"`
	Family   int    `json:"family"`
	Metric   int    `json:"metric,omitempty"`
	Type     string `json:"type"`
	Scope    string `json:"scope"`
	Protocol string `json:"protocol"`
	Table    string `json:"table,omitempty"`
}

// NetplanDiff represents differences between current state and netplan config
type NetplanDiff struct {
	Interfaces               map[string]*InterfaceDiff    `json:"interfaces,omitempty"`
	MissingInterfacesSystem  map[string]*MissingInterface `json:"missing_interfaces_system,omitempty"`
	MissingInterfacesNetplan map[string]*MissingInterface `json:"missing_interfaces_netplan,omitempty"`
}

// InterfaceDiff represents differences for a specific interface
type InterfaceDiff struct {
	Index        int        `json:"index"`
	Name         string     `json:"name"`
	ID           string     `json:"id"`
	SystemState  *DiffState `json:"system_state,omitempty"`
	NetplanState *DiffState `json:"netplan_state,omitempty"`
}

// DiffState represents configuration differences
type DiffState struct {
	MissingAddresses []string       `json:"missing_addresses,omitempty"`
	MissingRoutes    []*RouteStatus `json:"missing_routes,omitempty"`
}

// MissingInterface represents an interface missing from system or netplan
type MissingInterface struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// NetplanTryResult represents the result of a netplan try operation
type NetplanTryResult struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Applied    bool   `json:"applied"`
	RolledBack bool   `json:"rolled_back"`
	Error      string `json:"error,omitempty"`
}

// ConfigBackup represents a netplan configuration backup
type ConfigBackup struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
	FilePath    string    `json:"file_path"`
	Size        int64     `json:"size"`
}

// SystemNetworkInfo represents overall system network information
type SystemNetworkInfo struct {
	Hostname       string              `json:"hostname"`
	DefaultGateway *net.IP             `json:"default_gateway,omitempty"`
	DNSServers     []string            `json:"dns_servers"`
	SearchDomains  []string            `json:"search_domains"`
	NetplanVersion string              `json:"netplan_version"`
	Renderer       Renderer            `json:"renderer"`
	InterfaceCount int                 `json:"interface_count"`
	Interfaces     []*NetworkInterface `json:"interfaces"`
}

// Request types for API operations

// InterfaceRequest represents a request to operate on an interface
type InterfaceRequest struct {
	Name   string         `json:"name"             binding:"required"`
	State  InterfaceState `json:"state,omitempty"`
	Action string         `json:"action,omitempty"`
}

// AddressRequest represents a request to add/remove an IP address
type AddressRequest struct {
	Interface string `json:"interface" binding:"required"`
	Address   string `json:"address"   binding:"required"`
}

// RouteRequest represents a request to add/remove a route
type RouteRequest struct {
	To     string `json:"to"               binding:"required"`
	Via    string `json:"via,omitempty"`
	From   string `json:"from,omitempty"`
	Device string `json:"device,omitempty"`
	Table  string `json:"table,omitempty"`
	Metric int    `json:"metric,omitempty"`
}

// NetplanConfigRequest represents a request to update netplan configuration
type NetplanConfigRequest struct {
	Config     *NetplanConfig `json:"config"                       binding:"required"`
	BackupDesc string         `json:"backup_description,omitempty"`
}

// NetplanTryRequest represents a request to try netplan configuration
type NetplanTryRequest struct {
	Config  *NetplanConfig `json:"config,omitempty"`
	Timeout int            `json:"timeout,omitempty"` // seconds
}

// BackupRequest represents a request to create a backup
type BackupRequest struct {
	Description string `json:"description"`
}

// RestoreRequest represents a request to restore from backup
type RestoreRequest struct {
	BackupID string `json:"backup_id" binding:"required"`
}

// GlobalDNSRequest represents a request to update global DNS settings
type GlobalDNSRequest struct {
	Addresses []string `json:"addresses" binding:"required"`
	Search    []string `json:"search,omitempty"`
}

// SafeConfigRequest represents a request to safely apply netplan configuration
type SafeConfigRequest struct {
	Config  *NetplanConfig     `json:"config"  binding:"required"`
	Options *SafeConfigOptions `json:"options,omitempty"`
}

// SafeConfigOptions represents options for safe configuration management
type SafeConfigOptions struct {
	// Connectivity monitoring
	ConnectivityTargets     []string      `json:"connectivity_targets"`
	ConnectivityTimeout     time.Duration `json:"connectivity_timeout"`
	ConnectivityInterval    time.Duration `json:"connectivity_interval"`
	MaxConnectivityFailures int           `json:"max_connectivity_failures"`

	// Validation options
	SkipPreValidation  bool `json:"skip_pre_validation"`
	SkipPostValidation bool `json:"skip_post_validation"`

	// Backup and rollback
	AutoBackup        bool          `json:"auto_backup"`
	AutoRollback      bool          `json:"auto_rollback"`
	RollbackTimeout   time.Duration `json:"rollback_timeout"`
	BackupDescription string        `json:"backup_description"`

	// Application strategy
	GracePeriod          time.Duration `json:"grace_period"`
	ValidateInterfaces   bool          `json:"validate_interfaces"`
	ValidateRoutes       bool          `json:"validate_routes"`
	ValidateConnectivity bool          `json:"validate_connectivity"`
}

// SafeConfigResult represents the result of a safe configuration operation
type SafeConfigResult struct {
	Success    bool   `json:"success"`
	Applied    bool   `json:"applied"`
	RolledBack bool   `json:"rolled_back"`
	BackupID   string `json:"backup_id,omitempty"`
	Error      string `json:"error,omitempty"`
	Message    string `json:"message"`

	// Validation results
	PreValidation  *ValidationResult `json:"pre_validation,omitempty"`
	PostValidation *ValidationResult `json:"post_validation,omitempty"`

	// Connectivity results
	Connectivity *ConnectivityResult `json:"connectivity,omitempty"`

	// Timing information
	StartTime      time.Time     `json:"start_time"`
	ApplyTime      time.Time     `json:"apply_time,omitempty"`
	CompletionTime time.Time     `json:"completion_time"`
	TotalDuration  time.Duration `json:"total_duration"`
}

// ValidationResult represents validation results
type ValidationResult struct {
	Success        bool     `json:"success"`
	SyntaxValid    bool     `json:"syntax_valid"`
	InterfaceValid bool     `json:"interface_valid"`
	RouteValid     bool     `json:"route_valid"`
	Errors         []string `json:"errors,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

// ConnectivityResult represents connectivity test results
type ConnectivityResult struct {
	InitialSuccess bool            `json:"initial_success"`
	FinalSuccess   bool            `json:"final_success"`
	TargetResults  map[string]bool `json:"target_results"`
	FailedChecks   int             `json:"failed_checks"`
	TotalChecks    int             `json:"total_checks"`
	MonitoringTime time.Duration   `json:"monitoring_time"`
}
