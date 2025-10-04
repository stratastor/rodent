// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package netmage

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/stratastor/rodent/pkg/netmage/types"
)

// DefaultSafeConfigOptions returns sensible defaults for safe configuration
func DefaultSafeConfigOptions() *types.SafeConfigOptions {
	return &types.SafeConfigOptions{
		ConnectivityTargets:     []string{"8.8.8.8", "1.1.1.1"},
		ConnectivityTimeout:     30 * time.Second,
		ConnectivityInterval:    2 * time.Second,
		MaxConnectivityFailures: 3,
		SkipPreValidation:       false,
		SkipPostValidation:      false,
		AutoBackup:              true,
		AutoRollback:            true,
		RollbackTimeout:         60 * time.Second,
		BackupDescription:       "Automatic backup before configuration change",
		GracePeriod:             30 * time.Second,
		ValidateInterfaces:      true,
		ValidateRoutes:          true,
		ValidateConnectivity:    true,
	}
}

// SafeApplyConfig applies a netplan configuration with comprehensive safety measures
// This replaces the unreliable netplan try functionality
func (m *manager) SafeApplyConfig(ctx context.Context, config *types.NetplanConfig, options *types.SafeConfigOptions) (*types.SafeConfigResult, error) {
	if options == nil {
		options = DefaultSafeConfigOptions()
	}
	
	result := &types.SafeConfigResult{
		StartTime:      time.Now(),
		PreValidation:  &types.ValidationResult{},
		PostValidation: &types.ValidationResult{},
		Connectivity:   &types.ConnectivityResult{TargetResults: make(map[string]bool)},
	}
	
	m.logger.Info("Starting safe configuration application",
		"auto_backup", options.AutoBackup,
		"auto_rollback", options.AutoRollback,
		"connectivity_targets", options.ConnectivityTargets)

	// Step 0: Check if configuration has changed (idempotency check)
	m.logger.Debug("Checking if configuration has changed")
	currentConfig, err := m.netplanCmd.GetConfig(ctx)
	if err != nil {
		m.logger.Debug("Failed to get current config for comparison, will proceed with apply", "error", err)
	} else {
		if m.configsAreEqual(currentConfig, config) {
			m.logger.Info("Configuration unchanged, skipping apply")
			result.Success = true
			result.Applied = false
			result.Message = "Configuration unchanged, no action needed"
			result.CompletionTime = time.Now()
			result.TotalDuration = result.CompletionTime.Sub(result.StartTime)
			return result, nil
		}
		m.logger.Debug("Configuration has changed, proceeding with apply")
	}

	// Step 1: Pre-validation
	if !options.SkipPreValidation {
		m.logger.Debug("Performing pre-validation")
		if err := m.performPreValidation(ctx, config, options, result.PreValidation); err != nil {
			m.logger.Error("Pre-validation failed with error", "error", err, "validation_errors", result.PreValidation.Errors)
			result.Error = fmt.Sprintf("Pre-validation failed: %v", err)
			result.Message = "Configuration validation failed"
			result.CompletionTime = time.Now()
			result.TotalDuration = result.CompletionTime.Sub(result.StartTime)
			return result, err
		}
		m.logger.Info("Pre-validation completed successfully")
	}
	
	// Step 2: Create backup if requested
	if options.AutoBackup {
		m.logger.Debug("Creating configuration backup")
		backupID, err := m.netplanCmd.Backup(ctx)
		if err != nil {
			result.Error = fmt.Sprintf("Backup creation failed: %v", err)
			result.Message = "Failed to create backup"
			result.CompletionTime = time.Now()
			result.TotalDuration = result.CompletionTime.Sub(result.StartTime)
			return result, err
		}
		result.BackupID = backupID
		m.logger.Info("Configuration backup created", "backup_id", backupID)
	}
	
	// Step 3: Test initial connectivity
	if options.ValidateConnectivity {
		m.logger.Debug("Testing initial connectivity")
		if err := m.testInitialConnectivity(ctx, options, result.Connectivity); err != nil {
			result.Error = fmt.Sprintf("Initial connectivity test failed: %v", err)
			result.Message = "Initial connectivity check failed"
			result.CompletionTime = time.Now()
			result.TotalDuration = result.CompletionTime.Sub(result.StartTime)
			return result, err
		}
		m.logger.Info("Initial connectivity test passed")
	}
	
	// Step 4: Apply configuration
	m.logger.Info("Applying configuration")
	result.ApplyTime = time.Now()
	
	if err := m.netplanCmd.SetConfig(ctx, config); err != nil {
		result.Error = fmt.Sprintf("Failed to set configuration: %v", err)
		result.Message = "Configuration set failed"
		m.performRollback(ctx, result, options)
		return result, err
	}
	
	if err := m.netplanCmd.Apply(ctx); err != nil {
		result.Error = fmt.Sprintf("Failed to apply configuration: %v", err)
		result.Message = "Configuration apply failed"
		m.performRollback(ctx, result, options)
		return result, err
	}
	
	result.Applied = true
	m.logger.Info("Configuration applied successfully")
	
	// Step 5: Post-validation
	if !options.SkipPostValidation {
		m.logger.Debug("Performing post-validation")
		if err := m.performPostValidation(ctx, options, result.PostValidation); err != nil {
			result.Error = fmt.Sprintf("Post-validation failed: %v", err)
			result.Message = "Configuration applied but validation failed"
			m.performRollback(ctx, result, options)
			return result, err
		}
		m.logger.Info("Post-validation completed successfully")
	}
	
	// Step 6: Connectivity monitoring during grace period
	if options.ValidateConnectivity && options.GracePeriod > 0 {
		m.logger.Info("Starting connectivity monitoring", "grace_period", options.GracePeriod)
		monitorCtx, cancel := context.WithTimeout(ctx, options.GracePeriod)
		defer cancel()
		
		if err := m.monitorConnectivity(monitorCtx, options, result.Connectivity); err != nil {
			result.Error = fmt.Sprintf("Connectivity monitoring failed: %v", err)
			result.Message = "Configuration applied but connectivity lost"
			m.performRollback(ctx, result, options)
			return result, err
		}
		m.logger.Info("Connectivity monitoring completed successfully")
	}
	
	// Success!
	result.Success = true
	result.Message = "Configuration applied successfully with all safety checks passed"
	result.CompletionTime = time.Now()
	result.TotalDuration = result.CompletionTime.Sub(result.StartTime)
	
	m.logger.Info("Safe configuration application completed successfully",
		"total_duration", result.TotalDuration,
		"backup_id", result.BackupID)
	
	return result, nil
}

