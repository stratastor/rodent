// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

// Package smb provides comprehensive SMB/CIFS share management for StrataSTOR Rodent.
//
// # Architecture Overview
//
// The SMB package manages Samba file shares with support for both standalone and
// Active Directory integrated modes. It uses a template-based configuration system
// that separates configuration storage (JSON) from generated Samba configs.
//
// # Security Modes
//
// The package supports two security modes that can be automatically detected or
// explicitly configured:
//
//  1. Standalone Mode (security = user):
//     - Uses local system users and groups from /etc/passwd and /etc/group
//     - No domain controller required
//     - Suitable for simple file sharing scenarios
//     - Example valid users: "alice", "bob", "admins"
//
//  2. Active Directory Mode (security = ADS):
//     - Integrates with AD DC (self-hosted or external)
//     - Supports domain users and groups
//     - Requires winbind and domain membership
//     - Example valid users: "DOMAIN\alice", "DOMAIN\Domain Admins"
//
// Mode detection (automatic when security_mode = "auto"):
//   - If AD.DC.Enabled = true → ADS mode
//   - If AD.Mode = "external" with DCs configured → ADS mode
//   - Otherwise → Standalone mode
//
// # Directory Structure
//
// All Rodent-managed SMB configurations are stored in:
//   ~/.rodent/shares/smb/
//
// File types and their purposes:
//
//   global.conf              - JSON source of truth for global Samba settings
//   global.smb.conf          - Generated [global] section for smb.conf
//   <sharename>.json         - JSON source of truth for individual share
//   <sharename>.smb.conf     - Generated [sharename] section for smb.conf
//
// Templates are stored in:
//   ~/.rodent/templates/smb/
//
//   global.tmpl              - Template for [global] section
//   share.tmpl               - Template for share sections
//
// # Data Flow
//
// The configuration flow follows a strict pattern:
//
//   1. JSON Config Files (Source of Truth)
//      ├─ global.conf           (SMBGlobalConfig struct)
//      └─ <sharename>.json      (SMBShareConfig struct)
//            ↓
//   2. Template Rendering
//      ├─ generateGlobalConfig() → renders global.tmpl → global.smb.conf
//      └─ generateShareConfig()  → renders share.tmpl → <share>.smb.conf
//            ↓
//   3. Main Config Assembly (updateMainConfig)
//      ├─ Read global.smb.conf
//      ├─ Read all *.smb.conf share files
//      └─ Assemble into /etc/samba/smb.conf
//            ↓
//   4. Service Reload (optional, runtime only)
//      └─ smbcontrol smbd reload-config
//
// # Key Operations
//
// Configuration Generation (GenerateConfig):
//   - Called at service startup (before Samba starts)
//   - Imports existing /etc/samba/smb.conf if needed
//   - Auto-detects and adapts to security mode changes
//   - Creates backups when mode changes
//
// Share Management:
//   - CreateShare: Validates → Save JSON → Generate config → Update main → Reload
//   - UpdateShare: Validate → Save JSON → Regenerate → Update main → Reload
//   - DeleteShare: Remove JSON and .smb.conf → Update main → Reload
//
// Global Config Management:
//   - UpdateGlobalConfig: Validate → Save JSON → Regenerate → Update main → Reload
//   - GetGlobalConfig: Read and parse global.conf JSON
//
// # Security Mode Migration
//
// When the security mode changes (e.g., AD DC enabled/disabled), the system:
//   1. Detects mode change during GenerateConfig()
//   2. Creates backup of global.conf with timestamp
//   3. Generates new SMBGlobalConfig for the target mode
//   4. Saves new global.conf (JSON source)
//   5. Regenerates global.smb.conf from template
//   6. Updates /etc/samba/smb.conf
//
// Existing shares continue to work because:
//   - Share configs are mode-agnostic (same template structure)
//   - User validation happens at ACL/share creation time
//   - Both "user" and "DOMAIN\user" formats can coexist
//
// # User/Group Validation
//
// The package integrates with pkg/facl for access control validation:
//
//   - Local users/groups: Validated against /etc/passwd and /etc/group
//   - AD users/groups: Validated via LDAP queries (DOMAIN\user format)
//   - Mixed mode: Both types can be used in the same share
//
// # File Lifecycle
//
// Creating a share:
//   1. API receives SMBShareConfig
//   2. Manager.CreateShare() validates config
//   3. Saves to <sharename>.json
//   4. Calls generateShareConfig() → creates <sharename>.smb.conf
//   5. Calls updateMainConfig() → updates /etc/samba/smb.conf
//   6. Calls ReloadConfig() → tells Samba to reload
//
// Updating a share:
//   1. API receives updated SMBShareConfig
//   2. Manager.UpdateShare() validates changes
//   3. Updates <sharename>.json
//   4. Regenerates <sharename>.smb.conf
//   5. Updates /etc/samba/smb.conf
//   6. Reloads Samba service
//
// Deleting a share:
//   1. API requests share deletion
//   2. Manager.DeleteShare() removes <sharename>.json
//   3. Removes <sharename>.smb.conf
//   4. Updates /etc/samba/smb.conf (share section removed)
//   5. Reloads Samba service
//
// # Backup Strategy
//
// Backups are created in specific scenarios:
//
//   Initial Import:
//     - When first importing from /etc/samba/smb.conf
//     - Creates: /etc/samba/smb.conf.YYYYMMDD-HHMMSS.bak
//     - Preserves original manual configuration
//
//   Security Mode Change:
//     - When switching between standalone and AD modes
//     - Creates: ~/.rodent/shares/smb/global.conf.YYYYMMDD-HHMMSS.bak
//     - Allows rollback if mode change causes issues
//
// Runtime updates do NOT create backups as Rodent JSON configs are the source of truth.
//
// # Thread Safety
//
// All public methods use mutex locking:
//   - RWMutex for read/write operations
//   - Lock held during: validate → save → generate → update cycle
//   - Prevents concurrent modifications
//
// # Error Handling
//
// The package uses pkg/errors for structured error reporting:
//   - All errors include operation metadata
//   - Errors are wrapped with context
//   - Failed operations log warnings but may continue (best-effort)
//
// Example error with metadata:
//   errors.Wrap(err, errors.SharesOperationFailed).
//       WithMetadata("operation", "save_config").
//       WithMetadata("share_name", name)
//
// # Template System
//
// Templates are loaded at initialization:
//   - Embedded in binary via templates.go
//   - Fallback to default content if embedding fails
//   - Cached in memory for performance
//
// Template functions available:
//   - join: Joins string slice with separator (e.g., {{join .ValidUsers ", "}})
//
// # Integration Points
//
// The SMB manager integrates with:
//
//   - pkg/facl: For filesystem ACL management and user validation
//   - internal/services/samba: For Samba service lifecycle (start/stop/reload)
//   - internal/services/addc: For AD DC container management
//   - internal/services/domain: For domain join operations
//   - pkg/shares/api: REST and gRPC API handlers
//
// See also:
//   - types.go: Data structures (SMBGlobalConfig, SMBShareConfig)
//   - parser.go: Parsing existing smb.conf files
//   - service.go: Samba service management (ServiceManager)
//

