/*
 * Copyright 2024-2025 Raamsri Kumar <raam@tinkershack.in> 
 * Copyright 2024-2025 The StrataSTOR Authors and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/zfs/common"
)

// Unlike Pool operations, Dataset API maynot be RESTFUL.
// Having dataset values with "/" in the URI params is inconvenient
// and may lead to confusion. Hence, we will pass information in the body
// to keep the URI clean and simple.
//
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

		dataset.POST("/diff",
			ValidateDiffConfig(),
			h.diffDataset)

		// Property operations
		properties := dataset.Group("/properties",
			ValidateZFSEntityName(common.TypeZFSEntityMask))
		{
			properties.GET("", h.listProperties)
		}

		property := dataset.Group("/property",
			ValidateZFSEntityName(common.TypeZFSEntityMask))
		{
			property.GET("",
				ValidatePropertyName(),
				h.getProperty)
			property.PUT("",
				ValidateZFSProperties(),
				h.setProperty)
			property.PUT("/inherit",
				ValidateZFSProperties(),
				h.inheritProperty)
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
				ValidateZFSProperties(),
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
				ValidateZFSProperties(),
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
				ValidateZFSProperties(),
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
				ValidateZFSProperties(),
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

		// Permission operations
		permissions := dataset.Group("/permissions",
			ValidateZFSEntityName(common.TypeDatasetMask))
		{
			permissions.GET("", h.listPermissions)
			permissions.POST("",
				ValidatePermissionConfig(),
				h.allowPermissions)
			permissions.DELETE("",
				ValidateUnallowConfig(),
				h.unallowPermissions)
		}

		// Share operations
		share := dataset.Group("/share")
		{
			share.POST("", h.shareDataset)
			share.DELETE("", h.unshareDataset)
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

// TODO: Perhaps Pool APIs can also be refactored to send param values in the body to maintain consistency?
// 	Pool APIs do not have the same issue as Dataset APIs, but it would be pragmatic to have a consistent approach.

// API Routes
//
// Pool Operations:
//
//	POST   /api/v1/pools
//	  Request:  {"name": "mypool", "vdev_spec": [{"type": "mirror", "devices": ["/dev/sda", "/dev/sdb"]}]}
//	  Response: 201 Created
//
//	GET    /api/v1/pools
//	  Response: {"pools": [{"name": "mypool", "state": "ONLINE", ...}]}
//
//	DELETE /api/v1/pools/:name
//	  Response: 204 No Content
//
// Import/Export:
//
//	POST   /api/v1/pools/import
//	  Request:  {"name": "mypool", "force": false}
//	  Response: 200 OK
//
//	POST   /api/v1/pools/:name/export
//	  Response: 200 OK
//
// Status and Properties:
//
//	GET    /api/v1/pools/:name/status
//	  Response: {"name": "mypool", "state": "ONLINE", "vdevs": [...]}
//
//	GET    /api/v1/pools/:name/properties/:property
//	  Response: {"value": "on", "source": {"type": "local"}}
//
//	PUT    /api/v1/pools/:name/properties/:property
//	  Request:  {"value": "off"}
//	  Response: 200 OK
//
// Maintenance:
//
//	POST   /api/v1/pools/:name/scrub
//	  Request:  {"stop": false}
//	  Response: 200 OK
//
//	POST   /api/v1/pools/:name/resilver
//	  Response: 200 OK
//
// Device Operations:
//
//	POST   /api/v1/pools/:name/devices/attach
//	  Request:  {"device": "/dev/sdc", "new_device": "/dev/sdd"}
//	  Response: 200 OK
//
//	POST   /api/v1/pools/:name/devices/detach
//	  Request:  {"device": "/dev/sdc"}
//	  Response: 200 OK
//
//	POST   /api/v1/pools/:name/devices/replace
//	  Request:  {"old_device": "/dev/sdc", "new_device": "/dev/sdd"}
//	  Response: 200 OK
//
// Error Responses:
//
//	400 Bad Request:      Invalid input (name, device path, property)
//	403 Forbidden:        Permission denied
//	404 Not Found:        Pool not found
//	500 Internal Error:   Command execution failed
func (h *PoolHandler) RegisterRoutes(router *gin.RouterGroup) {
	pools := router.Group("/pools")
	{
		// Create/List/Destroy
		pools.POST("",
			ValidatePoolName(),
			ValidateNameLength(),
			EnhancedValidateDevicePaths(),
			ValidatePoolProperties(common.CreatePoolPropContext),
			h.createPool)
		pools.GET("", h.listPools)
		pools.DELETE("/:name", ValidatePoolName(), h.destroyPool)

		// Import/Export
		pools.POST("/import",
			ValidatePoolProperties(common.ImportPoolPropContext),
			h.importPool)
		pools.POST("/:name/export", ValidatePoolName(), h.exportPool)

		// Status and properties
		pools.GET("/:name/status", ValidatePoolName(), h.getPoolStatus)
		pools.GET("/:name/properties/:property",
			ValidatePoolName(),
			ValidatePoolProperty(common.ValidPoolGetPropContext),
			h.getProperty)
		pools.PUT("/:name/properties/:property",
			ValidatePoolName(),
			ValidatePoolProperty(common.AnytimePoolPropContext),
			ValidatePropertyValue(),
			h.setProperty)

		// Maintenance
		pools.POST("/:name/scrub", ValidatePoolName(), h.scrubPool)
		pools.POST("/:name/resilver", ValidatePoolName(), h.resilverPool)

		// Device operations
		devices := pools.Group("/:name/devices", ValidatePoolName())
		{
			// TODO: Validate device paths
			devices.POST("/attach", h.attachDevice)
			devices.POST("/detach", h.detachDevice)
			devices.POST("/replace",
				h.replaceDevice)
		}
	}
}
