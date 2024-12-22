// pkg/zfs/api/dataset.go

package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
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

func NewDatasetHandler(manager *dataset.Manager) *DatasetHandler {
	return &DatasetHandler{manager: manager}
}

// API Routes
//
// Base Path: /api/v1/datasets
//
// Filesystem Operations:
//
//	POST   /filesystems
//	  Request:  {"name": "pool/fs1", "properties": {"compression": "on"}}
//	  Response: 201 Created
//
//	GET    /filesystems
//	  Response: {"datasets": [{"name": "pool/fs1", "type": "filesystem", ...}]}
//
// Volume Operations:
//
//	POST   /volumes
//	  Request:  {"name": "pool/vol1", "size": "10G", "properties": {"volblocksize": "8K"}}
//	  Response: 201 Created
//
//	GET    /volumes
//	  Response: {"datasets": [{"name": "pool/vol1", "type": "volume", ...}]}
//
// Common Operations:
//
//	DELETE /datasets/:name
//	  Response: 204 No Content
//
//	POST   /datasets/:name/rename
//	  Request:  {"new_name": "pool/newname", "create_parent": false}
//	  Response: 200 OK
//
// Properties:
//
//	GET    /datasets/:name/properties/:property
//	  Response: {"value": "on", "source": {"type": "local"}}
//
//	PUT    /datasets/:name/properties/:property
//	  Request:  {"value": "off"}
//	  Response: 200 OK
//
// Snapshots:
//
//	POST   /datasets/:dataset/snapshots
//	  Request:  {"name": "snap1", "recursive": true}
//	  Response: 201 Created
//
//	GET    /datasets/:dataset/snapshots
//	  Response: {"snapshots": [{"name": "pool/fs@snap1", ...}]}
//
//	DELETE /datasets/:dataset/snapshots/:snapshot
//	  Response: 204 No Content
//
//	POST   /datasets/:dataset/snapshots/:snapshot/rollback
//	  Request:  {"force": true, "recursive": false}
//	  Response: 200 OK
//
// Bookmarks:
//
//	POST   /datasets/:dataset/bookmarks
//	  Request:  {"name": "mark1", "snapshot": "snap1"}
//	  Response: 201 Created
//
//	GET    /datasets/:dataset/bookmarks
//	  Response: {"bookmarks": [{"name": "pool/fs#mark1", ...}]}
//
// Error Responses:
//
//	400 Bad Request:      Invalid input (name, property, size)
//	403 Forbidden:        Permission denied or quota exceeded
//	404 Not Found:        Dataset/snapshot not found
//	500 Internal Error:   Command execution failed
//
// Clones:
//
//	POST   /datasets/clones
func (h *DatasetHandler) RegisterRoutes(router *gin.RouterGroup) {
	datasets := router.Group("/datasets")
	{
		// Filesystem operations
		filesystems := datasets.Group("/filesystems")
		{
			filesystems.POST("", h.createFilesystem)
			filesystems.GET("", h.listFilesystems)
		}

		// Volume operations
		volumes := datasets.Group("/volumes")
		{
			volumes.POST("", h.createVolume)
			volumes.GET("", h.listVolumes)
		}

		// Common operations (with validation)
		datasets.DELETE("/:name", ValidateDatasetName(), h.destroyDataset)
		datasets.POST("/:name/rename", ValidateDatasetName(), h.renameDataset)

		// Property operations (with validation)
		datasets.GET("/:name/properties/:property",
			ValidateDatasetName(),
			ValidatePropertyName(),
			h.getProperty)
		datasets.PUT("/:name/properties/:property",
			ValidateDatasetName(),
			ValidatePropertyName(),
			h.setProperty)

		// Snapshot operations (with validation)
		snapshots := datasets.Group("/:dataset/snapshots", ValidateDatasetName())
		{
			snapshots.POST("", h.createSnapshot)
			snapshots.GET("", h.listSnapshots)
			snapshots.DELETE("/:snapshot", h.destroySnapshot)
			snapshots.POST("/:snapshot/rollback", h.rollbackSnapshot)
		}

		// Clone operations
		datasets.POST("/clones", h.createClone)
	}
}

func (h *DatasetHandler) createFilesystem(c *gin.Context) {
	var cfg dataset.FilesystemConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.CreateFilesystem(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusCreated)
}

