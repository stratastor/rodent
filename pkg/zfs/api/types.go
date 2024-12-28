package api

import (
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

// DatasetHandler provides HTTP endpoints for ZFS dataset operations.
// It implements the following features:
//   - Filesystem creation and management
//   - Volume creation and management
//   - Snapshot operations
//   - Clone operations
//   - Property management
//
// All operations use proper validation and error handling.
type DatasetHandler struct {
	manager *dataset.Manager
}

// PoolHandler provides HTTP endpoints for ZFS pool operations.
// It implements the following features:
//   - Pool creation and destruction
//   - Import/export operations
//   - Status and property management
//   - Device management (attach/detach/replace)
//   - Maintenance operations (scrub/resilver)
//
// All operations use proper validation and error handling.
type PoolHandler struct {
	manager *pool.Manager
}

// Request types

type createFilesystemRequest struct {
	Name       string            `json:"name" binding:"required"`
	Properties map[string]string `json:"properties"`
	Parents    bool              `json:"parents"`
	MountPoint string            `json:"mountpoint"`
}

type createVolumeRequest struct {
	Name       string            `json:"name" binding:"required"`
	Size       string            `json:"size" binding:"required"`
	Properties map[string]string `json:"properties"`
	Sparse     bool              `json:"sparse"`
	BlockSize  string            `json:"blocksize"`
}

type createSnapshotRequest struct {
	Name       string            `json:"name" binding:"required"`
	Recursive  bool              `json:"recursive"`
	Properties map[string]string `json:"properties"`
}

type createCloneRequest struct {
	Name         string            `json:"name" binding:"required"`
	Properties   map[string]string `json:"properties"`
	CreateParent bool              `json:"create_parent"`
}

type rollbackRequest struct {
	Force     bool `json:"force"`
	Recursive bool `json:"recursive"`
}

type mountRequest struct {
	MountPoint string   `json:"mountpoint,omitempty"`
	Options    []string `json:"options,omitempty"`
}

// Response types match dataset package types
type Property = dataset.Property
type Dataset = dataset.Dataset
type SnapshotInfo = dataset.SnapshotInfo
