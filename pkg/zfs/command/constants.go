// pkg/zfs/command/constants.go

package command

import "time"

const (
	// Base commands
	// TODO: Make these configurable?
	BinZFS   = "/usr/local/sbin/zfs"
	BinZpool = "/usr/local/sbin/zpool"

	maxCommandArgs = 64

	// Default timeout for command execution
	DefaultTimeout = 30 * time.Second
)

// Dangerous characters that could enable command injection
var dangerousChars = "&|><$`\\[];{}"

// Commands that support JSON output
var JSONSupportedCommands = map[string]bool{
	"zfs get":       true,
	"zfs list":      true,
	"zfs version":   true,
	"zpool get":     true,
	"zpool list":    true,
	"zpool status":  true,
	"zpool version": true,
	"zpool history": true,
}

// Commands that require sudo
var SudoRequiredCommands = map[string]bool{
	"zfs create":       true,
	"zfs destroy":      true,
	"zfs rename":       true,
	"zfs snapshot":     true,
	"zfs rollback":     true,
	"zfs clone":        true,
	"zfs promote":      true,
	"zfs mount":        true,
	"zfs unmount":      true,
	"zfs set":          true,
	"zfs allow":        true,
	"zfs unallow":      true,
	"zfs share":        true,
	"zfs unshare":      true,
	"zpool create":     true,
	"zpool destroy":    true,
	"zpool import":     true,
	"zpool export":     true,
	"zpool scrub":      true,
	"zpool initialize": true,
	"zpool attach":     true,
	"zpool detach":     true,
	"zpool set":        true,
}
