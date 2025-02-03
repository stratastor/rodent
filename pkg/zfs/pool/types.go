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
	Status     string           `json:"status,omitempty"`
	Action     string           `json:"action,omitempty"`
	MsgID      string           `json:"msgid,omitempty"`
	MoreInfo   string           `json:"moreinfo,omitempty"`
	ScanStats  *ScanStats       `json:"scan_stats,omitempty"`
	VDevs      map[string]*VDev `json:"vdevs,omitempty"`
	ErrorCount string           `json:"error_count,omitempty"`
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
	Name           string           `json:"name"`
	VDevType       string           `json:"vdev_type"`
	GUID           string           `json:"guid"`
	State          string           `json:"state"`
	Path           string           `json:"path,omitempty"`
	VDevs          map[string]*VDev `json:"vdevs,omitempty"` // Nested vdevs as map
	ReadErrors     string           `json:"read_errors"`
	WriteErrors    string           `json:"write_errors"`
	ChecksumErrors string           `json:"checksum_errors"`
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
	Name         string            `json:"name"`
	Dir          string            `json:"dir"` // Search directory
	Properties   map[string]string `json:"properties"`
	Force        bool              `json:"force"`
	AllowDestroy bool              `json:"allow_destroy"`
	Paths        []string          `json:"paths"` // Device paths to search
}
