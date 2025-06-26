// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// handleDatasetList returns a handler for listing ZFS datasets
func handleDatasetList(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var listConfig dataset.ListConfig
		if err := parseJSONPayload(cmd, &listConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's List method with the correct signature
		result, err := h.manager.List(ctx, listConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetList)
		}

		// Return success response with the result
		return successResponse(req.RequestId, "ZFS datasets list", result)
	}
}

// handleDatasetDelete returns a handler for deleting a ZFS dataset
func handleDatasetDelete(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var destroyConfig dataset.DestroyConfig
		if err := parseJSONPayload(cmd, &destroyConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if destroyConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "dataset name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Destroy method
		result, err := h.manager.Destroy(ctx, destroyConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetDestroy)
		}

		// Return success response with the result
		return successResponse(req.RequestId, "Dataset destroyed successfully", result)
	}
}

// handleDatasetRename returns a handler for renaming a ZFS dataset
func handleDatasetRename(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var renameConfig dataset.RenameConfig
		if err := parseJSONPayload(cmd, &renameConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if renameConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "dataset name is required")
		}

		if renameConfig.NewName == "" {
			return nil, errors.New(errors.ServerRequestValidation, "new dataset name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Rename method
		err := h.manager.Rename(ctx, renameConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetRename)
		}

		return successResponse(req.RequestId, "Dataset renamed successfully", nil)
	}
}

// handleDatasetDiff returns a handler for comparing ZFS datasets
func handleDatasetDiff(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var diffConfig dataset.DiffConfig
		if err := parseJSONPayload(cmd, &diffConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if len(diffConfig.Names) < 2 {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"at least two dataset names are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's Diff method
		result, err := h.manager.Diff(ctx, diffConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetOperation)
		}

		return successResponse(req.RequestId, "Dataset difference completed", result)
	}
}

// handleDatasetPropertyList returns a handler for listing ZFS dataset properties
func handleDatasetPropertyList(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameConfig dataset.NameConfig
		if err := parseJSONPayload(cmd, &nameConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if nameConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "dataset name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's ListProperties method
		properties, err := h.manager.ListProperties(ctx, nameConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetGetProperty)
		}

		return successResponse(req.RequestId, "Dataset properties", properties)
	}
}

// handleDatasetPropertyGet returns a handler for getting a ZFS dataset property
func handleDatasetPropertyGet(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var propertyConfig dataset.PropertyConfig
		if err := parseJSONPayload(cmd, &propertyConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if propertyConfig.Name == "" || propertyConfig.Property == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"dataset name and property name are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's GetProperty method
		property, err := h.manager.GetProperty(ctx, propertyConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetGetProperty)
		}

		return successResponse(req.RequestId, "Dataset property", property)
	}
}

// handleDatasetPropertySet returns a handler for setting a ZFS dataset property
func handleDatasetPropertySet(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var setPropertyConfig dataset.SetPropertyConfig
		if err := parseJSONPayload(cmd, &setPropertyConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if setPropertyConfig.Name == "" || setPropertyConfig.Property == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"dataset name and property name are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's SetProperty method
		err := h.manager.SetProperty(ctx, setPropertyConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetSetProperty)
		}

		return successResponse(req.RequestId, "Dataset property set", nil)
	}
}

// handleDatasetPropertyInherit returns a handler for inheriting a ZFS dataset property
func handleDatasetPropertyInherit(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var inheritConfig dataset.InheritConfig
		if err := parseJSONPayload(cmd, &inheritConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if inheritConfig.Name == "" || inheritConfig.Property == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"dataset name and property name are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's InheritProperty method
		err := h.manager.InheritProperty(ctx, inheritConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetOperation)
		}

		return successResponse(req.RequestId, "Dataset property inherited", nil)
	}
}

