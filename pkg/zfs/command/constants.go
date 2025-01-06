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
