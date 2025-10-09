// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
)

// SgSesExecutor wraps sg_ses command execution for SCSI Enclosure Services
type SgSesExecutor struct {
	logger   logger.Logger
	executor *command.CommandExecutor
	path     string
}

// NewSgSesExecutor creates a new sg_ses executor
func NewSgSesExecutor(l logger.Logger, path string, useSudo bool) *SgSesExecutor {
	executor := command.NewCommandExecutor(useSudo)
	executor.Timeout = 15 * time.Second

	return &SgSesExecutor{
		logger:   l,
		executor: executor,
		path:     path,
	}
}

// GetStatus gets enclosure status (all pages)
func (s *SgSesExecutor) GetStatus(ctx context.Context, device string) ([]byte, error) {
	s.logger.Debug("getting enclosure status", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path,
		"--page=all",
		device,
	)
}

// GetConfiguration gets enclosure configuration
func (s *SgSesExecutor) GetConfiguration(ctx context.Context, device string) ([]byte, error) {
	s.logger.Debug("getting enclosure configuration", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path,
		"--page=cf",
		device,
	)
}

// GetEnclosureStatus gets enclosure status diagnostic page
func (s *SgSesExecutor) GetEnclosureStatus(ctx context.Context, device string) ([]byte, error) {
	s.logger.Debug("getting enclosure status page", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path,
		"--page=es",
		device,
	)
}

// GetAdditionalElementStatus gets additional element status
func (s *SgSesExecutor) GetAdditionalElementStatus(ctx context.Context, device string) ([]byte, error) {
	s.logger.Debug("getting additional element status", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path,
		"--page=aes",
		device,
	)
}

// GetElementDescriptors gets element descriptors
func (s *SgSesExecutor) GetElementDescriptors(ctx context.Context, device string) ([]byte, error) {
	s.logger.Debug("getting element descriptors", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path,
		"--page=ed",
		device,
	)
}

// SetLocate sets or clears the locate LED for a device slot
func (s *SgSesExecutor) SetLocate(ctx context.Context, device string, index int, enable bool) ([]byte, error) {
	action := "set"
	if !enable {
		action = "clear"
	}
	s.logger.Info("setting locate LED", "device", device, "index", index, "enable", enable)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path,
		"--index="+string(rune(index)),
		"--"+action,
		"locate",
		device,
	)
}

// GetAllVerbose gets all enclosure information with verbose output
func (s *SgSesExecutor) GetAllVerbose(ctx context.Context, device string) ([]byte, error) {
	s.logger.Debug("getting all enclosure info (verbose)", "device", device)
	return s.executor.ExecuteWithCombinedOutput(ctx, s.path,
		"--page=all",
		"--verbose",
		"--verbose",
		device,
	)
}
