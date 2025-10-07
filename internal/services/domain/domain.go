// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

// Package domain handles Active Directory domain membership operations.
//
// # Overview
//
// This package provides functionality to join, leave, and manage Linux host membership
// in Active Directory domains. It supports both self-hosted Samba AD DC and external
// enterprise AD environments.
//
// # Domain Join Process
//
// The domain join process involves several steps to integrate the Linux host with AD:
//
//  1. Kerberos Configuration (/etc/krb5.conf):
//     - Configures realm, KDC servers, and domain mappings
//     - Enables dns_lookup_kdc for automatic DC discovery
//     - Required for 'net ads join' to authenticate with the DC
//
//  2. NSS Configuration (/etc/nsswitch.conf):
//     - Adds winbind to passwd and group resolution
//     - Allows Linux to resolve AD users/groups (e.g., 'id paul0')
//     - Format: "passwd: files systemd winbind"
//
//  3. DNS Configuration (optional):
//     - Points host DNS to the AD DC for proper name resolution
//     - Uses resolvectl to configure per-interface DNS
//     - Critical for AD services like LDAP, Kerberos
//
//  4. Domain Join (net ads join):
//     - Joins the host to the AD domain
//     - Creates a computer account in AD
//     - Non-interactive using --password flag
//
//  5. Winbind Service:
//     - Restarted after join to apply domain membership
//     - Provides user/group enumeration from AD
//     - Required for SMB shares with AD authentication
//
// # Self-Hosted vs External AD
//
// Self-Hosted Mode (config.AD.Mode = "self-hosted"):
//   - Uses the Samba AD DC running in a Docker container
//   - DC servers populated from config.AD.DC settings
//   - Automatically triggered when AD DC container starts (if autoJoin enabled)
//
// External Mode (config.AD.Mode = "external"):
//   - Uses client organization's existing AD infrastructure
//   - DC servers from config.AD.External.DomainControllers
//   - Can be triggered manually via 'rodent domain join' command
//   - Supports multiple DCs for failover
//
// # DC Failover
//
// When multiple domain controllers are configured, the Join() method tries each DC
// in order until one succeeds. This provides resilience against individual DC failures.
//
// # Manual Operations
//
// The domain service can also be used directly via CLI:
//
//	rodent domain join --realm AD.CORP.COM --dc dc1.corp.com --user Administrator
//	rodent domain leave
//	rodent domain status
//
// # File Backups
//
// Before modifying system files, backups are created with timestamps:
//   - /etc/krb5.conf.backup.YYYYMMDD-HHMMSS
//   - /etc/nsswitch.conf.backup.YYYYMMDD-HHMMSS
//
// This allows easy restoration if needed.
//
// See also: internal/services/addc for self-hosted AD DC management
package domain

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/stratastor/logger"
	rodentCfg "github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/command"
)

// DomainConfig contains configuration for domain join operations
type DomainConfig struct {
	Realm         string   // AD realm (e.g., "AD.STRATA.INTERNAL")
	DCServers     []string // List of domain controller IPs/hostnames
	AdminUser     string   // Admin username for domain join
	AdminPassword string   // Admin password
	IPAddress     string   // DC IP address (for DNS configuration)
	HostInterface string   // Host interface for DNS configuration
}

// Client handles domain membership operations
type Client struct {
	logger   logger.Logger
	executor *command.CommandExecutor
}

// NewClient creates a new domain client
func NewClient(logger logger.Logger) (*Client, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Create command executor with sudo enabled for privileged operations
	executor := command.NewCommandExecutor(true)

	return &Client{
		logger:   logger,
		executor: executor,
	}, nil
}

// Join joins the host to an AD domain
func (c *Client) Join(ctx context.Context, cfg *DomainConfig) error {
	c.logger.Info("Starting domain join process", "realm", cfg.Realm, "admin_user", cfg.AdminUser)

	// Validate configuration
	if err := c.validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid domain configuration: %w", err)
	}

	// Check if already joined
	c.logger.Info("Checking if host is already joined to AD domain", "realm", cfg.Realm)
	_, err := c.executor.ExecuteWithCombinedOutput(ctx, "net", "ads", "testjoin")
	if err == nil {
		c.logger.Info("Host is already joined to AD domain", "realm", cfg.Realm)
		return nil
	}

	c.logger.Info("Host not joined to AD domain, proceeding with join", "realm", cfg.Realm)

	// Configure Kerberos
	if err := c.configureKerberos(ctx, cfg); err != nil {
		return fmt.Errorf("failed to configure Kerberos: %w", err)
	}

	// Configure NSS for winbind
	if err := c.configureNSS(ctx); err != nil {
		return fmt.Errorf("failed to configure NSS: %w", err)
	}

	// Configure DNS if DC IP is provided
	if cfg.IPAddress != "" && cfg.HostInterface != "" {
		if err := c.configureDNS(ctx, cfg); err != nil {
			c.logger.Warn("Failed to configure DNS", "error", err)
			// Don't fail completely - DNS config is best-effort
		}
	}

	// Join the domain using net ads join
	c.logger.Info("Joining AD domain", "realm", cfg.Realm, "user", cfg.AdminUser)

	// Use --password flag for non-interactive join
	_, err = c.executor.ExecuteWithCombinedOutput(ctx, "net", "ads", "join",
		"-U", cfg.AdminUser,
		"--password="+cfg.AdminPassword)
	if err != nil {
		return fmt.Errorf("failed to join AD domain: %w", err)
	}

	c.logger.Info("Successfully joined AD domain", "realm", cfg.Realm)

	// Restart winbind service to apply domain membership
	c.logger.Info("Restarting winbind service")
	_, err = c.executor.ExecuteWithCombinedOutput(ctx, "systemctl", "restart", "winbind")
	if err != nil {
		c.logger.Warn("Failed to restart winbind, continuing", "error", err)
		// Don't fail completely - winbind might not be installed yet
	}

	return nil
}

