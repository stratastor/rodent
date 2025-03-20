package common

import (
	"os"
	"path/filepath"
)

// BaseDir holds the application's base directory
var BaseDir string

func init() {
	// Try to detect base directory
	execPath, err := os.Executable()
	if err == nil {
		BaseDir = filepath.Dir(execPath)
	} else {
		// Fallback to working directory
		BaseDir, _ = os.Getwd()
	}
}

// ResolvePath resolves a path relative to the application's base directory
func ResolvePath(relativePath string) string {
	// If path is absolute, return it unchanged
	if filepath.IsAbs(relativePath) {
		return relativePath
	}

	// Check if path exists relative to BaseDir
	path := filepath.Join(BaseDir, relativePath)
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Try development path (one level up from binary)
	path = filepath.Join(BaseDir, "..", relativePath)
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Return the original path joined with BaseDir as fallback
	return filepath.Join(BaseDir, relativePath)
}
