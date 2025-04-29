// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
)

// ValidateScheduleConfig validates schedule configuration parameters
func ValidateScheduleConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}
		var schedule ScheduleSpec
		if err := c.ShouldBindJSON(&schedule); err != nil {
			APIError(c, errors.New(errors.ZFSRequestValidationError, err.Error()))
			return
		}
		ResetBody(c, body)

		if err := ValidateScheduleSpec(schedule); err != nil {
			APIError(c, err)
			return
		}

		c.Next()
	}
}

// ValidateSnapshotPolicyConfig validates snapshot policy configuration parameters
func ValidateSnapshotPolicyConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}
		var params EditPolicyParams
		if err := c.ShouldBindJSON(&params); err != nil {
			APIError(c, errors.New(errors.ZFSRequestValidationError, err.Error()))
			return
		}
		ResetBody(c, body)

		// Create a temporary policy to validate
		policy := NewSnapshotPolicy(params)
		if err := ValidatePolicy(policy); err != nil {
			APIError(c, err)
			return
		}

		c.Set("snapshotPolicy", policy)
		c.Next()
	}
}

// ValidateRunPolicyParams validates run policy parameters
func ValidateRunPolicyParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}
		var params RunPolicyParams
		if err := c.ShouldBindJSON(&params); err != nil {
			APIError(c, errors.New(errors.ZFSRequestValidationError, err.Error()))
			return
		}
		ResetBody(c, body)

		// Ensure ID is present
		if params.ID == "" {
			APIError(c,
				errors.New(errors.ZFSRequestValidationError, "policy ID is required"),
			)
			return
		}

		// Store validated params in context
		c.Set("runPolicyParams", params)
		c.Next()
	}
}
