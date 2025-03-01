package common

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

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
