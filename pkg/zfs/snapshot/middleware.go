// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
)

// ValidateScheduleConfig validates schedule configuration parameters
func ValidateScheduleConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var schedule ScheduleSpec
		if err := c.ShouldBindJSON(&schedule); err != nil {
			c.JSON(http.StatusBadRequest, errors.New(errors.ZFSRequestValidationError, err.Error()))
			c.Abort()
			return
		}

		if err := ValidateScheduleSpec(schedule); err != nil {
			c.JSON(http.StatusBadRequest, err)
			c.Abort()
			return
		}

		c.Next()
	}
}

// ValidateSnapshotPolicyConfig validates snapshot policy configuration parameters
func ValidateSnapshotPolicyConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params EditPolicyParams
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(http.StatusBadRequest, errors.New(errors.ZFSRequestValidationError, err.Error()))
			c.Abort()
			return
		}

		// Create a temporary policy to validate
		policy := NewSnapshotPolicy(params)
		if err := ValidatePolicy(policy); err != nil {
			c.JSON(http.StatusBadRequest, err)
			c.Abort()
			return
		}

		c.Next()
	}
}

// ValidateRunPolicyParams validates run policy parameters
func ValidateRunPolicyParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params RunPolicyParams
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(http.StatusBadRequest, errors.New(errors.ZFSRequestValidationError, err.Error()))
			c.Abort()
			return
		}

		// Ensure ID is present
		if params.ID == "" {
			c.JSON(http.StatusBadRequest, errors.New(errors.ZFSRequestValidationError, "policy ID is required"))
			c.Abort()
			return
		}

		// Store validated params in context
		c.Set("runPolicyParams", params)
		c.Next()
	}
}