// handleFilesystemList returns a handler for listing ZFS filesystems
func handleFilesystemList(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var listConfig dataset.ListConfig
		if err := parseJSONPayload(cmd, &listConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Set the type to filesystem
		listConfig.Type = "filesystem"

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's List method
		result, err := h.manager.List(ctx, listConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetList)
		}

		return successResponse(req.RequestId, "ZFS filesystems list", result)
	}
}

// handleFilesystemCreate returns a handler for creating a ZFS filesystem
func handleFilesystemCreate(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var filesystemConfig dataset.FilesystemConfig
		if err := parseJSONPayload(cmd, &filesystemConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if filesystemConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "filesystem name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the CreateFilesystem method
		result, err := h.manager.CreateFilesystem(ctx, filesystemConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetCreate)
		}

		return successResponse(req.RequestId, "Filesystem created successfully", result)
	}
}

// handleFilesystemMount returns a handler for mounting a ZFS filesystem
func handleFilesystemMount(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var mountConfig dataset.MountConfig
		if err := parseJSONPayload(cmd, &mountConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if mountConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "filesystem name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the Mount method
		err := h.manager.Mount(ctx, mountConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSMountOperationFailed)
		}

		return successResponse(req.RequestId, "Filesystem mounted successfully", nil)
	}
}

// handleFilesystemUnmount returns a handler for unmounting a ZFS filesystem
func handleFilesystemUnmount(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var unmountConfig dataset.UnmountConfig
		if err := parseJSONPayload(cmd, &unmountConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if unmountConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "filesystem name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the Unmount method
		err := h.manager.Unmount(ctx, unmountConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSUnmountOperationFailed)
		}

		return successResponse(req.RequestId, "Filesystem unmounted successfully", nil)
	}
}

// handleVolumeList returns a handler for listing ZFS volumes
func handleVolumeList(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var listConfig dataset.ListConfig
		if err := parseJSONPayload(cmd, &listConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Set the type to volume
		listConfig.Type = "volume"

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's List method
		result, err := h.manager.List(ctx, listConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetList)
		}

		return successResponse(req.RequestId, "ZFS volumes list", result)
	}
}

// handleVolumeCreate returns a handler for creating a ZFS volume
func handleVolumeCreate(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var volumeConfig dataset.VolumeConfig
		if err := parseJSONPayload(cmd, &volumeConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if volumeConfig.Name == "" || volumeConfig.Size == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"volume name and size are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the CreateVolume method
		result, err := h.manager.CreateVolume(ctx, volumeConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSVolumeOperationFailed)
		}

		return successResponse(req.RequestId, "Volume created successfully", result)
	}
}

// handleSnapshotList returns a handler for listing ZFS snapshots
func handleSnapshotList(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var listConfig dataset.ListConfig
		if err := parseJSONPayload(cmd, &listConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Set the type to snapshot
		listConfig.Type = "snapshot"

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's List method
		result, err := h.manager.List(ctx, listConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotList)
		}

		return successResponse(req.RequestId, "ZFS snapshots list", result)
	}
}

// handleSnapshotCreate returns a handler for creating a ZFS snapshot
func handleSnapshotCreate(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var snapshotConfig dataset.SnapshotConfig
		if err := parseJSONPayload(cmd, &snapshotConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if snapshotConfig.Name == "" || snapshotConfig.SnapName == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"dataset name and snapshot name are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the CreateSnapshot method
		err := h.manager.CreateSnapshot(ctx, snapshotConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotFailed)
		}

		return successResponse(req.RequestId, "Snapshot created successfully", nil)
	}
}

// handleSnapshotRollback returns a handler for rolling back to a ZFS snapshot
func handleSnapshotRollback(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var rollbackConfig dataset.RollbackConfig
		if err := parseJSONPayload(cmd, &rollbackConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if rollbackConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "snapshot name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the Rollback method
		err := h.manager.Rollback(ctx, rollbackConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSSnapshotRollback)
		}

		return successResponse(req.RequestId, "Rolled back to snapshot successfully", nil)
	}
}

