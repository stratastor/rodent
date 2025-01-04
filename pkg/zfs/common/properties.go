package common

// List of native ZFS properties as per OpenZFS documentation.
var nativeDatasetProps = map[string]struct{}{
	// TODO: Differentiate read-only and settable properties

	// Read-Only Properties
	"type":              {},
	"creation":          {},
	"used":              {},
	"available":         {},
	"referenced":        {},
	"compressratio":     {},
	"mounted":           {},
	"origin":            {},
	"logicalused":       {},
	"logicalreferenced": {},
	"objsetid":          {},
	"written":           {},
	"clones":            {},
	"createtxg":         {},
	"guid":              {},

	// Settable Properties
	"acltype":              {},
	"aclmode":              {},
	"atime":                {},
	"canmount":             {},
	"checksum":             {},
	"compression":          {},
	"dedup":                {},
	"devices":              {},
	"dnodesize":            {},
	"encryption":           {},
	"exec":                 {},
	"filesystem_limit":     {},
	"keyformat":            {},
	"keylocation":          {},
	"logbias":              {},
	"mlslabel":             {},
	"mountpoint":           {},
	"overlay":              {},
	"pbkdf2iters":          {},
	"primarycache":         {},
	"quota":                {},
	"readonly":             {},
	"recordsize":           {},
	"redundant_metadata":   {},
	"refquota":             {},
	"refreservation":       {},
	"reservation":          {},
	"secondarycache":       {},
	"setuid":               {},
	"sharenfs":             {},
	"sharesmb":             {},
	"snapdev":              {},
	"snapdir":              {},
	"snapshot_limit":       {},
	"sync":                 {},
	"special_small_blocks": {},
	"snapshots_visible":    {},
	"utf8only":             {},
	"version":              {},
	"volblocksize":         {},
	"volsize":              {},
	"zoned":                {},

	// Feature Flags (Catch-all for dynamic properties)
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
		if !zpropValidChar(c) {
			return false
		}
		if c == ':' {
			foundSep = true
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
