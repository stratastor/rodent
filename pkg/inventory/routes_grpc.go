// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"context"
	"encoding/json"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterInventoryGRPCHandlers registers all inventory-related command handlers with Toggle
func RegisterInventoryGRPCHandlers(handler *Handler) {
	client.RegisterCommandHandler(proto.CmdInventoryGet, handleInventoryGet(handler))
}

// handleInventoryGet returns a handler for getting the complete Rodent inventory
func handleInventoryGet(h *Handler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		ctx := context.Background()

		// Parse query options from payload (if provided)
		opts := CollectOptions{
			DetailLevel: DetailLevelBasic, // Default
		}

		if len(cmd.Payload) > 0 {
			var payload struct {
				DetailLevel string   `json:"detail_level,omitempty"`
				Include     []string `json:"include,omitempty"`
				Exclude     []string `json:"exclude,omitempty"`
			}
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				// Log but don't fail - use defaults
				h.logger.Warn("Failed to parse inventory options from payload", "error", err)
			} else {
				if payload.DetailLevel != "" {
					opts.DetailLevel = DetailLevel(payload.DetailLevel)
				}
				opts.Include = payload.Include
				opts.Exclude = payload.Exclude
			}
		}

		h.logger.Debug("Collecting inventory via gRPC",
			"request_id", req.RequestId,
			"detail_level", opts.DetailLevel)

		// Collect inventory
		inventory, err := h.collector.CollectInventory(ctx, opts)
		if err != nil {
			return errorResponse(req.RequestId, errors.Wrap(err, errors.ServerInternalError))
		}

		return successResponse(req.RequestId, "Inventory collected", inventory)
	}
}

// successResponse creates a successful response with the provided data
func successResponse(requestID string, message string, data interface{}) (*proto.CommandResponse, error) {
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

// errorResponse creates an error response with the provided error
func errorResponse(_ string, err error) (*proto.CommandResponse, error) {
	return nil, err
}