// handleCloneCreate returns a handler for creating a ZFS clone
func handleCloneCreate(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var cloneConfig dataset.CloneConfig
		if err := parseJSONPayload(cmd, &cloneConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if cloneConfig.Name == "" || cloneConfig.CloneName == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"snapshot name and clone name are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the Clone method
		err := h.manager.Clone(ctx, cloneConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSCloneError)
		}

		return successResponse(req.RequestId, "Clone created successfully", nil)
	}
}

// handleClonePromote returns a handler for promoting a ZFS clone
func handleClonePromote(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameConfig dataset.NameConfig
		if err := parseJSONPayload(cmd, &nameConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if nameConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "clone name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the PromoteClone method
		err := h.manager.PromoteClone(ctx, nameConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSClonePromoteFailed)
		}

		return successResponse(req.RequestId, "Clone promoted successfully", nil)
	}
}

// handleBookmarkList returns a handler for listing ZFS bookmarks
func handleBookmarkList(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var listConfig dataset.ListConfig
		if err := parseJSONPayload(cmd, &listConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Set the type to bookmark
		listConfig.Type = "bookmark"

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the manager's List method
		result, err := h.manager.List(ctx, listConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSBookmarkFailed)
		}

		return successResponse(req.RequestId, "ZFS bookmarks list", result)
	}
}

// handleBookmarkCreate returns a handler for creating a ZFS bookmark
func handleBookmarkCreate(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var bookmarkConfig dataset.BookmarkConfig
		if err := parseJSONPayload(cmd, &bookmarkConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if bookmarkConfig.Name == "" || bookmarkConfig.BookmarkName == "" {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"snapshot name and bookmark name are required",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the CreateBookmark method
		err := h.manager.CreateBookmark(ctx, bookmarkConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSBookmarkFailed)
		}

		return successResponse(req.RequestId, "Bookmark created successfully", nil)
	}
}

// handlePermissionList returns a handler for listing ZFS permissions
func handlePermissionList(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameConfig dataset.NameConfig
		if err := parseJSONPayload(cmd, &nameConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if nameConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "dataset name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the ListPermissions method
		result, err := h.manager.ListPermissions(ctx, nameConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPermissionError)
		}

		return successResponse(req.RequestId, "Dataset permissions", result)
	}
}

// handlePermissionAllow returns a handler for allowing ZFS permissions
func handlePermissionAllow(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var allowConfig dataset.AllowConfig
		if err := parseJSONPayload(cmd, &allowConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if allowConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "dataset name is required")
		}

		if len(allowConfig.Permissions) == 0 {
			return nil, errors.New(errors.ServerRequestValidation, "permissions are required")
		}

		if !allowConfig.Everyone && len(allowConfig.Users) == 0 && len(allowConfig.Groups) == 0 {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"either everyone, users, or groups must be specified",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the Allow method
		err := h.manager.Allow(ctx, allowConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPermissionError)
		}

		return successResponse(req.RequestId, "Permissions allowed successfully", nil)
	}
}

// handlePermissionUnallow returns a handler for removing ZFS permissions
func handlePermissionUnallow(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var unallowConfig dataset.UnallowConfig
		if err := parseJSONPayload(cmd, &unallowConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if unallowConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "dataset name is required")
		}

		if !unallowConfig.Everyone && len(unallowConfig.Users) == 0 &&
			len(unallowConfig.Groups) == 0 {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"either everyone, users, or groups must be specified",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the Unallow method
		err := h.manager.Unallow(ctx, unallowConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSPermissionError)
		}

		return successResponse(req.RequestId, "Permissions removed successfully", nil)
	}
}

// handleShareDataset returns a handler for sharing a ZFS dataset
func handleShareDataset(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var shareConfig dataset.ShareConfig
		if err := parseJSONPayload(cmd, &shareConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if shareConfig.Name == "" && !shareConfig.All {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"dataset name is required when not sharing all",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the Share method
		err := h.manager.Share(ctx, shareConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetOperation)
		}

		return successResponse(req.RequestId, "Dataset shared successfully", nil)
	}
}

