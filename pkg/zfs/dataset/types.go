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
	Type       string            `json:"type" binding:"required"`
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
	Parsable                   bool `json:"parsable"` // -P  Print machine-parsable  verbose  information  about  the  created dataset
	Verbose                    bool `json:"verbose"`
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
	Parsable   bool `json:"parsable"` // -p  Print machine-parsable  verbose  information  about  the  created dataset
	Verbose    bool `json:"verbose"`
}

// VolumeConfig for volume creation
type VolumeConfig struct {
	NameConfig
	Size       string            `json:"size" binding:"required"` // -V size
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
	SnapName   string            `json:"snap_name" binding:"required"`
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
	CloneName  string            `json:"clone_name" binding:"required"`
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
	NewName    string `json:"new_name" binding:"required"`
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

type SetPropertyConfig struct {
	PropertyConfig
	Value string `json:"value" binding:"required"`
}

type PropertyConfig struct {
	NameConfig
	Property string `json:"property" binding:"required"`
}
