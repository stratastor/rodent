// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package types

import "time"

// SMARTInfo represents SMART/health information for a disk
type SMARTInfo struct {
	// Device info
	DeviceID    string      `json:"device_id"`
	DeviceType  DeviceType  `json:"device_type"` // HDD, SSD, NVMe

	// SMART status
	Enabled      bool   `json:"enabled"`       // SMART enabled
	Available    bool   `json:"available"`     // SMART available
	OverallStatus string `json:"overall_status"` // PASSED, FAILED, etc.

	// For SATA/SAS devices
	Attributes map[int]*SMARTAttribute `json:"attributes,omitempty"` // SMART attributes

	// For NVMe devices
	NVMeHealth *NVMeHealth `json:"nvme_health,omitempty"`

	// Temperature
	Temperature      int  `json:"temperature"`       // Current temperature (Celsius)
	TemperatureValid bool `json:"temperature_valid"` // Whether temperature is valid

	// Self-test results
	SelfTestStatus      string            `json:"self_test_status,omitempty"`       // Last self-test status
	SelfTestRemaining   int               `json:"self_test_remaining,omitempty"`    // Percentage remaining
	SelfTestLogs        []*SelfTestEntry  `json:"self_test_logs,omitempty"`         // Recent self-test log entries

	// Error log
	ErrorLogCount int              `json:"error_log_count"`          // Number of errors in log
	ErrorLogs     []*ErrorLogEntry `json:"error_logs,omitempty"`     // Recent error log entries

	// Power and age
	PowerOnHours  uint64 `json:"power_on_hours"`   // Total power-on hours
	PowerCycles   uint64 `json:"power_cycles"`     // Total power cycles

	// Metadata
	LastUpdated time.Time         `json:"last_updated"` // When SMART data was last read
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SMARTAttribute represents a single SMART attribute (for SATA/SAS)
type SMARTAttribute struct {
	ID          int    `json:"id"`           // Attribute ID
	Name        string `json:"name"`         // Attribute name
	Value       int    `json:"value"`        // Normalized value (0-255)
	Worst       int    `json:"worst"`        // Worst value ever seen
	Threshold   int    `json:"threshold"`    // Failure threshold
	RawValue    uint64 `json:"raw_value"`    // Raw value
	WhenFailed  string `json:"when_failed"`  // When threshold was crossed (if ever)
	Flags       string `json:"flags"`        // Attribute flags
	Type        string `json:"type"`         // Pre-fail or Old_age
	Updated     string `json:"updated"`      // Always or Offline
	FailureNear bool   `json:"failure_near"` // True if value <= threshold
}

// NVMeHealth represents NVMe-specific health information
type NVMeHealth struct {
	CriticalWarning      int     `json:"critical_warning"`       // Critical warning flags
	Temperature          int     `json:"temperature"`            // Temperature (Celsius)
	AvailableSpare       int     `json:"available_spare"`        // Available spare (%)
	AvailableSpareThresh int     `json:"available_spare_thresh"` // Spare threshold (%)
	PercentUsed          int     `json:"percent_used"`           // Percentage used (endurance)
	DataUnitsRead        uint64  `json:"data_units_read"`        // Data units read
	DataUnitsWritten     uint64  `json:"data_units_written"`     // Data units written
	HostReadCommands     uint64  `json:"host_read_commands"`     // Host read commands
	HostWriteCommands    uint64  `json:"host_write_commands"`    // Host write commands
	ControllerBusyTime   uint64  `json:"controller_busy_time"`   // Controller busy time
	PowerCycles          uint64  `json:"power_cycles"`           // Power cycles
	PowerOnHours         uint64  `json:"power_on_hours"`         // Power on hours
	UnsafeShutdowns      uint64  `json:"unsafe_shutdowns"`       // Unsafe shutdowns
	MediaErrors          uint64  `json:"media_errors"`           // Media errors
	ErrorLogEntries      uint64  `json:"error_log_entries"`      // Error log entries
	WarningTempTime      uint64  `json:"warning_temp_time"`      // Warning temp time
	CriticalTempTime     uint64  `json:"critical_temp_time"`     // Critical temp time
}

// SelfTestEntry represents a SMART self-test log entry
type SelfTestEntry struct {
	Number         int       `json:"number"`          // Test number
	Description    string    `json:"description"`     // Test description (Short, Extended, etc.)
	Status         string    `json:"status"`          // Test status (Completed, Aborted, Failed, etc.)
	PercentRemain  int       `json:"percent_remain"`  // Percent remaining (0 if completed)
	Lifetime       uint64    `json:"lifetime"`        // Lifetime hours
	LBA            uint64    `json:"lba,omitempty"`   // First failed LBA (if failed)
	CompletedAt    time.Time `json:"completed_at"`    // When test completed
}

// ErrorLogEntry represents a SMART error log entry
type ErrorLogEntry struct {
	Number      int       `json:"number"`       // Error number
	Lifetime    uint64    `json:"lifetime"`     // Lifetime hours when error occurred
	State       string    `json:"state"`        // Device state
	Type        string    `json:"type"`         // Error type
	Details     string    `json:"details"`      // Error details
	OccurredAt  time.Time `json:"occurred_at"`  // When error occurred
}

// SMARTThresholds represents configurable SMART thresholds
type SMARTThresholds struct {
	// Temperature thresholds
	TempWarning  int `json:"temp_warning"`   // Temperature warning threshold (C)
	TempCritical int `json:"temp_critical"`  // Temperature critical threshold (C)

	// Reallocated sectors
	ReallocatedSectorsWarning  int `json:"reallocated_sectors_warning"`
	ReallocatedSectorsCritical int `json:"reallocated_sectors_critical"`

	// Pending sectors
	PendingSectorsWarning  int `json:"pending_sectors_warning"`
	PendingSectorsCritical int `json:"pending_sectors_critical"`

	// Power-on hours
	PowerOnHoursWarning  uint64 `json:"power_on_hours_warning"`
	PowerOnHoursCritical uint64 `json:"power_on_hours_critical"`

	// NVMe-specific
	NVMePercentUsedWarning  int `json:"nvme_percent_used_warning"`
	NVMePercentUsedCritical int `json:"nvme_percent_used_critical"`
	MediaErrorsWarning      uint64 `json:"media_errors_warning"`
	MediaErrorsCritical     uint64 `json:"media_errors_critical"`
}

// NewSMARTInfo creates a new SMARTInfo instance
func NewSMARTInfo(deviceID string, deviceType DeviceType) *SMARTInfo {
	return &SMARTInfo{
		DeviceID:    deviceID,
		DeviceType:  deviceType,
		Attributes:  make(map[int]*SMARTAttribute),
		LastUpdated: time.Now(),
		Metadata:    make(map[string]string),
	}
}

// DefaultSMARTThresholds returns default SMART thresholds
func DefaultSMARTThresholds() *SMARTThresholds {
	return &SMARTThresholds{
		TempWarning:                DefaultTempWarning,
		TempCritical:               DefaultTempCritical,
		ReallocatedSectorsWarning:  DefaultReallocatedSectorsWarning,
		ReallocatedSectorsCritical: DefaultReallocatedSectorsCritical,
		PendingSectorsWarning:      DefaultPendingSectorsWarning,
		PendingSectorsCritical:     DefaultPendingSectorsCritical,
		PowerOnHoursWarning:        DefaultPowerOnHoursWarning,
		PowerOnHoursCritical:       DefaultPowerOnHoursCritical,
		NVMePercentUsedWarning:     DefaultNVMePercentUsedWarning,
		NVMePercentUsedCritical:    DefaultNVMePercentUsedCritical,
		MediaErrorsWarning:         DefaultMediaErrorsWarning,
		MediaErrorsCritical:        DefaultMediaErrorsCritical,
	}
}

// EvaluateHealth evaluates disk health based on SMART data and thresholds
func (s *SMARTInfo) EvaluateHealth(thresholds *SMARTThresholds) (HealthStatus, string) {
	if !s.Available || !s.Enabled {
		return HealthUnknown, "SMART not available or not enabled"
	}

	// Check overall SMART status
	if s.OverallStatus == "FAILED" {
		return HealthFailed, "SMART overall status: FAILED"
	}

	reasons := []string{}
	status := HealthHealthy

	// Check temperature
	if s.TemperatureValid {
		if s.Temperature >= thresholds.TempCritical {
			status = HealthCritical
			reasons = append(reasons, "Temperature critical")
		} else if s.Temperature >= thresholds.TempWarning {
			if status == HealthHealthy {
				status = HealthWarning
			}
			reasons = append(reasons, "Temperature warning")
		}
	}

	// Check SATA/SAS attributes
	if len(s.Attributes) > 0 {
		// Reallocated sectors (ID 5)
		if attr, ok := s.Attributes[5]; ok {
			if attr.RawValue >= uint64(thresholds.ReallocatedSectorsCritical) {
				status = HealthCritical
				reasons = append(reasons, "Reallocated sectors critical")
			} else if attr.RawValue >= uint64(thresholds.ReallocatedSectorsWarning) {
				if status == HealthHealthy {
					status = HealthWarning
				}
				reasons = append(reasons, "Reallocated sectors warning")
			}
		}

		// Pending sectors (ID 197)
		if attr, ok := s.Attributes[197]; ok {
			if attr.RawValue >= uint64(thresholds.PendingSectorsCritical) {
				status = HealthCritical
				reasons = append(reasons, "Pending sectors critical")
			} else if attr.RawValue >= uint64(thresholds.PendingSectorsWarning) {
				if status == HealthHealthy {
					status = HealthWarning
				}
				reasons = append(reasons, "Pending sectors warning")
			}
		}

		// Check for any attribute failures
		for id, attr := range s.Attributes {
			if attr.FailureNear {
				status = HealthCritical
				reasons = append(reasons, "SMART attribute "+attr.Name+" (ID "+string(rune(id))+") failure imminent")
			}
		}
	}

	// Check NVMe health
	if s.NVMeHealth != nil {
		nvme := s.NVMeHealth

		// Critical warning flags
		if nvme.CriticalWarning != 0 {
			status = HealthCritical
			reasons = append(reasons, "NVMe critical warning flags set")
		}

		// Available spare
		if nvme.AvailableSpare < nvme.AvailableSpareThresh {
			status = HealthCritical
			reasons = append(reasons, "NVMe available spare below threshold")
		}

		// Percent used (endurance)
		if nvme.PercentUsed >= thresholds.NVMePercentUsedCritical {
			status = HealthCritical
			reasons = append(reasons, "NVMe endurance critical")
		} else if nvme.PercentUsed >= thresholds.NVMePercentUsedWarning {
			if status == HealthHealthy {
				status = HealthWarning
			}
			reasons = append(reasons, "NVMe endurance warning")
		}

		// Media errors
		if nvme.MediaErrors >= thresholds.MediaErrorsCritical {
			status = HealthCritical
			reasons = append(reasons, "NVMe media errors critical")
		} else if nvme.MediaErrors >= thresholds.MediaErrorsWarning {
			if status == HealthHealthy {
				status = HealthWarning
			}
			reasons = append(reasons, "NVMe media errors warning")
		}
	}

	// Check power-on hours
	if s.PowerOnHours >= thresholds.PowerOnHoursCritical {
		if status == HealthHealthy || status == HealthWarning {
			status = HealthWarning // Don't escalate to critical just for age
		}
		reasons = append(reasons, "Power-on hours high")
	} else if s.PowerOnHours >= thresholds.PowerOnHoursWarning {
		if status == HealthHealthy {
			status = HealthWarning
		}
		reasons = append(reasons, "Power-on hours warning")
	}

	// Check error log
	if s.ErrorLogCount > 0 {
		if status == HealthHealthy {
			status = HealthWarning
		}
		reasons = append(reasons, "Errors in SMART log")
	}

	// Build reason string
	reason := "All checks passed"
	if len(reasons) > 0 {
		reason = ""
		for i, r := range reasons {
			if i > 0 {
				reason += "; "
			}
			reason += r
		}
	}

	return status, reason
}

// HasRecentErrors returns true if there are recent errors in SMART log
func (s *SMARTInfo) HasRecentErrors(since time.Duration) bool {
	if len(s.ErrorLogs) == 0 {
		return false
	}

	cutoff := time.Now().Add(-since)
	for _, entry := range s.ErrorLogs {
		if entry.OccurredAt.After(cutoff) {
			return true
		}
	}
	return false
}

// GetLastSelfTest returns the most recent self-test entry
func (s *SMARTInfo) GetLastSelfTest() *SelfTestEntry {
	if len(s.SelfTestLogs) == 0 {
		return nil
	}
	return s.SelfTestLogs[0]
}

// IsTestRunning returns true if a self-test is currently running
func (s *SMARTInfo) IsTestRunning() bool {
	return s.SelfTestRemaining > 0 && s.SelfTestRemaining < 100
}
