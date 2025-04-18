// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/httpclient"
)

// ToggleClient defines the interface for both REST and gRPC clients
type ToggleClient interface {
	// Register registers the Rodent node with Toggle
	Register(ctx context.Context) (*RegistrationResult, error)
	
	// GetOrgID extracts the organization ID from the JWT
	GetOrgID() (string, error)
	
	// Connect establishes a bidirectional streaming connection with Toggle
	// This is only implemented for gRPC clients; will return an error for REST clients
	Connect(ctx context.Context) (*StreamConnection, error)
}

// RESTClientAdapter adapts the existing REST client to the ToggleClient interface
type RESTClientAdapter struct {
	*Client
}

// Connect implements the Connect method for the REST client
// Since REST clients cannot establish bidirectional streams, this always returns an error
func (a *RESTClientAdapter) Connect(ctx context.Context) (*StreamConnection, error) {
	return nil, fmt.Errorf("streaming connections are not supported for REST clients")
}

// Register implements the Register method for the REST client
func (a *RESTClientAdapter) Register(ctx context.Context) (*RegistrationResult, error) {
	// Extract org ID from JWT
	orgID, err := a.GetOrgID()
	if err != nil {
		return nil, err
	}

	registerPath := fmt.Sprintf("/api/v1/toggle/organizations/%s/rodent-nodes", orgID)

	// Create empty payload
	payload := map[string]string{}

	reqCfg := httpclient.RequestConfig{
		Path: registerPath,
		Body: payload,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	resp, err := a.HTTPClient.NewRequest(reqCfg).Post()
	if err != nil {
		return nil, err
	}

	if resp != nil {
		if resp.StatusCode() == http.StatusOK {
			a.Logger.Info("Node already registered with Toggle service", "orgID", orgID)
			return &RegistrationResult{
				Message: "Node already registered",
			}, nil
		}
	}

	if resp.StatusCode() != http.StatusCreated {
		return nil, fmt.Errorf("registration failed with status %d: %s",
			resp.StatusCode(), resp.String())
	}

	// Parse the response
	var regResponse struct {
		Message     string    `json:"message"`
		Domain      string    `json:"domain"`
		Certificate string    `json:"certificate"`
		PrivateKey  string    `json:"private_key"`
		ExpiresOn   time.Time `json:"expires_on"`
	}

	if err := json.Unmarshal(resp.Body(), &regResponse); err != nil {
		return nil, fmt.Errorf("failed to parse registration response: %w", err)
	}

	result := &RegistrationResult{
		Message:     regResponse.Message,
		Domain:      regResponse.Domain,
		Certificate: regResponse.Certificate,
		PrivateKey:  regResponse.PrivateKey,
		ExpiresOn:   regResponse.ExpiresOn,
	}

	a.Logger.Info("Registration successful with Toggle service (REST)",
		"orgID", orgID, "domain", result.Domain,
		"expiresOn", result.ExpiresOn)

	return result, nil
}

// NewClientAdapter creates a new adapter for the existing REST client
func NewClientAdapter(client *Client) *RESTClientAdapter {
	return &RESTClientAdapter{
		Client: client,
	}
}

// NewToggleClient creates a new client based on the JWT claims
func NewToggleClient(l logger.Logger, jwt string, baseURL string, rpcAddr string) (ToggleClient, error) {
	// Check if the JWT indicates a private network
	isPrivate, err := IsPrivateNetwork(jwt)
	if err != nil {
		l.Warn("Failed to determine network type from JWT", "error", err)
		// Default to REST client on error
		isPrivate = false
	}

	if isPrivate {
		l.Info("Creating gRPC client for private network")
		// Use rpcAddr for gRPC if provided, otherwise fall back to baseURL
		serverAddr := rpcAddr
		if serverAddr == "" {
			serverAddr = baseURL
			l.Warn("No specific gRPC address provided, using baseURL", "baseURL", baseURL)
		}
		return NewGRPCClient(l, jwt, serverAddr)
	}

	// For public networks, use the REST client
	l.Info("Creating REST client for public network")
	client, err := NewClient(l, jwt, baseURL)
	if err != nil {
		return nil, err
	}
	
	return NewClientAdapter(client), nil
}