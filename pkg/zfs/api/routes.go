package api

import (
	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/zfs/common"
)

// Dataset Operations:
//
//	GET    /dataset              List datasets
//	DELETE /dataset              Destroy dataset
//	  Request:  {"name": "tank/ds1", "recursive_destroy_dependents": false}
//	  Response: 204 No Content
//
//	POST   /dataset/rename       Rename dataset
//	  Request:  {"name": "tank/ds1", "new_name": "tank/ds2", "force": true}
//	  Response: 200 OK
//
// Property Operations:
//
//	GET    /dataset/properties   List all properties
//	  Request:  {"name": "tank/ds1"}
//	  Response: {"result": {"tank/ds1": {"properties": {...}}}}
//
//	GET    /dataset/property     Get specific property
//	  Request:  {"name": "tank/ds1", "property": "compression"}
//	  Response: {"result": {"tank/ds1": {"properties": {"compression": {...}}}}}
//
//	PUT    /dataset/property     Set property
//	  Request:  {"name": "tank/ds1", "property": "compression", "value": "on"}
//	  Response: 201 Created
//
// Filesystem Operations:
//
//	GET    /dataset/filesystems  List filesystems
//	  Request:  {"recursive": false}
//	  Response: {"result": {"datasets": [...]}}
//
//	POST   /dataset/filesystem   Create filesystem
//	  Request:  {"name": "tank/fs1", "properties": {"compression": "on"}}
//	  Response: 201 Created
//
// Volume Operations:
//
//	GET    /dataset/volumes      List volumes
//	  Request:  {"recursive": false}
//	  Response: {"result": {"datasets": [...]}}
//
//	POST   /dataset/volume       Create volume
//	  Request:  {"name": "tank/vol1", "size": "10G", "properties": {...}}
//	  Response: 201 Created
//
// Snapshot Operations:
//
//	GET    /dataset/snapshots    List snapshots
//	  Request:  {"name": "tank/fs1"}
//	  Response: {"result": {"datasets": [...]}}
//
//	POST   /dataset/snapshot     Create snapshot
//	  Request:  {"name": "tank/fs1", "snap_name": "snap1"}
//	  Response: 201 Created
//
//	POST   /dataset/snapshot/rollback  Rollback to snapshot
//	  Request:  {"name": "tank/fs1@snap1", "destroy_recent": true}
//	  Response: 200 OK
//
// Clone Operations:
//
//	POST   /dataset/clone        Create clone
//	  Request:  {"name": "tank/fs1@snap1", "clone_name": "tank/clone1"}
//	  Response: 201 Created
//
//	POST   /dataset/clone/promote Promote clone
//	  Request:  {"name": "tank/clone1"}
//	  Response: 200 OK
//
// Bookmark Operations:
//
//	GET    /dataset/bookmarks    List bookmarks
//	  Request:  {"name": "tank/fs1"}
//	  Response: {"result": {"datasets": [...]}}
//
//	POST   /dataset/bookmark     Create bookmark
//	  Request:  {"name": "tank/fs1@snap1", "bookmark_name": "mark1"}
//	  Response: 201 Created
//
// Mount Operations:
//
//	POST   /dataset/mount        Mount dataset
//	  Request:  {"name": "tank/fs1", "force": true}
//	  Response: 200 OK
//
//	POST   /dataset/unmount      Unmount dataset
//	  Request:  {"name": "tank/fs1", "force": true}
//	  Response: 204 No Content
//
// Data Transfer:
//
//	POST   /dataset/transfer/send Send dataset
//	  Response: 200 OK
//
//	GET    /dataset/transfer/resume-token Get resume token
//	  Request:  {"name": "tank/backup"}
//	  Response: {"result": "token-string"}
func (h *DatasetHandler) RegisterRoutes(router *gin.RouterGroup) {
	dataset := router.Group("/dataset")
	{
		// TODO: Add appropriate validation middlewares

		// Dataset operations
		dataset.GET("", h.listDatasets)

		dataset.DELETE("",
			ValidateZFSEntityName(common.TypeZFSEntityMask),
			h.destroyDataset)

		dataset.POST("/rename",
			// TODO: Validate NewName?
			ValidateZFSEntityName(common.TypeDatasetMask),
			h.renameDataset)

		// Property operations
		properties := dataset.Group("/properties",
			ValidateZFSEntityName(common.TypeZFSEntityMask))
		{
			properties.GET("", h.listProperties)
		}

		property := dataset.Group("/property",
			ValidateZFSEntityName(common.TypeZFSEntityMask),
			ValidatePropertyName())
		{
			property.GET("", h.getProperty)
			// TODO: Accommmodate multiple property values
			property.PUT("",
				ValidatePropertyValue(),
				h.setProperty)
		}

		// Filesystem operations
		filesystems := dataset.Group("/filesystems")
		{
			filesystems.GET("", h.listFilesystems)
		}

		filesystem := dataset.Group("/filesystem")
		{
			filesystem.POST("",
				ValidateMountPoint(),
				ValidateProperties(),
				h.createFilesystem)

			// Mount operations
			filesystem.POST("/mount",
				ValidateZFSEntityName(common.TypeFilesystem),
				ValidateMountPoint(),
				h.mountDataset)

			filesystem.POST("/unmount",
				ValidateZFSEntityName(common.TypeFilesystem),
				h.unmountDataset)

		}

		// Volume operations
		volumes := dataset.Group("/volumes")
		{
			volumes.GET("", h.listVolumes)
		}
		volume := dataset.Group("/volume")
		{
			volume.POST("",
				ValidateVolumeSize(),
				ValidateBlockSize(),
				ValidateProperties(),
				h.createVolume)
		}

		// Snapshot operations
		snapshots := dataset.Group("/snapshots")
		{
			snapshots.GET("", h.listSnapshots)
		}
		snapshot := dataset.Group("/snapshot")
		{
			snapshot.POST("",
				ValidateZFSEntityName(common.TypeFilesystem|common.TypeVolume),
				ValidateProperties(),
				h.createSnapshot)

			snapshot.POST("/rollback",
				ValidateZFSEntityName(common.TypeSnapshot),
				h.rollbackSnapshot)
		}

		// Clone operations
		clone := dataset.Group("/clone")
		{
			clone.POST("",
				ValidateZFSEntityName(common.TypeSnapshot),
				ValidateCloneConfig(),
				ValidateProperties(),
				h.createClone)

			clone.POST("/promote",
				ValidateZFSEntityName(common.TypeFilesystem),
				h.promoteClone)
		}

		// Bookmark operations
		bookmarks := dataset.Group("/bookmarks")
		{
			bookmarks.GET("", h.listBookmarks)
		}
		bookmark := dataset.Group("/bookmark")
		{
			bookmark.POST("",
				ValidateZFSEntityName(common.TypeSnapshot|common.TypeBookmark),
				h.createBookmark)
		}

		// Data transfer operations
		transfer := dataset.Group("/transfer")
		{
			transfer.POST("/send",
				h.sendDataset)

			transfer.GET("/resume-token",
				ValidateZFSEntityName(common.TypeFilesystem),
				h.getResumeToken)
		}
	}
}
