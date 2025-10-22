// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"strings"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
)

// SmartctlExecutor wraps smartctl command execution with JSON output
type SmartctlExecutor struct {
	logger   logger.Logger
	executor *command.CommandExecutor
	path     string
}

// NewSmartctlExecutor creates a new smartctl executor
func NewSmartctlExecutor(l logger.Logger, path string, useSudo bool) *SmartctlExecutor {
	executor := command.NewCommandExecutor(useSudo)
	executor.Timeout = 60 * time.Second // SMART operations can take longer

	return &SmartctlExecutor{
		logger:   l,
		executor: executor,
		path:     path,
	}
}

// GetInfo gets SMART information for a device (JSON format)
func (s *SmartctlExecutor) GetInfo(ctx context.Context, device string) ([]byte, error) {
	s.logger.Debug("getting SMART info", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path, "--json", "--info", device)
}

// GetAll gets all SMART information including attributes (JSON format)
// This is the primary method to get complete SMART data
func (s *SmartctlExecutor) GetAll(ctx context.Context, device string) ([]byte, error) {
	s.logger.Debug("getting all SMART data", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path, "--json", "--all", device)
}

// GetHealth gets SMART health status (JSON format)
func (s *SmartctlExecutor) GetHealth(ctx context.Context, device string) ([]byte, error) {
	s.logger.Debug("getting SMART health", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path, "--json", "--health", device)
}

// GetAttributes gets SMART attributes (JSON format)
func (s *SmartctlExecutor) GetAttributes(ctx context.Context, device string) ([]byte, error) {
	s.logger.Debug("getting SMART attributes", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path, "--json", "--attributes", device)
}

// StartQuickTest starts a quick/short SMART self-test
func (s *SmartctlExecutor) StartQuickTest(ctx context.Context, device string) ([]byte, error) {
	s.logger.Info("starting quick SMART self-test", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path, "--json", "--test=short", device)
}

// StartExtensiveTest starts an extensive/long SMART self-test
func (s *SmartctlExecutor) StartExtensiveTest(ctx context.Context, device string) ([]byte, error) {
	s.logger.Info("starting extensive SMART self-test", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path, "--json", "--test=long", device)
}

// AbortTest aborts a running SMART self-test
func (s *SmartctlExecutor) AbortTest(ctx context.Context, device string) ([]byte, error) {
	s.logger.Info("aborting SMART self-test", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path, "--json", "--abort", device)
}

// Scan scans for all SMART-capable devices (JSON format)
func (s *SmartctlExecutor) Scan(ctx context.Context) ([]byte, error) {
	s.logger.Debug("scanning for devices")
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path, "--json", "--scan")
}

// ScanOpen scans for all devices and tries to open them (JSON format)
func (s *SmartctlExecutor) ScanOpen(ctx context.Context) ([]byte, error) {
	s.logger.Debug("scanning for devices (open)")
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path, "--json", "--scan-open")
}

// CanRunSelfTests checks if the device supports SMART self-tests
// Returns true if the device reports self-test capability
// Note: This is a best-effort check - environment detection should be the primary
// indicator for cloud platforms where self-tests don't work
func (s *SmartctlExecutor) CanRunSelfTests(ctx context.Context, device string) (bool, error) {
	s.logger.Debug("checking SMART self-test capabilities", "device", device)

	// Try to get all SMART data which includes self-test capabilities
	output, err := s.executor.ExecuteWithCombinedOutput(ctx, s.path, "--json", "--all", device)
	if err != nil {
		// If we can't get SMART data, assume tests are not supported
		s.logger.Debug("failed to get SMART data for capability check",
			"device", device,
			"error", err)
		return false, nil
	}

	// Simple string check for self-test capability indicators
	// This is a heuristic - the environment check is more reliable
	outputStr := strings.ToLower(string(output))
	hasTestCapability := strings.Contains(outputStr, "self_test") ||
		strings.Contains(outputStr, "selftest") ||
		strings.Contains(outputStr, "short_test") ||
		strings.Contains(outputStr, "long_test")

	return hasTestCapability, nil
}
