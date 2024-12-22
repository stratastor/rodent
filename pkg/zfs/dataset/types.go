// pkg/zfs/dataset/types.go

package dataset

// Dataset represents a ZFS dataset (filesystem, volume, or snapshot)
type Dataset struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Pool       string              `json:"pool"`
	CreateTXG  string              `json:"createtxg"`
	Properties map[string]Property `json:"properties"`
}

// Property represents a dataset property
type Property struct {
	Value  interface{} `json:"value"`
	Source Source      `json:"source"`
}

// Source indicates property value origin
type Source struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// Configs for different operations
type FilesystemConfig struct {
	Name       string            `json:"name" binding:"required"`
	Properties map[string]string `json:"properties"`
	Parents    bool              `json:"parents"`
	MountPoint string            `json:"mountpoint"`
}

type VolumeConfig struct {
	Name       string            `json:"name" binding:"required"`
	Size       string            `json:"size" binding:"required"`
	Properties map[string]string `json:"properties"`
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

type CloneConfig struct {
	Name       string            `json:"name" binding:"required"`
	Snapshot   string            `json:"snapshot" binding:"required"`
	Properties map[string]string `json:"properties"`
}

type RenameConfig struct {
	NewName      string `json:"new_name" binding:"required"`
	CreateParent bool   `json:"create_parent"`
	Force        bool   `json:"force"`
}

type RollbackConfig struct {
	Snapshot  string `json:"snapshot" binding:"required"`
	Force     bool   `json:"force"`
	Recursive bool   `json:"recursive"`
}

type MountConfig struct {
	Dataset    string   `json:"dataset" binding:"required"`
	MountPoint string   `json:"mountpoint,omitempty"`
	Options    []string `json:"options,omitempty"`
}
