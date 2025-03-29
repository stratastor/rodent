// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package toggle

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/services/traefik"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/httpclient"
)

const (
	// DefaultRetryInterval is the interval between retry attempts
	DefaultRetryInterval = 1 * time.Minute
)

// RegisterNode registers this Rodent node with the Toggle service
func RegisterNode(
	ctx context.Context,
	c *client.Client,
) error {
	// Extract org ID from JWT
	orgID, err := c.GetOrgID()
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

	resp, err := c.HTTPClient.NewRequest(reqCfg).Post()
	if err != nil {
		return fmt.Errorf("registration request failed: %w", err)
	}

	if resp != nil {
		if resp.StatusCode() == http.StatusFound {
			c.Logger.Info("Node already registered with Toggle service", "orgID", orgID)
			return nil
		}
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

	c.Logger.Info("Registration successful with Toggle service",
		"orgID", orgID, "domain", regResponse.Domain,
		"expiresOn", regResponse.ExpiresOn)

	cfg := config.GetConfig()
	if !cfg.Development.Enabled {
		traefikSvc, err := traefik.NewClient(c.Logger)
		if err != nil {
			return fmt.Errorf("failed to create Traefik client: %w", err)
		}
		if err := traefikSvc.InstallCertificate(ctx, traefik.CertificateData{
			Domain:      regResponse.Domain,
			Certificate: regResponse.Certificate,
			PrivateKey:  regResponse.PrivateKey,
			ExpiresOn:   regResponse.ExpiresOn,
		}); err != nil {
			return fmt.Errorf("failed to install certificate: %w", err)
		}

		c.Logger.Info("Certificate installed successfully")
	}
	return nil
}

// StartRegistrationProcess begins the async process of registering with Toggle
func StartRegistrationProcess(ctx context.Context, l logger.Logger) {
	cfg := config.GetConfig()

	if !cfg.StrataSecure {
		if l != nil {
			l.Info("StrataSecure is disabled, skipping registration")
		}
		return
	}

	// Skip if JWT is not configured
	if cfg.Toggle.JWT == "" {
		if l != nil {
			l.Info("Toggle JWT not configured, skipping registration")
		}
		return
	}

	// Create a Toggle client
	client, err := client.NewClient(l, cfg.Toggle.JWT, cfg.Toggle.BaseURL)
	if err != nil {
		if l != nil {
			l.Error("Failed to create Toggle client", "error", err)
		}
		return
	}

	go runRegistrationProcess(ctx, client, l)
}

func runRegistrationProcess(
	ctx context.Context,
	client *client.Client,
	l logger.Logger,
) {
	retryInterval := DefaultRetryInterval

	for {
		err := RegisterNode(ctx, client)
		if err == nil {
			// Registration successful
			return
		}

		// Log failure and retry
		if l != nil {
			l.Error("Failed to register with Toggle service", "error", err)
			l.Info("Will retry registration in 1 minute")
		}

		// Wait for retry interval or context cancellation
		select {
		case <-time.After(retryInterval):
			// Continue to retry
		case <-ctx.Done():
			// Context cancelled, stop retrying
			if l != nil {
				l.Info("Registration process stopped due to context cancellation")
			}
			return
		}
	}
}
