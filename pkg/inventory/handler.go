// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
)

// Handler handles REST API requests for inventory
type Handler struct {
	collector *Collector
	logger    logger.Logger
}

// APIResponse represents a standardized API response format
type APIResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents error information in API responses
type APIError struct {
	Code    int                    `json:"code"`
	Domain  string                 `json:"domain"`
	Message string                 `json:"message"`
	Details string                 `json:"details,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// NewHandler creates a new inventory API handler
func NewHandler(collector *Collector, logger logger.Logger) *Handler {
	return &Handler{
		collector: collector,
		logger:    logger,
	}
}

// sendSuccess sends a successful response with the standardized format
func (h *Handler) sendSuccess(c *gin.Context, statusCode int, result interface{}) {
	response := APIResponse{
		Success: true,
		Result:  result,
	}
	c.JSON(statusCode, response)
}

// sendError sends an error response with the standardized format
func (h *Handler) sendError(c *gin.Context, err error) {
	response := APIResponse{
		Success: false,
	}

	if rodentErr, ok := err.(*errors.RodentError); ok {
		h.logger.Error("Inventory API error",
			"error", err,
			"code", rodentErr.Code,
			"domain", rodentErr.Domain,
			"path", c.Request.URL.Path)

		response.Error = &APIError{
			Code:    int(rodentErr.Code),
			Domain:  string(rodentErr.Domain),
			Message: rodentErr.Message,
			Details: rodentErr.Details,
		}

		// Add metadata if available
		if len(rodentErr.Metadata) > 0 {
			response.Error.Meta = make(map[string]interface{})
			for k, v := range rodentErr.Metadata {
				response.Error.Meta[k] = v
			}
		}

		c.JSON(rodentErr.HTTPStatus, response)
		return
	}

	// Fallback for non-RodentError
	h.logger.Error("Inventory API error", "error", err, "path", c.Request.URL.Path)
	response.Error = &APIError{
		Code:    500,
		Domain:  "INVENTORY",
		Message: "Internal server error",
		Details: err.Error(),
	}
	c.JSON(http.StatusInternalServerError, response)
}

// GetInventory handles GET /inventory
func (h *Handler) GetInventory(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	opts := h.parseCollectOptions(c)

	h.logger.Debug("Collecting inventory",
		"detail_level", opts.DetailLevel,
		"include", opts.Include,
		"exclude", opts.Exclude)

	// Collect inventory
	inventory, err := h.collector.CollectInventory(ctx, opts)
	if err != nil {
		h.sendError(c, errors.Wrap(err, errors.ServerInternalError))
		return
	}

	h.sendSuccess(c, http.StatusOK, inventory)
}

// parseCollectOptions parses query parameters into CollectOptions
func (h *Handler) parseCollectOptions(c *gin.Context) CollectOptions {
	opts := CollectOptions{
		DetailLevel: DetailLevelBasic, // Default detail level
	}

	// Parse detail_level parameter
	if detailLevel := c.Query("detail_level"); detailLevel != "" {
		switch DetailLevel(detailLevel) {
		case DetailLevelSummary, DetailLevelBasic, DetailLevelFull:
			opts.DetailLevel = DetailLevel(detailLevel)
		default:
			h.logger.Warn("Invalid detail_level parameter, using default", "value", detailLevel)
		}
	}

	// Parse include parameter (comma-separated list)
	if include := c.Query("include"); include != "" {
		opts.Include = strings.Split(include, ",")
		// Trim whitespace from each item
		for i := range opts.Include {
			opts.Include[i] = strings.TrimSpace(opts.Include[i])
		}
	}

	// Parse exclude parameter (comma-separated list)
	if exclude := c.Query("exclude"); exclude != "" {
		opts.Exclude = strings.Split(exclude, ",")
		// Trim whitespace from each item
		for i := range opts.Exclude {
			opts.Exclude[i] = strings.TrimSpace(opts.Exclude[i])
		}
	}

	return opts
}
