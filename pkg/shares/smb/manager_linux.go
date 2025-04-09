//go:build linux
// +build linux

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