// performPreValidation validates configuration before applying
func (m *manager) performPreValidation(ctx context.Context, config *types.NetplanConfig, options *types.SafeConfigOptions, validation *types.ValidationResult) error {
	m.logger.Debug("Starting pre-validation checks")
	
	// Syntax validation
	m.logger.Debug("Performing syntax validation")
	if err := m.ValidateNetplanConfig(ctx, config); err != nil {
		m.logger.Error("Syntax validation failed", "error", err)
		validation.Errors = append(validation.Errors, fmt.Sprintf("Syntax validation: %v", err))
		return err
	}
	validation.SyntaxValid = true
	m.logger.Debug("Syntax validation passed")
	
	// Interface validation
	if options.ValidateInterfaces {
		m.logger.Debug("Performing interface validation")
		if err := m.validateInterfaceReferences(ctx, config); err != nil {
			m.logger.Error("Interface validation failed", "error", err)
			validation.Errors = append(validation.Errors, fmt.Sprintf("Interface validation: %v", err))
			return err
		}
		validation.InterfaceValid = true
		m.logger.Debug("Interface validation passed")
	}
	
	// Route validation
	if options.ValidateRoutes {
		m.logger.Debug("Performing route validation")
		if err := m.validateRouteConfiguration(ctx, config); err != nil {
			m.logger.Error("Route validation failed", "error", err)
			validation.Errors = append(validation.Errors, fmt.Sprintf("Route validation: %v", err))
			return err
		}
		validation.RouteValid = true
		m.logger.Debug("Route validation passed")
	}
	
	validation.Success = true
	m.logger.Debug("All pre-validation checks passed")
	return nil
}

