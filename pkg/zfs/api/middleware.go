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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/common"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

var (
	// ZFS naming conventions
	filesystemNameRegex = regexp.MustCompile(
		`^[a-zA-Z0-9][a-zA-Z0-9_.-]*(/[a-zA-Z0-9][a-zA-Z0-9_.-]*)*$`,
	)
	snapshotNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	volumeSizeRegex   = regexp.MustCompile(`^\d+[KMGTP]?$`)
	bookmarkNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

	poolNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]*$`)

	devicePathRegex = regexp.MustCompile(
		`^/dev/(?:` +
			// Standard device paths
			`[hsv]d[a-z]\d*|` +
			// NVMe devices
			`nvme\d+n\d+(?:p\d+)?|` +
			// Device mapper paths
			`mapper/[a-zA-Z0-9._-]+|` +
			// By-id paths (includes WWN, serial numbers)
			`disk/by-id/[a-zA-Z0-9._-]+(?:-part\d+)?|` +
			// By-path (includes PCI paths)
			`disk/by-path/(?:pci-)?[a-zA-Z0-9/:._-]+(?:-part\d+)?|` +
			// By-uuid paths
			`disk/by-uuid/[a-fA-F0-9-]+|` +
			// By-label paths
			`disk/by-label/[a-zA-Z0-9._-]+|` +
			// By-partuuid paths
			`disk/by-partuuid/[a-fA-F0-9-]+` +
			`)$`)

	// TODO: Validate property names? Track ZFS property list? Or just let ZFS handle it?
	// propertyValueRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:/@+-]*$`)

	quotaRegex      = regexp.MustCompile(`^\d+[KMGTP]?(:|$)`)
	mountPointRegex = regexp.MustCompile(`^/[a-zA-Z0-9/._-]*$`)

	// Size limits
	maxPropertyValueLen = 1024
	maxDevicePaths      = 64
	maxNameLength       = 255

	// Restricted paths/devices
	restrictedPaths = map[string]bool{
		"/":     true,
		"/boot": true,
		"/etc":  true,
		"/proc": true,
		"/sys":  true,
	}

	restrictedDevices = map[string]bool{
		"/dev/sda":  true, // system disk
		"/dev/hda":  true,
		"/dev/xvda": true,
	}
)

// ErrorHandler adds structured error handling
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			// Default to 500
			status := http.StatusInternalServerError

			// Convert to our error type
			if re, ok := err.Err.(*errors.RodentError); ok {
				// Use appropriate status code
				if re.HTTPStatus != 0 {
					status = re.HTTPStatus
				}

				// Return structured error response
				c.JSON(status, re)
			} else {
				// Return generic error for unknown types
				c.JSON(status, gin.H{
					"error": err.Error(),
				})
			}
		}
	}
}

// Helper to add errors to context
func APIError(c *gin.Context, err error) {
	c.Error(err)
	c.Abort()
}

// ReadResetBody reads and resets the request body so it can be re-read by subsequent handlers
func ReadResetBody(c *gin.Context) ([]byte, error) {
	// Read and store the raw body
	body, err := c.GetRawData()
	if err != nil {
		return nil, err
	}

	// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	return body, nil
}

// ResetBody resets the request body so it can be re-read by subsequent handlers
func ResetBody(c *gin.Context, body []byte) {
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
}

// ValidateDatasetName validates dataset name format
func ValidateDatasetName() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name != "" && !filesystemNameRegex.MatchString(name) {
			APIError(c, errors.New(errors.ZFSDatasetInvalidName, "Invalid dataset name format"))
			return
		}
		c.Next()
	}
}

// ValidatePropertyName validates property name
func ValidatePropertyName() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Property string `json:"property" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		property := req.Property
		if property == "" || !isValidDatasetProperty(property) {
			APIError(
				c,
				errors.New(
					errors.ZFSDatasetInvalidProperty,
					fmt.Sprintf("Invalid property name [%s]", property),
				),
			)
			return
		}
		c.Next()
	}
}

func ValidateVolumeSize() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Size string `json:"size"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		if !volumeSizeRegex.MatchString(req.Size) {
			APIError(c, errors.New(errors.ZFSInvalidSize, "Invalid volume size format"))
			return
		}
		c.Next()
	}
}

