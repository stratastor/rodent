// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/services/state"
	"github.com/stratastor/rodent/pkg/httpclient"
)

const (
	// DefaultToggleBaseURL is the default URL for the Toggle API
	DefaultToggleBaseURL = "https://strata.foo"
)

// Client provides methods to interact with the Toggle API
type Client struct {
	HTTPClient *httpclient.Client
	Logger     logger.Logger
	baseURL    string
	jwt        string
}

// NewClient creates a new Toggle client
func NewClient(l logger.Logger, jwt string, baseURL string) (*Client, error) {
	cfg := config.GetConfig()
	if l == nil {
		var err error
		l, err = logger.NewTag(config.NewLoggerConfig(cfg), "toggle")
		if err != nil {
			return nil, fmt.Errorf("failed to create logger: %w", err)
		}
	}

	// Use provided baseURL or default
	if baseURL == "" {
		baseURL = DefaultToggleBaseURL
	}

	// Rest of the implementation remains the same
	clientCfg := httpclient.NewClientConfig()
	clientCfg.BaseURL = baseURL
	clientCfg.RetryCount = 3
	clientCfg.RetryWaitTime = 5 * time.Second
	clientCfg.BearerToken = jwt
	clientCfg.TLSConfig = &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	if cfg.Environment != "prod" && cfg.Environment != "production" {
		clientCfg.TLSConfig = nil
	}

	return &Client{
		HTTPClient: httpclient.NewClient(clientCfg),
		Logger:     l,
		baseURL:    baseURL,
		jwt:        jwt,
	}, nil
}

// ConfigChangeData contains information about a configuration change
type ConfigChangeData struct {
	ServiceName string    `json:"service_name"`
	ConfigPath  string    `json:"config_path"`
	UpdatedAt   time.Time `json:"updated_at"`
	Status      string    `json:"status"`
}

// ReportServiceConfigChange reports a service configuration change to the Toggle service
func (c *Client) ReportServiceConfigChange(
	ctx context.Context,
	serviceName string,
	data ConfigChangeData,
) error {
	if c.baseURL == "" || c.jwt == "" {
		c.Logger.Debug("Toggle service reporting disabled (no baseURL or JWT)")
		return nil
	}

	endpoint := fmt.Sprintf("%s/api/v1/nodes/configs", c.baseURL)

	// Create request body
	reqBody := map[string]interface{}{
		"service_name": data.ServiceName,
		"config_path":  data.ConfigPath,
		"updated_at":   data.UpdatedAt,
		"status":       data.Status,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to serialize request body: %w", err)
	}

	// Create request
	req := c.HTTPClient.R().
		SetContext(ctx).
		SetBody(jsonData).
		SetHeader("Content-Type", "application/json").
		SetAuthToken(c.jwt)

	// Execute request
	resp, err := req.Post(endpoint)
	if err != nil {
		return fmt.Errorf("failed to report configuration change: %w", err)
	}

	// Check response status
	if resp.StatusCode() >= 400 {
		return fmt.Errorf("configuration change report failed with status %d: %s",
			resp.StatusCode(), resp.String())
	}

	c.Logger.Debug("Successfully reported configuration change",
		"service", data.ServiceName,
		"status", data.Status,
		"configPath", data.ConfigPath)

	return nil
}

// ReportServiceState reports a service state change to the Toggle service
func (c *Client) ReportServiceState(ctx context.Context, event state.StateChangeEvent) error {
	if c.baseURL == "" || c.jwt == "" {
		c.Logger.Debug("Toggle service reporting disabled (no baseURL or JWT)")
		return nil
	}

	endpoint := fmt.Sprintf("%s/api/v1/nodes/services/status", c.baseURL)

	// Create request body
	reqBody := map[string]interface{}{
		"service_name": event.ServiceName,
		"prev_state":   string(event.PrevState),
		"new_state":    string(event.NewState),
		"timestamp":    event.Timestamp,
	}

	// Add details if available
	if event.Details != nil {
		reqBody["details"] = event.Details
	}

	// Convert to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to serialize request body: %w", err)
	}

	// Create request
	req := c.HTTPClient.R().
		SetContext(ctx).
		SetBody(jsonData).
		SetHeader("Content-Type", "application/json").
		SetAuthToken(c.jwt)

	// Execute request
	resp, err := req.Post(endpoint)
	if err != nil {
		return fmt.Errorf("failed to report service state: %w", err)
	}

	// Check response status
	if resp.StatusCode() >= 400 {
		return fmt.Errorf("service state report failed with status %d: %s",
			resp.StatusCode(), resp.String())
	}

	c.Logger.Debug("Successfully reported service state",
		"service", event.ServiceName,
		"state", string(event.NewState))

	return nil
}

func (c *Client) GetOrgID() (string, error) {
	return extractSubFromJWT(c.jwt)
}

// extractSubFromJWT extracts the 'sub' claim from a JWT
func extractSubFromJWT(tokenString string) (string, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid JWT format")
	}

	// Decode the payload (second part)
	payload, err := decodeBase64UrlSafe(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Extract the 'sub' claim
	sub, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("sub claim not found or not a string in JWT")
	}

	return sub, nil
}

// decodeBase64UrlSafe decodes base64url-encoded data
func decodeBase64UrlSafe(s string) ([]byte, error) {
	// Add padding if necessary
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}

	// Replace URL-safe chars with standard base64 chars
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")

	return base64.StdEncoding.DecodeString(s)
}