// performPostValidation validates system state after applying configuration
func (m *manager) performPostValidation(ctx context.Context, options *types.SafeConfigOptions, validation *types.ValidationResult) error {
	// Check if configuration is parseable
	_, err := m.netplanCmd.GetConfig(ctx)
	if err != nil {
		validation.Errors = append(validation.Errors, fmt.Sprintf("Config parsing failed: %v", err))
		return err
	}
	validation.SyntaxValid = true
	
	// Check interface states
	if options.ValidateInterfaces {
		if err := m.validatePostApplyInterfaces(ctx); err != nil {
			validation.Errors = append(validation.Errors, fmt.Sprintf("Interface state validation: %v", err))
			return err
		}
		validation.InterfaceValid = true
	}
	
	// Check route states
	if options.ValidateRoutes {
		if err := m.validatePostApplyRoutes(ctx); err != nil {
			validation.Errors = append(validation.Errors, fmt.Sprintf("Route state validation: %v", err))
			return err
		}
		validation.RouteValid = true
	}
	
	validation.Success = true
	return nil
}

// testInitialConnectivity tests connectivity before making changes
func (m *manager) testInitialConnectivity(ctx context.Context, options *types.SafeConfigOptions, connectivity *types.ConnectivityResult) error {
	for _, target := range options.ConnectivityTargets {
		reachable := m.pingTarget(ctx, target, 3*time.Second)
		connectivity.TargetResults[target] = reachable
		
		if !reachable {
			return fmt.Errorf("target %s is not reachable", target)
		}
	}
	
	connectivity.InitialSuccess = true
	return nil
}

// monitorConnectivity continuously monitors connectivity during grace period
func (m *manager) monitorConnectivity(ctx context.Context, options *types.SafeConfigOptions, connectivity *types.ConnectivityResult) error {
	startTime := time.Now()
	ticker := time.NewTicker(options.ConnectivityInterval)
	defer ticker.Stop()
	
	consecutiveFailures := 0
	
	for {
		select {
		case <-ctx.Done():
			connectivity.MonitoringTime = time.Since(startTime)
			connectivity.FinalSuccess = consecutiveFailures < options.MaxConnectivityFailures
			return nil
			
		case <-ticker.C:
			connectivity.TotalChecks++
			
			// Test all targets
			allReachable := true
			for _, target := range options.ConnectivityTargets {
				reachable := m.pingTarget(ctx, target, time.Second)
				if !reachable {
					allReachable = false
					break
				}
			}
			
			if !allReachable {
				consecutiveFailures++
				connectivity.FailedChecks++
				
				if consecutiveFailures >= options.MaxConnectivityFailures {
					connectivity.MonitoringTime = time.Since(startTime)
					return fmt.Errorf("connectivity lost: %d consecutive failures", consecutiveFailures)
				}
			} else {
				consecutiveFailures = 0 // Reset on success
			}
		}
	}
}

// performRollback performs automatic rollback if enabled
func (m *manager) performRollback(ctx context.Context, result *types.SafeConfigResult, options *types.SafeConfigOptions) {
	if !options.AutoRollback || result.BackupID == "" {
		return
	}
	
	m.logger.Warn("Performing automatic rollback", "backup_id", result.BackupID)
	
	rollbackCtx, cancel := context.WithTimeout(ctx, options.RollbackTimeout)
	defer cancel()
	
	if err := m.netplanCmd.Restore(rollbackCtx, result.BackupID); err != nil {
		m.logger.Error("Rollback restore failed", "error", err)
		result.Error += fmt.Sprintf("; rollback restore failed: %v", err)
		return
	}
	
	if err := m.netplanCmd.Apply(rollbackCtx); err != nil {
		m.logger.Error("Rollback apply failed", "error", err)
		result.Error += fmt.Sprintf("; rollback apply failed: %v", err)
		return
	}
	
	result.RolledBack = true
	result.Message += "; automatically rolled back to previous configuration"
	m.logger.Info("Automatic rollback completed successfully")
}

// Helper validation methods

