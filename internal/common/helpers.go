package common

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
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

// Helper to add errors to context
func APIError(c *gin.Context, err error) {
	// Check if response has already been written
	if c.Writer.Written() {
		return
	}
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
