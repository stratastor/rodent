package common

// List of native ZFS properties as per OpenZFS documentation.
var nativeProperties = map[string]struct{}{
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

// isNativeProperty checks if a property name is a native ZFS property.
func isNativeProperty(name string) bool {
	_, exists := nativeProperties[name]
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

// IsValidZFSProperty checks if the given property name is valid (native or user-defined).
func IsValidZFSProperty(name string) bool {
	if isNativeProperty(name) {
		return true
	}

	return isUserProperty(name)
}