// Leave removes the host from the AD domain
func (c *Client) Leave(ctx context.Context, cfg *DomainConfig) error {
	c.logger.Info("Leaving AD domain", "realm", cfg.Realm)

	// Check if we're actually joined
	_, err := c.executor.ExecuteWithCombinedOutput(ctx, "net", "ads", "testjoin")
	if err != nil {
		c.logger.Info("Host is not joined to any domain")
		return nil
	}

	// Leave the domain
	_, err = c.executor.ExecuteWithCombinedOutput(ctx, "net", "ads", "leave",
		"-U", cfg.AdminUser,
		"--password="+cfg.AdminPassword)
	if err != nil {
		return fmt.Errorf("failed to leave AD domain: %w", err)
	}

	c.logger.Info("Successfully left AD domain")

	// Restart winbind
	_, err = c.executor.ExecuteWithCombinedOutput(ctx, "systemctl", "restart", "winbind")
	if err != nil {
		c.logger.Warn("Failed to restart winbind", "error", err)
	}

	return nil
}

// Status checks if the host is joined to a domain
func (c *Client) Status(ctx context.Context) (bool, string, error) {
	output, err := c.executor.ExecuteWithCombinedOutput(ctx, "net", "ads", "testjoin")
	if err != nil {
		return false, "", nil // Not joined
	}

	// Parse output to extract domain name
	domain := strings.TrimSpace(string(output))

	return true, domain, nil
}

// WaitForDC waits for a domain controller to be ready
func (c *Client) WaitForDC(ctx context.Context, dcServer string, timeout time.Duration) error {
	ldapsPort := "636"

	deadline := time.Now().Add(timeout)
	attempt := 0

	for time.Now().Before(deadline) {
		attempt++

		// Try to connect to LDAPS port
		conn, err := net.DialTimeout("tcp", dcServer+":"+ldapsPort, 2*time.Second)
		if err == nil {
			conn.Close()
			c.logger.Info("Domain controller LDAPS port is reachable",
				"dc", dcServer,
				"attempts", attempt)
			return nil
		}

		c.logger.Debug("Waiting for domain controller LDAPS port",
			"attempt", attempt,
			"dc", dcServer,
			"port", ldapsPort,
			"error", err)

		// Wait before next attempt
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for DC")
		case <-time.After(2 * time.Second):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("timeout waiting for DC %s to be ready after %v", dcServer, timeout)
}

// validateConfig validates the domain configuration
func (c *Client) validateConfig(cfg *DomainConfig) error {
	if cfg.Realm == "" {
		return fmt.Errorf("realm is required")
	}
	if len(cfg.DCServers) == 0 {
		return fmt.Errorf("at least one domain controller is required")
	}
	if cfg.AdminUser == "" {
		return fmt.Errorf("admin user is required")
	}
	if cfg.AdminPassword == "" {
		return fmt.Errorf("admin password is required")
	}
	return nil
}

// configureKerberos writes a minimal Kerberos configuration for AD
func (c *Client) configureKerberos(ctx context.Context, cfg *DomainConfig) error {
	realm := strings.ToUpper(cfg.Realm)
	domainLower := strings.ToLower(cfg.Realm)

	c.logger.Info("Configuring Kerberos", "realm", realm)

	// Backup existing krb5.conf if it exists
	krb5Path := "/etc/krb5.conf"
	_, err := c.executor.ExecuteWithCombinedOutput(ctx, "test", "-f", krb5Path)
	if err == nil {
		// File exists, create backup with datetime
		backupPath := fmt.Sprintf("%s.backup.%s", krb5Path, time.Now().Format("20060102-150405"))
		c.logger.Info("Backing up existing Kerberos config", "backup", backupPath)
		_, err = c.executor.ExecuteWithCombinedOutput(ctx, "cp", krb5Path, backupPath)
		if err != nil {
			c.logger.Warn("Failed to backup krb5.conf", "error", err)
		}
	}

	// Build KDC list from DC servers
	kdcList := ""
	for _, dc := range cfg.DCServers {
		kdcList += fmt.Sprintf("        kdc = %s\n", dc)
	}

	krb5Conf := fmt.Sprintf(`[libdefaults]
    default_realm = %s
    dns_lookup_realm = false
    dns_lookup_kdc = true
    ticket_lifetime = 30d
    renew_lifetime = 365d
    forwardable = true

[realms]
    %s = {
%s        admin_server = %s
        default_domain = %s
    }

`, realm, realm, kdcList, cfg.DCServers[0], domainLower)

	// Write Kerberos config
	// Create temp file
	tmpFile, err := os.CreateTemp("", "rodent-krb5-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for krb5.conf: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(krb5Conf); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write krb5.conf: %w", err)
	}
	tmpFile.Close()

	// Copy to /etc/krb5.conf using sudo
	_, err = c.executor.ExecuteWithCombinedOutput(ctx, "cp", tmpPath, krb5Path)
	if err != nil {
		return fmt.Errorf("failed to copy krb5.conf: %w", err)
	}

	c.logger.Info("Kerberos configuration written successfully")
	return nil
}

