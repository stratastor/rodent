package smb

import (
	"time"
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
}

// SMBShareStats provides statistics about an SMB share
type SMBShareStats struct {
	ActiveSessions int           `json:"active_sessions"`
	OpenFiles      int           `json:"open_files"`
	Sessions       []SMBSession  `json:"sessions,omitempty"`
	Files          []SMBOpenFile `json:"files,omitempty"`
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
