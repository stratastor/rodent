// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/constants"
	"github.com/stratastor/rodent/pkg/httpclient"
)

const (
	// DefaultToggleBaseURL is the default URL for the Toggle API
	DefaultToggleBaseURL = "https://strata.foo"

	// DefaultRetryInterval is the interval between retry attempts
	DefaultRetryInterval = 1 * time.Minute
)

// Client provides methods to interact with the Toggle API
type Client struct {
	httpClient *httpclient.Client
	logger     logger.Logger
	baseURL    string
	jwt        string
}

// NewClient creates a new Toggle client
func NewClient(jwt string, l logger.Logger) *Client {
	cfg := config.GetConfig()
	if l == nil {
		var err error
		l, err = logger.NewTag(config.NewLoggerConfig(cfg), "toggle")
		if err != nil {
			fmt.Printf("Failed to create logger for Toggle client: %v\n", err)
		}
	}
	baseURL := DefaultToggleBaseURL
	if cfg.Toggle.BaseURL != "" {
		baseURL = cfg.Toggle.BaseURL
	}

	clientCfg := httpclient.NewClientConfig()
	clientCfg.BaseURL = baseURL
	clientCfg.RetryCount = 3
	clientCfg.RetryWaitTime = 5 * time.Second
	clientCfg.BearerToken = jwt

	return &Client{
		httpClient: httpclient.NewClient(clientCfg),
		logger:     l,
		baseURL:    baseURL,
		jwt:        jwt,
	}
}

// RegisterNode registers this Rodent node with the Toggle service
func (c *Client) RegisterNode(ctx context.Context) error {
	// Extract org ID from JWT
	orgID, err := extractSubFromJWT(c.jwt)
	if err != nil {
		return fmt.Errorf("failed to extract organization ID from JWT: %w", err)
	}

	payload := map[string]string{}

	registerPath := fmt.Sprintf("/api/v1/toggle/organizations/%s/rodent-nodes", orgID)

	reqCfg := httpclient.RequestConfig{
		Path: registerPath,
		Body: payload,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	resp, err := c.httpClient.NewRequest(reqCfg).Post()
	if err != nil {
		return fmt.Errorf("registration request failed: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated {
		return fmt.Errorf("registration failed with status %d: %s",
			resp.StatusCode(), resp.String())
	}

	// Parse the response which now contains certificate information
	var regResponse struct {
		Message     string    `json:"message"`
		Domain      string    `json:"domain"`
		Certificate string    `json:"certificate"`
		PrivateKey  string    `json:"private_key"`
		ExpiresOn   time.Time `json:"expires_on"`
	}

	if err := json.Unmarshal(resp.Body(), &regResponse); err != nil {
		return fmt.Errorf("failed to parse registration response: %w", err)
	}

	// Ensure certificate directory exists
	certDir := constants.DefaultTraefikCertDir
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create certificate directory: %w", err)
	}

	// Write certificate to file
	certFile := filepath.Join(certDir, regResponse.Domain+".pem")
	if err := os.WriteFile(certFile, []byte(regResponse.Certificate), 0644); err != nil {
		return fmt.Errorf("failed to write certificate file: %w", err)
	}

	// Write private key to file
	keyFile := filepath.Join(certDir, regResponse.Domain+".key")
	if err := os.WriteFile(keyFile, []byte(regResponse.PrivateKey), 0600); err != nil {
		return fmt.Errorf("failed to write private key file: %w", err)
	}

	c.logger.Info("Successfully registered with Toggle service",
		"orgID", orgID, "domain", regResponse.Domain,
		"certFile", certFile, "keyFile", keyFile, "expiresOn", regResponse.ExpiresOn)

	return nil
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
