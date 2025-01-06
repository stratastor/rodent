/*
 * Copyright 2024 Raamsri Kumar <raam@tinkershack.in> and The StrataSTOR Authors 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */package common

import (
	"strings"

	"github.com/stratastor/rodent/pkg/errors"
)

// This is adapted and extended from ZFS name validation functions from OpenZFS: zfs_namecheck.c

// Maximum lengths and limits
const (
	MaxDatasetNameLen = 256 // ZFS_MAX_DATASET_NAME_LEN
	MaxPermSetLen     = 64  // ZFS_PERMSET_MAXLEN
	MaxDatasetNesting = 50  // zfs_max_dataset_nesting default value
)

// Dataset types that can be combined as bitmasks
type DatasetType uint8

const (
	TypeInvalid    DatasetType = 0
	TypeFilesystem DatasetType = 1 << iota
	TypeSnapshot
	TypeVolume
	TypePool
	TypeBookmark
	TypeVDev
)

const (
	// TypeDatasetMask represents any dataset type (filesystem, volume, or snapshot)
	TypeDatasetMask   = TypeFilesystem | TypeVolume | TypeSnapshot
	TypeZFSEntityMask = TypeFilesystem | TypeVolume | TypeSnapshot | TypeBookmark
)

// DatasetComponent represents the parsed components of a dataset name
type DatasetComponent struct {
	Base     string      // Base dataset name (filesystem/volume)
	Snapshot string      // Snapshot component (after @), if any
	Bookmark string      // Bookmark component (after #), if any
	Type     DatasetType // Type of the dataset
}

// isValidChar follows the valid_char() function logic
func isValidChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == ':' || c == ' '
}

// GetDatasetDepth returns the nesting depth of a dataset path
func GetDatasetDepth(path string) int {
	depth := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			depth++
		}
		// Stop at snapshot or bookmark delimiter
		if path[i] == '@' || path[i] == '#' {
			break
		}
	}
	return depth
}

// ComponentNameCheck validates a single component name
func ComponentNameCheck(name string) error {
	if len(name) >= MaxDatasetNameLen {
		return errors.New(errors.ZFSNameTooLong, "component name too long")
	}

	if len(name) == 0 {
		return errors.New(errors.ZFSNameEmptyComponent, "component name empty")
	}

	for _, c := range name {
		if !isValidChar(c) {
			return errors.New(errors.ZFSNameInvalidChar, "invalid character in component name")
		}
	}
	return nil
}

func EntityNameCheck(path string) error {
	if len(path) >= MaxDatasetNameLen {
		return errors.New(errors.ZFSNameTooLong, "name too long: "+path)
	}

	if len(path) == 0 {
		return errors.New(errors.ZFSNameEmptyComponent, "name empty: "+path)
	}

	if path[0] == '/' {
		return errors.New(errors.ZFSNameLeadingSlash, "name cannot start with '/': "+path)
	}

	// Check for trailing slash
	if path[len(path)-1] == '/' {
		return errors.New(errors.ZFSNameTrailingSlash, "trailing slash: "+path)
	}

	foundDelim := false
	start := 0

	for start < len(path) {
		// Find end of current component
		end := start
		for end < len(path) && path[end] != '/' && path[end] != '@' && path[end] != '#' {
			end++
		}

		// Zero-length components are not allowed
		if start == end {
			return errors.New(errors.ZFSNameEmptyComponent, "invalid/empty component after '/', '@' or '#': "+path)
		}

		// Validate component characters
		component := path[start:end]
		for _, c := range component {
			if !isValidChar(c) && c != '%' {
				return errors.New(errors.ZFSNameInvalidChar, "invalid character: "+path)
			}
		}

		// Check for "." and ".."
		if component == "." {
			return errors.New(errors.ZFSNameSelfRef, "self reference: "+path)
		}
		if component == ".." {
			return errors.New(errors.ZFSNameParentRef, "parent reference: "+path)
		}

		// If we hit the end, we're done
		if end == len(path) {
			break
		}

		// Handle delimiters
		if path[end] == '@' || path[end] == '#' {
			if foundDelim {
				return errors.New(errors.ZFSNameMultipleDelimiters, "multiple delimiters: "+path)
			}
			foundDelim = true

			// Check next character after delimiter
			if end+1 >= len(path) {
				return errors.New(errors.ZFSNameEmptyComponent, "empty component after delimiter: "+path)
			}
		}

		// Check for slashes after delimiter
		if path[end] == '/' && foundDelim {
			return errors.New(errors.ZFSNameTrailingSlash, "slash after delimiter: "+path)
		}

		start = end + 1
	}

	return DatasetNestCheck(path)
}

// ValidateZFSName performs comprehensive ZFS name validation based on type
func ValidateZFSName(path string, dtype DatasetType) error {
	// Check for snapshot delimiter
	hasAtSign := strings.Contains(path, "@")
	if !dtype.IsSnapshot() && hasAtSign {
		return errors.New(errors.ZFSNameNoAtSign,
			"snapshot delimiter '@' is not expected here")
	}
	if dtype.IsSnapshot() && !hasAtSign {
		return errors.New(errors.ZFSNameNoAtSign,
			"missing '@' delimiter in snapshot name")
	}

	// Check for bookmark delimiter
	hasPound := strings.Contains(path, "#")
	if !dtype.IsBookmark() && hasPound {
		return errors.New(errors.ZFSNameNoPound,
			"bookmark delimiter '#' is not expected here")
	}
	if dtype.IsBookmark() && !hasPound {
		return errors.New(errors.ZFSNameNoPound,
			"missing '#' delimiter in bookmark name")
	}

	// Perform base entity name validation
	if err := EntityNameCheck(path); err != nil {
		return err
	}

	return nil
}

