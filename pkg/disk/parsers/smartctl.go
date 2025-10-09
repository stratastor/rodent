// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"encoding/json"
	"time"

	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// SmartctlJSON represents the JSON output structure from smartctl
type SmartctlJSON struct {
	JSONFormatVersion []int `json:"json_format_version"`
	Smartctl          struct {
		Version      []int  `json:"version"`
		PreRelease   bool   `json:"pre_release"`
		SVNRevision  string `json:"svn_revision"`
		PlatformInfo string `json:"platform_info"`
		BuildInfo    string `json:"build_info"`
		ExitStatus   int    `json:"exit_status"`
	} `json:"smartctl"`
	Device struct {
		Name     string `json:"name"`
		InfoName string `json:"info_name"`
		Type     string `json:"type"`
		Protocol string `json:"protocol"`
	} `json:"device"`
	ModelName       string `json:"model_name"`
	SerialNumber    string `json:"serial_number"`
	FirmwareVersion string `json:"firmware_version"`
	UserCapacity    struct {
		Blocks uint64 `json:"blocks"`
		Bytes  uint64 `json:"bytes"`
	} `json:"user_capacity"`
	LogicalBlockSize int `json:"logical_block_size"`
	SmartSupport     struct {
		Available bool `json:"available"`
		Enabled   bool `json:"enabled"`
	} `json:"smart_support"`
	SmartStatus struct {
		Passed bool `json:"passed"`
		NVMe   *struct {
			Value int `json:"value"`
		} `json:"nvme,omitempty"`
	} `json:"smart_status"`

	// SATA/SAS SMART attributes
	ATASmartAttributes *struct {
		Revision int `json:"revision"`
		Table    []struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			Value      int    `json:"value"`
			Worst      int    `json:"worst"`
			Thresh     int    `json:"thresh"`
			WhenFailed string `json:"when_failed"`
			Flags      struct {
				Value         int    `json:"value"`
				String        string `json:"string"`
				Prefail       bool   `json:"prefail"`
				Updated       bool   `json:"updated"`
				Performance   bool   `json:"performance"`
				ErrorRate     bool   `json:"error_rate"`
				EventCount    bool   `json:"event_count"`
				AutoKeep      bool   `json:"auto_keep"`
			} `json:"flags"`
			Raw struct {
				Value  uint64 `json:"value"`
				String string `json:"string"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes,omitempty"`

	// NVMe SMART data
	NVMeSmartHealthInformationLog *struct {
		CriticalWarning          int    `json:"critical_warning"`
		Temperature              int    `json:"temperature"`
		AvailableSpare           int    `json:"available_spare"`
		AvailableSpareThreshold  int    `json:"available_spare_threshold"`
		PercentageUsed           int    `json:"percentage_used"`
		DataUnitsRead            uint64 `json:"data_units_read"`
		DataUnitsWritten         uint64 `json:"data_units_written"`
		HostReads                uint64 `json:"host_reads"`
		HostWrites               uint64 `json:"host_writes"`
		ControllerBusyTime       uint64 `json:"controller_busy_time"`
		PowerCycles              uint64 `json:"power_cycles"`
		PowerOnHours             uint64 `json:"power_on_hours"`
		UnsafeShutdowns          uint64 `json:"unsafe_shutdowns"`
		MediaErrors              uint64 `json:"media_errors"`
		NumErrLogEntries         uint64 `json:"num_err_log_entries"`
		WarningTempTime          uint64 `json:"warning_temp_time"`
		CriticalCompTime         uint64 `json:"critical_comp_time,omitempty"`
		TemperatureSensors       []int  `json:"temperature_sensors,omitempty"`
	} `json:"nvme_smart_health_information_log,omitempty"`

	PowerOnTime *struct {
		Hours int `json:"hours"`
	} `json:"power_on_time,omitempty"`
	PowerCycleCount *int `json:"power_cycle_count,omitempty"`
	Temperature     *struct {
		Current int `json:"current"`
	} `json:"temperature,omitempty"`
}

// ParseSmartctlJSON parses smartctl JSON output into SMARTInfo
func ParseSmartctlJSON(jsonData []byte, deviceID string) (*types.SMARTInfo, error) {
	var smart SmartctlJSON
	if err := json.Unmarshal(jsonData, &smart); err != nil {
		return nil, errors.Wrap(err, errors.DiskSMARTParseFailed).
			WithMetadata("device_id", deviceID).
			WithMetadata("operation", "unmarshal_json")
	}

	// Determine device type
	deviceType := types.DeviceTypeUnknown
	switch smart.Device.Protocol {
	case "NVMe":
		deviceType = types.DeviceTypeNVMe
	case "ATA", "SATA":
		deviceType = types.DeviceTypeSSD // Could be HDD or SSD, need to check ROTA
	case "SCSI", "SAS":
		deviceType = types.DeviceTypeSSD // Could be HDD or SSD
	}

	info := types.NewSMARTInfo(deviceID, deviceType)
	info.Enabled = smart.SmartSupport.Enabled
	info.Available = smart.SmartSupport.Available

	// Overall status
	if smart.SmartStatus.Passed {
		info.OverallStatus = "PASSED"
	} else {
		info.OverallStatus = "FAILED"
	}

	// Parse based on device type
	if smart.NVMeSmartHealthInformationLog != nil {
		// NVMe device
		info.DeviceType = types.DeviceTypeNVMe
		parseNVMeHealth(smart.NVMeSmartHealthInformationLog, info)
	}

	if smart.ATASmartAttributes != nil {
		// SATA/SAS device
		parseATAAttributes(smart.ATASmartAttributes, info)
	}

	// Temperature
	if smart.Temperature != nil {
		info.Temperature = smart.Temperature.Current
		info.TemperatureValid = true
	} else if smart.NVMeSmartHealthInformationLog != nil {
		info.Temperature = smart.NVMeSmartHealthInformationLog.Temperature
		info.TemperatureValid = true
	}

	// Power-on hours
	if smart.PowerOnTime != nil {
		info.PowerOnHours = uint64(smart.PowerOnTime.Hours)
	} else if smart.NVMeSmartHealthInformationLog != nil {
		info.PowerOnHours = smart.NVMeSmartHealthInformationLog.PowerOnHours
	}

	// Power cycles
	if smart.PowerCycleCount != nil {
		info.PowerCycles = uint64(*smart.PowerCycleCount)
	} else if smart.NVMeSmartHealthInformationLog != nil {
		info.PowerCycles = smart.NVMeSmartHealthInformationLog.PowerCycles
	}

	info.LastUpdated = time.Now()

	return info, nil
}

// parseNVMeHealth parses NVMe SMART health information
func parseNVMeHealth(nvme *struct {
	CriticalWarning         int    `json:"critical_warning"`
	Temperature             int    `json:"temperature"`
	AvailableSpare          int    `json:"available_spare"`
	AvailableSpareThreshold int    `json:"available_spare_threshold"`
	PercentageUsed          int    `json:"percentage_used"`
	DataUnitsRead           uint64 `json:"data_units_read"`
	DataUnitsWritten        uint64 `json:"data_units_written"`
	HostReads               uint64 `json:"host_reads"`
	HostWrites              uint64 `json:"host_writes"`
	ControllerBusyTime      uint64 `json:"controller_busy_time"`
	PowerCycles             uint64 `json:"power_cycles"`
	PowerOnHours            uint64 `json:"power_on_hours"`
	UnsafeShutdowns         uint64 `json:"unsafe_shutdowns"`
	MediaErrors             uint64 `json:"media_errors"`
	NumErrLogEntries        uint64 `json:"num_err_log_entries"`
	WarningTempTime         uint64 `json:"warning_temp_time"`
	CriticalCompTime        uint64 `json:"critical_comp_time,omitempty"`
	TemperatureSensors      []int  `json:"temperature_sensors,omitempty"`
}, info *types.SMARTInfo) {
	info.NVMeHealth = &types.NVMeHealth{
		CriticalWarning:      nvme.CriticalWarning,
		Temperature:          nvme.Temperature,
		AvailableSpare:       nvme.AvailableSpare,
		AvailableSpareThresh: nvme.AvailableSpareThreshold,
		PercentUsed:          nvme.PercentageUsed,
		DataUnitsRead:        nvme.DataUnitsRead,
		DataUnitsWritten:     nvme.DataUnitsWritten,
		HostReadCommands:     nvme.HostReads,
		HostWriteCommands:    nvme.HostWrites,
		ControllerBusyTime:   nvme.ControllerBusyTime,
		PowerCycles:          nvme.PowerCycles,
		PowerOnHours:         nvme.PowerOnHours,
		UnsafeShutdowns:      nvme.UnsafeShutdowns,
		MediaErrors:          nvme.MediaErrors,
		ErrorLogEntries:      nvme.NumErrLogEntries,
		WarningTempTime:      nvme.WarningTempTime,
		CriticalTempTime:     nvme.CriticalCompTime,
	}

	info.ErrorLogCount = int(nvme.NumErrLogEntries)
}

// parseATAAttributes parses ATA SMART attributes
func parseATAAttributes(ata *struct {
	Revision int `json:"revision"`
	Table    []struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Value      int    `json:"value"`
		Worst      int    `json:"worst"`
		Thresh     int    `json:"thresh"`
		WhenFailed string `json:"when_failed"`
		Flags      struct {
			Value         int    `json:"value"`
			String        string `json:"string"`
			Prefail       bool   `json:"prefail"`
			Updated       bool   `json:"updated"`
			Performance   bool   `json:"performance"`
			ErrorRate     bool   `json:"error_rate"`
			EventCount    bool   `json:"event_count"`
			AutoKeep      bool   `json:"auto_keep"`
		} `json:"flags"`
		Raw struct {
			Value  uint64 `json:"value"`
			String string `json:"string"`
		} `json:"raw"`
	} `json:"table"`
}, info *types.SMARTInfo) {
	for _, attr := range ata.Table {
		attrType := "Old_age"
		if attr.Flags.Prefail {
			attrType = "Pre-fail"
		}

		updated := "Offline"
		if attr.Flags.Updated {
			updated = "Always"
		}

		failureNear := false
		if attr.Value <= attr.Thresh && attr.Thresh > 0 {
			failureNear = true
		}

		info.Attributes[attr.ID] = &types.SMARTAttribute{
			ID:          attr.ID,
			Name:        attr.Name,
			Value:       attr.Value,
			Worst:       attr.Worst,
			Threshold:   attr.Thresh,
			RawValue:    attr.Raw.Value,
			WhenFailed:  attr.WhenFailed,
			Flags:       attr.Flags.String,
			Type:        attrType,
			Updated:     updated,
			FailureNear: failureNear,
		}

		// Extract specific important attributes
		switch attr.ID {
		case 194: // Temperature
			info.Temperature = int(attr.Raw.Value)
			info.TemperatureValid = true
		case 9: // Power-On Hours
			info.PowerOnHours = attr.Raw.Value
		case 12: // Power Cycle Count
			info.PowerCycles = attr.Raw.Value
		}
	}
}
