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

package common

// List of native ZFS properties as per OpenZFS documentation.
var nativeDatasetProps = map[string]struct{}{
	// Filesystem Properties
	"type":                 {},
	"mountpoint":           {},
	"canmount":             {},
	"mounted":              {},
	"version":              {},
	"volsize":              {},
	"volblocksize":         {},
	"origin":               {},
	"clones":               {},
	"recordsize":           {},
	"compression":          {},
	"checksum":             {},
	"sync":                 {},
	"redundant_metadata":   {},
	"dedup":                {},
	"primarycache":         {},
	"secondarycache":       {},
	"prefetch":             {},
	"logbias":              {},
	"atime":                {},
	"relatime":             {},
	"dnodesize":            {},
	"special_small_blocks": {},
	"volthreading":         {},
	"direct":               {},

	// Quota and Space Management
	"quota":            {},
	"reservation":      {},
	"refquota":         {},
	"refreservation":   {},
	"filesystem_limit": {},
	"snapshot_limit":   {},

	// Space Usage Properties
	"used":                 {},
	"available":            {},
	"referenced":           {},
	"compressratio":        {},
	"refcompressratio":     {},
	"usedbysnapshots":      {},
	"usedbydataset":        {},
	"usedbychildren":       {},
	"usedbyrefreservation": {},
	"userrefs":             {},
	"written":              {},
	"logicalused":          {},
	"logicalreferenced":    {},
	"filesystem_count":     {},
	"snapshot_count":       {},

	// Identification Properties
	"guid":      {},
	"createtxg": {},
	"objsetid":  {},

	// Sharing Properties
	"sharenfs": {},
	"sharesmb": {},

	// Snapshot Properties
	"snapdir": {},
	"snapdev": {},

	// Access Control Properties
	"acltype":    {},
	"aclmode":    {},
	"aclinherit": {},
	"xattr":      {},
	"devices":    {},
	"setuid":     {},
	"exec":       {},
	"readonly":   {},
	"jailed":     {},
	"vscan":      {},
	"nbmand":     {},
	"overlay":    {},
	"zoned":      {},

	// Encryption Properties
	"encryption":     {},
	"keyformat":      {},
	"keylocation":    {},
	"keystatus":      {},
	"encryptionroot": {},
	"pbkdf2iters":    {},

	// SELinux Context Properties
	"context":     {},
	"fscontext":   {},
	"defcontext":  {},
	"rootcontext": {},

	// Miscellaneous Properties
	"receive_resume_token": {},
	"redact_snaps":         {},
	"copies":               {},
	"normalization":        {},
	"casesensitivity":      {},
	"utf8only":             {},
	"longname":             {},
	"defer_destroy":        {},
	"volmode":              {},

	"feature@...": {},
}

// zpropValidChar checks if the character is a valid user property character.
func zpropValidChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == ':'
}

// isNativeDatasetProp checks if a property name is a native ZFS property.
func isNativeDatasetProp(name string) bool {
	_, exists := nativeDatasetProps[name]
	return exists
}

// isUserProperty checks if the property name is a valid user-defined property.
func isUserProperty(name string) bool {
	foundSep := false

	for _, c := range name {
		if c == ':' {
			foundSep = true
		}
		if !zpropValidChar(c) {
			return false
		}
	}

	return foundSep
}

// IsValidDatasetProperty checks if the given property name is valid (native or user-defined).
func IsValidDatasetProperty(name string) bool {
	if isNativeDatasetProp(name) {
		return true
	}

	return isUserProperty(name)
}

type PoolPropContext uint

const (
	InvalidPoolPropContext PoolPropContext = 0
	AnytimePoolPropContext PoolPropContext = 1 << iota
	CreatePoolPropContext
	ImportPoolPropContext
	ReadonlyPoolPropContext
	UserPoolPropContext
)

const (
	NativePoolPropContext   = AnytimePoolPropContext | CreatePoolPropContext | ImportPoolPropContext
	ValidPoolSetPropContext = NativePoolPropContext | UserPoolPropContext
	ValidPoolGetPropContext = NativePoolPropContext | ReadonlyPoolPropContext | UserPoolPropContext
)

// Categories of ZFS pool properties
var readonlyPoolProps = map[string]struct{}{
	"size":          {},
	"capacity":      {},
	"altroot":       {}, // readonly after being set at creation/import
	"health":        {},
	"guid":          {},
	"version":       {},
	"bootfs":        {},
	"delegation":    {},
	"autoreplace":   {},
	"cachefile":     {},
	"failmode":      {},
	"listsnapshots": {},
	"autoexpand":    {},
	"dedupratio":    {},
	"free":          {},
	"allocated":     {},
	"readonly":      {}, // readonly after being set at import
	"ashift":        {},
	"comment":       {},
	"multihost":     {},
	"checkpoint":    {},
	"feature@...":   {}, // readonly feature flags
}

// Categories of ZFS pool properties
var anytimePoolProps = map[string]struct{}{
	"listsnapshots": {},
	"autoexpand":    {},
	"autoreplace":   {},
	"delegation":    {},
	"failmode":      {},
	"cachefile":     {},
	"comment":       {},
	"multihost":     {},
	"autotrim":      {},
	"ashift":        {}, // Can be set anytime
}

var createPoolProps = map[string]struct{}{
	"altroot": {}, // Settable at creation or import
}

var importPoolProps = map[string]struct{}{
	"readonly": {}, // Settable only at import time
}

// IsValidPoolProperty checks if the given property name is valid for a ZFS pool in the given context.
func IsValidPoolProperty(name string, context PoolPropContext) bool {
	if isUserProperty(name) {
		return true
	}

	if context&AnytimePoolPropContext != 0 {
		if _, exists := anytimePoolProps[name]; exists {
			return true
		}
	}

	if context&CreatePoolPropContext != 0 {
		if _, exists := createPoolProps[name]; exists {
			return true
		}
		if _, exists := anytimePoolProps[name]; exists {
			return true
		}
	}

	if context&ImportPoolPropContext != 0 {
		if _, exists := importPoolProps[name]; exists {
			return true
		}
		if _, exists := createPoolProps[name]; exists {
			return true
		}
		if _, exists := anytimePoolProps[name]; exists {
			return true
		}
	}

	if context&ReadonlyPoolPropContext != 0 {
		if _, exists := readonlyPoolProps[name]; exists {
			return true
		}
	}

	return false
}
