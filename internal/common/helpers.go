package common

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/errors"
)

// Global logger
var Log logger.Logger

func init() {
	var err error
	Log, err = logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "global")
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
}

// GenUUID generates a new UUID using V7, but falls back to V4 if V7 errors
func UUID7() string {
	id := ""
	uv7, err := uuid.NewV7()
	if err != nil {
		id = uuid.New().String()
	} else {
		id = uv7.String()
	}
	return id
}

// Helper to add errors to context
func APIError(c *gin.Context, err error) {
	if rodentErr, ok := err.(*errors.RodentError); ok {
		// Do not include command in the error response
		rodentErr.Metadata["command"] = ""
		if rodentErr.Metadata["output"] != "" {
			rodentErr.Message += " - " + rodentErr.Metadata["output"]
		}
		c.JSON(rodentErr.HTTPStatus, gin.H{
			"error": gin.H{
				"code":      rodentErr.Code,
				"domain":    rodentErr.Domain,
				"message":   rodentErr.Message,
				"details":   rodentErr.Details,
				"metadata":  rodentErr.Metadata,
				"timestamp": time.Now().Format(time.RFC3339),
			},
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message":   err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			},
		})
	}
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