// ValidateZFSEntityName validates any ZFS entity name
func ValidateZFSEntityName(dtype common.DatasetType) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Name  string   `json:"name"`
			Names []string `json:"names"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		names := req.Names
		if req.Name != "" {
			names = append(names, req.Name)
		}

		// Check if either Name or Names is provided
		if req.Name == "" && len(names) == 0 {
			APIError(
				c,
				errors.New(
					errors.ServerRequestValidation,
					"Either 'name' or 'names' must be provided",
				),
			)
			return
		}

		for _, name := range req.Names {
			if name == "" {
				APIError(c, errors.New(errors.ZFSDatasetInvalidName, "Invalid dataset name format"))
				return
			}

			// Validate name format
			switch dtype {
			case common.TypeZFSEntityMask:
				err := common.EntityNameCheck(name)
				if err != nil {
					APIError(c, err)
					return
				}
			case common.TypeDatasetMask:
				err := common.DatasetNameCheck(name)
				if err != nil {
					APIError(c, err)
					return
				}
			case common.TypeBookmark | common.TypeSnapshot:
				// This is the case for clone creation where the name can be either a bookmark or snapshot
				errbm := common.ValidateZFSName(name, common.TypeBookmark)
				errsnap := common.ValidateZFSName(name, common.TypeSnapshot)
				if errbm != nil && errsnap != nil {
					APIError(
						c,
						errors.New(
							errors.ZFSNameInvalid,
							"Name expected to be either a bookmark or snapshot",
						),
					)
					return
				}
			default:
				err := common.ValidateZFSName(name, dtype)
				if err != nil {
					APIError(c, err)
					return
				}
			}
		}
		c.Next()
	}
}

// isValidDatasetProperty maintains a list of valid ZFS properties
func isValidDatasetProperty(property string) bool {
	return common.IsValidDatasetProperty(property)
}

// ValidatePoolName validates pool name format
func ValidatePoolName() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name != "" && !poolNameRegex.MatchString(name) {
			APIError(c, errors.New(errors.ZFSPoolInvalidName, "Invalid pool name format"))
			return
		}
		c.Next()
	}
}

// ValidatePoolOperation validates common pool operation parameters
func ValidatePoolOperation() gin.HandlerFunc {
	// TODO: What operation to validate? Placeholder for now.
	// Name validation already has a function.
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			APIError(c, errors.New(errors.ZFSPoolInvalidName, "Pool name required"))
			return
		}

		if !poolNameRegex.MatchString(name) {
			APIError(c, errors.New(errors.ZFSPoolInvalidName, "Invalid pool name format"))
			return
		}
		c.Next()
	}
}

// ValidateDevicePaths validates device paths in pool creation
func ValidateDevicePaths() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var cfg pool.CreateConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		for _, spec := range cfg.VDevSpec {
			for _, device := range spec.Devices {
				if !devicePathRegex.MatchString(device) {
					APIError(c, errors.New(errors.ZFSPoolInvalidDevice, "Invalid device path"))
					return
				}
			}
		}
		c.Next()
	}
}

// ValidateMountPoint ensures mount points are safe
func ValidateMountPoint() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			MountPoint string `json:"mountpoint"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		if req.MountPoint != "" {
			// Check format
			if !mountPointRegex.MatchString(req.MountPoint) {
				APIError(c, errors.New(errors.ZFSInvalidMountPoint, "Invalid mount point format"))
				return
			}

			// Check restricted paths
			if restrictedPaths[req.MountPoint] {
				APIError(c, errors.New(errors.ZFSRestrictedMountPoint, "Mount point not allowed"))
				return
			}
		}
		c.Next()
	}
}

// ValidatePropertyValue ensures property values are safe
func ValidatePropertyValue() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		// Check length
		if len(req.Value) > maxPropertyValueLen {
			APIError(c, errors.New(errors.ZFSPropertyValueTooLong, "Property value too long"))
			return
		}

		c.Next()
	}
}

// EnhancedValidateDevicePaths adds additional device safety checks
func EnhancedValidateDevicePaths() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var cfg pool.CreateConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		// Check total number of devices
		totalDevices := 0
		for _, spec := range cfg.VDevSpec {
			totalDevices += len(spec.Devices)
			if totalDevices > maxDevicePaths {
				APIError(c, errors.New(errors.ZFSPoolTooManyDevices, "Too many devices specified"))
				return
			}

			for _, device := range spec.Devices {
				// Basic path validation
				if !devicePathRegex.MatchString(device) {
					APIError(c, errors.New(errors.ZFSPoolInvalidDevice, "Invalid device path"))
					return
				}

				// Check restricted devices
				if restrictedDevices[device] {
					APIError(c, errors.New(errors.ZFSPoolRestrictedDevice, "Device not allowed"))
					return
				}
			}
		}
		c.Next()
	}
}

// ValidateNameLength checks name length for all ZFS entities
func ValidateNameLength() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if len(name) > maxNameLength {
			APIError(c, errors.New(errors.ZFSNameTooLong, "Name exceeds maximum length"))
			return
		}
		c.Next()
	}
}

func ValidatePoolProperty(propCtx common.PoolPropContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		property := c.Param("property")
		if !common.IsValidPoolProperty(property, propCtx) {
			APIError(c, errors.New(errors.ZFSPropertyError, "Invalid pool property name"))
			return
		}
		c.Next()
	}
}

