package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

type PoolHandler struct {
	manager *pool.Manager
}

func NewPoolHandler(manager *pool.Manager) *PoolHandler {
	return &PoolHandler{manager: manager}
}

func (h *PoolHandler) RegisterRoutes(router *gin.RouterGroup) {
	pools := router.Group("/pools")
	{
		pools.POST("", ValidateDevicePaths(), h.createPool)
		pools.GET("", h.listPools)
		pools.DELETE("/:name", ValidatePoolName(), h.destroyPool)

		// Import/Export
		pools.POST("/import", h.importPool)
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
			h.setProperty)

		// Maintenance
		pools.POST("/:name/scrub", ValidatePoolName(), h.scrubPool)
		pools.POST("/:name/resilver", ValidatePoolName(), h.resilverPool)
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
		if errors.Is(err, errors.ZFSPoolPropertyNotFound) {
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