// DatasetNameCheck validates dataset names (no bookmarks allowed)
func DatasetNameCheck(path string) error {
	if err := EntityNameCheck(path); err != nil {
		return errors.Wrap(err, errors.ZFSNameInvalid)
	}

	// Dataset cannot contain '#'
	for i := 0; i < len(path); i++ {
		if path[i] == '#' {
			return errors.New(errors.ZFSNameInvalidChar, "dataset cannot contain bookmark delimiter")
		}
	}
	return nil
}

// BookmarkNameCheck validates bookmark names (must contain '#')
func BookmarkNameCheck(path string) error {
	if err := EntityNameCheck(path); err != nil {
		return errors.Wrap(err, errors.ZFSNameInvalid)
	}

	hasPound := false
	for i := 0; i < len(path); i++ {
		if path[i] == '#' {
			hasPound = true
			break
		}
	}
	if !hasPound {
		return errors.New(errors.ZFSNameNoPound, "bookmark name must contain '#'")
	}
	return nil
}

// SnapshotNameCheck validates snapshot names (must contain '@')
func SnapshotNameCheck(path string) error {
	if err := EntityNameCheck(path); err != nil {
		return errors.Wrap(err, errors.ZFSNameInvalid)
	}

	hasAt := false
	for i := 0; i < len(path); i++ {
		if path[i] == '@' {
			hasAt = true
			break
		}
	}
	if !hasAt {
		return errors.New(errors.ZFSNameNoAtSign, "snapshot name must contain '@'")
	}
	return nil
}

// PoolNameCheck validates pool names
func PoolNameCheck(name string) error {
	// Check length including space for internal datasets
	maxLen := MaxDatasetNameLen - 2 - (len("$ORIGIN") * 2)
	if len(name) >= maxLen {
		return errors.New(errors.ZFSNameTooLong, "name too long")
	}

	// Must begin with a letter
	if len(name) == 0 || !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) {
		return errors.New(errors.ZFSNameNoLetter, "name must begin with a letter")
	}

	// Check for valid characters
	for _, c := range name {
		if !isValidChar(c) {
			return errors.New(errors.ZFSNameInvalidChar, "invalid character in name")
		}
	}

	// Check reserved names
	if name == "mirror" || name == "raidz" || name == "draid" {
		return errors.New(errors.ZFSNameReserved, "internally reserved name")
	}

	return nil
}

// MountpointNameCheck validates mountpoint paths
func MountpointNameCheck(path string) error {
	if path == "" || path[0] != '/' {
		return errors.New(errors.ZFSNameLeadingSlash, "mountpoint must start with '/'")
	}

	start := 1
	for i := 1; i < len(path); i++ {
		if path[i] == '/' || i == len(path)-1 {
			end := i
			if i == len(path)-1 && path[i] != '/' {
				end = i + 1
			}
			if end-start >= MaxDatasetNameLen {
				return errors.New(errors.ZFSNameTooLong, "component name too long")
			}
			start = i + 1
		}
	}

	return nil
}

// DatasetNestCheck validates dataset nesting depth
func DatasetNestCheck(path string) error {
	if GetDatasetDepth(path) >= MaxDatasetNesting {
		return errors.New(errors.ZFSNameTooLong, "dataset nesting too deep")
	}
	return nil
}

// ParseDatasetName validates and splits a dataset name into its components
func ParseDatasetName(name string) (*DatasetComponent, error) {
	// First validate the name
	if err := EntityNameCheck(name); err != nil {
		return nil, err
	}

	comp := &DatasetComponent{
		Type: TypeInvalid,
	}

	// Find first occurrence of @ or #
	var delimIdx = -1
	var delim rune
	for i, c := range name {
		if c == '@' || c == '#' {
			delimIdx = i
			delim = c
			break
		}
	}

	// Parse base component
	if delimIdx == -1 {
		comp.Base = name
		// Determine if it's filesystem or volume (would need additional check)
		comp.Type = TypeFilesystem
		return comp, nil
	}

	// Split into base and snapshot/bookmark
	comp.Base = name[:delimIdx]

	switch delim {
	case '@':
		comp.Snapshot = name[delimIdx+1:]
		comp.Type = TypeSnapshot
	case '#':
		comp.Bookmark = name[delimIdx+1:]
		comp.Type = TypeBookmark
	}

	return comp, nil
}

// String returns the full dataset name
func (dc *DatasetComponent) String() string {
	switch {
	case dc.Snapshot != "":
		return dc.Base + "@" + dc.Snapshot
	case dc.Bookmark != "":
		return dc.Base + "#" + dc.Bookmark
	default:
		return dc.Base
	}
}

// IsDataset returns true if the type is filesystem, volume or snapshot
func (dt DatasetType) IsDataset() bool {
	return dt&TypeDatasetMask != 0
}

// IsSnapshot returns true if type is snapshot
func (dt DatasetType) IsSnapshot() bool {
	return dt&TypeSnapshot != 0
}

// IsFilesystem returns true if type is filesystem
func (dt DatasetType) IsFilesystem() bool {
	return dt&TypeFilesystem != 0
}

// IsVolume returns true if type is volume
func (dt DatasetType) IsVolume() bool {
	return dt&TypeVolume != 0
}

// IsBookmark returns true if type is bookmark
func (dt DatasetType) IsBookmark() bool {
	return dt&TypeBookmark != 0
}
