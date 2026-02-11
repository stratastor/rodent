// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/errors"
)

// TunnelRequest is the envelope sent from Toggle through the gRPC stream
type TunnelRequest struct {
	Service string            `json:"service"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   string            `json:"query,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"` // base64-encoded
}

// TunnelResponse is the envelope sent back through the gRPC stream
type TunnelResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"` // base64-encoded
}

// proxyHTTPRequest validates the tunnel request against config and forwards it
// to the target local service. Returns the serialized TunnelResponse as bytes.
func proxyHTTPRequest(tunnelReq *TunnelRequest) ([]byte, error) {
	cfg := config.GetConfig()

	// Look up service in config allowlist
	svc, exists := cfg.Tunnel.Services[tunnelReq.Service]
	if !exists {
		return nil, errors.New(
			errors.ServerRequestValidation,
			fmt.Sprintf("tunnel service %q not configured", tunnelReq.Service),
		)
	}

	// Validate HTTP method
	if !isMethodAllowed(tunnelReq.Method, svc.AllowedMethods) {
		return nil, errors.New(
			errors.ServerRequestValidation,
			fmt.Sprintf("method %q not allowed for service %q", tunnelReq.Method, tunnelReq.Service),
		)
	}

	// Validate path prefix
	if !isPathAllowed(tunnelReq.Path, svc.AllowedPaths) {
		return nil, errors.New(
			errors.ServerRequestValidation,
			fmt.Sprintf("path %q not allowed for service %q", tunnelReq.Path, tunnelReq.Service),
		)
	}

	// Build target URL
	targetURL := strings.TrimRight(svc.Address, "/") + tunnelReq.Path
	if tunnelReq.Query != "" {
		targetURL += "?" + tunnelReq.Query
	}

	// Decode body if present
	var bodyReader io.Reader
	if tunnelReq.Body != "" {
		bodyBytes, err := base64.StdEncoding.DecodeString(tunnelReq.Body)
		if err != nil {
			return nil, errors.Wrap(err, errors.ServerRequestValidation)
		}
		bodyReader = strings.NewReader(string(bodyBytes))
	}

	// Build HTTP request
	httpReq, err := http.NewRequest(tunnelReq.Method, targetURL, bodyReader)
	if err != nil {
		return nil, errors.Wrap(err, errors.ServerResponseError)
	}

	// Copy headers from tunnel request
	for k, v := range tunnelReq.Headers {
		httpReq.Header.Set(k, v)
	}

	// Parse timeout from config
	timeout := 30 * time.Second
	if svc.Timeout != "" {
		if parsed, err := time.ParseDuration(svc.Timeout); err == nil {
			timeout = parsed
		}
	}

	// Execute request
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, errors.ServerResponseError)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, errors.ServerResponseError)
	}

	// Collect response headers (single-value for simplicity)
	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	// Build tunnel response
	tunnelResp := TunnelResponse{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       base64.StdEncoding.EncodeToString(respBody),
	}

	return json.Marshal(tunnelResp)
}

func isMethodAllowed(method string, allowed []string) bool {
	if len(allowed) == 0 {
		return true // no restriction
	}
	method = strings.ToUpper(method)
	for _, m := range allowed {
		if strings.ToUpper(m) == method {
			return true
		}
	}
	return false
}

func isPathAllowed(path string, allowedPrefixes []string) bool {
	if len(allowedPrefixes) == 0 {
		return true // no restriction
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
