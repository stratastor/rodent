// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package facl

import (
	"fmt"
	"strings"
)

// ACLType represents the type of ACL (POSIX or NFSv4)
type ACLType string

const (
	// ACLTypePOSIX represents POSIX ACLs (used by most Linux filesystems)
	ACLTypePOSIX ACLType = "posix"
	// ACLTypeNFSv4 represents NFSv4 ACLs (used by ZFS and NFS)
	ACLTypeNFSv4 ACLType = "nfsv4"
)

// PermissionType represents the type of permission in an ACL entry
type PermissionType string

const (
	// POSIX Permission Types
	PermReadData   PermissionType = "r" // Read data/list directory
	PermWriteData  PermissionType = "w" // Write data/create files
	PermExecute    PermissionType = "x" // Execute/search directory
	PermReadACL    PermissionType = "a" // Read ACL
	PermWriteACL   PermissionType = "A" // Write ACL
	PermChown      PermissionType = "C" // Change ownership
	PermReadAttrs  PermissionType = "R" // Read attributes
	PermWriteAttrs PermissionType = "W" // Write attributes

	// NFSv4 additional permissions
	PermDelete          PermissionType = "d" // Delete
	PermDeleteChild     PermissionType = "D" // Delete child
	PermReadNamedAttrs  PermissionType = "N" // Read named attributes
	PermWriteNamedAttrs PermissionType = "n" // Write named attributes
	PermSynchronize     PermissionType = "s" // Synchronize
)

// EntryType represents the type of an ACL entry
type EntryType string

const (
	// Entry Types
	EntryUser       EntryType = "user"
	EntryGroup      EntryType = "group"
	EntryOwner      EntryType = "owner"
	EntryOwnerGroup EntryType = "group::owner"
	EntryEveryone   EntryType = "everyone"
	EntryMask       EntryType = "mask"
	EntryOther      EntryType = "other"
)

// ACLFlags represents inheritance flags for an ACL entry (NFSv4)
type ACLFlags string

const (
	FlagInheritOnly        ACLFlags = "i" // Inherit only
	FlagNoPropagateInherit ACLFlags = "n" // No propagate inherit
	FlagInherit            ACLFlags = "f" // File inherit
	FlagDirectoryInherit   ACLFlags = "d" // Directory inherit
)

// AccessType represents whether an ACL entry allows or denies access
type AccessType string

const (
	AccessAllow AccessType = "allow"
	AccessDeny  AccessType = "deny"
)

// ACLEntry represents a single ACL entry
type ACLEntry struct {
	Type        EntryType        `json:"type"`
	Principal   string           `json:"principal,omitempty"` // User or group name
	Permissions []PermissionType `json:"permissions"`
	Flags       []ACLFlags       `json:"flags,omitempty"` // NFSv4 only
	Access      AccessType       `json:"access"`          // NFSv4 only
	IsDefault   bool             `json:"is_default"`      // Whether this is a default ACL
}

// ACLConfig holds the complete ACL configuration for a path
type ACLConfig struct {
	Path      string     `json:"path"      binding:"required"`
	Type      ACLType    `json:"type"      binding:"required"`
	Entries   []ACLEntry `json:"entries"   binding:"required"`
	Recursive bool       `json:"recursive"`
}

type ACLRemoveConfig struct {
	ACLConfig
	RemoveAllXattr bool `json:"remove_all_xattr"`
	RemoveDefault  bool `json:"remove_default"`
}

// ACLListConfig contains parameters for listing ACLs
type ACLListConfig struct {
	Path      string `json:"path"      binding:"required"`
	Recursive bool   `json:"recursive"`
}

// ACLListResult contains the result of listing ACLs
type ACLListResult struct {
	Path     string          `json:"path"`
	Type     ACLType         `json:"type"`
	Entries  []ACLEntry      `json:"entries"`
	Children []ACLListResult `json:"children,omitempty"`
}

// FormatPermissions formats permissions as a string
func FormatPermissions(perms []PermissionType) string {
	var sb strings.Builder
	for _, p := range perms {
		sb.WriteString(string(p))
	}
	return sb.String()
}

// FormatFlags formats flags as a string
func FormatFlags(flags []ACLFlags) string {
	var sb strings.Builder
	for _, f := range flags {
		sb.WriteString(string(f))
	}
	return sb.String()
}

// String formats an ACL entry as a string in getfacl/setfacl format
func (e ACLEntry) String() string {
	prefix := ""
	if e.IsDefault {
		prefix = "default:"
	}

	// Format permissions directly as rwx string instead of using FormatPermissions
	permsStr := ""
	for _, p := range e.Permissions {
		permsStr += string(p)
	}

	switch e.Type {
	case EntryUser:
		if e.Principal == "" {
			return fmt.Sprintf("%suser::%s", prefix, permsStr)
		}
		// Escape spaces with \040 for setfacl
		escapedPrincipal := strings.ReplaceAll(e.Principal, " ", "\\040")
		return fmt.Sprintf("%suser:%s:%s", prefix, escapedPrincipal, permsStr)
	case EntryGroup:
		if e.Principal == "" {
			return fmt.Sprintf("%sgroup::%s", prefix, permsStr)
		}
		// Escape spaces with \040 for setfacl
		escapedPrincipal := strings.ReplaceAll(e.Principal, " ", "\\040")
		return fmt.Sprintf("%sgroup:%s:%s", prefix, escapedPrincipal, permsStr)
	case EntryOwner:
		return fmt.Sprintf("%suser::%s", prefix, permsStr)
	case EntryOwnerGroup:
		return fmt.Sprintf("%sgroup::%s", prefix, permsStr)
	case EntryMask:
		return fmt.Sprintf("%smask::%s", prefix, permsStr)
	case EntryOther:
		return fmt.Sprintf("%sother::%s", prefix, permsStr)
	case EntryEveryone:
		// Fall back to other for everyone on POSIX
		return fmt.Sprintf("%sother::%s", prefix, permsStr)
	default:
		return ""
	}
}
