// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package privilege

// Config contains configuration for the privilege operations module
type Config struct {
	// AllowedPaths defines paths that can be accessed with sudo
	AllowedPaths []string `yaml:"allowed_paths" json:"allowed_paths"`
	
	// AllowedCommands defines commands that can be executed with sudo
	AllowedCommands []string `yaml:"allowed_commands" json:"allowed_commands"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		AllowedPaths: []string{
			"/etc/samba/smb.conf",
			"/etc/samba/conf.d",
			"/etc/hosts",
			"/etc/resolv.conf",
			"/etc/krb5.conf",
		},
		AllowedCommands: []string{
			"smbcontrol",
			"smbstatus",
			"systemctl",
		},
	}
}