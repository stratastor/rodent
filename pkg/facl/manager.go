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

	// First get existing ACLs to ensure we preserve base entries
	existingResult, err := m.GetACL(ctx, ACLListConfig{
		Path: cfg.Path,
	})
	if err != nil {
		return errors.Wrap(err, errors.FACLReadError).
			WithMetadata("path", cfg.Path)
	}

	// Extract base entries that we need to preserve
	var baseEntries []ACLEntry
	for _, entry := range existingResult.Entries {
		// Keep only the basic required entries (owner, group, other)
		if (entry.Type == EntryOwner || entry.Type == EntryOwnerGroup ||
			entry.Type == EntryOther) && !entry.IsDefault {
			baseEntries = append(baseEntries, entry)
		}
	}

	// Check if provided entries already include required base entries
	hasOwner, hasOwnerGroup, hasOther := false, false, false
	for _, entry := range cfg.Entries {
		if !entry.IsDefault {
			if entry.Type == EntryOwner {
				hasOwner = true
			} else if entry.Type == EntryOwnerGroup {
				hasOwnerGroup = true
			} else if entry.Type == EntryOther {
				hasOther = true
			}
		}
	}

	// Merge base entries with provided entries
	var mergedEntries []ACLEntry
	// Add required base entries if not already provided
	if !hasOwner {
		for _, entry := range baseEntries {
			if entry.Type == EntryOwner {
				mergedEntries = append(mergedEntries, entry)
				break
			}
		}
	}
	if !hasOwnerGroup {
		for _, entry := range baseEntries {
			if entry.Type == EntryOwnerGroup {
				mergedEntries = append(mergedEntries, entry)
				break
			}
		}
	}
	if !hasOther {
		for _, entry := range baseEntries {
			if entry.Type == EntryOther {
				mergedEntries = append(mergedEntries, entry)
				break
			}
		}
	}

	// Add all provided entries
	mergedEntries = append(mergedEntries, cfg.Entries...)

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

	// For debugging: collect entries to log them
	var entryStrings []string

	// Write ACL entries to temporary file
	for _, entry := range mergedEntries {
		entryStr := entry.String()
		entryStrings = append(entryStrings, entryStr)
		// m.logger.Debug("Writing ACL entry",
		// 	"entry", entryStr,
		// 	"principal", entry.Principal,
		// 	"type", entry.Type)
		_, err := tempFile.WriteString(entryStr + "\n")
		if err != nil {
			return errors.Wrap(err, errors.FACLWriteError).
				WithMetadata("path", cfg.Path)
		}
	}
	tempFile.Close()

	// Log the entries for debugging
	m.logger.Debug("ACL entries being set",
		"path", cfg.Path,
		"entries", strings.Join(entryStrings, ", "),
		"temp_file", tempFile.Name())
	// Log the actual content of the file
	fileContent, err := os.ReadFile(tempFile.Name())
	if err == nil {
		m.logger.Debug("File content", "content", string(fileContent))
	}

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

	// Modify existing ACLs
	args = append(args, "-M")

	// Create temporary file with ACL entries
	tempFile, err := os.CreateTemp("", "rodent-acl-*.txt")
	if err != nil {
		return errors.Wrap(err, errors.FACLWriteError).
			WithMetadata("path", cfg.Path)
	}
	defer os.Remove(tempFile.Name())

	// Write ACL entries to temporary file
	for _, entry := range cfg.Entries {
		entryStr := entry.String()
		m.logger.Debug("Writing ACL entry for modify",
			"entry", entryStr,
			"principal", entry.Principal,
			"type", entry.Type)
		_, err := tempFile.WriteString(entryStr + "\n")
		if err != nil {
			return errors.Wrap(err, errors.FACLWriteError).
				WithMetadata("path", cfg.Path)
		}
	}
	tempFile.Close()

	args = append(args, tempFile.Name())
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
func (m *ACLManager) RemoveACL(ctx context.Context, cfg ACLRemoveConfig) error {
	// Validate path
	if err := validatePath(cfg.Path); err != nil {
		return err
	}

	args := []string{}

	if !cfg.RemoveAllXattr {

		if cfg.RemoveDefault {
			args = append(args, "-k") // Remove default ACLs
		}

		if cfg.Recursive {
			args = append(args, "-R") // Apply recursively
		}

		// Validate entries
		if !cfg.RemoveDefault && len(cfg.Entries) == 0 {
			return errors.New(errors.FACLInvalidInput, "No ACL entries provided")
		}
		if len(cfg.Entries) > 0 {

			// Use -X for removing ACLs with a file input
			args = append(args, "-X")

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
				prefix := ""
				if entry.IsDefault {
					prefix = "default:"
				}
				switch entry.Type {
				case EntryUser:
					if entry.Principal == "" {
						// Can't remove base user entry
						continue
					}
					// Escape spaces with \040 for setfacl
					escapedPrincipal := strings.ReplaceAll(entry.Principal, " ", "\\040")
					entryStr = fmt.Sprintf("%suser:%s", prefix, escapedPrincipal)
				case EntryGroup:
					if entry.Principal == "" {
						// Can't remove base group entry
						continue
					}
					// Escape spaces with \040 for setfacl
					escapedPrincipal := strings.ReplaceAll(entry.Principal, " ", "\\040")
					entryStr = fmt.Sprintf("%sgroup:%s", prefix, escapedPrincipal)
				default:
					// Can't remove base entries
					continue
				}

				_, err := tempFile.WriteString(entryStr + "\n")
				if err != nil {
					return errors.Wrap(err, errors.FACLWriteError).
						WithMetadata("path", cfg.Path)
				}
			}
			tempFile.Close()

			args = append(args, tempFile.Name())
		}
	} else if cfg.RemoveAllXattr {
		args = append(args, "-b") // Remove all ACLs
	}

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

		// TODO: "\\" may not always be the correct filter
		// TODO: Check principal regardless of default domain
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

	// Check if we're in testing mode
	if os.Getenv("RODENT_TESTING") != "" {
		// For testing, we'll just assume it's ZFS
		return true, nil
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
		// use errors.New instead of errors.Wrap with nil err
		return errors.New(errors.FACLUnsupportedFS,
			"Path is not on a ZFS filesystem").WithMetadata("path", path)
	}

	return nil
}
