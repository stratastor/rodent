// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all inventory-related routes with the given router group
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// GET /inventory - Get complete Rodent inventory
	router.GET("", h.GetInventory)
}
