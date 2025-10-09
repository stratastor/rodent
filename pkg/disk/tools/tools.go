// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// ToolStatus represents the availability status of a tool
type ToolStatus struct {
	Name      string
	Path      string
	Available bool
	Version   string
	Error     string
}

// ToolChecker manages tool availability checking
type ToolChecker struct {
	logger    logger.Logger
	executor  *command.CommandExecutor
	toolPaths map[string]string // tool name -> configured path
	cache     map[string]*ToolStatus
	mu        sync.RWMutex
}

// NewToolChecker creates a new tool checker
func NewToolChecker(l logger.Logger, config *types.ToolsConfig) *ToolChecker {
	tc := &ToolChecker{
		logger:    l,
		executor:  command.NewCommandExecutor(false), // No sudo for version checks
		toolPaths: make(map[string]string),
		cache:     make(map[string]*ToolStatus),
	}

	// Set shorter timeout for version checks
	tc.executor.Timeout = 5 * time.Second

	// Load configured tool paths
	tc.toolPaths["smartctl"] = config.SmartctlPath
	tc.toolPaths["lsblk"] = config.LsblkPath
	tc.toolPaths["lsscsi"] = config.LsscsiPath
	tc.toolPaths["udevadm"] = config.UdevadmPath
	tc.toolPaths["sg_ses"] = config.SgSesPath

	return tc
}

// CheckAll checks availability of all tools
func (tc *ToolChecker) CheckAll() map[string]*ToolStatus {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	results := make(map[string]*ToolStatus)

	for tool, path := range tc.toolPaths {
		status := tc.checkTool(tool, path)
		tc.cache[tool] = status
		results[tool] = status
	}

	return results
}

// CheckTool checks availability of a specific tool
func (tc *ToolChecker) CheckTool(toolName string) (*ToolStatus, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	path, exists := tc.toolPaths[toolName]
	if !exists {
		return nil, errors.New(errors.DiskToolNotFound, "unknown tool").
			WithMetadata("tool", toolName)
	}

	status := tc.checkTool(toolName, path)
	tc.cache[toolName] = status

	return status, nil
}

// checkTool performs the actual tool check (must be called with lock held)
func (tc *ToolChecker) checkTool(toolName, configuredPath string) *ToolStatus {
	status := &ToolStatus{
		Name: toolName,
		Path: configuredPath,
	}

	// First try configured path
	if configuredPath != "" {
		if version, err := tc.getToolVersion(configuredPath, toolName); err == nil {
			status.Available = true
			status.Version = version
			status.Path = configuredPath
			return status
		}
	}

	// Try to find in PATH
	path, err := exec.LookPath(toolName)
	if err != nil {
		status.Available = false
		status.Error = fmt.Sprintf("tool not found in PATH or configured location: %v", err)
		return status
	}

	version, err := tc.getToolVersion(path, toolName)
	if err != nil {
		status.Available = false
		status.Error = fmt.Sprintf("tool found but version check failed: %v", err)
		status.Path = path
		return status
	}

	status.Available = true
	status.Version = version
	status.Path = path
	return status
}

// getToolVersion attempts to get the version of a tool
func (tc *ToolChecker) getToolVersion(path, toolName string) (string, error) {
	ctx := context.Background()

	// Use the command executor to get version
	output, err := tc.executor.ExecuteWithCombinedOutput(ctx, path, "--version")
	if err != nil {
		// Some tools exit non-zero even for --version
		// Try to parse output anyway
		if len(output) == 0 {
			return "", err
		}
	}

	version := tc.parseVersion(string(output), toolName)
	return version, nil
}

// parseVersion extracts version string from command output
func (tc *ToolChecker) parseVersion(output, toolName string) string {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return "unknown"
	}

	// First line usually contains version info
	firstLine := strings.TrimSpace(lines[0])

	switch toolName {
	case "smartctl":
		// "smartctl 7.2 2020-12-30 r5155 [x86_64-linux-5.10.0-8-amd64] (local build)"
		if strings.Contains(firstLine, "smartctl") {
			parts := strings.Fields(firstLine)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	case "lsblk":
		// "lsblk from util-linux 2.36.1"
		if strings.Contains(firstLine, "util-linux") {
			parts := strings.Fields(firstLine)
			if len(parts) >= 4 {
				return parts[3]
			}
		}
	case "lsscsi":
		// "lsscsi version: 0.31"
		if strings.Contains(firstLine, "version") {
			parts := strings.Split(firstLine, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	case "udevadm":
		// "systemd 247 (247.3-6)"
		parts := strings.Fields(firstLine)
		if len(parts) >= 2 {
			return parts[1]
		}
	case "sg_ses":
		// "sg_ses version: 2.26 20200421"
		if strings.Contains(firstLine, "version") {
			parts := strings.Fields(firstLine)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	// Fallback: return first line (truncated)
	if len(firstLine) > 50 {
		return firstLine[:50] + "..."
	}
	return firstLine
}

// IsAvailable returns whether a tool is available
func (tc *ToolChecker) IsAvailable(toolName string) bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	status, exists := tc.cache[toolName]
	return exists && status.Available
}

// GetPath returns the path to a tool
func (tc *ToolChecker) GetPath(toolName string) (string, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	status, exists := tc.cache[toolName]
	if !exists {
		return "", errors.New(errors.DiskToolNotFound, "tool not checked").
			WithMetadata("tool", toolName)
	}

	if !status.Available {
		return "", errors.New(errors.DiskToolNotFound, status.Error).
			WithMetadata("tool", toolName)
	}

	return status.Path, nil
}

// GetStatus returns the cached status of a tool
func (tc *ToolChecker) GetStatus(toolName string) (*ToolStatus, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	status, exists := tc.cache[toolName]
	return status, exists
}

// ValidateRequired validates that all required tools are available
func (tc *ToolChecker) ValidateRequired(requiredTools []string) error {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	missing := []string{}

	for _, tool := range requiredTools {
		status, exists := tc.cache[tool]
		if !exists || !status.Available {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		return errors.New(errors.DiskToolNotFound,
			fmt.Sprintf("required tools not available: %s", strings.Join(missing, ", "))).
			WithMetadata("missing_tools", strings.Join(missing, ", "))
	}

	return nil
}

// CheckOptional checks optional tools and logs warnings
func (tc *ToolChecker) CheckOptional(optionalTools []string) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	for _, tool := range optionalTools {
		status, exists := tc.cache[tool]
		if !exists || !status.Available {
			tc.logger.Warn("optional tool not available",
				"tool", tool,
				"reason", "some features may be unavailable")
		}
	}
}

// GetAllStatuses returns all cached tool statuses
func (tc *ToolChecker) GetAllStatuses() map[string]*ToolStatus {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]*ToolStatus, len(tc.cache))
	for k, v := range tc.cache {
		statusCopy := *v
		result[k] = &statusCopy
	}

	return result
}

// Refresh re-checks all tools
func (tc *ToolChecker) Refresh() map[string]*ToolStatus {
	return tc.CheckAll()
}
