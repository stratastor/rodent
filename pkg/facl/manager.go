// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package facl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/ad"
	"github.com/stratastor/rodent/pkg/errors"
)

// BinGetfacl is the path to the getfacl binary
const BinGetfacl = "/usr/bin/getfacl"

// BinSetfacl is the path to the setfacl binary
const BinSetfacl = "/usr/bin/setfacl"

// ACLManager handles filesystem ACL operations
type ACLManager struct {
	logger   logger.Logger
	adClient *ad.ADClient
}

func (m *ACLManager) Close() {
	// No resources to close
	// Keeping for posterity
}

// NewACLManager creates a new ACL manager
func NewACLManager(logger logger.Logger, adClient *ad.ADClient) *ACLManager {
	return &ACLManager{
		logger:   logger,
		adClient: adClient,
	}
}

// GetACL retrieves the ACLs for a path
func (m *ACLManager) GetACL(ctx context.Context, cfg ACLListConfig) (ACLListResult, error) {
	// Validate path
	if err := validatePath(cfg.Path); err != nil {
		return ACLListResult{}, err
	}

	// Determine ACL type based on filesystem
	aclType, err := m.detectACLType(cfg.Path)
	if err != nil {
		return ACLListResult{}, err
	}

	args := []string{"-c", "-p"} // -c: omit comments, -p: show permissions only

	// Not supported yet
	// if aclType == ACLTypeNFSv4 {
	// 	args = append(args, "--nfs4") // Use NFSv4 ACLs
	// }

	args = append(args, cfg.Path)

	// Execute getfacl command
	out, err := command.ExecCommand(ctx, m.logger, BinGetfacl, args...)
	if err != nil {
		return ACLListResult{}, errors.Wrap(err, errors.FACLReadError).
			WithMetadata("path", cfg.Path)
	}

	// Parse getfacl output
	entries, err := parseGetfaclOutput(string(out), aclType)
	if err != nil {
		return ACLListResult{}, errors.Wrap(err, errors.FACLParseError).
			WithMetadata("path", cfg.Path)
	}

	result := ACLListResult{
		Path:    cfg.Path,
		Type:    aclType,
		Entries: entries,
	}

	// Handle recursive listing if requested
	if cfg.Recursive {
		fileInfo, err := os.Stat(cfg.Path)
		if err != nil {
			return result, errors.Wrap(err, errors.FACLReadError).
				WithMetadata("path", cfg.Path)
		}

		if fileInfo.IsDir() {
			files, err := os.ReadDir(cfg.Path)
			if err != nil {
				return result, errors.Wrap(err, errors.FACLReadError).
					WithMetadata("path", cfg.Path)
			}

			for _, file := range files {
				childPath := filepath.Join(cfg.Path, file.Name())
				childCfg := ACLListConfig{
					Path:      childPath,
					Recursive: true,
				}

				childResult, err := m.GetACL(ctx, childCfg)
				if err != nil {
					m.logger.Error("Failed to get ACL for child path",
						"path", childPath, "error", err)
					continue
				}

				result.Children = append(result.Children, childResult)
			}
		}
	}

	return result, nil
}

// SetACL sets ACLs for a path
func (m *ACLManager) SetACL(ctx context.Context, cfg ACLConfig) error {
	// Validate path
	if err := validatePath(cfg.Path); err != nil {
		return err
	}

	// Validate entries
	if len(cfg.Entries) == 0 {
		return errors.New(errors.FACLInvalidInput, "No ACL entries provided")
	}

	// Build setfacl arguments
	args := []string{}

	if cfg.Recursive {
		args = append(args, "-R") // Apply recursively
	}

	if cfg.Type == ACLTypeNFSv4 {
		// NFSv4 ACLs are not yet supported
		// args = append(args, "--nfs4") // Use NFSv4 ACLs
	}

	// Create temporary file with ACL entries
	tempFile, err := os.CreateTemp("", "rodent-acl-*.txt")
	if err != nil {
		return errors.Wrap(err, errors.FACLWriteError).
			WithMetadata("path", cfg.Path)
	}
	defer os.Remove(tempFile.Name())

	// Write ACL entries to temporary file
	for _, entry := range cfg.Entries {
		_, err := tempFile.WriteString(entry.String() + "\n")
		if err != nil {
			return errors.Wrap(err, errors.FACLWriteError).
				WithMetadata("path", cfg.Path)
		}
	}
	tempFile.Close()

	// Use --set to replace all ACLs
	args = append(args, "--set-file="+tempFile.Name())
	args = append(args, cfg.Path)

	// Execute setfacl command
	out, err := command.ExecCommand(ctx, m.logger, BinSetfacl, args...)
	if err != nil {
		return errors.Wrap(err, errors.FACLWriteError).
			WithMetadata("path", cfg.Path).
			WithMetadata("output", string(out))
	}

	return nil
}

