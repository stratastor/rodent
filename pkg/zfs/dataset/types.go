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

// Common configuration for dataset operations
type CreateConfig struct {
	Name       string            `json:"name" binding:"required"`
	Type       string            `json:"type" binding:"required"`
	Properties map[string]string `json:"properties,omitempty"`
	Parents    bool              `json:"parents"`    // Create parent datasets
	MountPoint string            `json:"mountpoint"` // Override mountpoint
}

// FilesystemConfig for filesystem-specific creation
type FilesystemConfig struct {
	Name        string            `json:"name" binding:"required"`
	Properties  map[string]string `json:"properties,omitempty"`
	Parents     bool              `json:"parents"`
	MountPoint  string            `json:"mountpoint"`
	Quota       string            `json:"quota,omitempty"`
	Reservation string            `json:"reservation,omitempty"`
}

// VolumeConfig for volume creation
type VolumeConfig struct {
	Name       string            `json:"name" binding:"required"`
	Size       string            `json:"size" binding:"required"`
	Properties map[string]string `json:"properties,omitempty"`
	Sparse     bool              `json:"sparse"`
	BlockSize  string            `json:"blocksize,omitempty"`
	Parents    bool              `json:"parents"`
}

type SnapshotConfig struct {
	Name       string            `json:"name" binding:"required"`
	Dataset    string            `json:"dataset" binding:"required"`
	Recursive  bool              `json:"recursive"`
	Properties map[string]string `json:"properties,omitempty"`
}

// SnapshotInfo represents ZFS snapshot information
type SnapshotInfo struct {
	Name       string              `json:"name"`
	Dataset    string              `json:"dataset"`
	CreateTXG  string              `json:"createtxg"`
	Properties map[string]Property `json:"properties"`
}

// SnapshotListOptions defines parameters for listing snapshots
type SnapshotListOptions struct {
	Dataset   string   `json:"dataset"`
	Recursive bool     `json:"recursive"`
	Sort      string   `json:"sort,omitempty"`
	Types     []string `json:"types,omitempty"`
}

// CloneConfig for clone creation
type CloneConfig struct {
	Name       string            `json:"name" binding:"required"`
	Snapshot   string            `json:"snapshot" binding:"required"`
	Properties map[string]string `json:"properties,omitempty"`
}

// BookmarkConfig for bookmark creation
type BookmarkConfig struct {
	Snapshot string `json:"snapshot" binding:"required"`
	Bookmark string `json:"bookmark" binding:"required"`
}

// RenameConfig for dataset renaming
type RenameConfig struct {
	Name         string `json:"name" binding:"required"`
	NewName      string `json:"new_name" binding:"required"`
	CreateParent bool   `json:"create_parent"`
	Force        bool   `json:"force"`
}

type RollbackConfig struct {
	Snapshot  string `json:"snapshot" binding:"required"`
	Force     bool   `json:"force"`
	Recursive bool   `json:"recursive"`
}

// ListOptions defines parameters for listing datasets
type ListOptions struct {
	Type       string   `json:"type,omitempty"`
	Recursive  bool     `json:"recursive"`
	Depth      int      `json:"depth,omitempty"`
	Properties []string `json:"properties,omitempty"`
	Sort       string   `json:"sort,omitempty"`
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

// MountConfig defines mount options
type MountConfig struct {
	Dataset    string   `json:"dataset" binding:"required"`
	MountPoint string   `json:"mountpoint,omitempty"`
	Options    []string `json:"options,omitempty"`
	Overlay    bool     `json:"overlay"`
	Force      bool     `json:"force"`
}
