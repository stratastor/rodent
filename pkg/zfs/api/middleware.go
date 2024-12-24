package api

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

var (
	// ZFS naming conventions
	datasetNameRegex  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*(/[a-zA-Z0-9][a-zA-Z0-9_.-]*)*$`)
	snapshotNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	volumeSizeRegex   = regexp.MustCompile(`^\d+[KMGTP]?$`)
	bookmarkNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

	poolNameRegex   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]*$`)
	devicePathRegex = regexp.MustCompile(`^/dev/[a-zA-Z0-9/]+$`)

	propertyValueRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:/@+-]*$`)
	quotaRegex         = regexp.MustCompile(`^\d+[KMGTP]?(:|$)`)
	mountPointRegex    = regexp.MustCompile(`^/[a-zA-Z0-9/._-]*$`)

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

// ValidateDatasetName validates dataset name format
func ValidateDatasetName() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name != "" && !datasetNameRegex.MatchString(name) {
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
		property := c.Param("property")
		if property == "" || !isValidProperty(property) {
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
		var req struct {
			Size string `json:"size"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			return
		}
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

// ValidateZFSName validates any ZFS entity name
func ValidateZFSName(nameType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			return
		}

		var valid bool
		var errCode errors.ErrorCode
		var message string

		switch nameType {
		case "dataset":
			valid = datasetNameRegex.MatchString(name)
			errCode = errors.ZFSDatasetInvalidName
			message = "Invalid dataset name format"
		case "snapshot":
			valid = snapshotNameRegex.MatchString(name)
			errCode = errors.ZFSSnapshotInvalidName
			message = "Invalid snapshot name format"
		case "bookmark":
			valid = bookmarkNameRegex.MatchString(name)
			errCode = errors.ZFSBookmarkInvalidName
			message = "Invalid bookmark name format"
		}

		if !valid {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errCode, message),
			)
			return
		}
		c.Next()
	}
}

// isValidProperty maintains a list of valid ZFS properties
func isValidProperty(property string) bool {
	validProps := map[string]bool{
		"compression":    true,
		"atime":          true,
		"quota":          true,
		"recordsize":     true,
		"mountpoint":     true,
		"readonly":       true,
		"snapdir":        true,
		"sync":           true,
		"refquota":       true,
		"refreservation": true,
		"canmount":       true,
		"exec":           true,
		"setuid":         true,
		"devices":        true,
	}
	return validProps[property]
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

// ValidateDeviceInput validates device path input
func ValidateDeviceInput(paramName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req map[string]string
		if err := c.ShouldBindJSON(&req); err != nil {
			return
		}

		device, ok := req[paramName]
		if !ok || !devicePathRegex.MatchString(device) {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSPoolInvalidDevice,
					fmt.Sprintf("Invalid device path: %s", paramName)),
			)
			return
		}
		c.Next()
	}
}

// ValidateDevicePaths validates device paths in pool creation
func ValidateDevicePaths() gin.HandlerFunc {
	return func(c *gin.Context) {
		var cfg pool.CreateConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			return
		}

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
		var req struct {
			MountPoint string `json:"mountpoint"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			return
		}

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
		var req struct {
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			return
		}

		// Check length
		if len(req.Value) > maxPropertyValueLen {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSPropertyValueTooLong, "Property value too long"),
			)
			return
		}

		// Check format
		if !propertyValueRegex.MatchString(req.Value) {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				errors.New(errors.ZFSInvalidPropertyValue, "Invalid property value format"),
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
		var cfg pool.CreateConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			return
		}

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