// ModifyACL modifies specific ACL entries
func (m *ACLManager) ModifyACL(ctx context.Context, cfg ACLConfig) error {
	// Validate path
	if err := validatePath(cfg.Path); err != nil {
		return err
	}

	// Validate entries
	if len(cfg.Entries) == 0 {
		return errors.New(errors.FACLInvalidInput, "No ACL entries provided")
	}

	// Build setfacl arguments
	args := []string{}

	if cfg.Recursive {
		args = append(args, "-R") // Apply recursively
	}

	if cfg.Type == ACLTypeNFSv4 {
		// NFSv4 ACLs are not yet supported
		// args = append(args, "--nfs4") // Use NFSv4 ACLs
	}

	// Add --modify flag to modify existing ACLs
	args = append(args, "-m")

	// Create temporary file with ACL entries
	tempFile, err := os.CreateTemp("", "rodent-acl-*.txt")
	if err != nil {
		return errors.Wrap(err, errors.FACLWriteError).
			WithMetadata("path", cfg.Path)
	}
	defer os.Remove(tempFile.Name())

	// Write ACL entries to temporary file
	for _, entry := range cfg.Entries {
		_, err := tempFile.WriteString(entry.String() + "\n")
		if err != nil {
			return errors.Wrap(err, errors.FACLWriteError).
				WithMetadata("path", cfg.Path)
		}
	}
	tempFile.Close()

	args = append(args, "--file="+tempFile.Name())
	args = append(args, cfg.Path)

	// Execute setfacl command
	out, err := command.ExecCommand(ctx, m.logger, BinSetfacl, args...)
	if err != nil {
		return errors.Wrap(err, errors.FACLWriteError).
			WithMetadata("path", cfg.Path).
			WithMetadata("output", string(out))
	}

	return nil
}

// RemoveACL removes specific ACL entries
func (m *ACLManager) RemoveACL(ctx context.Context, cfg ACLConfig) error {
	// Validate path
	if err := validatePath(cfg.Path); err != nil {
		return err
	}

	// Validate entries
	if len(cfg.Entries) == 0 {
		return errors.New(errors.FACLInvalidInput, "No ACL entries provided")
	}

	// Build setfacl arguments
	args := []string{}

	if cfg.Recursive {
		args = append(args, "-R") // Apply recursively
	}

	if cfg.Type == ACLTypeNFSv4 {
		// NFSv4 ACLs are not yet supported
		// args = append(args, "--nfs4") // Use NFSv4 ACLs
	}

	// Add --remove flag to remove ACLs
	args = append(args, "-x")

	// Create temporary file with ACL entries to remove
	tempFile, err := os.CreateTemp("", "rodent-acl-*.txt")
	if err != nil {
		return errors.Wrap(err, errors.FACLWriteError).
			WithMetadata("path", cfg.Path)
	}
	defer os.Remove(tempFile.Name())

	// Write ACL entries to temporary file
	for _, entry := range cfg.Entries {
		// For removal, we only need the type and principal
		var entryStr string
		switch entry.Type {
		case EntryUser:
			if entry.Principal == "" {
				entryStr = "user::"
			} else {
				entryStr = fmt.Sprintf("user:%s", entry.Principal)
			}
		case EntryGroup:
			if entry.Principal == "" {
				entryStr = "group::"
			} else {
				entryStr = fmt.Sprintf("group:%s", entry.Principal)
			}
		case EntryOwner:
			entryStr = "owner::"
		case EntryOwnerGroup:
			entryStr = "group::"
		case EntryMask:
			entryStr = "mask::"
		case EntryOther:
			entryStr = "other::"
		case EntryEveryone:
			entryStr = "everyone@"
		}

		_, err := tempFile.WriteString(entryStr + "\n")
		if err != nil {
			return errors.Wrap(err, errors.FACLWriteError).
				WithMetadata("path", cfg.Path)
		}
	}
	tempFile.Close()

	args = append(args, "--file="+tempFile.Name())
	args = append(args, cfg.Path)

	// Execute setfacl command
	out, err := command.ExecCommand(ctx, m.logger, BinSetfacl, args...)
	if err != nil {
		return errors.Wrap(err, errors.FACLWriteError).
			WithMetadata("path", cfg.Path).
			WithMetadata("output", string(out))
	}

	return nil
}

