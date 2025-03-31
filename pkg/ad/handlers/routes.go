// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all AD-related routes with the given router group
func (h *ADHandler) RegisterRoutes(router *gin.RouterGroup) {
	// User operations
	users := router.Group("/users")
	{
		users.GET("", h.ListUsers)
		users.POST("", h.CreateUser)
		users.GET("/:username", h.GetUser)
		users.PUT("/:username", h.UpdateUser)
		users.DELETE("/:username", h.DeleteUser)
		users.GET("/:username/groups", h.GetUserGroups)
	}

	// Group operations
	groups := router.Group("/groups")
	{
		groups.GET("", h.ListGroups)
		groups.POST("", h.CreateGroup)
		groups.GET("/:groupname", h.GetGroup)
		groups.PUT("/:groupname", h.UpdateGroup)
		groups.DELETE("/:groupname", h.DeleteGroup)
		groups.GET("/:groupname/members", h.GetGroupMembers)
		groups.POST("/:groupname/members", h.AddGroupMembers)
		groups.DELETE("/:groupname/members", h.RemoveGroupMembers)
	}

	// Computer operations
	computers := router.Group("/computers")
	{
		computers.GET("", h.ListComputers)
		computers.POST("", h.CreateComputer)
		computers.GET("/:computername", h.GetComputer)
		computers.PUT("/:computername", h.UpdateComputer)
		computers.DELETE("/:computername", h.DeleteComputer)
	}
}