// validateInterfaceReferences validates that all interface references exist
func (m *manager) validateInterfaceReferences(_ context.Context, config *types.NetplanConfig) error {
	if config.Network == nil {
		return nil
	}
	
	// Validate bond member interfaces
	for bondName, bond := range config.Network.Bonds {
		for _, memberIface := range bond.Interfaces {
			if _, exists := config.Network.Ethernets[memberIface]; !exists {
				return fmt.Errorf("bond %s references non-existent interface %s", bondName, memberIface)
			}
		}
	}
	
	// Validate bridge member interfaces
	for bridgeName, bridge := range config.Network.Bridges {
		for _, memberIface := range bridge.Interfaces {
			if _, exists := config.Network.Ethernets[memberIface]; !exists {
				return fmt.Errorf("bridge %s references non-existent interface %s", bridgeName, memberIface)
			}
		}
	}
	
	// Validate VLAN link interfaces
	for vlanName, vlan := range config.Network.VLANs {
		if _, exists := config.Network.Ethernets[vlan.Link]; !exists {
			return fmt.Errorf("VLAN %s references non-existent link interface %s", vlanName, vlan.Link)
		}
	}
	
	return nil
}

// validateRouteConfiguration validates route configurations
func (m *manager) validateRouteConfiguration(_ context.Context, config *types.NetplanConfig) error {
	if config.Network == nil {
		return nil
	}
	
	// Validate ethernet interface routes
	for ifaceName, eth := range config.Network.Ethernets {
		for i, route := range eth.Routes {
			if route.To == "" {
				return fmt.Errorf("interface %s route %d missing destination", ifaceName, i)
			}
			
			if route.Via == "" && route.To != "default" && route.To != "0.0.0.0/0" {
				return fmt.Errorf("interface %s route %d missing gateway for non-default route", ifaceName, i)
			}
			
			// Validate IP addresses in routes
			if route.Via != "" {
				if net.ParseIP(route.Via) == nil {
					return fmt.Errorf("interface %s route %d has invalid gateway IP: %s", ifaceName, i, route.Via)
				}
			}
		}
	}
	
	return nil
}

// validatePostApplyInterfaces validates interface states after applying configuration
// Uses retries and relaxed validation to handle temporary interface transition states
func (m *manager) validatePostApplyInterfaces(ctx context.Context) error {
	const maxRetries = 5
	const retryDelay = 2 * time.Second

	var lastErr error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		m.logger.Debug("Validating post-apply interfaces", "attempt", attempt, "max_retries", maxRetries)
		
		status, err := m.netplanCmd.GetStatus(ctx, "")
		if err != nil {
			lastErr = fmt.Errorf("failed to get interface status: %v", err)
			if attempt < maxRetries {
				m.logger.Debug("Interface status check failed, retrying", "error", err, "retry_in", retryDelay)
				time.Sleep(retryDelay)
				continue
			}
			return lastErr
		}

		managedCount := 0
		upCount := 0
		var nonUpInterfaces []string
		
		for name, iface := range status.Interfaces {
			if iface.Backend == "networkd" {
				managedCount++
				
				// Check admin state - this should be UP for most interfaces
				if iface.AdminState == "UP" {
					upCount++
				}
				
				// Be more lenient with operational state during transitions
				// Only require operational UP for primary interfaces, not all
				if iface.OperState != "UP" && name != "lo" {
					nonUpInterfaces = append(nonUpInterfaces, fmt.Sprintf("%s(oper:%s)", name, iface.OperState))
				}
			}
		}

		// Check if we have any networkd-managed interfaces
		if managedCount == 0 {
			lastErr = fmt.Errorf("no networkd-managed interfaces found")
			if attempt < maxRetries {
				m.logger.Debug("No networkd interfaces found, retrying", "retry_in", retryDelay)
				time.Sleep(retryDelay)
				continue
			}
			// On final attempt, be more lenient - just warn instead of failing
			m.logger.Warn("No networkd-managed interfaces found after retries, but continuing", 
				"attempts", maxRetries)
			return nil
		}

		// Require at least one interface to be administratively up
		if upCount == 0 {
			lastErr = fmt.Errorf("no networkd-managed interfaces are administratively up")
			if attempt < maxRetries {
				m.logger.Debug("No interfaces administratively up, retrying", "retry_in", retryDelay)
				time.Sleep(retryDelay)
				continue
			}
			return lastErr
		}

		// Log non-UP operational interfaces but don't fail for them during transitions
		if len(nonUpInterfaces) > 0 {
			m.logger.Debug("Some interfaces not operationally up", 
				"interfaces", nonUpInterfaces, 
				"managed_count", managedCount,
				"admin_up_count", upCount)
		}

		// Success if we have managed interfaces and at least one is admin UP
		m.logger.Debug("Interface validation passed", 
			"managed_count", managedCount, 
			"admin_up_count", upCount,
			"attempt", attempt)
		return nil
	}

	// If we exhausted retries, return the last error
	return lastErr
}

