// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package smb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/stratastor/rodent/internal/system/privilege"
	"github.com/stratastor/rodent/pkg/errors"
)

// SMBConfigParser parses an existing SMB configuration file
type SMBConfigParser struct {
	filePath string
	content  string
}

// NewSMBConfigParser creates a new SMB config parser
func NewSMBConfigParser(filePath string, fileOps privilege.FileOperations) (*SMBConfigParser, error) {
	var data []byte
	var err error

	// Read the file
	if fileOps != nil {
		// Use privileged operations
		data, err = fileOps.ReadFile(context.Background(), filePath)
	} else {
		// Fallback to direct file operations
		data, err = os.ReadFile(filePath)
	}

	if err != nil {
		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "read_config").
			WithMetadata("file", filePath)
	}

	return &SMBConfigParser{
		filePath: filePath,
		content:  string(data),
	}, nil
}

// ParseShares parses all share sections from the SMB config
func (p *SMBConfigParser) ParseShares() (map[string]*SMBShareConfig, error) {
	shares := make(map[string]*SMBShareConfig)

	// Regular expressions for parsing
	sectionRegex := regexp.MustCompile(`\[(.*?)\]`)
	paramRegex := regexp.MustCompile(`^\s*([^#;][^=]*?)\s*=\s*(.*?)\s*$`)

	// Split content into lines
	lines := strings.Split(p.content, "\n")

	var currentSection string
	var currentConfig *SMBShareConfig

	for _, line := range lines {
		// Check for section header
		if matches := sectionRegex.FindStringSubmatch(line); len(matches) > 1 {
			sectionName := matches[1]
			currentSection = sectionName

			// Skip global and special sections
			if currentSection == "global" ||
				currentSection == "homes" ||
				currentSection == "printers" ||
				currentSection == "print$" {
				currentConfig = nil
				continue
			}

			// Create new share config
			currentConfig = &SMBShareConfig{
				Name:             currentSection,
				Tags:             make(map[string]string),
				CustomParameters: make(map[string]string),
			}
			shares[currentSection] = currentConfig
			continue
		}

		// Skip if not in a valid share section
		if currentConfig == nil {
			continue
		}

		// Parse parameter = value lines
		if matches := paramRegex.FindStringSubmatch(line); len(matches) > 2 {
			param := strings.ToLower(strings.TrimSpace(matches[1]))
			value := strings.TrimSpace(matches[2])

			// Handle known parameters
			switch param {
			case "path":
				currentConfig.Path = value
			case "comment":
				currentConfig.Description = value
			case "read only":
				currentConfig.ReadOnly = (value == "yes" || value == "true" || value == "1")
			case "browsable", "browseable":
				currentConfig.Browsable = (value == "yes" || value == "true" || value == "1")
			case "guest ok":
				currentConfig.GuestOk = (value == "yes" || value == "true" || value == "1")
			case "public":
				currentConfig.Public = (value == "yes" || value == "true" || value == "1")
			case "valid users":
				currentConfig.ValidUsers = parseList(value)
			case "invalid users":
				currentConfig.InvalidUsers = parseList(value)
			case "read list":
				currentConfig.ReadList = parseList(value)
			case "write list":
				currentConfig.WriteList = parseList(value)
			case "admin users":
				currentConfig.AdminUsers = parseList(value)
			case "create mask":
				currentConfig.CreateMask = value
			case "directory mask":
				currentConfig.DirectoryMask = value
			case "force create mode":
				currentConfig.ForceMask = value
			case "force directory mode":
				currentConfig.ForceDirectoryMask = value
			case "inherit acls":
				currentConfig.InheritACLs = (value == "yes" || value == "true" || value == "1")
			case "map acl inherit":
				currentConfig.MapACLInherit = (value == "yes" || value == "true" || value == "1")
			case "veto files":
				currentConfig.VetoFiles = parseList(value)
			case "hide files":
				currentConfig.HideFiles = parseList(value)
			case "follow symlinks":
				currentConfig.FollowSymlinks = (value == "yes" || value == "true" || value == "1")
			default:
				// Store as custom parameter
				currentConfig.CustomParameters[param] = value
			}
		}
	}

	// Set defaults and validate shares
	for _, share := range shares {
		// Set default values if missing
		if share.Description == "" {
			share.Description = fmt.Sprintf("Imported share %s", share.Name)
		}
		share.Enabled = true
		share.Tags["imported"] = "true"
		share.Tags["imported_date"] = time.Now().Format(time.RFC3339)

		// Validate path
		if share.Path == "" {
			// Remove share if path is empty
			delete(shares, share.Name)
		}
	}

	return shares, nil
}

