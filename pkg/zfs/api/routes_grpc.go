// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// RegisterZFSGRPCHandlers registers all ZFS-related command handlers with Toggle
func RegisterZFSGRPCHandlers(poolHandler *PoolHandler, datasetHandler *DatasetHandler) {
	// Pool operations
	client.RegisterCommandHandler(CmdPoolList, handlePoolList(poolHandler))
	client.RegisterCommandHandler(CmdPoolStatus, handlePoolStatus(poolHandler))
	client.RegisterCommandHandler(CmdPoolCreate, handlePoolCreate(poolHandler))
	client.RegisterCommandHandler(CmdPoolDestroy, handlePoolDestroy(poolHandler))
	client.RegisterCommandHandler(CmdPoolImport, handlePoolImport(poolHandler))
	client.RegisterCommandHandler(CmdPoolExport, handlePoolExport(poolHandler))
	client.RegisterCommandHandler(CmdPoolPropertyList, handlePoolPropertyList(poolHandler))
	client.RegisterCommandHandler(CmdPoolPropertyGet, handlePoolPropertyGet(poolHandler))
	client.RegisterCommandHandler(CmdPoolPropertySet, handlePoolPropertySet(poolHandler))
	client.RegisterCommandHandler(CmdPoolScrub, handlePoolScrub(poolHandler))
	client.RegisterCommandHandler(CmdPoolResilver, handlePoolResilver(poolHandler))
	client.RegisterCommandHandler(CmdPoolDeviceAttach, handlePoolDeviceAttach(poolHandler))
	client.RegisterCommandHandler(CmdPoolDeviceDetach, handlePoolDeviceDetach(poolHandler))
	client.RegisterCommandHandler(CmdPoolDeviceReplace, handlePoolDeviceReplace(poolHandler))

	// Dataset operations
	client.RegisterCommandHandler(CmdDatasetList, handleDatasetList(datasetHandler))
	client.RegisterCommandHandler(CmdDatasetDelete, handleDatasetDelete(datasetHandler))
	client.RegisterCommandHandler(CmdDatasetRename, handleDatasetRename(datasetHandler))
	client.RegisterCommandHandler(CmdDatasetDiff, handleDatasetDiff(datasetHandler))

	// Property operations
	client.RegisterCommandHandler(CmdDatasetPropertyList, handleDatasetPropertyList(datasetHandler))
	client.RegisterCommandHandler(CmdDatasetPropertyGet, handleDatasetPropertyGet(datasetHandler))
	client.RegisterCommandHandler(CmdDatasetPropertySet, handleDatasetPropertySet(datasetHandler))
	client.RegisterCommandHandler(
		CmdDatasetPropertyInherit,
		handleDatasetPropertyInherit(datasetHandler),
	)

	// Filesystem operations
	client.RegisterCommandHandler(CmdFilesystemList, handleFilesystemList(datasetHandler))
	client.RegisterCommandHandler(CmdFilesystemCreate, handleFilesystemCreate(datasetHandler))
	client.RegisterCommandHandler(CmdFilesystemMount, handleFilesystemMount(datasetHandler))
	client.RegisterCommandHandler(CmdFilesystemUnmount, handleFilesystemUnmount(datasetHandler))

	// Volume operations
	client.RegisterCommandHandler(CmdVolumeList, handleVolumeList(datasetHandler))
	client.RegisterCommandHandler(CmdVolumeCreate, handleVolumeCreate(datasetHandler))

	// Snapshot operations
	client.RegisterCommandHandler(CmdSnapshotList, handleSnapshotList(datasetHandler))
	client.RegisterCommandHandler(CmdSnapshotCreate, handleSnapshotCreate(datasetHandler))
	client.RegisterCommandHandler(CmdSnapshotRollback, handleSnapshotRollback(datasetHandler))

	// Clone operations
	client.RegisterCommandHandler(CmdCloneCreate, handleCloneCreate(datasetHandler))
	client.RegisterCommandHandler(CmdClonePromote, handleClonePromote(datasetHandler))

	// Bookmark operations
	client.RegisterCommandHandler(CmdBookmarkList, handleBookmarkList(datasetHandler))
	client.RegisterCommandHandler(CmdBookmarkCreate, handleBookmarkCreate(datasetHandler))

	// Permission operations
	client.RegisterCommandHandler(CmdPermissionList, handlePermissionList(datasetHandler))
	client.RegisterCommandHandler(CmdPermissionAllow, handlePermissionAllow(datasetHandler))
	client.RegisterCommandHandler(CmdPermissionUnallow, handlePermissionUnallow(datasetHandler))

	// Share operations
	client.RegisterCommandHandler(CmdShareDataset, handleShareDataset(datasetHandler))
	client.RegisterCommandHandler(CmdUnshareDataset, handleUnshareDataset(datasetHandler))

	// Data transfer operations
	client.RegisterCommandHandler(CmdTransferSend, handleTransferSend(datasetHandler))
	client.RegisterCommandHandler(CmdTransferResumeToken, handleTransferResumeToken(datasetHandler))
}

// Helper for parsing JSON payload from a command request
func parseJSONPayload(cmd *proto.CommandRequest, out interface{}) error {
	if len(cmd.Payload) == 0 {
		return fmt.Errorf("empty payload")
	}
	return json.Unmarshal(cmd.Payload, out)
}

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// Define context keys - using typed constants to avoid string literal warnings
const (
	requestIDKey = contextKey("request_id")
)

// Helper to create a valid context for gRPC handlers
func createHandlerContext(req *proto.ToggleRequest) context.Context {
	// Create a context with request ID
	ctx := context.Background()
	if req.RequestId != "" {
		ctx = context.WithValue(ctx, requestIDKey, req.RequestId)
	}
	return ctx
}

// Helper to create a successful response with JSON payload
func successResponse(
	requestID string,
	message string,
	data interface{},
) (*proto.CommandResponse, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"result": data,
	})
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

func successPoolResponse(
	requestID string,
	message string,
	data interface{},
) (*proto.CommandResponse, error) {
	payload, err := json.Marshal(data)
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