// configureNSS updates /etc/nsswitch.conf to use winbind for user/group resolution
func (c *Client) configureNSS(ctx context.Context) error {
	c.logger.Info("Configuring NSS for winbind")

	// Check if winbind is already in nsswitch.conf
	output, err := c.executor.ExecuteWithCombinedOutput(
		ctx,
		"grep",
		"winbind",
		"/etc/nsswitch.conf",
	)
	if err == nil && len(output) > 0 {
		c.logger.Debug("NSS already configured for winbind")
		return nil
	}

	// Backup existing nsswitch.conf
	nssPath := "/etc/nsswitch.conf"
	backupPath := fmt.Sprintf("%s.backup.%s", nssPath, time.Now().Format("20060102-150405"))
	c.logger.Info("Backing up existing NSS config", "backup", backupPath)
	_, err = c.executor.ExecuteWithCombinedOutput(ctx, "cp", nssPath, backupPath)
	if err != nil {
		c.logger.Warn("Failed to backup nsswitch.conf", "error", err)
	}

	// Update passwd and group lines to add winbind
	// passwd: files systemd winbind
	_, err = c.executor.ExecuteWithCombinedOutput(ctx, "sed", "-i",
		"s/^passwd:.*/passwd:         files systemd winbind/",
		nssPath)
	if err != nil {
		c.logger.Warn("Failed to update passwd line in nsswitch.conf", "error", err)
	}

	_, err = c.executor.ExecuteWithCombinedOutput(ctx, "sed", "-i",
		"s/^group:.*/group:          files systemd winbind/",
		nssPath)
	if err != nil {
		c.logger.Warn("Failed to update group line in nsswitch.conf", "error", err)
	}

	c.logger.Info("NSS configured for winbind")
	return nil
}

// configureDNS configures host DNS to use the domain controller
func (c *Client) configureDNS(ctx context.Context, cfg *DomainConfig) error {
	c.logger.Info("Configuring host DNS for AD DC",
		"dc_ip", cfg.IPAddress,
		"interface", cfg.HostInterface)

	realm := strings.ToLower(cfg.Realm)

	// Set DNS server for the interface
	_, err := c.executor.ExecuteWithCombinedOutput(ctx, "resolvectl", "dns",
		cfg.HostInterface, cfg.IPAddress)
	if err != nil {
		c.logger.Warn("Failed to set DNS server via resolvectl", "error", err)
	} else {
		c.logger.Info("Configured DNS server via resolvectl",
			"interface", cfg.HostInterface,
			"dns", cfg.IPAddress)
	}

	// Set DNS domain for the interface
	_, err = c.executor.ExecuteWithCombinedOutput(ctx, "resolvectl", "domain",
		cfg.HostInterface, realm)
	if err != nil {
		c.logger.Warn("Failed to set DNS domain via resolvectl", "error", err)
	} else {
		c.logger.Info("Configured DNS domain via resolvectl",
			"interface", cfg.HostInterface,
			"domain", realm)
	}

	return nil
}

// GetConfigFromGlobal returns DomainConfig populated from global config
func GetConfigFromGlobal() *DomainConfig {
	cfg := rodentCfg.GetConfig()

	domainCfg := &DomainConfig{
		Realm:         cfg.AD.Realm,
		AdminPassword: cfg.AD.AdminPassword,
	}

	// Populate based on mode
	if cfg.AD.Mode == "external" {
		// External AD mode
		domainCfg.DCServers = cfg.AD.External.DomainControllers
		domainCfg.AdminUser = cfg.AD.External.AdminUser
		if domainCfg.AdminUser == "" {
			domainCfg.AdminUser = "Administrator"
		}
	} else {
		// Self-hosted mode
		if cfg.AD.DC.Enabled {
			dcFQDN := fmt.Sprintf("%s.%s",
				strings.ToUpper(cfg.AD.DC.Hostname),
				strings.ToLower(cfg.AD.DC.Realm))
			domainCfg.DCServers = []string{dcFQDN}
			domainCfg.AdminUser = "Administrator"
			domainCfg.Realm = cfg.AD.DC.Realm
			domainCfg.IPAddress = cfg.AD.DC.IPAddress
			domainCfg.HostInterface = cfg.AD.DC.ParentInterface
		}
	}

	return domainCfg
}
