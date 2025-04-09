// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/pkg/errors"
)

var (
	// Path traversal patterns to watch for
	pathTraversalRegex = regexp.MustCompile(`(^|/|\\)\.\.($|/|\\)`)

	// Non-printable characters
	nonPrintableRegex = regexp.MustCompile(`[^\x20-\x7E]`)

	// Valid path characters (alphanumeric, plus common safe symbols)
	validPathCharRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-./]+$`)
)

// ValidatePath checks if a path is safe and valid
func ValidatePath(path string) error {
	// Path must not be empty
	if path == "" {
		return errors.New(errors.SharesInvalidInput, "Path cannot be empty")
	}

	// Path must be absolute
	if !strings.HasPrefix(path, "/") {
		return errors.New(errors.SharesInvalidInput, "Path must be absolute")
	}

	// Clean the path to remove any redundant elements
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts
	if pathTraversalRegex.MatchString(cleanPath) {
		return errors.New(errors.SharesInvalidInput, "Path contains directory traversal sequences").
			WithMetadata("path", path)
	}

	// Check for non-printable characters
	if nonPrintableRegex.MatchString(cleanPath) {
		return errors.New(errors.SharesInvalidInput, "Path contains non-printable characters").
			WithMetadata("path", path)
	}

	// Check for valid path characters
	if !validPathCharRegex.MatchString(cleanPath) {
		return errors.New(errors.SharesInvalidInput, "Path contains invalid characters").
			WithMetadata("path", path)
	}

	return nil
}

// ValidateFilesystemPath middleware for validating filesystem paths
func ValidateFilesystemPath() gin.HandlerFunc {
	return func(c *gin.Context) {
		pathParam := c.Param("path")
		if pathParam == "" {
			pathParam = c.Query("path")
			if pathParam == "" {
				// Try to get path from JSON body
				var body map[string]interface{}
				if err := c.ShouldBindJSON(&body); err == nil {
					if path, ok := body["path"].(string); ok {
						pathParam = path
					}
				}

				// Reset request body for subsequent handlers
				if c.Request.Body != nil {
					data, _ := json.Marshal(body)
					c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
				}
			}
		}

		// URL decode the path if needed
		decodedPath, err := url.PathUnescape(pathParam)
		if err != nil {
			APIError(c, errors.New(errors.SharesInvalidInput, "Invalid URL encoding in path").
				WithMetadata("path", pathParam).
				WithMetadata("error", err.Error()))
			return
		}

		// Validate the path
		if err := ValidatePath(decodedPath); err != nil {
			APIError(c, err)
			return
		}

		// Store the clean path in the context
		cleanPath := filepath.Clean(decodedPath)
		c.Set("cleanPath", cleanPath)

		c.Next()
	}
}

// GetCleanPath gets the cleaned and validated path from the context
func GetCleanPath(c *gin.Context) string {
	if path, exists := c.Get("cleanPath"); exists {
		return path.(string)
	}
	return ""
}
