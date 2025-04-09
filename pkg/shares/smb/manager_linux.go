//go:build linux
// +build linux

// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package smb

import (
	"os"
	"syscall"
	"time"
)

// getFileCreationTime returns the creation time of a file (Linux specific)
func getFileCreationTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}

	// Get the stat_t struct
	stat := info.Sys().(*syscall.Stat_t)

	// For Linux systems
	// Use Ctim field, which represents status change time
	return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
}