// ResolveADUsers resolves Active Directory users/groups for ACL entries
func (m *ACLManager) ResolveADUsers(ctx context.Context, entries []ACLEntry) ([]ACLEntry, error) {
	if m.adClient == nil {
		return entries, nil // No AD client, return original entries
	}

	result := make([]ACLEntry, len(entries))
	copy(result, entries)

	for i, entry := range result {
		if entry.Type != EntryUser && entry.Type != EntryGroup {
			continue
		}

		if entry.Principal == "" {
			continue
		}

		// Check if it's a domain user/group (DOMAIN\User format)
		if strings.Contains(entry.Principal, "\\") {
			parts := strings.SplitN(entry.Principal, "\\", 2)
			if len(parts) != 2 {
				continue
			}

			// domain := parts[0]
			name := parts[1]

			// Verify user/group exists in AD
			var exists bool
			var err error

			if entry.Type == EntryUser {
				_, err = m.adClient.SearchUser(name)
				exists = (err == nil)
			} else {
				_, err = m.adClient.SearchGroup(name)
				exists = (err == nil)
			}

			if !exists {
				return nil, errors.New(errors.FACLInvalidPrincipal,
					fmt.Sprintf("AD principal not found: %s", entry.Principal))
			}

			// Keep the original format for now
			result[i].Principal = entry.Principal
		}
	}

	return result, nil
}

// detectACLType determines the ACL type supported by the filesystem
func (m *ACLManager) detectACLType(path string) (ACLType, error) {
	// Check if the path is on a ZFS filesystem
	_, err := isZFSPath(path)
	if err != nil {
		// Don't fail on filesystem detection error, just log it
		m.logger.Warn("Failed to detect filesystem type, defaulting to POSIX ACLs",
			"path", path, "error", err)
		return ACLTypePOSIX, nil
	}

	// Default to POSIX ACLs
	// On Linux, we only support POSIX ACLs regardless of filesystem type
	// NFSv4 ACLs are not yet supported on Linux ZFS according to documentation
	return ACLTypePOSIX, nil
}

// isZFSPath checks if a path is on a ZFS filesystem
func isZFSPath(path string) (bool, error) {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}

	// Check if file exists
	_, err = os.Stat(absPath)
	if err != nil {
		return false, err
	}

	// Execute mount command to get filesystem type
	out, err := command.ExecCommand(context.Background(), common.Log,
		"/bin/mount", "-l")
	if err != nil {
		return false, err
	}

	// Parse mount output to find filesystem type
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		// Check if the mount point is a parent of the path
		mountPoint := fields[2]
		if strings.HasPrefix(absPath, mountPoint) {
			// Check if type is zfs
			typeInfo := fields[4]
			if strings.Contains(typeInfo, "zfs") {
				return true, nil
			}
		}
	}

	return false, nil
}

// validatePath validates the path for ACL operations
func validatePath(path string) error {
	// Check if path is empty
	if path == "" {
		return errors.New(errors.FACLInvalidInput, "Path cannot be empty")
	}

	// Check if path contains dangerous characters
	if strings.ContainsAny(path, "&|><$`\\[];{}") {
		return errors.New(errors.FACLInvalidInput,
			"Path contains invalid characters")
	}

	// Check if path exists
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New(errors.FACLPathNotFound,
				"Path does not exist").WithMetadata("path", path)
		}
		return errors.Wrap(err, errors.FACLReadError).
			WithMetadata("path", path)
	}

	isZFS, err := isZFSPath(path)
	if err != nil {
		return errors.Wrap(err, errors.FACLReadError).
			WithMetadata("path", path)
	}

	if !isZFS {
		return errors.Wrap(err, errors.FACLUnsupportedFS).
			WithMetadata("path", path)
	}

	return nil
}
