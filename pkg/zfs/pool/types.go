// pkg/zfs/pool/types.go

package pool

// PoolStatus represents the full status of a ZPool
type PoolStatus struct {
	OutputVersion struct {
		Command   string `json:"command"`
		VersMajor int    `json:"vers_major"`
		VersMinor int    `json:"vers_minor"`
	} `json:"output_version"`
	Pools map[string]Pool `json:"pools"`
}

// Pool represents a ZFS storage pool
type Pool struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	State      string              `json:"state"`
	GUID       string              `json:"pool_guid"`
	HostID     string              `json:"hostid,omitempty"`
	Hostname   string              `json:"hostname,omitempty"`
	TXG        string              `json:"txg"`
	SPAVersion string              `json:"spa_version"`
	ZPLVersion string              `json:"zpl_version"`
	Properties map[string]Property `json:"properties"`
	VDevs      []VDev              `json:"vdevs"`
}

// Property represents a pool property with source information
type Property struct {
	Value  interface{} `json:"value"`
	Source Source      `json:"source"`
}

// Source indicates where the property value came from
type Source struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// VDev represents a virtual device in the pool
type VDev struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	State    string `json:"state"`
	Path     string `json:"path,omitempty"`
	GUID     string `json:"guid,omitempty"`
	Stats    Stats  `json:"stats,omitempty"`
	Children []VDev `json:"children,omitempty"`
}

// Stats holds VDev performance statistics
type Stats struct {
	ReadErrors     int64 `json:"read_errors"`
	WriteErrors    int64 `json:"write_errors"`
	ChecksumErrors int64 `json:"checksum_errors"`
	Operations     struct {
		Read     int64 `json:"read"`
		Write    int64 `json:"write"`
		Checksum int64 `json:"checksum"`
	} `json:"operations"`
}

// CreateConfig defines parameters for pool creation
type CreateConfig struct {
	Name       string
	VDevSpec   []VDevSpec
	Properties map[string]string
	Features   map[string]bool
	Force      bool
	MountPoint string
}

// VDevSpec defines virtual device configuration for pool creation
type VDevSpec struct {
	Type     string     // mirror, raidz, etc.
	Devices  []string   // Device paths
	Children []VDevSpec // For nested vdev configurations
}

// ImportConfig defines parameters for pool import
type ImportConfig struct {
	Name         string
	Dir          string // Search directory
	Properties   map[string]string
	Force        bool
	AllowDestroy bool
	Paths        []string // Device paths to search
}