package smb

import (
	"time"

	"github.com/stratastor/rodent/pkg/shares"
)

// SMBShareConfig represents configuration for an SMB share
type SMBShareConfig struct {
	// Base share configuration
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Path        string            `json:"path"`
	Enabled     bool              `json:"enabled"`
	Tags        map[string]string `json:"tags,omitempty"`

	// SMB specific configuration
	ReadOnly           bool     `json:"read_only"`
	Browsable          bool     `json:"browsable"`
	GuestOk            bool     `json:"guest_ok"`
	Public             bool     `json:"public"`
	ValidUsers         []string `json:"valid_users,omitempty"`
	InvalidUsers       []string `json:"invalid_users,omitempty"`
	ReadList           []string `json:"read_list,omitempty"`
	WriteList          []string `json:"write_list,omitempty"`
	AdminUsers         []string `json:"admin_users,omitempty"`
	CreateMask         string   `json:"create_mask,omitempty"`
	DirectoryMask      string   `json:"directory_mask,omitempty"`
	ForceMask          string   `json:"force_mask,omitempty"`
	ForceDirectoryMask string   `json:"force_directory_mask,omitempty"`
	InheritACLs        bool     `json:"inherit_acls"`
	MapACLInherit      bool     `json:"map_acl_inherit"`
	VetoFiles          []string `json:"veto_files,omitempty"`
	HideFiles          []string `json:"hide_files,omitempty"`
	FollowSymlinks     bool     `json:"follow_symlinks"`

	// Advanced configuration
	CustomParameters map[string]string `json:"custom_parameters,omitempty"`
}

// NewSMBShareConfig creates a new SMB share configuration with default values
func NewSMBShareConfig(name, path string) *SMBShareConfig {
	return &SMBShareConfig{
		Name:        name,
		Path:        path,
		Description: "SMB share managed by Rodent",
		Enabled:     true,
		Tags:        make(map[string]string),

		// Default SMB settings for better compatibility and security
		ReadOnly:       false,
		Browsable:      true,
		GuestOk:        false,
		Public:         false,
		InheritACLs:    true,
		MapACLInherit:  true,
		FollowSymlinks: true,
		CreateMask:     "0644",
		DirectoryMask:  "0755",

		// Initialize maps
		CustomParameters: map[string]string{
			"create mask":          "0644",
			"directory mask":       "0755",
			"vfs objects":          "acl_xattr",
			"map archive":          "no",
			"map readonly":         "no",
			"store dos attributes": "yes",
		},
	}
}

