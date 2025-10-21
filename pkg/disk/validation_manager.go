// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// ValidationResult represents the result of disk validation
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Issues   []ValidationIssue `json:"issues,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
}

// ValidationIssue represents a validation problem
type ValidationIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // error, warning
	Message  string `json:"message"`
}

// ValidateDisk validates whether a disk is suitable for ZFS use
func (m *Manager) ValidateDisk(deviceID string) (*ValidationResult, error) {
	m.cacheMu.RLock()
	disk, exists := m.deviceCache[deviceID]
	m.cacheMu.RUnlock()

	if !exists {
		return nil, errors.New(errors.DiskNotFound, "device not found").
			WithMetadata("device_id", deviceID)
	}

	result := &ValidationResult{
		Valid:    true,
		Issues:   make([]ValidationIssue, 0),
		Warnings: make([]string, 0),
	}

	// Check health status
	if disk.Health == types.HealthFailed || disk.Health == types.HealthCritical {
		result.Valid = false
		result.Issues = append(result.Issues, ValidationIssue{
			Code:     "DISK_FAILING",
			Severity: "error",
			Message:  "Disk health is " + string(disk.Health) + ": " + disk.HealthReason,
		})
	} else if disk.Health == types.HealthWarning {
		result.Warnings = append(result.Warnings, "Disk health is WARNING: "+disk.HealthReason)
	}

	// Check SMART status
	if disk.SMARTInfo != nil && disk.SMARTInfo.OverallStatus != "PASSED" {
		result.Warnings = append(result.Warnings, "SMART status: "+disk.SMARTInfo.OverallStatus)
	}

	// Check if disk is too small (minimum 1GB for practical use)
	if disk.SizeBytes < 1024*1024*1024 {
		result.Valid = false
		result.Issues = append(result.Issues, ValidationIssue{
			Code:     "DISK_TOO_SMALL",
			Severity: "error",
			Message:  "Disk is too small for ZFS use (< 1GB)",
		})
	}

	// Check state
	deviceState, err := m.stateManager.GetDeviceState(deviceID)
	if err == nil {
		if deviceState.State == types.DiskStateQuarantined {
			result.Valid = false
			result.Issues = append(result.Issues, ValidationIssue{
				Code:     "DISK_QUARANTINED",
				Severity: "error",
				Message:  "Disk is quarantined",
			})
		}
	}

	m.logger.Debug("disk validation completed",
		"device_id", deviceID,
		"valid", result.Valid,
		"issues", len(result.Issues),
		"warnings", len(result.Warnings))

	return result, nil
}