// handleUnshareDataset returns a handler for unsharing a ZFS dataset
func handleUnshareDataset(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var unshareConfig dataset.UnshareConfig
		if err := parseJSONPayload(cmd, &unshareConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if unshareConfig.Name == "" && !unshareConfig.All {
			return nil, errors.New(
				errors.ServerRequestValidation,
				"dataset name is required when not unsharing all",
			)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the Unshare method
		err := h.manager.Unshare(ctx, unshareConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetOperation)
		}

		return successResponse(req.RequestId, "Dataset unshared successfully", nil)
	}
}

// handleTransferSend returns a handler for sending a ZFS dataset
func handleTransferSend(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var transferConfig dataset.TransferConfig
		if err := parseJSONPayload(cmd, &transferConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the SendReceive method
		err := h.manager.SendReceive(ctx, transferConfig.SendConfig, transferConfig.ReceiveConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetSend)
		}

		return successResponse(req.RequestId, "Dataset transfer completed successfully", nil)
	}
}

// handleTransferResumeToken returns a handler for getting a ZFS resume token
func handleTransferResumeToken(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var nameConfig dataset.NameConfig
		if err := parseJSONPayload(cmd, &nameConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Validate required fields
		if nameConfig.Name == "" {
			return nil, errors.New(errors.ServerRequestValidation, "dataset name is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Call the GetResumeToken method
		token, err := h.manager.GetResumeToken(ctx, nameConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetNoReceiveToken)
		}

		return successResponse(req.RequestId, "Resume token retrieved", token)
	}
}

// Managed transfer handlers

// handleManagedTransferStart returns a handler for starting a managed ZFS transfer
func handleManagedTransferStart(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var transferConfig dataset.TransferConfig
		if err := parseJSONPayload(cmd, &transferConfig); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		// Start the managed transfer
		transferID, err := h.transferManager.StartTransfer(ctx, transferConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ZFSDatasetSend)
		}

		return successResponse(req.RequestId, "Managed transfer started", map[string]string{"transfer_id": transferID})
	}
}

// handleManagedTransferList returns a handler for listing all transfers
func handleManagedTransferList(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var listRequest struct {
			Type string `json:"type,omitempty"`
		}
		
		// Parse optional payload for transfer type filter
		if len(cmd.Payload) > 0 {
			if err := parseJSONPayload(cmd, &listRequest); err != nil {
				return nil, errors.Wrap(err, errors.ServerRequestValidation)
			}
		}
		
		// Default to active transfers if no type specified
		transferType := listRequest.Type
		if transferType == "" {
			transferType = "active"
		}
		
		var transfers []*dataset.TransferInfo
		switch dataset.TransferType(transferType) {
		case dataset.TransferTypeAll:
			transfers = h.transferManager.ListTransfersByType(dataset.TransferTypeAll)
		case dataset.TransferTypeActive:
			transfers = h.transferManager.ListTransfersByType(dataset.TransferTypeActive)
		case dataset.TransferTypeCompleted:
			transfers = h.transferManager.ListTransfersByType(dataset.TransferTypeCompleted)
		case dataset.TransferTypeFailed:
			transfers = h.transferManager.ListTransfersByType(dataset.TransferTypeFailed)
		default:
			return nil, errors.New(errors.ServerRequestValidation, "Invalid transfer type. Use: all, active, completed, failed")
		}
		
		result := map[string]interface{}{
			"transfers": transfers,
			"type":      transferType,
			"count":     len(transfers),
		}
		
		return successResponse(req.RequestId, "Transfer list retrieved", result)
	}
}

