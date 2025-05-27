// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package netmage

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// validateIPAddressFormat validates IP address format (supports both single IP and CIDR)
func validateIPAddressFormat(address string) error {
	if address == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	// Check if it's CIDR notation
	if strings.Contains(address, "/") {
		_, _, err := net.ParseCIDR(address)
		if err != nil {
			return fmt.Errorf("invalid CIDR notation: %v", err)
		}
		return nil
	}

	// Check if it's a plain IP address
	ip := net.ParseIP(address)
	if ip == nil {
		return fmt.Errorf("invalid IP address format")
	}

	return nil
}

// validateMACAddressFormat validates MAC address format
func validateMACAddressFormat(mac string) error {
	if mac == "" {
		return fmt.Errorf("MAC address cannot be empty")
	}

	// MAC address regex patterns (supports multiple formats)
	patterns := []string{
		`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`,         // XX:XX:XX:XX:XX:XX or XX-XX-XX-XX-XX-XX
		`^([0-9A-Fa-f]{4}\.){2}([0-9A-Fa-f]{4})$`,           // XXXX.XXXX.XXXX
		`^([0-9A-Fa-f]{12})$`,                                // XXXXXXXXXXXX
	}

	for _, pattern := range patterns {
		matched, err := regexp.MatchString(pattern, mac)
		if err != nil {
			continue
		}
		if matched {
			return nil
		}
	}

	return fmt.Errorf("invalid MAC address format")
}

// validateHostnameFormat validates hostname format
func validateHostnameFormat(hostname string) error {
	if hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	if len(hostname) > 253 {
		return fmt.Errorf("hostname too long (max 253 characters)")
	}

	// Hostname regex pattern
	pattern := `^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`
	matched, err := regexp.MatchString(pattern, hostname)
	if err != nil {
		return fmt.Errorf("error validating hostname: %v", err)
	}

	if !matched {
		return fmt.Errorf("invalid hostname format")
	}

	return nil
}

// validatePortNumber validates port number
func validatePortNumber(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %d (must be between 1 and 65535)", port)
	}
	return nil
}

// validateVLANID validates VLAN ID
func validateVLANID(vlanID int) error {
	if vlanID < 1 || vlanID > 4094 {
		return fmt.Errorf("invalid VLAN ID: %d (must be between 1 and 4094)", vlanID)
	}
	return nil
}

// validateMTU validates MTU value
func validateMTU(mtu int) error {
	if mtu < 68 || mtu > 65536 {
		return fmt.Errorf("invalid MTU: %d (must be between 68 and 65536)", mtu)
	}
	return nil
}

// validateCIDRNotation validates CIDR notation
func validateCIDRNotation(cidr string) error {
	if cidr == "" {
		return fmt.Errorf("CIDR notation cannot be empty")
	}

	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR notation: %v", err)
	}

	return nil
}

// validateRouteMetric validates route metric
func validateRouteMetric(metric int) error {
	if metric < 0 || metric > 4294967295 {
		return fmt.Errorf("invalid route metric: %d (must be between 0 and 4294967295)", metric)
	}
	return nil
}

// validateBondMode validates bond mode
func validateBondMode(mode string) error {
	validModes := []string{
		"balance-rr", "active-backup", "balance-xor", "broadcast",
		"802.3ad", "balance-tlb", "balance-alb",
		"0", "1", "2", "3", "4", "5", "6", // numeric modes
	}

	for _, validMode := range validModes {
		if mode == validMode {
			return nil
		}
	}

	return fmt.Errorf("invalid bond mode: %s", mode)
}

// validateBridgeSTPState validates bridge STP state
func validateBridgeSTPState(stp bool) error {
	// STP can be true or false, no validation needed
	return nil
}

// validateDHCPIdentifier validates DHCP identifier
func validateDHCPIdentifier(identifier string) error {
	validIdentifiers := []string{"duid", "mac"}
	
	for _, valid := range validIdentifiers {
		if identifier == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid DHCP identifier: %s (must be 'duid' or 'mac')", identifier)
}

// validateWiFiMode validates WiFi mode
func validateWiFiMode(mode string) error {
	validModes := []string{"infrastructure", "ap", "adhoc"}
	
	for _, validMode := range validModes {
		if mode == validMode {
			return nil
		}
	}

	return fmt.Errorf("invalid WiFi mode: %s", mode)
}

// validateWiFiBand validates WiFi band
func validateWiFiBand(band string) error {
	validBands := []string{"2.4GHz", "5GHz", "6GHz"}
	
	for _, validBand := range validBands {
		if band == validBand {
			return nil
		}
	}

	return fmt.Errorf("invalid WiFi band: %s", band)
}

// validateWiFiChannel validates WiFi channel
func validateWiFiChannel(channel int) error {
	// Simplified validation - covers most common channels
	// 2.4GHz: 1-14, 5GHz: 36, 40, 44, 48, 52, 56, 60, 64, etc.
	validChannels2_4 := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}
	validChannels5 := []int{36, 40, 44, 48, 52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124, 128, 132, 136, 140, 144, 149, 153, 157, 161, 165}

	// Check 2.4GHz channels
	for _, ch := range validChannels2_4 {
		if channel == ch {
			return nil
		}
	}

	// Check 5GHz channels
	for _, ch := range validChannels5 {
		if channel == ch {
			return nil
		}
	}

	return fmt.Errorf("invalid WiFi channel: %d", channel)
}

// validateTunnelMode validates tunnel mode
func validateTunnelMode(mode string) error {
	validModes := []string{
		"sit", "gre", "ip6gre", "ipip", "ip6ip6", "ip6tnl",
		"vti", "vti6", "wireguard", "isatap", "6rd",
	}
	
	for _, validMode := range validModes {
		if mode == validMode {
			return nil
		}
	}

	return fmt.Errorf("invalid tunnel mode: %s", mode)
}

// validateTTL validates tunnel TTL value
func validateTTL(ttl int) error {
	if ttl < 1 || ttl > 255 {
		return fmt.Errorf("invalid TTL: %d (must be between 1 and 255)", ttl)
	}
	return nil
}

// validateRoutingTableID validates routing table ID
func validateRoutingTableID(tableID int) error {
	if tableID < 0 || tableID > 4294967295 {
		return fmt.Errorf("invalid routing table ID: %d (must be between 0 and 4294967295)", tableID)
	}
	return nil
}

// validateRoutePriority validates route priority
func validateRoutePriority(priority int) error {
	if priority < 0 || priority > 4294967295 {
		return fmt.Errorf("invalid route priority: %d (must be between 0 and 4294967295)", priority)
	}
	return nil
}

// validateTypeOfService validates type of service field
func validateTypeOfService(tos int) error {
	if tos < 0 || tos > 255 {
		return fmt.Errorf("invalid type of service: %d (must be between 0 and 255)", tos)
	}
	return nil
}

// validateMark validates packet mark
func validateMark(mark int) error {
	if mark < 0 || mark > 4294967295 {
		return fmt.Errorf("invalid mark: %d (must be between 0 and 4294967295)", mark)
	}
	return nil
}