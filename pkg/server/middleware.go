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

package server

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
)

// LoggerMiddleware creates a dedicated middleware function for better reusability and testing
func LoggerMiddleware(l logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Skip logging for health check endpoints
		if path == "/health" {
			c.Next()
			return
		}

		// Get or generate request ID
		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
			c.Header("X-Request-Id", requestID)
		}

		// Store request ID in context for error correlation
		c.Set("request_id", requestID)

		// Process request
		c.Next()

		// Build common request attributes
		attrs := []slog.Attr{
			slog.String("request_id", requestID),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", query),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.Int("bytes_out", c.Writer.Size()),
			slog.String("ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
		}

		// Add optional request headers
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			attrs = append(attrs, slog.String("forwarded_for", xff))
		}

		// Handle errors if present
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				if re, ok := err.Err.(*errors.RodentError); ok {
					// Add RodentError fields
					attrs = append(attrs,
						slog.Int("error_code", int(re.Code)),
						slog.String("error_domain", string(re.Domain)),
						slog.String("error_message", re.Message),
						slog.String("error_details", re.Details),
					)

					// Add metadata as individual fields
					for k, v := range re.Metadata {
						attrs = append(attrs, slog.String("error_metadata_"+k, v))
					}
				} else {
					// Handle non-RodentErrors
					attrs = append(attrs, slog.String("error", err.Error()))
				}
			}

			// Log as error for 5xx, warn for 4xx
			switch {
			case c.Writer.Status() >= 500:
				l.Error("Server Error", logAttrs(attrs)...)
			case c.Writer.Status() >= 400:
				l.Warn("Client Error", logAttrs(attrs)...)
			}
		} else {
			// Log successful requests
			l.Info("Request", logAttrs(attrs)...)
		}
	}
}

// Helper to convert slog.Attr slice to interface slice
func logAttrs(attrs []slog.Attr) []interface{} {
	args := make([]interface{}, len(attrs)*2)
	for i, attr := range attrs {
		args[i*2] = attr.Key
		args[i*2+1] = attr.Value.Any()
	}
	return args
}
