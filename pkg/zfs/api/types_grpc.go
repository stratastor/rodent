// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

// Command type constants for ZFS operations
// These should match the enum in base.proto
const (
	// Pool operations
	CmdPoolList          = "zfs.pool.list"
	CmdPoolStatus        = "zfs.pool.status"
	CmdPoolCreate        = "zfs.pool.create"
	CmdPoolDestroy       = "zfs.pool.destroy"
	CmdPoolImport        = "zfs.pool.import"
	CmdPoolExport        = "zfs.pool.export"
	CmdPoolPropertyList  = "zfs.pool.property.list"
	CmdPoolPropertyGet   = "zfs.pool.property.get"
	CmdPoolPropertySet   = "zfs.pool.property.set"
	CmdPoolScrub         = "zfs.pool.scrub"
	CmdPoolResilver      = "zfs.pool.resilver"
	CmdPoolDeviceAttach  = "zfs.pool.device.attach"
	CmdPoolDeviceDetach  = "zfs.pool.device.detach"
	CmdPoolDeviceReplace = "zfs.pool.device.replace"

	// Dataset operations
	CmdDatasetList   = "zfs.dataset.list"
	CmdDatasetDelete = "zfs.dataset.delete"
	CmdDatasetRename = "zfs.dataset.rename"
	CmdDatasetDiff   = "zfs.dataset.diff"

	// Property operations
	CmdDatasetPropertyList    = "zfs.dataset.property.list"
	CmdDatasetPropertyGet     = "zfs.dataset.property.get"
	CmdDatasetPropertySet     = "zfs.dataset.property.set"
	CmdDatasetPropertyInherit = "zfs.dataset.property.inherit"

	// Filesystem operations
	CmdFilesystemList    = "zfs.filesystem.list"
	CmdFilesystemCreate  = "zfs.filesystem.create"
	CmdFilesystemMount   = "zfs.filesystem.mount"
	CmdFilesystemUnmount = "zfs.filesystem.unmount"

	// Volume operations
	CmdVolumeList   = "zfs.volume.list"
	CmdVolumeCreate = "zfs.volume.create"

	// Snapshot operations
	CmdSnapshotList     = "zfs.snapshot.list"
	CmdSnapshotCreate   = "zfs.snapshot.create"
	CmdSnapshotRollback = "zfs.snapshot.rollback"

	// Clone operations
	CmdCloneCreate  = "zfs.clone.create"
	CmdClonePromote = "zfs.clone.promote"

	// Bookmark operations
	CmdBookmarkList   = "zfs.bookmark.list"
	CmdBookmarkCreate = "zfs.bookmark.create"

	// Permission operations
	CmdPermissionList    = "zfs.permission.list"
	CmdPermissionAllow   = "zfs.permission.allow"
	CmdPermissionUnallow = "zfs.permission.unallow"

	// Share operations
	CmdShareDataset   = "zfs.share.dataset"
	CmdUnshareDataset = "zfs.unshare.dataset"

	// Data transfer operations
	CmdTransferSend        = "zfs.transfer.send"
	CmdTransferResumeToken = "zfs.transfer.resume_token"
)
