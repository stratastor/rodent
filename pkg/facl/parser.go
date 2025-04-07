// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package facl

import (
	"strconv"
	"strings"

	"github.com/stratastor/rodent/pkg/errors"
)

// parseGetfaclOutput parses the output of getfacl command
func parseGetfaclOutput(output string, aclType ACLType) ([]ACLEntry, error) {
	lines := strings.Split(output, "\n")
	var entries []ACLEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header lines
		if strings.HasPrefix(line, "#") {
			continue
		}

		var entry ACLEntry
		var err error

		// Handle default ACL entries
		isDefault := false
		if strings.HasPrefix(line, "default:") {
			isDefault = true
			line = strings.TrimPrefix(line, "default:")
		}

		// Parse based on ACL type
		if aclType == ACLTypePOSIX {
			entry, err = parsePOSIXACLEntry(line)
		} else {
			entry, err = parseNFSv4ACLEntry(line)
		}

		if err != nil {
			return nil, err
		}

		// Mark as default if needed
		if isDefault {
			entry.IsDefault = true
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// parsePOSIXACLEntry parses a POSIX ACL entry line
func parsePOSIXACLEntry(line string) (ACLEntry, error) {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return ACLEntry{}, errors.New(errors.FACLParseError,
			"Invalid POSIX ACL format").WithMetadata("line", line)
	}

	entry := ACLEntry{
		Access: AccessAllow, // POSIX ACLs are always "allow"
	}

	// Parse entry type and principal
	switch parts[0] {
	case "user":
		if len(parts) >= 3 && parts[1] != "" {
			entry.Type = EntryUser
			entry.Principal = unescapePrincipal(parts[1])
		} else {
			entry.Type = EntryOwner // user:: is the owner
		}
	case "group":
		if len(parts) >= 3 && parts[1] != "" {
			entry.Type = EntryGroup
			entry.Principal = unescapePrincipal(parts[1])
		} else {
			entry.Type = EntryOwnerGroup // group:: is the owning group
		}
	case "owner":
		entry.Type = EntryOwner
	case "mask":
		entry.Type = EntryMask
	case "other":
		entry.Type = EntryOther
	default:
		return ACLEntry{}, errors.New(errors.FACLParseError,
			"Unknown POSIX ACL entry type").WithMetadata("type", parts[0])
	}

	// Parse permissions
	var perms string
	if len(parts) >= 3 && parts[1] != "" {
		perms = parts[2]
	} else if len(parts) >= 2 {
		perms = parts[len(parts)-1]
	}

	entry.Permissions = parsePermissions(perms)

	return entry, nil
}

// parseNFSv4ACLEntry parses an NFSv4 ACL entry line
func parseNFSv4ACLEntry(line string) (ACLEntry, error) {
	// NFSv4 ACL format: principal:permissions:inheritance_flags:access_type
	parts := strings.Split(line, ":")
	if len(parts) < 4 {
		return ACLEntry{}, errors.New(errors.FACLParseError,
			"Invalid NFSv4 ACL format").WithMetadata("line", line)
	}

	entry := ACLEntry{}

	// Parse principal
	principal := parts[0]
	if strings.HasSuffix(principal, "@") {
		// Special principals
		switch principal {
		case "owner@":
			entry.Type = EntryOwner
		case "group@":
			entry.Type = EntryOwnerGroup
		case "everyone@":
			entry.Type = EntryEveryone
		default:
			// Named user or group
			if strings.HasPrefix(principal, "user:") {
				entry.Type = EntryUser
				entry.Principal = strings.TrimPrefix(principal, "user:")
			} else if strings.HasPrefix(principal, "group:") {
				entry.Type = EntryGroup
				entry.Principal = strings.TrimPrefix(principal, "group:")
			} else {
				return ACLEntry{}, errors.New(errors.FACLParseError,
					"Invalid NFSv4 principal").WithMetadata("principal", principal)
			}
		}
	} else {
		// Named user or group
		if strings.HasPrefix(principal, "user:") {
			entry.Type = EntryUser
			entry.Principal = strings.TrimPrefix(principal, "user:")
		} else if strings.HasPrefix(principal, "group:") {
			entry.Type = EntryGroup
			entry.Principal = strings.TrimPrefix(principal, "group:")
		} else {
			// Assume it's a user
			entry.Type = EntryUser
			entry.Principal = principal
		}
	}

	// Parse permissions
	entry.Permissions = parsePermissions(parts[1])

	// Parse inheritance flags
	if len(parts) > 2 && parts[2] != "" {
		entry.Flags = parseFlags(parts[2])
	}

	// Parse access type
	if len(parts) > 3 {
		switch parts[3] {
		case "allow":
			entry.Access = AccessAllow
		case "deny":
			entry.Access = AccessDeny
		default:
			entry.Access = AccessAllow // Default to allow
		}
	} else {
		entry.Access = AccessAllow // Default to allow
	}

	return entry, nil
}

// parsePermissions converts a permission string to a slice of PermissionType
func parsePermissions(perms string) []PermissionType {
	var result []PermissionType

	for _, p := range perms {
		switch p {
		case 'r':
			result = append(result, PermReadData)
		case 'w':
			result = append(result, PermWriteData)
		case 'x':
			result = append(result, PermExecute)
		case 'd':
			result = append(result, PermDelete)
		case 'D':
			result = append(result, PermDeleteChild)
		case 'a':
			result = append(result, PermReadACL)
		case 'A':
			result = append(result, PermWriteACL)
		case 'R':
			result = append(result, PermReadAttrs)
		case 'W':
			result = append(result, PermWriteAttrs)
		case 'C':
			result = append(result, PermChown)
		case 'N':
			result = append(result, PermReadNamedAttrs)
		case 'n':
			result = append(result, PermWriteNamedAttrs)
		case 's':
			result = append(result, PermSynchronize)
		}
	}

	return result
}

// parseFlags converts a flag string to a slice of ACLFlags
func parseFlags(flags string) []ACLFlags {
	var result []ACLFlags

	for _, f := range flags {
		switch f {
		case 'f':
			result = append(result, FlagInherit)
		case 'd':
			result = append(result, FlagDirectoryInherit)
		case 'i':
			result = append(result, FlagInheritOnly)
		case 'n':
			result = append(result, FlagNoPropagateInherit)
		}
	}

	return result
}

// unescapePrincipal handles escaped characters in principal names
func unescapePrincipal(principal string) string {
	// First, handle the \040 style octal escapes (e.g., domain\040users)
	result := ""
	i := 0
	for i < len(principal) {
		if i+3 < len(principal) && principal[i] == '\\' {
			// Try to parse as octal
			octalStr := principal[i+1 : i+4]
			if val, err := strconv.ParseInt(octalStr, 8, 32); err == nil {
				result += string(rune(val))
				i += 4
				continue
			}
		}
		result += string(principal[i])
		i++
	}

	return result
}

// Add this to pkg/facl/types.go
// escapePrincipal escapes special characters in principal names
func escapePrincipal(principal string) string {
	// For setfacl, just escape spaces with backslash
	return strings.ReplaceAll(principal, " ", "\\ ")
}
