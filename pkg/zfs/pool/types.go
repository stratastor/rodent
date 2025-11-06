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

package pool

// ListResult represents the output of zpool list/get commands
type ListResult struct {
	Pools map[string]Pool `json:"pools"`
}

// ImportablePoolsResult represents pools available for import
type ImportablePoolsResult struct {
	Pools []ImportablePool `json:"pools"`
}

// ImportablePool represents a pool that can be imported
type ImportablePool struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	State  string `json:"state"`
	Action string `json:"action"`
	Config string `json:"config"` // Raw config output
}

// PoolStatus represents the full status of a ZPool
type PoolStatus struct {
	Pools map[string]Pool `json:"pools"`
}

// Pool represents a ZFS storage pool
type Pool struct {
	Name       string              `json:"name"`
	Type       string              `json:"type,omitempty"`
	State      string              `json:"state"`
	GUID       string              `json:"pool_guid"`
	TXG        string              `json:"txg"`
	SPAVersion string              `json:"spa_version"`
	ZPLVersion string              `json:"zpl_version"`
	Properties map[string]Property `json:"properties,omitempty"`

	// Fields from zpool status
	Status           string             `json:"status,omitempty"`
	Action           string             `json:"action,omitempty"`
	MsgID            string             `json:"msgid,omitempty"`
	MoreInfo         string             `json:"moreinfo,omitempty"`
	ScanStats        *ScanStats         `json:"scan_stats,omitempty"`
	RaidzExpandStats *RaidzExpandStats  `json:"raidz_expand_stats,omitempty"`
	CheckpointStats  *CheckpointStats   `json:"checkpoint_stats,omitempty"`
	RemovalStats     *RemovalStats      `json:"removal_stats,omitempty"`
	VDevs            map[string]*VDev   `json:"vdevs,omitempty"`
	ErrorCount       string             `json:"error_count,omitempty"`
}

// ScanStats represents pool scanning status
type ScanStats struct {
	Function           string `json:"function"`
	State              string `json:"state"`
	StartTime          string `json:"start_time"`
	EndTime            string `json:"end_time"`
	ToExamine          string `json:"to_examine"`
	Examined           string `json:"examined"`
	Skipped            string `json:"skipped"`
	Processed          string `json:"processed"`
	Errors             string `json:"errors"`
	BytesPerScan       string `json:"bytes_per_scan"`
	PassStart          string `json:"pass_start"`
	ScrubPause         string `json:"scrub_pause"`
	ScrubSpentPaused   string `json:"scrub_spent_paused"`
	IssuedBytesPerScan string `json:"issued_bytes_per_scan"`
	Issued             string `json:"issued"`
}

// RaidzExpandStats represents RAIDZ expansion status (ZFS 2.3+)
type RaidzExpandStats struct {
	Name                 string `json:"name"`
	State                string `json:"state"`
	ExpandingVdev        string `json:"expanding_vdev"`
	StartTime            string `json:"start_time"`
	EndTime              string `json:"end_time"`
	ToReflow             string `json:"to_reflow"`
	Reflowed             string `json:"reflowed"`
	WaitingForResilver   string `json:"waiting_for_resilver"`
}

// CheckpointStats represents pool checkpoint status
type CheckpointStats struct {
	State     string `json:"state"`
	StartTime string `json:"start_time"`
	Space     string `json:"space"`
}