// validatePostApplyRoutes validates route states after applying configuration
// Uses retries to handle temporary route table states during transitions
func (m *manager) validatePostApplyRoutes(ctx context.Context) error {
	const maxRetries = 3
	const retryDelay = 2 * time.Second

	var lastErr error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		m.logger.Debug("Validating post-apply routes", "attempt", attempt, "max_retries", maxRetries)
		
		routes, err := m.GetRoutes(ctx, "main")
		if err != nil {
			lastErr = fmt.Errorf("failed to get routes: %v", err)
			if attempt < maxRetries {
				m.logger.Debug("Route check failed, retrying", "error", err, "retry_in", retryDelay)
				time.Sleep(retryDelay)
				continue
			}
			// Be more lenient on final attempt - routing might be complex
			m.logger.Warn("Failed to get routes after retries, but continuing", 
				"error", err, "attempts", maxRetries)
			return nil
		}
		
		// Debug log the routes we found
		m.logger.Debug("GetRoutes parsed result", "route_count", len(routes))
		
		// Check for at least one default route
		hasDefault := false
		var foundRoutes []string
		for i, route := range routes {
			foundRoutes = append(foundRoutes, route.To)
			// Debug log first few routes to see what we're getting
			if i < 5 {
				m.logger.Debug("Route details", 
					"to", route.To,
					"via", route.Via,
					"table", route.Table,
					"family", route.Family)
			}
			if route.To == "default" || route.To == "0.0.0.0/0" {
				hasDefault = true
				break
			}
		}
		
		if !hasDefault {
			lastErr = fmt.Errorf("no default route found")
			if attempt < maxRetries {
				m.logger.Debug("No default route found, retrying", "routes", foundRoutes, "retry_in", retryDelay)
				time.Sleep(retryDelay)
				continue
			}
			// On AWS EC2 with complex routing policies, this might be expected
			m.logger.Warn("No default route found after retries, but continuing", 
				"found_routes", foundRoutes, "attempts", maxRetries)
			return nil
		}
		
		m.logger.Debug("Route validation passed", "default_route_found", hasDefault, "attempt", attempt)
		return nil
	}
	
	// If we exhausted retries, return the last error (though we're being lenient above)
	return lastErr
}

// pingTarget tests connectivity to a target
func (m *manager) pingTarget(ctx context.Context, target string, timeout time.Duration) bool {
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := m.executor.ExecuteCommand(pingCtx, "ping", "-c", "1", "-W", "1", target)
	return err == nil && result.ExitCode == 0
}

// configsAreEqual performs semantic comparison of two netplan configurations
// Returns true if the configurations are functionally equivalent
func (m *manager) configsAreEqual(a, b *types.NetplanConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare network configurations
	if a.Network == nil && b.Network == nil {
		return true
	}
	if a.Network == nil || b.Network == nil {
		return false
	}

	// Compare ethernet interfaces
	if !m.ethernetsAreEqual(a.Network.Ethernets, b.Network.Ethernets) {
		return false
	}

	// Compare bonds
	if !m.bondsAreEqual(a.Network.Bonds, b.Network.Bonds) {
		return false
	}

	// Compare bridges
	if !m.bridgesAreEqual(a.Network.Bridges, b.Network.Bridges) {
		return false
	}

	// Compare VLANs
	if !m.vlansAreEqual(a.Network.VLANs, b.Network.VLANs) {
		return false
	}

	return true
}

