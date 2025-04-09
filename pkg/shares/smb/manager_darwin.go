//go:build darwin
// +build darwin

package smb

import (
	"os"
	"syscall"
	"time"
)

// getFileCreationTime returns the creation time of a file (macOS specific)
func getFileCreationTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}

	// Get the stat_t struct
	stat := info.Sys().(*syscall.Stat_t)
	return time.Unix(stat.Ctimespec.Sec, stat.Ctimespec.Nsec)
}