// ValidatePoolProperties validates zpool properties, not dataset
// PoolPropContext is used to determine the context of the property validation:
// - AnytimePoolPropContext: Properties that can be set at any time
// - CreatePoolPropContext: Properties that can be set at pool creation time
// - ImportPoolPropContext: Properties that can be set only at pool import time
// - ValidPoolSetPropContext: Properties that can be set at any time
// - ValidPoolGetPropContext: Properties that can be read any time
func ValidatePoolProperties(propCtx common.PoolPropContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Properties map[string]string `json:"properties"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		for k, v := range req.Properties {
			// Validate property name
			if !common.IsValidPoolProperty(k, propCtx) {
				APIError(c, errors.New(errors.ZFSPropertyError, "Invalid pool property name"))
				return
			}

			// Validate property value
			if len(v) > maxPropertyValueLen {
				APIError(c, errors.New(errors.ZFSPropertyValueTooLong, "Property value too long"))
				return
			}

		}
		c.Next()
	}
}

// ValidateZFSProperties validates ZFS dataset properties, not zpool properties
func ValidateZFSProperties() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Properties map[string]string `json:"properties"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		for k, v := range req.Properties {
			// Validate property name
			if !isValidDatasetProperty(k) {
				APIError(
					c,
					errors.New(
						errors.ZFSDatasetInvalidProperty,
						fmt.Sprintf("Invalid property name: %s", k),
					),
				)
				return
			}

			// Validate property value
			if len(v) > maxPropertyValueLen {
				APIError(c, errors.New(errors.ZFSPropertyValueTooLong, "Property value too long"))
				return
			}

		}
		c.Next()
	}
}

// ValidateBlockSize validates volume block size
func ValidateBlockSize() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			BlockSize string `json:"blocksize"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		if req.BlockSize != "" && !volumeSizeRegex.MatchString(req.BlockSize) {
			APIError(c, errors.New(errors.ZFSInvalidSize, "Invalid block size format"))
			return
		}
		c.Next()
	}
}

// ValidateCloneConfig validates clone creation parameters
func ValidateCloneConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read and store the raw body
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req dataset.CloneConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		// Validate clone name
		if !filesystemNameRegex.MatchString(req.CloneName) {
			APIError(c, errors.New(errors.ZFSDatasetInvalidName, "Invalid clone name format"))
			return
		}

		c.Next()
	}
}

// ValidateDiffConfig validates diff operation parameters
func ValidateDiffConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req dataset.DiffConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		ResetBody(c, body)

		// Validate number of names
		if len(req.Names) != 2 {
			APIError(c, errors.New(errors.CommandInvalidInput, "Exactly two names required"))
			return
		}

		// Validate each name
		for _, name := range req.Names {
			if err := common.ValidateZFSName(name, common.TypeSnapshot); err != nil {
				if err2 := common.ValidateZFSName(name, common.TypeFilesystem); err2 != nil {
					APIError(c, errors.New(errors.ZFSNameInvalid, "Invalid name format"))
					return
				}
			}
		}

		c.Next()
	}
}

// ValidatePermissionConfig validates ZFS permission operations
func ValidatePermissionConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req dataset.AllowConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		ResetBody(c, body)

		// Check mutually exclusive flags
		if (len(req.Users) > 0 && (len(req.Groups) > 0 || req.Everyone)) ||
			(len(req.Groups) > 0 && req.Everyone) {
			APIError(
				c,
				errors.New(
					errors.CommandInvalidInput,
					"Users, groups, and everyone flags are mutually exclusive",
				),
			)
			return
		}

		// Validate permission set name format if present
		if req.SetName != "" && !strings.HasPrefix(req.SetName, "@") {
			APIError(c, errors.New(errors.ZFSNameInvalid, "Permission set name must start with @"))
			return
		}

		// Validate permissions
		for _, perm := range req.Permissions {
			// Check if it's a permission set reference
			if strings.HasPrefix(perm, "@") {
				continue
			}
			// Check if it's a valid permission
			if _, ok := dataset.ZFSPermissions[perm]; !ok {
				APIError(
					c,
					errors.New(errors.ZFSNameInvalid, fmt.Sprintf("Invalid permission: %s", perm)),
				)
				return
			}
		}

		// Validate users/groups format
		userGroupRegex := regexp.MustCompile(`^[a-zA-Z0-9_][-a-zA-Z0-9_.]*[$]?$`)
		for _, user := range req.Users {
			if !userGroupRegex.MatchString(user) {
				APIError(
					c,
					errors.New(
						errors.ZFSNameInvalid,
						fmt.Sprintf("Invalid username format: %s", user),
					),
				)
				return
			}
		}
		for _, group := range req.Groups {
			if !userGroupRegex.MatchString(group) {
				APIError(
					c,
					errors.New(
						errors.ZFSNameInvalid,
						fmt.Sprintf("Invalid group name format: %s", group),
					),
				)
				return
			}
		}

		c.Next()
	}
}

// ValidateUnallowConfig validates ZFS unallow operations
func ValidateUnallowConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := ReadResetBody(c)
		if err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req dataset.UnallowConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			APIError(c, errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		ResetBody(c, body)

		// Similar checks as ValidatePermissionConfig
		if (len(req.Users) > 0 && (len(req.Groups) > 0 || req.Everyone)) ||
			(len(req.Groups) > 0 && req.Everyone) {
			APIError(
				c,
				errors.New(
					errors.CommandInvalidInput,
					"Users, groups, and everyone flags are mutually exclusive",
				),
			)
			return
		}

		// Validate permission set name if present
		if req.SetName != "" && !strings.HasPrefix(req.SetName, "@") {
			APIError(c, errors.New(errors.ZFSNameInvalid, "Permission set name must start with @"))
			return
		}

		c.Next()
	}
}