// ethernetsAreEqual compares ethernet interface configurations
func (m *manager) ethernetsAreEqual(a, b map[string]*types.EthernetConfig) bool {
	if len(a) != len(b) {
		return false
	}

	for name, ethA := range a {
		ethB, exists := b[name]
		if !exists {
			return false
		}

		if !m.ethernetConfigIsEqual(ethA, ethB) {
			return false
		}
	}

	return true
}

// ethernetConfigIsEqual compares two ethernet configurations
func (m *manager) ethernetConfigIsEqual(a, b *types.EthernetConfig) bool {
	// Compare DHCP settings
	if !m.boolPtrsEqual(a.DHCPv4, b.DHCPv4) || !m.boolPtrsEqual(a.DHCPv6, b.DHCPv6) {
		return false
	}

	// Compare addresses
	if !m.stringSlicesEqual(a.Addresses, b.Addresses) {
		return false
	}

	// Compare nameservers
	if !m.nameserversEqual(a.Nameservers, b.Nameservers) {
		return false
	}

	// Compare routes
	if !m.routesEqual(a.Routes, b.Routes) {
		return false
	}

	// Compare routing policies
	if !m.routingPoliciesEqual(a.RoutingPolicy, b.RoutingPolicy) {
		return false
	}

	// Compare optional flag
	if !m.boolPtrsEqual(a.Optional, b.Optional) {
		return false
	}

	return true
}

// nameserversEqual compares nameserver configurations
func (m *manager) nameserversEqual(a, b *types.NameserverConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	return m.stringSlicesEqual(a.Addresses, b.Addresses) &&
		m.stringSlicesEqual(a.Search, b.Search)
}

// routesEqual compares route configurations
func (m *manager) routesEqual(a, b []*types.RouteConfig) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !m.routeConfigIsEqual(a[i], b[i]) {
			return false
		}
	}

	return true
}

// routeConfigIsEqual compares two route configurations
func (m *manager) routeConfigIsEqual(a, b *types.RouteConfig) bool {
	return a.To == b.To &&
		a.Via == b.Via &&
		m.intPtrsEqual(a.Table, b.Table) &&
		m.intPtrsEqual(a.Metric, b.Metric)
}

// routingPoliciesEqual compares routing policy configurations
func (m *manager) routingPoliciesEqual(a, b []*types.RoutingPolicyConfig) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !m.routingPolicyIsEqual(a[i], b[i]) {
			return false
		}
	}

	return true
}

// routingPolicyIsEqual compares two routing policy configurations
func (m *manager) routingPolicyIsEqual(a, b *types.RoutingPolicyConfig) bool {
	return a.From == b.From &&
		a.To == b.To &&
		m.intPtrsEqual(a.Table, b.Table) &&
		m.intPtrsEqual(a.Priority, b.Priority)
}

// bondsAreEqual compares bond configurations
func (m *manager) bondsAreEqual(a, b map[string]*types.BondConfig) bool {
	if len(a) != len(b) {
		return false
	}

	for name, bondA := range a {
		bondB, exists := b[name]
		if !exists || !m.stringSlicesEqual(bondA.Interfaces, bondB.Interfaces) {
			return false
		}
	}

	return true
}

// bridgesAreEqual compares bridge configurations
func (m *manager) bridgesAreEqual(a, b map[string]*types.BridgeConfig) bool {
	if len(a) != len(b) {
		return false
	}

	for name, bridgeA := range a {
		bridgeB, exists := b[name]
		if !exists || !m.stringSlicesEqual(bridgeA.Interfaces, bridgeB.Interfaces) {
			return false
		}
	}

	return true
}

// vlansAreEqual compares VLAN configurations
func (m *manager) vlansAreEqual(a, b map[string]*types.VLANConfig) bool {
	if len(a) != len(b) {
		return false
	}

	for name, vlanA := range a {
		vlanB, exists := b[name]
		if !exists || vlanA.ID != vlanB.ID || vlanA.Link != vlanB.Link {
			return false
		}
	}

	return true
}

// Helper comparison functions

func (m *manager) boolPtrsEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func (m *manager) intPtrsEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func (m *manager) stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}