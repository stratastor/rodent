// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// Command type constants for ZFS operations
// These use the shared constants from the proto package
const (
	// Pool operations
	CmdPoolList          = proto.ZFSCmdPoolList
	CmdPoolStatus        = proto.ZFSCmdPoolStatus
	CmdPoolCreate        = proto.ZFSCmdPoolCreate
	CmdPoolDestroy       = proto.ZFSCmdPoolDestroy
	CmdPoolImport        = proto.ZFSCmdPoolImport
	CmdPoolExport        = proto.ZFSCmdPoolExport
	CmdPoolPropertyList  = proto.ZFSCmdPoolPropertyList
	CmdPoolPropertyGet   = proto.ZFSCmdPoolPropertyGet
	CmdPoolPropertySet   = proto.ZFSCmdPoolPropertySet
	CmdPoolScrub         = proto.ZFSCmdPoolScrub
	CmdPoolResilver      = proto.ZFSCmdPoolResilver
	CmdPoolDeviceAttach  = proto.ZFSCmdPoolDeviceAttach
	CmdPoolDeviceDetach  = proto.ZFSCmdPoolDeviceDetach
	CmdPoolDeviceReplace = proto.ZFSCmdPoolDeviceReplace

	// Dataset operations
	CmdDatasetList   = proto.ZFSCmdDatasetList
	CmdDatasetDelete = proto.ZFSCmdDatasetDelete
	CmdDatasetRename = proto.ZFSCmdDatasetRename
	CmdDatasetDiff   = proto.ZFSCmdDatasetDiff

	// Property operations
	CmdDatasetPropertyList    = proto.ZFSCmdDatasetPropertyList
	CmdDatasetPropertyGet     = proto.ZFSCmdDatasetPropertyGet
	CmdDatasetPropertySet     = proto.ZFSCmdDatasetPropertySet
	CmdDatasetPropertyInherit = proto.ZFSCmdDatasetPropertyInherit

	// Filesystem operations
	CmdFilesystemList    = proto.ZFSCmdFilesystemList
	CmdFilesystemCreate  = proto.ZFSCmdFilesystemCreate
	CmdFilesystemMount   = proto.ZFSCmdFilesystemMount
	CmdFilesystemUnmount = proto.ZFSCmdFilesystemUnmount

	// Volume operations
	CmdVolumeList   = proto.ZFSCmdVolumeList
	CmdVolumeCreate = proto.ZFSCmdVolumeCreate

	// Snapshot operations
	CmdSnapshotList     = proto.ZFSCmdSnapshotList
	CmdSnapshotCreate   = proto.ZFSCmdSnapshotCreate
	CmdSnapshotRollback = proto.ZFSCmdSnapshotRollback

	// Clone operations
	CmdCloneCreate  = proto.ZFSCmdCloneCreate
	CmdClonePromote = proto.ZFSCmdClonePromote

	// Bookmark operations
	CmdBookmarkList   = proto.ZFSCmdBookmarkList
	CmdBookmarkCreate = proto.ZFSCmdBookmarkCreate

	// Permission operations
	CmdPermissionList    = proto.ZFSCmdPermissionList
	CmdPermissionAllow   = proto.ZFSCmdPermissionAllow
	CmdPermissionUnallow = proto.ZFSCmdPermissionUnallow

	// Share operations
	CmdShareDataset   = proto.ZFSCmdShareDataset
	CmdUnshareDataset = proto.ZFSCmdUnshareDataset

	// Data transfer operations
	CmdTransferSend        = proto.ZFSCmdTransferSend
	CmdTransferResumeToken = proto.ZFSCmdTransferResumeToken
)