// RemovalStats represents vdev removal status
type RemovalStats struct {
	Name          string `json:"name"`
	State         string `json:"state"`
	RemovingVdev  string `json:"removing_vdev"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	ToCopy        string `json:"to_copy"`
	Copied        string `json:"copied"`
	MappingMemory string `json:"mapping_memory"`
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
	Name            string           `json:"name"`
	VDevType        string           `json:"vdev_type"`
	GUID            string           `json:"guid"`
	State           string           `json:"state"`
	Path            string           `json:"path,omitempty"`
	PhysPath        string           `json:"phys_path,omitempty"`
	DevID           string           `json:"devid,omitempty"`
	Class           string           `json:"class,omitempty"`
	AllocSpace      string           `json:"alloc_space,omitempty"`
	TotalSpace      string           `json:"total_space,omitempty"`
	DefSpace        string           `json:"def_space,omitempty"`
	RepDevSize      string           `json:"rep_dev_size,omitempty"`
	PhysSpace       string           `json:"phys_space,omitempty"`
	ScanProcessed   string           `json:"scan_processed,omitempty"` // Data processed during scan/resilver
	VDevs           map[string]*VDev `json:"vdevs,omitempty"`          // Nested vdevs as map
	ReadErrors      string           `json:"read_errors"`
	WriteErrors     string           `json:"write_errors"`
	ChecksumErrors  string           `json:"checksum_errors"`
	SlowIOs         string           `json:"slow_ios,omitempty"`
	CheckpointSpace string           `json:"checkpoint_space,omitempty"`
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
	Name       string            `json:"name"`
	VDevSpec   []VDevSpec        `json:"vdev_spec"`
	Properties map[string]string `json:"properties"`
	Features   map[string]bool   `json:"features"`
	Force      bool              `json:"force"`
	MountPoint string            `json:"mount_point"`
}

// VDevSpec defines virtual device configuration for pool creation
type VDevSpec struct {
	Type     string     `json:"type"`     // mirror, raidz, etc.
	Devices  []string   `json:"devices"`  // Device paths
	Children []VDevSpec `json:"children"` // For nested vdev configurations
}

// ImportConfig defines parameters for pool import
type ImportConfig struct {
	Name                string            `json:"name"`
	Dir                 string            `json:"dir"` // Search directory
	Properties          map[string]string `json:"properties"`
	Force               bool              `json:"force"`
	AllowDestroy        bool              `json:"allow_destroy"`
	RewindToCheckpoint  bool              `json:"rewind_to_checkpoint"`  // Import pool rewound to checkpoint (--rewind-to-checkpoint)
	Paths               []string          `json:"paths"` // Device paths to search
}

// ScrubConfig defines parameters for pool scrub operations
// Note: When all flags are false, 'zpool scrub <pool>' either:
//   - Resumes a paused scrub if one exists
//   - Starts a new scrub if no scrub is in progress
type ScrubConfig struct {
	Name     string `json:"name"`
	Stop     bool   `json:"stop"`     // Stop scrubbing (-s)
	Pause    bool   `json:"pause"`    // Pause scrubbing (-p)
	Continue bool   `json:"continue"` // Continue from last saved txg (-C)
}

// AddConfig defines parameters for adding vdevs to a pool
type AddConfig struct {
	Name     string     `json:"name"`
	VDevSpec []VDevSpec `json:"vdev_spec"`
	Force    bool       `json:"force"`
}

// OfflineConfig defines parameters for taking a device offline
type OfflineConfig struct {
	Name      string `json:"name"`
	Device    string `json:"device"`
	Temporary bool   `json:"temporary"`
}

// OnlineConfig defines parameters for bringing a device online
type OnlineConfig struct {
	Name   string `json:"name"`
	Device string `json:"device"`
	Expand bool   `json:"expand"`
}

// ClearConfig defines parameters for clearing pool errors
type ClearConfig struct {
	Name   string `json:"name"`
	Device string `json:"device,omitempty"` // Optional: clear specific device
}

// InitializeConfig defines parameters for initializing devices
type InitializeConfig struct {
	Name    string   `json:"name"`
	Devices []string `json:"devices,omitempty"` // Optional: specific devices
	Cancel  bool     `json:"cancel"`
	Suspend bool     `json:"suspend"`
}

// TrimConfig defines parameters for trimming pool devices
type TrimConfig struct {
	Name    string   `json:"name"`
	Devices []string `json:"devices,omitempty"` // Optional: specific devices
	Cancel  bool     `json:"cancel"`
	Suspend bool     `json:"suspend"`
	Secure  bool     `json:"secure"`
	Rate    int      `json:"rate,omitempty"` // Trim rate limit
}

// CheckpointConfig defines parameters for checkpoint operations
type CheckpointConfig struct {
	Name    string `json:"name"`
	Discard bool   `json:"discard"`
}

// SplitConfig defines parameters for splitting a mirrored pool
type SplitConfig struct {
	Name       string            `json:"name"`
	NewPool    string            `json:"new_pool"`
	Devices    []string          `json:"devices,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	MountPoint string            `json:"mount_point,omitempty"`
}

// WaitConfig defines parameters for waiting on pool activities
type WaitConfig struct {
	Name       string   `json:"name"`
	Activities []string `json:"activities"` // e.g., "scrub", "resilver", "initialize", "trim"
}

// HistoryResult represents pool command history
type HistoryResult struct {
	History []HistoryEntry `json:"history"`
}

// HistoryEntry represents a single history entry
type HistoryEntry struct {
	Time        string `json:"time"`
	Command     string `json:"command"`
	User        string `json:"user,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
	Zone        string `json:"zone,omitempty"`
	Description string `json:"description,omitempty"`
}

// EventsResult represents pool events
type EventsResult struct {
	Events []PoolEvent `json:"events"`
}

// PoolEvent represents a single pool event
type PoolEvent struct {
	Time    string            `json:"time"`
	Class   string            `json:"class"`
	Pool    string            `json:"pool,omitempty"`
	VDev    string            `json:"vdev,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

// IOStatResult represents I/O statistics
type IOStatResult struct {
	Pools map[string]PoolIOStats `json:"pools"`
}

// PoolIOStats represents I/O statistics for a pool
type PoolIOStats struct {
	Name       string              `json:"name"`
	Alloc      string              `json:"alloc"`
	Free       string              `json:"free"`
	Operations IOOperations        `json:"operations"`
	Bandwidth  IOBandwidth         `json:"bandwidth"`
	VDevStats  map[string]VDevStat `json:"vdev_stats,omitempty"`
}

// IOOperations represents I/O operation counts
type IOOperations struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}

// IOBandwidth represents I/O bandwidth
type IOBandwidth struct {
	Read  string `json:"read"`
	Write string `json:"write"`
}

// VDevStat represents statistics for a virtual device
type VDevStat struct {
	Alloc      string       `json:"alloc"`
	Free       string       `json:"free"`
	Operations IOOperations `json:"operations"`
	Bandwidth  IOBandwidth  `json:"bandwidth"`
	Errors     IOErrors     `json:"errors"`
}

// IOErrors represents I/O error counts
type IOErrors struct {
	Read     int64 `json:"read"`
	Write    int64 `json:"write"`
	Checksum int64 `json:"checksum"`
}