func (h *DatasetHandler) listFilesystems(c *gin.Context) {
	recursive := c.Query("recursive") == "true"

	datasets, err := h.manager.List(c.Request.Context(), recursive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	// Filter for filesystems only
	var filesystems []dataset.Dataset
	for _, ds := range datasets {
		if ds.Type == "filesystem" {
			filesystems = append(filesystems, ds)
		}
	}

	c.JSON(http.StatusOK, gin.H{"filesystems": filesystems})
}

func (h *DatasetHandler) listVolumes(c *gin.Context) {
	recursive := c.Query("recursive") == "true"

	datasets, err := h.manager.List(c.Request.Context(), recursive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	// Filter for volumes only
	var volumes []dataset.Dataset
	for _, ds := range datasets {
		if ds.Type == "volume" {
			volumes = append(volumes, ds)
		}
	}

	c.JSON(http.StatusOK, gin.H{"volumes": volumes})
}

func (h *DatasetHandler) createVolume(c *gin.Context) {
	var cfg dataset.VolumeConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.CreateVolume(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusCreated)
}

type createDatasetRequest struct {
	Name       string            `json:"name" binding:"required"`
	Type       string            `json:"type" binding:"required"`
	Properties map[string]string `json:"properties"`
	Parents    bool              `json:"parents"`
	MountPoint string            `json:"mountpoint"`
}

// func (h *DatasetHandler) createDataset(c *gin.Context) {
// 	var req createDatasetRequest
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
// 		return
// 	}

// 	cfg := dataset.CreateConfig{
// 		Name:       req.Name,
// 		Type:       req.Type,
// 		Properties: req.Properties,
// 		Parents:    req.Parents,
// 		MountPoint: req.MountPoint,
// 	}

// 	if err := h.manager.Create(c.Request.Context(), cfg); err != nil {
// 		c.JSON(http.StatusInternalServerError, err)
// 		return
// 	}

// 	c.Status(http.StatusCreated)
// }

func (h *DatasetHandler) listDatasets(c *gin.Context) {
	recursive := c.Query("recursive") == "true"

	datasets, err := h.manager.List(c.Request.Context(), recursive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"datasets": datasets})
}

func (h *DatasetHandler) destroyDataset(c *gin.Context) {
	name := c.Param("name")
	recursive := c.Query("recursive") == "true"

	if err := h.manager.Destroy(c.Request.Context(), name, recursive); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *DatasetHandler) getProperty(c *gin.Context) {
	name := c.Param("name")
	property := c.Param("property")

	prop, err := h.manager.GetProperty(c.Request.Context(), name, property)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errors.ZFSDatasetPropertyNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, err)
		return
	}

	c.JSON(http.StatusOK, prop)
}

type setPropertyRequest struct {
	Value string `json:"value" binding:"required"`
}

func (h *DatasetHandler) setProperty(c *gin.Context) {
	name := c.Param("name")
	property := c.Param("property")

	var req setPropertyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.SetProperty(c.Request.Context(), name, property, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)
}

// Snapshot operations
func (h *DatasetHandler) createSnapshot(c *gin.Context) {
	dataset := c.Param("dataset")
	var cfg dataset.SnapshotConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	cfg.Dataset = dataset
	if err := h.manager.CreateSnapshot(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusCreated)
}

func (h *DatasetHandler) listSnapshots(c *gin.Context) {
	dataset := c.Param("dataset")
	snapshots, err := h.manager.ListSnapshots(c.Request.Context(), dataset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"snapshots": snapshots})
}

func (h *DatasetHandler) destroySnapshot(c *gin.Context) {
	dataset := c.Param("dataset")
	snapshot := c.Param("snapshot")

	if err := h.manager.DestroySnapshot(c.Request.Context(), dataset, snapshot); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *DatasetHandler) rollbackSnapshot(c *gin.Context) {
	dataset := c.Param("dataset")
	snapshot := c.Param("snapshot")

	var cfg dataset.RollbackConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	cfg.Snapshot = snapshot
	if err := h.manager.Rollback(c.Request.Context(), dataset, cfg); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)
}

// Clone operations
func (h *DatasetHandler) createClone(c *gin.Context) {
	var cfg dataset.CloneConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.Clone(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusCreated)
}

// Rename operation
func (h *DatasetHandler) renameDataset(c *gin.Context) {
	name := c.Param("name")
	var cfg dataset.RenameConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.Rename(c.Request.Context(), name, cfg); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)
}
