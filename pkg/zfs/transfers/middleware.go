// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package transfers

import (
	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/errors"
)

// APIError is a helper function for sending error responses
var APIError = common.APIError

// ValidateTransferPolicyConfig validates transfer policy configuration parameters
func ValidateTransferPolicyConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(
				c,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"),
			)
			return
		}
		var params EditTransferPolicyParams
		if err := c.ShouldBindJSON(&params); err != nil {
			APIError(c, errors.New(errors.TransferPolicyInvalidConfig, err.Error()))
			return
		}
		ResetBody(c, body)

		// Create a temporary policy to validate
		policy := NewTransferPolicy(params)
		if err := ValidatePolicy(policy); err != nil {
			APIError(c, err)
			return
		}

		c.Set("transferPolicy", policy)
		c.Next()
	}
}

// ValidateRunPolicyParams validates run policy parameters
func ValidateRunPolicyParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(
				c,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"),
			)
			return
		}
		var params RunTransferPolicyParams
		if err := c.ShouldBindJSON(&params); err != nil {
			APIError(c, errors.New(errors.TransferPolicyInvalidConfig, err.Error()))
			return
		}
		ResetBody(c, body)

		// Ensure PolicyID is present
		if params.PolicyID == "" {
			APIError(c,
				errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"),
			)
			return
		}

		// Store validated params in context
		c.Set("runPolicyParams", params)
		c.Next()
	}
}

// ValidateEnableDisableParams validates enable/disable policy parameters
func ValidateEnableDisableParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if policy_id is in path parameter
		policyID := c.Param("policy_id")
		if policyID == "" {
			APIError(
				c,
				errors.New(errors.TransferPolicyInvalidConfig, "policy ID is required"),
			)
			return
		}

		c.Next()
	}
}
