package api

import (
	"bytes"
	"io"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/common"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

var (
	// ZFS naming conventions
	filesystemNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*(/[a-zA-Z0-9][a-zA-Z0-9_.-]*)*$`)
	snapshotNameRegex   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	volumeSizeRegex     = regexp.MustCompile(`^\d+[KMGTP]?$`)
	bookmarkNameRegex   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

	poolNameRegex   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]*$`)
	devicePathRegex = regexp.MustCompile(`^/dev/[a-zA-Z0-9/]+$`)

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
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSDatasetInvalidName, "Invalid dataset name format"),
			)
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Property string `json:"property" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		property := req.Property
		if property == "" || !isValidDatasetProperty(property) {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSDatasetInvalidProperty, "Invalid property name"),
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
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
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSInvalidSize, "Invalid volume size format"),
			)
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Name  string   `json:"name"`
			Names []string `json:"names"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, err.Error()))
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
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Either 'name' or 'names' must be provided"))
			return
		}

		for _, name := range req.Names {
			if name == "" {
				c.AbortWithStatusJSON(
					http.StatusBadRequest,
					errors.New(errors.ZFSDatasetInvalidName, "Invalid dataset name format"),
				)
				return
			}

			// Validate name format
			switch dtype {
			case common.TypeZFSEntityMask:
				err := common.EntityNameCheck(name)
				if err != nil {
					c.AbortWithStatusJSON(
						http.StatusBadRequest,
						err,
					)
					return
				}
			case common.TypeDatasetMask:
				err := common.DatasetNameCheck(name)
				if err != nil {
					c.AbortWithStatusJSON(
						http.StatusBadRequest,
						err,
					)
					return
				}
			case common.TypeBookmark | common.TypeSnapshot:
				// This is the case for clone creation where the name can be either a bookmark or snapshot
				errbm := common.ValidateZFSName(name, common.TypeBookmark)
				errsnap := common.ValidateZFSName(name, common.TypeSnapshot)
				if errbm != nil && errsnap != nil {
					c.AbortWithStatusJSON(
						http.StatusBadRequest,
						errors.New(errors.ZFSNameInvalid, "Name expected to be either a bookmark or snapshot"),
					)
					return
				}
			default:
				err := common.ValidateZFSName(name, dtype)
				if err != nil {
					c.AbortWithStatusJSON(
						http.StatusBadRequest,
						err,
					)
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
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSPoolInvalidName, "Invalid pool name format"),
			)
			return
		}
		c.Next()
	}
}

// ValidatePoolOperation validates common pool operation parameters
func ValidatePoolOperation() gin.HandlerFunc {
	// TODO: What operation to validate? Placeholder for now.
	// Name validation already has a funciton.
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSPoolInvalidName, "Pool name required"),
			)
			return
		}

		if !poolNameRegex.MatchString(name) {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSPoolInvalidName, "Invalid pool name format"),
			)
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var cfg pool.CreateConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		for _, spec := range cfg.VDevSpec {
			for _, device := range spec.Devices {
				if !devicePathRegex.MatchString(device) {
					c.AbortWithStatusJSON(
						http.StatusBadRequest,
						errors.New(errors.ZFSPoolInvalidDevice, "Invalid device path"),
					)
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			MountPoint string `json:"mountpoint"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		if req.MountPoint != "" {
			// Check format
			if !mountPointRegex.MatchString(req.MountPoint) {
				c.AbortWithStatusJSON(
					http.StatusBadRequest,
					errors.New(errors.ZFSInvalidMountPoint, "Invalid mount point format"),
				)
				return
			}

			// Check restricted paths
			if restrictedPaths[req.MountPoint] {
				c.AbortWithStatusJSON(
					http.StatusForbidden,
					errors.New(errors.ZFSRestrictedMountPoint, "Mount point not allowed"),
				)
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		// Check length
		if len(req.Value) > maxPropertyValueLen {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSPropertyValueTooLong, "Property value too long"),
			)
			return
		}

		// Special handling for quota values
		if property := c.Param("property"); property == "quota" || property == "refquota" {
			if !quotaRegex.MatchString(req.Value) {
				c.AbortWithStatusJSON(
					http.StatusBadRequest,
					errors.New(errors.ZFSQuotaInvalid, "Invalid quota format"),
				)
				return
			}
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var cfg pool.CreateConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		// Check total number of devices
		totalDevices := 0
		for _, spec := range cfg.VDevSpec {
			totalDevices += len(spec.Devices)
			if totalDevices > maxDevicePaths {
				c.AbortWithStatusJSON(
					http.StatusBadRequest,
					errors.New(errors.ZFSPoolTooManyDevices, "Too many devices specified"),
				)
				return
			}

			for _, device := range spec.Devices {
				// Basic path validation
				if !devicePathRegex.MatchString(device) {
					c.AbortWithStatusJSON(
						http.StatusBadRequest,
						errors.New(errors.ZFSPoolInvalidDevice, "Invalid device path"),
					)
					return
				}

				// Check restricted devices
				if restrictedDevices[device] {
					c.AbortWithStatusJSON(
						http.StatusForbidden,
						errors.New(errors.ZFSPoolRestrictedDevice, "Device not allowed"),
					)
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
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSNameTooLong, "Name exceeds maximum length"),
			)
			return
		}
		c.Next()
	}
}

func ValidatePoolProperty(propCtx common.PoolPropContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		property := c.Param("property")
		if !common.IsValidPoolProperty(property, propCtx) {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSPropertyError, "Invalid pool property name"),
			)
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Properties map[string]string `json:"properties"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		for k, v := range req.Properties {
			// Validate property name
			if !common.IsValidPoolProperty(k, propCtx) {
				c.AbortWithStatusJSON(
					http.StatusBadRequest,
					errors.New(errors.ZFSPropertyError, "Invalid pool property name"),
				)
				return
			}

			// Validate property value
			if len(v) > maxPropertyValueLen {
				c.AbortWithStatusJSON(
					http.StatusBadRequest,
					errors.New(errors.ZFSPropertyValueTooLong, "Property value too long"),
				)
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			Properties map[string]string `json:"properties"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		for k, v := range req.Properties {
			// Validate property name
			if !isValidDatasetProperty(k) {
				c.AbortWithStatusJSON(
					http.StatusBadRequest,
					errors.New(errors.ZFSDatasetInvalidProperty, "Invalid property name"),
				)
				return
			}

			// Validate property value
			if len(v) > maxPropertyValueLen {
				c.AbortWithStatusJSON(
					http.StatusBadRequest,
					errors.New(errors.ZFSPropertyValueTooLong, "Property value too long"),
				)
				return
			}

			// Special handling for quota values
			if k == "quota" || k == "refquota" {
				if !quotaRegex.MatchString(v) {
					c.AbortWithStatusJSON(
						http.StatusBadRequest,
						errors.New(errors.ZFSQuotaInvalid, "Invalid quota format"),
					)
					return
				}
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req struct {
			BlockSize string `json:"blocksize"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		if req.BlockSize != "" && !volumeSizeRegex.MatchString(req.BlockSize) {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSInvalidSize, "Invalid block size format"),
			)
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
			c.JSON(http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, "Failed to read request body"))
			return
		}

		var req dataset.CloneConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ServerRequestValidation, err.Error()))
			return
		}
		// Reset the body so it can be re-read by `ShouldBindJSON` and subsequent handlers
		ResetBody(c, body)

		// Validate clone name
		if !filesystemNameRegex.MatchString(req.CloneName) {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSDatasetInvalidName, "Invalid clone name format"),
			)
			return
		}

		c.Next()
	}
}
