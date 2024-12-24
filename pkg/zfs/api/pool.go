package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

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

func NewPoolHandler(manager *pool.Manager) *PoolHandler {
	return &PoolHandler{manager: manager}
}

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
			h.createPool)
		pools.GET("", h.listPools)
		pools.DELETE("/:name", ValidatePoolName(), h.destroyPool)

		// Import/Export
		pools.POST("/import", ValidatePoolOperation(), h.importPool)
		pools.POST("/:name/export", ValidatePoolName(), h.exportPool)

		// Status and properties
		pools.GET("/:name/status", ValidatePoolName(), h.getPoolStatus)
		pools.GET("/:name/properties/:property",
			ValidatePoolName(),
			ValidatePropertyName(),
			h.getProperty)
		pools.PUT("/:name/properties/:property",
			ValidatePoolName(),
			ValidatePropertyName(),
			ValidatePropertyValue(),
			h.setProperty)

		// Maintenance
		pools.POST("/:name/scrub", ValidatePoolName(), h.scrubPool)
		pools.POST("/:name/resilver", ValidatePoolName(), h.resilverPool)

		// Device operations
		devices := pools.Group("/:name/devices", ValidatePoolName())
		{
			devices.POST("/attach", ValidateDeviceInput("device"), h.attachDevice)
			devices.POST("/detach", ValidateDeviceInput("device"), h.detachDevice)
			devices.POST("/replace",
				ValidateDeviceInput("old_device"),
				ValidateDeviceInput("new_device"),
				h.replaceDevice)
		}
	}
}

func (h *PoolHandler) listPools(c *gin.Context) {
	pools, err := h.manager.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"pools": pools})
}

func (h *PoolHandler) destroyPool(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	if err := h.manager.Destroy(c.Request.Context(), name, force); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PoolHandler) importPool(c *gin.Context) {
	var cfg pool.ImportConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.Import(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusOK)
}

func (h *PoolHandler) exportPool(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	if err := h.manager.Export(c.Request.Context(), name, force); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusOK)
}

func (h *PoolHandler) getPoolStatus(c *gin.Context) {
	name := c.Param("name")

	status, err := h.manager.Status(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *PoolHandler) getProperty(c *gin.Context) {
	name := c.Param("name")
	property := c.Param("property")

	prop, err := h.manager.GetProperty(c.Request.Context(), name, property)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errors.ErrZFSPoolPropertyNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, err)
		return
	}
	c.JSON(http.StatusOK, prop)
}

func (h *PoolHandler) setProperty(c *gin.Context) {
	name := c.Param("name")
	property := c.Param("property")

	var req struct {
		Value string `json:"value" binding:"required"`
	}
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

func (h *PoolHandler) scrubPool(c *gin.Context) {
	name := c.Param("name")
	stop := c.Query("stop") == "true"

	if err := h.manager.Scrub(c.Request.Context(), name, stop); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusOK)
}

func (h *PoolHandler) resilverPool(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.Resilver(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusOK)
}

func (h *PoolHandler) createPool(c *gin.Context) {
	var cfg pool.CreateConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.Create(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusCreated)
}

func (h *PoolHandler) attachDevice(c *gin.Context) {
	pool := c.Param("name")
	var req struct {
		Device    string `json:"device" binding:"required"`
		NewDevice string `json:"new_device" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.AttachDevice(c.Request.Context(), pool, req.Device, req.NewDevice); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusOK)
}

func (h *PoolHandler) detachDevice(c *gin.Context) {
	pool := c.Param("name")
	var req struct {
		Device string `json:"device" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.DetachDevice(c.Request.Context(), pool, req.Device); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusOK)
}

func (h *PoolHandler) replaceDevice(c *gin.Context) {
	pool := c.Param("name")
	var req struct {
		OldDevice string `json:"old_device" binding:"required"`
		NewDevice string `json:"new_device" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.New(errors.ServerRequestValidation, err.Error()))
		return
	}

	if err := h.manager.ReplaceDevice(c.Request.Context(), pool, req.OldDevice, req.NewDevice); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusOK)
}

type scrubRequest struct {
	Stop bool `json:"stop"`
}

type attachDeviceRequest struct {
	Device    string `json:"device" binding:"required"`
	NewDevice string `json:"new_device" binding:"required"`
}

type detachDeviceRequest struct {
	Device string `json:"device" binding:"required"`
}

type replaceDeviceRequest struct {
	OldDevice string `json:"old_device" binding:"required"`
	NewDevice string `json:"new_device" binding:"required"`
}

type setPropertyRequest struct {
	Value string `json:"value" binding:"required"`
}