// ParseGlobalSection parses the global section from the SMB config
func (p *SMBConfigParser) ParseGlobalSection() (*SMBGlobalConfig, error) {
	globalConfig := NewSMBGlobalConfig()
	globalConfig.CustomParameters = make(map[string]string)

	// Regular expressions for parsing
	sectionRegex := regexp.MustCompile(`\[(.*?)\]`)
	paramRegex := regexp.MustCompile(`^\s*([^#;][^=]*?)\s*=\s*(.*?)\s*$`)

	// Split content into lines
	lines := strings.Split(p.content, "\n")

	inGlobalSection := false

	for _, line := range lines {
		// Check for section header
		if matches := sectionRegex.FindStringSubmatch(line); len(matches) > 1 {
			sectionName := matches[1]
			if sectionName == "global" {
				inGlobalSection = true
			} else {
				inGlobalSection = false
			}
			continue
		}

		// Skip if not in global section
		if !inGlobalSection {
			continue
		}

		// Parse parameter = value lines
		if matches := paramRegex.FindStringSubmatch(line); len(matches) > 2 {
			param := strings.ToLower(strings.TrimSpace(matches[1]))
			value := strings.TrimSpace(matches[2])

			// Handle known parameters
			switch param {
			case "workgroup":
				globalConfig.WorkGroup = value
			case "server string":
				globalConfig.ServerString = value
			case "security":
				globalConfig.SecurityMode = value
			case "realm":
				globalConfig.Realm = value
			case "server role":
				globalConfig.ServerRole = value
			case "log level":
				globalConfig.LogLevel = value
			case "max log size":
				// Try to parse as int but don't fail if it's not
				if size, err := fmt.Sscanf(value, "%d", &globalConfig.MaxLogSize); err != nil ||
					size == 0 {
					globalConfig.MaxLogSize = 1000
				}
			case "winbind use default domain":
				globalConfig.WinbindUseDefaultDomain = (value == "yes" || value == "true" || value == "1")
			case "winbind offline logon":
				globalConfig.WinbindOfflineLogon = (value == "yes" || value == "true" || value == "1")
			case "kerberos method":
				globalConfig.KerberosMethod = value
			case "dedicated keytab file":
				globalConfig.DedicatedKeytabFile = value
			default:
				// Check if it's an idmap config parameter
				if strings.HasPrefix(param, "idmap config") {
					if globalConfig.IDMapConfig == nil {
						globalConfig.IDMapConfig = make(map[string]string)
					}
					globalConfig.IDMapConfig[param] = value
				} else {
					// Store as custom parameter
					globalConfig.CustomParameters[param] = value
				}
			}
		}
	}

	return globalConfig, nil
}

// Helper function to parse comma-separated or space-separated lists
func parseList(value string) []string {
	// Split by commas first, then by spaces
	var result []string

	// Try comma-separated
	if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
	} else {
		// Try space-separated
		parts := strings.Fields(value)
		for _, part := range parts {
			if part != "" {
				result = append(result, part)
			}
		}
	}

	return result
}

// BackupConfigFile creates a backup of an existing config file
// If fileOps is nil, it uses direct file operations
func BackupConfigFile(filePath string, fileOps privilege.FileOperations) (string, error) {
	var exists bool
	var err error

	// Check if file exists
	if fileOps != nil {
		exists, err = fileOps.Exists(context.Background(), filePath)
		if err != nil {
			return "", err
		}
		if !exists {
			return "", nil
		}
	} else {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return "", nil
		}
	}

	// Create backup name with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s.bak", filePath, timestamp)

	// Read original file and write to backup
	if fileOps != nil {
		// Use privileged operations
		data, err := fileOps.ReadFile(context.Background(), filePath)
		if err != nil {
			return "", errors.Wrap(err, errors.SharesOperationFailed).
				WithMetadata("operation", "read_for_backup").
				WithMetadata("file", filePath)
		}

		if err := fileOps.WriteFile(context.Background(), backupPath, data, 0644); err != nil {
			return "", errors.Wrap(err, errors.SharesOperationFailed).
				WithMetadata("operation", "write_backup").
				WithMetadata("file", backupPath)
		}
	} else {
		// Use direct file operations as fallback
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", errors.Wrap(err, errors.SharesOperationFailed).
				WithMetadata("operation", "read_for_backup").
				WithMetadata("file", filePath)
		}

		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return "", errors.Wrap(err, errors.SharesOperationFailed).
				WithMetadata("operation", "write_backup").
				WithMetadata("file", backupPath)
		}
	}

	return backupPath, nil
}

// CreateShareConfigFromSection creates a share config file from parsed SMB section
func CreateShareConfigFromSection(configDir string, share *SMBShareConfig) error {
	// Marshal to JSON with nice formatting
	filePath := filepath.Join(configDir, share.Name+configFileExt)

	// Marshal the share config to JSON with indentation
	data, err := json.MarshalIndent(share, "", "  ")
	if err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "marshal_share_config").
			WithMetadata("name", share.Name)
	}

	// Write the JSON data to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "write_share_config").
			WithMetadata("name", share.Name)
	}

	return nil
}
