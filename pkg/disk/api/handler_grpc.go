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

		return successResponse(req.RequestId, "Disk inventory retrieved", disks)
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

func handleDiskRefresh(h *DiskHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		// Refresh is same as discovery trigger
		if err := h.manager.TriggerDiscovery(ctx); err != nil {
			return errorResponse(req.RequestId, err)
		}

		return successResponse(req.RequestId, "Disk information refreshed", nil)
	}
}

// Health operation handlers

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
