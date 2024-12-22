package api

import (
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