// handleManagedTransferGet returns a handler for getting a specific transfer
func handleManagedTransferGet(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var idRequest struct {
			TransferID string `json:"transfer_id"`
		}
		if err := parseJSONPayload(cmd, &idRequest); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if idRequest.TransferID == "" {
			return nil, errors.New(errors.ServerRequestValidation, "transfer_id is required")
		}

		transfer, err := h.transferManager.GetTransfer(idRequest.TransferID)
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "Transfer details retrieved", transfer)
	}
}

// handleManagedTransferPause returns a handler for pausing a transfer
func handleManagedTransferPause(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var idRequest struct {
			TransferID string `json:"transfer_id"`
		}
		if err := parseJSONPayload(cmd, &idRequest); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if idRequest.TransferID == "" {
			return nil, errors.New(errors.ServerRequestValidation, "transfer_id is required")
		}

		err := h.transferManager.PauseTransfer(idRequest.TransferID)
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "Transfer paused successfully", nil)
	}
}

// handleManagedTransferResume returns a handler for resuming a transfer
func handleManagedTransferResume(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var idRequest struct {
			TransferID string `json:"transfer_id"`
		}
		if err := parseJSONPayload(cmd, &idRequest); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if idRequest.TransferID == "" {
			return nil, errors.New(errors.ServerRequestValidation, "transfer_id is required")
		}

		// Create context for the request
		ctx := createHandlerContext(req)

		err := h.transferManager.ResumeTransfer(ctx, idRequest.TransferID)
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "Transfer resumed successfully", nil)
	}
}

// handleManagedTransferStop returns a handler for stopping a transfer
func handleManagedTransferStop(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var idRequest struct {
			TransferID string `json:"transfer_id"`
		}
		if err := parseJSONPayload(cmd, &idRequest); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if idRequest.TransferID == "" {
			return nil, errors.New(errors.ServerRequestValidation, "transfer_id is required")
		}

		err := h.transferManager.StopTransfer(idRequest.TransferID)
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "Transfer stopped successfully", nil)
	}
}

// handleManagedTransferDelete returns a handler for deleting a transfer
func handleManagedTransferDelete(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var idRequest struct {
			TransferID string `json:"transfer_id"`
		}
		if err := parseJSONPayload(cmd, &idRequest); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if idRequest.TransferID == "" {
			return nil, errors.New(errors.ServerRequestValidation, "transfer_id is required")
		}

		err := h.transferManager.DeleteTransfer(idRequest.TransferID)
		if err != nil {
			return nil, err
		}

		return successResponse(req.RequestId, "Transfer deleted successfully", nil)
	}
}

// Transfer Log Handlers

// handleTransferLogGet returns a handler for getting full transfer log content
func handleTransferLogGet(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var idRequest struct {
			TransferID string `json:"transfer_id"`
		}
		if err := parseJSONPayload(cmd, &idRequest); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if idRequest.TransferID == "" {
			return nil, errors.New(errors.ServerRequestValidation, "transfer_id is required")
		}

		logContent, err := h.transferManager.GetTransferLog(idRequest.TransferID)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"transfer_id": idRequest.TransferID,
			"log_content": logContent,
			"type":        "full",
		}

		return successResponse(req.RequestId, "Transfer log retrieved", result)
	}
}

// handleTransferLogGist returns a handler for getting truncated transfer log content
func handleTransferLogGist(h *DatasetHandler) client.CommandHandler {
	return func(req *proto.ToggleRequest, cmd *proto.CommandRequest) (*proto.CommandResponse, error) {
		var idRequest struct {
			TransferID string `json:"transfer_id"`
		}
		if err := parseJSONPayload(cmd, &idRequest); err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}

		if idRequest.TransferID == "" {
			return nil, errors.New(errors.ServerRequestValidation, "transfer_id is required")
		}

		logGist, err := h.transferManager.GetTransferLogGist(idRequest.TransferID)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"transfer_id": idRequest.TransferID,
			"log_content": logGist,
			"type":        "gist",
		}

		return successResponse(req.RequestId, "Transfer log gist retrieved", result)
	}
}
