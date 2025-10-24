// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// Helper for parsing JSON payload from a command request
func parseJSONPayload(cmd *proto.CommandRequest, out interface{}) error {
	if len(cmd.Payload) == 0 {
		return errors.New(errors.ServerRequestValidation, "empty payload")
	}
	return json.Unmarshal(cmd.Payload, out)
}

// Helper to create a successful response with JSON payload
func successResponse(
	requestID string,
	message string,
	data interface{},
) (*proto.CommandResponse, error) {
	response := APIResponse{
		Success: true,
		Result:  data,
	}

	payload, err := json.Marshal(response)
	if err != nil {
		return nil, errors.Wrap(err, errors.ServerResponseError)
	}

	return &proto.CommandResponse{
		RequestId: requestID,
		Success:   true,
		Message:   message,
		Payload:   payload,
	}, nil
}

// Helper to create an error response
func errorResponse(_ string, err error) (*proto.CommandResponse, error) {
	return nil, err
}

// Inventory operation handlers

func handleDiskList(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// Parse optional filter from payload
		var filter *types.DiskFilter
		if len(cmd.Payload) > 0 {
			filter = &types.DiskFilter{}
			if err := parseJSONPayload(cmd, filter); err != nil {
				return errorResponse(req.RequestId, err)
			}
		}

		disks := h.manager.GetInventory(filter)

		return successResponse(req.RequestId, "Disk inventory retrieved", map[string]interface{}{
			"disks": disks,
			"count": len(disks),
		})
	}
}

func handleDiskListAvailable(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		// List available disks only
		filter := &types.DiskFilter{
			States: []types.DiskState{types.DiskStateAvailable},
		}

		disks := h.manager.GetInventory(filter)

		return successResponse(req.RequestId, "Available disks retrieved", map[string]interface{}{
			"disks": disks,
			"count": len(disks),
		})
	}
}

func handleDiskGet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string `json:"device_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if request.DeviceID == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "device_id is required"),
			)
		}

		disk, err := h.manager.GetDisk(request.DeviceID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Disk details retrieved", disk)
	}
}

func handleDiskDiscover(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		if err := h.manager.TriggerDiscovery(ctx); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Disk discovery triggered", nil)
	}
}

// Health operation handlers

func handleDiskHealthCheck(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		if err := h.manager.TriggerHealthCheck(ctx); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Health check triggered", nil)
	}
}

func handleDiskHealthGet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string `json:"device_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		disk, err := h.manager.GetDisk(request.DeviceID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		healthData := map[string]interface{}{
			"device_id":     disk.DeviceID,
			"health_status": disk.Health,
			"health_reason": disk.HealthReason,
			"smart_status":  "",
			"updated_at":    disk.UpdatedAt,
		}

		if disk.SMARTInfo != nil {
			healthData["smart_status"] = disk.SMARTInfo.OverallStatus
			healthData["temperature"] = disk.SMARTInfo.Temperature
		}

		return successResponse(req.RequestId, "Disk health retrieved", healthData)
	}
}

func handleDiskSMARTGet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string `json:"device_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		disk, err := h.manager.GetDisk(request.DeviceID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		if disk.SMARTInfo == nil {
			return errorResponse(
				req.RequestId,
				errors.New(
					errors.DiskSMARTNotAvailable,
					"SMART data not available for this device",
				),
			)
		}

		return successResponse(req.RequestId, "SMART data retrieved", disk.SMARTInfo)
	}
}

func handleDiskSMARTRefresh(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		// Trigger health check which refreshes SMART data
		if err := h.manager.TriggerHealthCheck(ctx); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "SMART data refresh triggered", nil)
	}
}

// Probe operation handlers

func handleDiskProbeStart(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID  string `json:"device_id"`
			ProbeType string `json:"probe_type"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if request.DeviceID == "" {
			return errorResponse(
				req.RequestId,
				errors.New(errors.ServerRequestValidation, "device_id is required"),
			)
		}

		probeType := types.ProbeType(request.ProbeType)
		if probeType == "" {
			probeType = types.ProbeTypeQuick
		}

		ctx := context.Background()
		probeID, err := h.manager.TriggerProbe(ctx, request.DeviceID, probeType)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Probe started", map[string]interface{}{
			"probe_id": probeID,
		})
	}
}

func handleDiskProbeCancel(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			ProbeID string `json:"probe_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.CancelProbe(request.ProbeID); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Probe cancelled", nil)
	}
}

func handleDiskProbeGet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			ProbeID string `json:"probe_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		probe, err := h.manager.GetProbeExecution(request.ProbeID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Probe details retrieved", probe)
	}
}

func handleDiskProbeList(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		probes := h.manager.GetActiveProbes()
		return successResponse(req.RequestId, "Active probes retrieved", map[string]interface{}{
			"probes": probes,
			"count":  len(probes),
		})
	}
}

func handleDiskProbeHistory(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string `json:"device_id"`
			Limit    int    `json:"limit,omitempty"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		history, err := h.manager.GetProbeHistory(request.DeviceID, request.Limit)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Probe history retrieved", map[string]interface{}{
			"history": history,
			"count":   len(history),
		})
	}
}

// Probe schedule operation handlers

func handleDiskProbeScheduleList(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		schedules := h.manager.GetProbeSchedules()
		return successResponse(req.RequestId, "Probe schedules retrieved", map[string]interface{}{
			"schedules": schedules,
			"count":     len(schedules),
		})
	}
}

func handleDiskProbeScheduleGet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			ScheduleID string `json:"schedule_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		schedule, err := h.manager.GetProbeSchedule(request.ScheduleID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Probe schedule retrieved", schedule)
	}
}

