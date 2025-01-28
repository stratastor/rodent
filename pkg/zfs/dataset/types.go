/*
 * Copyright 2024-2025 Raamsri Kumar <raam@tinkershack.in>
 * Copyright 2024-2025 The StrataSTOR Authors and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dataset

// Dataset represents a ZFS dataset (filesystem, volume, snapshot, bookmark and clone)
type Dataset struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"` // "filesystem", "volume", "snapshot", "bookmark"
	Pool       string              `json:"pool"`
	CreateTXG  string              `json:"createtxg"`
	Origin     string              `json:"origin,omitempty"`    // For clones
	Used       string              `json:"used,omitempty"`      // Space usage
	Available  string              `json:"available,omitempty"` // Available space
	Properties map[string]Property `json:"properties"`
}

// Property represents a dataset property
type Property struct {
	Value  interface{} `json:"value"`
	Source Source      `json:"source"`
}

// Source indicates property value origin
type Source struct {
	Type string `json:"type"` // "local", "default", "inherited", etc.
	Data string `json:"data"` // Additional source info
}

type ListResult struct {
	Datasets map[string]Dataset `json:"datasets"`
}

type ListConfig struct {
	Name string `json:"name"`

	// -r  Recursively display any children of the dataset on the command line
	Recursive bool `json:"recursive"`

	// -d Recursively display any children of the dataset, limiting the recursion to depth
	// Overrides -r
	Depth uint `json:"depth"`

	// A comma-separated list of properties to display.
	// The property must be:
	// -   One of the properties described in the “Native Properties” section of zfsprops(7)
	// -   A user property
	// -   The value name to display the dataset name
	// -   The  value  space  to  display  space  usage  properties  on  file  systems  and volumes.  This is a shortcut for specifying
	// 		-o name,avail,used,usedsnap,usedds,usedrefreserv,usedchild -t filesystem,volume.
	Properties []string `json:"properties"` // -o property[,property...]

	// -p  Display parsable (exact) property values
	Parsable bool `json:"parsable"`

	// -t type
	// A  comma-separated  list of types to display, where type is one of filesystem, snapshot, volume, bookmark, or all.
	// For example, specifying -t snapshot displays only snapshots.
	// fs, snap, or vol can be used as aliases for filesystem, snapshot, or volume.
	Type string `json:"type"`
}

// Common configuration for dataset operations
type CreateConfig struct {
	NameConfig
	Type       string            `json:"type"                 binding:"required"`
	Properties map[string]string `json:"properties,omitempty"`
	Parents    bool              `json:"parents"`    // Create parent datasets
	MountPoint string            `json:"mountpoint"` // Override mountpoint
}

type DestroyConfig struct {
	NameConfig
	RecursiveDestroyDependents bool `json:"recursive_destroy_dependents"` // Recursively destroy  all  dependents,  including  cloned file systems outside the target hierarchy
	RecursiveDestroyChildren   bool `json:"recursive_destroy_children"`   // Recursively destroy all children
	Force                      bool `json:"force"`
	DryRun                     bool `json:"dry_run"`
}

// DestroyResult represents the output of a destroy operation
type DestroyResult struct {
	Destroyed []string `json:"destroyed"` // List of datasets that would be/were destroyed
}

// CreateResult represents the output of a create operation
type CreateResult struct {
	Created    string            `json:"created"`              // Name of the created dataset
	Properties map[string]string `json:"properties,omitempty"` // Name and value KV of properties
}

// FilesystemConfig for filesystem-specific creation
type FilesystemConfig struct {
	NameConfig
	Properties map[string]string `json:"properties,omitempty"`

	// -P Creates all the non-existing parent datasets
	// ZFS doesn't error when -p is used against a dataset that already exists. Otherwise, it will error out:
	// Error: cannot create 'tank/fs': dataset already exists
	Parents bool `json:"parents"`

	DoNotMount bool `json:"do_not_mount"` // -u  Do not mount the newly created file system
	DryRun     bool `json:"dry_run"`
}

// VolumeConfig for volume creation
type VolumeConfig struct {
	NameConfig
	Size       string            `json:"size"                 binding:"required"` // -V size
	Properties map[string]string `json:"properties,omitempty"`
	Sparse     bool              `json:"sparse"` //  -s  Creates a sparse volume with no reservation
	// -b blocksize
	// Equivalent to -o volblocksize=blocksize.  If this option is specified in conjunction with -o volblocksize, the  resulting  be‐
	// havior is undefined
	BlockSize string `json:"blocksize,omitempty"`
	Parents   bool   `json:"parents"`
	DryRun    bool   `json:"dry_run"`
	Parsable  bool   `json:"parsable"` // -p  Print machine-parsable  verbose  information  about  the  created dataset
	Verbose   bool   `json:"verbose"`
}

type SnapshotConfig struct {
	NameConfig
	SnapName   string            `json:"snap_name"            binding:"required"`
	Recursive  bool              `json:"recursive"`
	Properties map[string]string `json:"properties,omitempty"`
}

// SnapshotInfo represents ZFS snapshot information
type SnapshotInfo struct {
	NameConfig
	Dataset    string              `json:"dataset"`
	CreateTXG  string              `json:"createtxg"`
	Properties map[string]Property `json:"properties"`
}

// CloneConfig for clone creation
type CloneConfig struct {
	NameConfig
	CloneName  string            `json:"clone_name"           binding:"required"`
	Properties map[string]string `json:"properties,omitempty"`
	Parents    bool              `json:"parents,omitempty"`
}

// BookmarkConfig for bookmark creation
type BookmarkConfig struct {
	NameConfig
	BookmarkName string `json:"bookmark_name" binding:"required"`
}

// RenameConfig for dataset renaming
type RenameConfig struct {
	NameConfig
	NewName    string `json:"new_name"     binding:"required"`
	Parents    bool   `json:"parents"`
	Force      bool   `json:"force"`
	Recursive  bool   `json:"recursive"`
	DoNotMount bool   `json:"do_not_mount"`
}

type RollbackConfig struct {
	NameConfig

	// The  -rR  options  do  not  recursively destroy the child snapshots of a recursive snapshot.
	// Only direct snapshots of the specified filesystem are destroyed by either of these options.

	// -r Destroy any snapshots and bookmarks more recent than the one specified
	DestroyRecent bool `json:"destroy_recent"`

	// -R Destroy any more recent snapshots and bookmarks, as well as any clones of those snapshots
	DestroyRecentClones bool `json:"destroy_recent_clones"`

	// -f Used with the -R option to force an unmount of any clone file systems that are to be destroyed
	ForceUnmount bool `json:"force_unmount"`
}

// DatasetStatus represents dataset status information
type DatasetStatus struct {
	Name       string `json:"name"`
	Available  string `json:"available"`
	Used       string `json:"used"`
	Referenced string `json:"referenced"`
	Mounted    bool   `json:"mounted"`
	Origin     string `json:"origin,omitempty"`
}

// TODO: implement -l and -a options
// MountConfig defines mount options
type MountConfig struct {
	NameConfig
	TempMountPoint string   `json:"temp_mountpoint,omitempty"`
	Recursive      bool     `json:"recursive"` // -R Mount the specified filesystems  along  with  all  of its children
	Options        []string `json:"options,omitempty"`
	Overlay        bool     `json:"overlay"` // -O  Overlay mount
	Force          bool     `json:"force"`   // -f Force mount
	Verbose        bool     `json:"verbose"` // -v Report mount progress
}

type UnmountConfig struct {
	NameConfig
	Force bool `json:"force"`
}

type NameConfig struct {
	Name string `json:"name" binding:"required"`
}

type NamesConfig struct {
	Names []string `json:"names" binding:"required"`
}

type SetPropertyConfig struct {
	PropertyConfig
	Value string `json:"value" binding:"required"`
}

type PropertyConfig struct {
	NameConfig
	Property string `json:"property" binding:"required"`
}

type InheritConfig struct {
	NamesConfig
	Property  string `json:"property"  binding:"required"`
	Recursive bool   `json:"recursive"`
	Revert    bool   `json:"revert"`
}

// DiffConfig defines configuration for ZFS diff operation
type DiffConfig struct {
	NamesConfig      // Embed NamesConfig to use Names slice
	Timestamps  bool `json:"timestamps,omitempty"` // Always true for API
	Types       bool `json:"types,omitempty"`      // Always true for API
	FileTypes   bool `json:"file_types,omitempty"` // Always true for API (-F)
}

// DiffEntry represents a single change detected by ZFS diff
type DiffEntry struct {
	Timestamp  float64 `json:"timestamp"`          // Unix timestamp with nanoseconds
	ChangeType string  `json:"change_type"`        // +, -, M, R
	FileType   string  `json:"file_type"`          // F (file), / (directory), @ (symlink), etc.
	Path       string  `json:"path"`               // Original path
	NewPath    string  `json:"new_path,omitempty"` // For renames (R)
}

// DiffResult represents the output of a ZFS diff operation
type DiffResult struct {
	Changes []DiffEntry `json:"changes"`
}

// Permission represents a ZFS permission
type Permission struct {
	Name string `json:"name"`
	Type string `json:"type"` // "subcommand", "other", "property"
	Note string `json:"note,omitempty"`
}

// ZFS permissions catalog
var ZFSPermissions = map[string]Permission{
	// Subcommand permissions
	"allow": {
		Name: "allow",
		Type: "subcommand",
		Note: "Add any permission to the permission set",
	},
	"clone":   {Name: "clone", Type: "subcommand", Note: "Clone the specified snapshot"},
	"create":  {Name: "create", Type: "subcommand", Note: "Create descendent datasets"},
	"destroy": {Name: "destroy", Type: "subcommand", Note: "Destroy the specified dataset"},
	"diff": {
		Name: "diff",
		Type: "subcommand",
		Note: "Report differences between snapshot and active dataset",
	},
	"hold": {
		Name: "hold",
		Type: "subcommand",
		Note: "Place a user reference on the specified snapshot",
	},
	"mount":   {Name: "mount", Type: "subcommand", Note: "Mount the specified dataset"},
	"promote": {Name: "promote", Type: "subcommand", Note: "Promote the specified clone"},
	"receive": {
		Name: "receive",
		Type: "subcommand",
		Note: "Create a snapshot with the specified data",
	},
	"release": {
		Name: "release",
		Type: "subcommand",
		Note: "Release a user reference from the specified snapshot",
	},
	"rename":   {Name: "rename", Type: "subcommand", Note: "Rename the specified dataset"},
	"rollback": {Name: "rollback", Type: "subcommand", Note: "Roll back the specified snapshot"},
	"send": {
		Name: "send",
		Type: "subcommand",
		Note: "Generate a send stream for the specified snapshot",
	},
	"share": {Name: "share", Type: "subcommand", Note: "Share the specified dataset"},
	"snapshot": {
		Name: "snapshot",
		Type: "subcommand",
		Note: "Create a snapshot with the given name",
	},
	"unmount": {Name: "unmount", Type: "subcommand", Note: "Unmount the specified dataset"},
	"unshare": {Name: "unshare", Type: "subcommand", Note: "Unshare the specified dataset"},

	// Other permissions
	"groupquota": {Name: "groupquota", Type: "other", Note: "Allow manipulation of group quotas"},
	"groupused":  {Name: "groupused", Type: "other", Note: "Allow reading of group space usage"},
	"userprop":   {Name: "userprop", Type: "other", Note: "Permission to change user properties"},
	"userquota":  {Name: "userquota", Type: "other", Note: "Allow manipulation of user quotas"},
	"userused":   {Name: "userused", Type: "other", Note: "Allow reading of user space usage"},

	// Property permissions (all natively supported properties)
	"aclinherit": {
		Name: "aclinherit",
		Type: "property",
		Note: "Access control list inheritance",
	},
	"aclmode":        {Name: "aclmode", Type: "property", Note: "Access control list mode"},
	"atime":          {Name: "atime", Type: "property", Note: "Update access time on read"},
	"canmount":       {Name: "canmount", Type: "property", Note: "If filesystem can be mounted"},
	"checksum":       {Name: "checksum", Type: "property", Note: "Data checksum"},
	"compression":    {Name: "compression", Type: "property", Note: "Data compression"},
	"copies":         {Name: "copies", Type: "property", Note: "Number of copies"},
	"dedup":          {Name: "dedup", Type: "property", Note: "Deduplication"},
	"devices":        {Name: "devices", Type: "property", Note: "Device files can be opened"},
	"exec":           {Name: "exec", Type: "property", Note: "Execution of processes allowed"},
	"mountpoint":     {Name: "mountpoint", Type: "property", Note: "Mountpoint"},
	"quota":          {Name: "quota", Type: "property", Note: "Maximum size of dataset"},
	"readonly":       {Name: "readonly", Type: "property", Note: "Read-only status"},
	"recordsize":     {Name: "recordsize", Type: "property", Note: "Suggested block size"},
	"refquota":       {Name: "refquota", Type: "property", Note: "Maximum size of dataset"},
	"refreservation": {Name: "refreservation", Type: "property", Note: "Minimum guaranteed space"},
	"reservation":    {Name: "reservation", Type: "property", Note: "Minimum guaranteed space"},
	"setuid":         {Name: "setuid", Type: "property", Note: "Respect setuid bit"},
	"snapdir":        {Name: "snapdir", Type: "property", Note: "If .zfs directory is visible"},
	"sync":           {Name: "sync", Type: "property", Note: "Sync write behavior"},
	"volsize":        {Name: "volsize", Type: "property", Note: "Volume logical size"},
}

// AllowConfig defines configuration for ZFS allow operation
type AllowConfig struct {
	NameConfig
	Permissions []string `json:"permissions"`          // Individual permissions or permission sets
	Users       []string `json:"users,omitempty"`      // Users to grant permissions (mutually exclusive with Groups and Everyone)
	Groups      []string `json:"groups,omitempty"`     // Groups to grant permissions (mutually exclusive with Users and Everyone)
	Everyone    bool     `json:"everyone,omitempty"`   // Grant to everyone (mutually exclusive with Users and Groups)
	Create      bool     `json:"create,omitempty"`     // Create time permissions
	Local       bool     `json:"local,omitempty"`      // Local permissions only
	Descendent  bool     `json:"descendent,omitempty"` // Descendent permissions
	SetName     string   `json:"set_name,omitempty"`   // Permission set name (must start with @)
}

// UnallowConfig defines configuration for ZFS unallow operation
type UnallowConfig struct {
	NameConfig
	Permissions []string `json:"permissions,omitempty"` // Individual permissions or permission sets to remove
	Users       []string `json:"users,omitempty"`       // Users to revoke from
	Groups      []string `json:"groups,omitempty"`      // Groups to revoke from
	Everyone    bool     `json:"everyone,omitempty"`    // Revoke from everyone
	Create      bool     `json:"create,omitempty"`      // Remove create time permissions
	Local       bool     `json:"local,omitempty"`       // Remove local permissions
	Descendent  bool     `json:"descendent,omitempty"`  // Remove descendent permissions
	Recursive   bool     `json:"recursive,omitempty"`   // Apply recursively
	SetName     string   `json:"set_name,omitempty"`    // Permission set to remove
}

// AllowResult represents parsed output of zfs allow command
type AllowResult struct {
	PermissionSets  map[string][]string `json:"permission_sets,omitempty"`
	CreateTime      []string            `json:"create_time,omitempty"`
	Local           map[string][]string `json:"local,omitempty"`
	Descendent      map[string][]string `json:"descendent,omitempty"`
	LocalDescendent map[string][]string `json:"local_descendent,omitempty"`
}

// ShareConfig defines configuration for ZFS share operation
type ShareConfig struct {
	Name     string `json:"name"`
	All      bool   `json:"all"`       // -a: Share all available ZFS filesystems
	LoadKeys bool   `json:"load_keys"` // -l: Load keys for encrypted filesystems
}

// UnshareConfig defines configuration for ZFS unshare operation
type UnshareConfig struct {
	Name string `json:"name"`
	All  bool   `json:"all"` // -a: Unshare all shared ZFS filesystems
}
