// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/internal/events"
)

const (
	// IMDS endpoints
	imdsTokenURL          = "http://169.254.169.254/latest/api/token"
	imdsSpotActionURL     = "http://169.254.169.254/latest/meta-data/spot/instance-action"
	imdsTokenTTLHeader    = "X-aws-ec2-metadata-token-ttl-seconds"
	imdsTokenTTLSeconds   = "21600" // 6 hours
	imdsTokenRequestHeader = "X-aws-ec2-metadata-token"

	// Polling and timeout configuration
	spotPollInterval   = 5 * time.Second
	spotHTTPTimeout    = 2 * time.Second
	imdsTokenRefreshAt = 5 * time.Hour
)

// spotInstanceAction represents the JSON response from the IMDS spot action endpoint
type spotInstanceAction struct {
	Action string `json:"action"`
	Time   string `json:"time"`
}

// SpotMonitor polls the EC2 instance metadata service to detect
// spot instance termination notices and emits an event when detected.
type SpotMonitor struct {
	ctx    context.Context
	cancel context.CancelFunc
	client *http.Client

	mu        sync.Mutex
	isRunning bool

	// IMDSv2 token caching
	token          string
	tokenExpiresAt time.Time
}

// NewSpotMonitor creates a new SpotMonitor.
func NewSpotMonitor(parentCtx context.Context) *SpotMonitor {
	ctx, cancel := context.WithCancel(parentCtx)
	return &SpotMonitor{
		ctx:    ctx,
		cancel: cancel,
		client: &http.Client{
			Timeout: spotHTTPTimeout,
		},
	}
}

// Start begins polling the IMDS spot termination endpoint.
func (s *SpotMonitor) Start() {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	go s.poll()
}

// Stop terminates the spot monitor.
func (s *SpotMonitor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}
	s.cancel()
	s.isRunning = false
}

// poll is the main loop that checks for spot termination notices.
func (s *SpotMonitor) poll() {
	ticker := time.NewTicker(spotPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			action, detected := s.checkSpotTermination()
			if detected {
				common.Log.Warn("EC2 spot termination notice received",
					"action", action.Action,
					"time", action.Time)
				events.EmitSystemSpotTermination(action.Action, action.Time)
				// Stop polling after emitting the event once
				return
			}
		}
	}
}

// refreshToken fetches a new IMDSv2 session token if needed.
func (s *SpotMonitor) refreshToken() error {
	if s.token != "" && time.Now().Before(s.tokenExpiresAt) {
		return nil
	}

	req, err := http.NewRequestWithContext(s.ctx, http.MethodPut, imdsTokenURL, nil)
	if err != nil {
		return fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set(imdsTokenTTLHeader, imdsTokenTTLSeconds)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching IMDS token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IMDS token request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading IMDS token response: %w", err)
	}

	s.token = string(body)
	s.tokenExpiresAt = time.Now().Add(imdsTokenRefreshAt)
	return nil
}

// checkSpotTermination queries the IMDS spot action endpoint.
// Returns the parsed action and true if a termination notice is found.
// Returns nil and false on 404 (no termination) or any error (not on EC2 / IMDS unavailable).
func (s *SpotMonitor) checkSpotTermination() (*spotInstanceAction, bool) {
	if err := s.refreshToken(); err != nil {
		// Silently continue -- not on EC2 or metadata service unavailable
		return nil, false
	}

	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, imdsSpotActionURL, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set(imdsTokenRequestHeader, s.token)

	resp, err := s.client.Do(req)
	if err != nil {
		// Connection error -- silently continue
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// 404 is normal -- no termination pending
		return nil, false
	}

	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}

	var action spotInstanceAction
	if err := json.Unmarshal(body, &action); err != nil {
		common.Log.Error("Failed to parse spot termination response", "error", err)
		return nil, false
	}

	return &action, true
}