func handleDiskProbeScheduleCreate(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var schedule types.ProbeSchedule
		if err := parseJSONPayload(cmd, &schedule); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.CreateProbeSchedule(&schedule); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Probe schedule created", &schedule)
	}
}

func handleDiskProbeScheduleUpdate(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			ScheduleID string              `json:"schedule_id"`
			Schedule   types.ProbeSchedule `json:"schedule"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.UpdateProbeSchedule(request.ScheduleID, &request.Schedule); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Probe schedule updated", nil)
	}
}

func handleDiskProbeScheduleDelete(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			ScheduleID string `json:"schedule_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.DeleteProbeSchedule(request.ScheduleID); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Probe schedule deleted", nil)
	}
}

func handleDiskProbeScheduleEnable(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			ScheduleID string `json:"schedule_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.EnableProbeSchedule(request.ScheduleID); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Probe schedule enabled", nil)
	}
}

func handleDiskProbeScheduleDisable(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			ScheduleID string `json:"schedule_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.DisableProbeSchedule(request.ScheduleID); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Probe schedule disabled", nil)
	}
}

// Topology operation handlers

func handleDiskTopologyGet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		topology, err := h.manager.GetTopology()
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Topology retrieved", topology)
	}
}

func handleDiskTopologyRefresh(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		if err := h.manager.RefreshTopology(ctx); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Topology refresh triggered", nil)
	}
}

func handleDiskTopologyControllers(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		controllers, err := h.manager.GetControllers()
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Controllers retrieved", map[string]interface{}{
			"controllers": controllers,
			"count":       len(controllers),
		})
	}
}

func handleDiskTopologyEnclosures(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		enclosures, err := h.manager.GetEnclosures()
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Enclosures retrieved", map[string]interface{}{
			"enclosures": enclosures,
			"count":      len(enclosures),
		})
	}
}

// State management operation handlers

func handleDiskStateGet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string `json:"device_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		deviceState, err := h.manager.GetDeviceState(request.DeviceID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Device state retrieved", deviceState)
	}
}

func handleDiskStateSet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string `json:"device_id"`
			State    string `json:"state"`
			Reason   string `json:"reason,omitempty"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		diskState := types.DiskState(request.State)
		if err := h.manager.SetDiskState(request.DeviceID, diskState, request.Reason); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Device state updated", nil)
	}
}

func handleDiskValidate(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string `json:"device_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		result, err := h.manager.ValidateDisk(request.DeviceID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Disk validation completed", result)
	}
}

func handleDiskQuarantine(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string `json:"device_id"`
			Reason   string `json:"reason,omitempty"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.QuarantineDisk(request.DeviceID, request.Reason); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Disk quarantined", nil)
	}
}

// Metadata operation handlers

func handleDiskTagsSet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string            `json:"device_id"`
			Tags     map[string]string `json:"tags"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.SetDiskTags(request.DeviceID, request.Tags); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Disk tags updated", nil)
	}
}

func handleDiskTagsDelete(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string   `json:"device_id"`
			TagKeys  []string `json:"tag_keys"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.DeleteDiskTags(request.DeviceID, request.TagKeys); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Disk tags deleted", nil)
	}
}

func handleDiskNotesSet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string `json:"device_id"`
			Notes    string `json:"notes"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.SetDiskNotes(request.DeviceID, request.Notes); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Disk notes updated", nil)
	}
}

// Statistics operation handlers

func handleDiskStatsGet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var request struct {
			DeviceID string `json:"device_id"`
		}
		if err := parseJSONPayload(cmd, &request); err != nil {
			return errorResponse(req.RequestId, err)
		}

		stats, err := h.manager.GetDeviceStatistics(request.DeviceID)
		if err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Device statistics retrieved", stats)
	}
}

func handleDiskStatsGlobal(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		stats := h.manager.GetStatistics()
		return successResponse(req.RequestId, "Global statistics retrieved", stats)
	}
}

// Monitoring operation handlers

func handleDiskMonitoringGet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		config := h.manager.GetMonitoringConfig()
		return successResponse(req.RequestId, "Monitoring configuration retrieved", config)
	}
}

func handleDiskMonitoringSet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var config types.MonitoringConfig
		if err := parseJSONPayload(cmd, &config); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.SetMonitoringConfig(&config); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Monitoring configuration updated", nil)
	}
}

// Configuration operation handlers

func handleDiskConfigGet(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		config := h.manager.GetConfig()
		return successResponse(req.RequestId, "Configuration retrieved", config)
	}
}

func handleDiskConfigUpdate(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var config types.DiskManagerConfig
		if err := parseJSONPayload(cmd, &config); err != nil {
			return errorResponse(req.RequestId, err)
		}

		if err := h.manager.UpdateConfig(&config); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Configuration updated", nil)
	}
}

func handleDiskConfigReload(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		if err := h.manager.ReloadConfig(); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Configuration reloaded", nil)
	}
}
