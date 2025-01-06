/*
 * Copyright 2024 Raamsri Kumar <raam@tinkershack.in> and The StrataSTOR Authors 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

func NewPoolHandler(manager *pool.Manager) *PoolHandler {
	return &PoolHandler{manager: manager}
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
