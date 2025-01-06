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

// pkg/zfs/pool/types.go

package pool

// PoolStatus represents the full status of a ZPool
type PoolStatus struct {
	OutputVersion struct {
		Command   string `json:"command"`
		VersMajor int    `json:"vers_major"`
		VersMinor int    `json:"vers_minor"`
	} `json:"output_version"`
	Pools map[string]*Pool `json:"pools"`
}

// Pool represents a ZFS storage pool
type Pool struct {
	Name       string           `json:"name"`
	State      string           `json:"state"`
	GUID       string           `json:"pool_guid"`
	TXG        string           `json:"txg"`
	SPAVersion string           `json:"spa_version"`
	ZPLVersion string           `json:"zpl_version"`
	VDevs      map[string]*VDev `json:"vdevs"` // Change []VDev to map[string]*VDev
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
