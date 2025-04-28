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
func (h *DatasetHandler) RegisterRoutes(router *gin.RouterGroup) {
	dataset := router.Group("/dataset")
	{
		// TODO: Add appropriate validation middlewares

		// Dataset operations
		dataset.POST("/list", h.listDatasets)

		dataset.POST("/delete",
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
			properties.POST("/list", h.listProperties)
		}

		property := dataset.Group("/property",
			ValidateZFSEntityName(common.TypeZFSEntityMask))
		{
			property.POST("/fetch",
				ValidatePropertyName(),
				h.getProperty)
			property.POST("",
				ValidateZFSProperties(),
				h.setProperty)
			property.POST("/inherit",
				ValidateZFSProperties(),
				h.inheritProperty)
		}

		// Filesystem operations
		filesystems := dataset.Group("/filesystems")
		{
			filesystems.POST("/list", h.listFilesystems)
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
			volumes.POST("/list", h.listVolumes)
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
			snapshots.POST("/list", h.listSnapshots)
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
			bookmarks.POST("/list", h.listBookmarks)
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
			permissions.POST("/list", h.listPermissions)
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

			transfer.POST("/resume-token/fetch",
				ValidateZFSEntityName(common.TypeFilesystem),
				h.getResumeToken)
		}
	}
}

// TODO: Perhaps Pool APIs can also be refactored to send param values in the body to maintain consistency?
// 	Pool APIs do not have the same issue as Dataset APIs, but it would be pragmatic to have a consistent approach.

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
		pools.GET("/:name/properties",
			ValidatePoolName(),
			h.getProperties)
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