// SMBGlobalConfig represents global SMB configuration
type SMBGlobalConfig struct {
	WorkGroup               string            `json:"workgroup"`
	ServerString            string            `json:"server_string,omitempty"`
	SecurityMode            string            `json:"security_mode"`
	Realm                   string            `json:"realm,omitempty"`
	ServerRole              string            `json:"server_role,omitempty"`
	LogLevel                string            `json:"log_level,omitempty"`
	MaxLogSize              int               `json:"max_log_size,omitempty"`
	WinbindUseDefaultDomain bool              `json:"winbind_use_default_domain,omitempty"`
	WinbindOfflineLogon     bool              `json:"winbind_offline_logon,omitempty"`
	IDMapConfig             map[string]string `json:"idmap_config,omitempty"`
	KerberosMethod          string            `json:"kerberos_method,omitempty"`
	DedicatedKeytabFile     string            `json:"dedicated_keytab_file,omitempty"`

	// Advanced configuration
	CustomParameters map[string]string `json:"custom_parameters,omitempty"`
}

// NewSMBGlobalConfig creates a new global SMB configuration with default values
func NewSMBGlobalConfig() *SMBGlobalConfig {
	return &SMBGlobalConfig{
		WorkGroup:               "WORKGROUP",
		ServerString:            "Rodent SMB Server",
		SecurityMode:            "user",
		ServerRole:              "standalone server",
		LogLevel:                "1",
		MaxLogSize:              1000,
		WinbindUseDefaultDomain: false,
		WinbindOfflineLogon:     false,
		IDMapConfig:             make(map[string]string),
		CustomParameters: map[string]string{
			"map to guest": "Bad User",
			"unix charset": "UTF-8",
			"dns proxy":    "no",
		},
	}
}

// NewSMBGlobalConfigWithAD creates a new global SMB configuration with Active Directory settings
func NewSMBGlobalConfigWithAD(realm, workgroup string) *SMBGlobalConfig {
	config := NewSMBGlobalConfig()
	config.WorkGroup = workgroup
	config.Realm = realm
	config.SecurityMode = "ADS"
	config.ServerRole = "member server"
	config.WinbindUseDefaultDomain = true
	config.WinbindOfflineLogon = true
	config.KerberosMethod = "secrets and keytab"

	// Default idmap backend configuration for AD
	config.IDMapConfig = map[string]string{
		"idmap config *:backend":                 "tdb",
		"idmap config *:range":                   "100000-199999",
		"idmap config " + workgroup + ":backend": "rid",
		"idmap config " + workgroup + ":range":   "200000-999999",
	}

	// Additional AD-specific parameters
	config.CustomParameters["winbind enum users"] = "yes"
	config.CustomParameters["winbind enum groups"] = "yes"
	config.CustomParameters["winbind nested groups"] = "yes"
	config.CustomParameters["winbind refresh tickets"] = "yes"
	config.CustomParameters["winbind nss info"] = "rfc2307"
	config.CustomParameters["dedicated keytab file"] = "/etc/krb5.keytab"

	return config
}

// SMBSession represents an active SMB session
type SMBSession struct {
	SessionID     string    `json:"session_id"`
	Username      string    `json:"username"`
	GroupName     string    `json:"group_name"`
	RemoteMachine string    `json:"remote_machine"`
	ConnectedAt   time.Time `json:"connected_at"`
	Encryption    string    `json:"encryption"`
	Signing       string    `json:"signing"`
}

// SMBOpenFile represents an open file on an SMB share
type SMBOpenFile struct {
	Path         string    `json:"path"`
	ShareName    string    `json:"share_name"`
	Username     string    `json:"username"`
	OpenedAt     time.Time `json:"opened_at"`
	AccessMode   string    `json:"access_mode"`
	AccessRights string    `json:"access_rights"`
	OpenID       string    `json:"open_id"`
}

// SMBShareStats provides statistics about an SMB share
type SMBShareStats struct {
	ActiveSessions int                `json:"active_sessions"`
	OpenFiles      int                `json:"open_files"`
	Sessions       []SMBSession       `json:"sessions,omitempty"`
	Files          []SMBOpenFile      `json:"files,omitempty"`
	Status         shares.ShareStatus `json:"status"`
	ConfModified   time.Time          `json:"conf_modified"`
}

// SMBServiceStatus represents the status of the SMB service
type SMBServiceStatus struct {
	Running        bool      `json:"running"`
	PID            int       `json:"pid,omitempty"`
	Version        string    `json:"version,omitempty"`
	StartTime      time.Time `json:"start_time,omitempty"`
	ConfigFile     string    `json:"config_file,omitempty"`
	ActiveSessions int       `json:"active_sessions"`
	ActiveShares   int       `json:"active_shares"`
}

// SMBBulkUpdateConfig represents a configuration for bulk updating SMB shares
type SMBBulkUpdateConfig struct {
	// Filter criteria
	ShareNames []string          `json:"share_names,omitempty"` // Update specific shares by name
	Tags       map[string]string `json:"tags,omitempty"`        // Update shares with matching tags
	All        bool              `json:"all,omitempty"`         // Update all shares

	// Parameters to update
	Parameters map[string]string `json:"parameters"` // SMB parameters to set
}

// SMBBulkUpdateResult represents the result of a bulk update operation
type SMBBulkUpdateResult struct {
	ShareName string `json:"share_name"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}
