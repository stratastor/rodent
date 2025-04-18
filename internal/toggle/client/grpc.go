// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// GRPCClient provides methods to interact with the Toggle API via gRPC
type GRPCClient struct {
	Logger  logger.Logger
	baseURL string
	jwt     string
	conn    *grpc.ClientConn
	client  proto.RodentServiceClient
}

// NewGRPCClient creates a new Toggle gRPC client
func NewGRPCClient(l logger.Logger, jwt string, baseURL string) (*GRPCClient, error) {
	if l == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Use provided baseURL or default
	if baseURL == "" {
		baseURL = DefaultToggleBaseURL
	}

	// Remove protocol prefix if present
	baseURL = removeProtocolPrefix(baseURL)

	// Get config to check environment
	cfg := config.GetConfig()
	var opts []grpc.DialOption

	// Use TLS for production environments, insecure for dev/test
	if cfg.Environment == "prod" || cfg.Environment == "production" {
		// Use TLS credentials for production
		creds := credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})
		opts = append(opts, grpc.WithTransportCredentials(creds))
		l.Info("Using TLS for gRPC connection")
	} else {
		// Use insecure credentials for development
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		l.Info("Using insecure connection for gRPC (development mode)")
	}

	// Add keepalive parameters for long-lived connections
	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout:             20 * time.Second, // wait 20 seconds for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}
	opts = append(opts, grpc.WithKeepaliveParams(kacp))

	// Allow clients to receive messages larger than default max size
	opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16*1024*1024))) // 16MB

	// Create new gRPC client with appropriate credentials
	conn, err := grpc.Dial(baseURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	client := proto.NewRodentServiceClient(conn)

	return &GRPCClient{
		Logger:  l,
		baseURL: baseURL,
		jwt:     jwt,
		conn:    conn,
		client:  client,
	}, nil
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetOrgID extracts the organization ID (sub claim) from the JWT
func (c *GRPCClient) GetOrgID() (string, error) {
	return ExtractSubFromJWT(c.jwt)
}

// Register registers the Rodent node with the Toggle service via gRPC
func (c *GRPCClient) Register(ctx context.Context) (*RegistrationResult, error) {
	if c.baseURL == "" || c.jwt == "" {
		c.Logger.Debug("Toggle service registration disabled (no baseURL or JWT)")
		return nil, fmt.Errorf("toggle service registration disabled (no baseURL or JWT)")
	}

	// Create metadata with JWT token for authentication
	// The JWT contains claims that identify the node, including "prv" for private network
	md := metadata.Pairs("authorization", "Bearer "+c.jwt)
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Create SystemInfo with node metrics
	// This could be expanded in the future to include actual system metrics
	sysInfo := &proto.SystemInfo{
		CpuUsage:    0,
		MemoryUsage: 0,
		DiskUsage:   0,
	}

	// Create registration request with system info
	req := &proto.RegisterRequest{
		SystemInfo: sysInfo,
	}

	// Call the gRPC Register method on the Toggle service
	resp, err := c.client.Register(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("registration request failed: %w", err)
	}

	// Debug log the entire response
	c.Logger.Debug("Received gRPC registration response",
		"success", resp.Success,
		"message", resp.Message,
		"domain", resp.Domain,
		"certificate_length", len(resp.Certificate),
		"private_key_length", len(resp.PrivateKey),
		"expires_on", resp.ExpiresOn,
		"expires_on_empty", resp.ExpiresOn == "",
	)

	// If not successful, return error with the message from the server
	if !resp.Success {
		return nil, fmt.Errorf("registration failed: %s", resp.Message)
	}

	// Create result object with the basic response data
	result := &RegistrationResult{
		Message: resp.Message,
	}

	// Check if this is just an "already registered" response
	// which doesn't include certificate data
	if resp.Certificate == "" && resp.Domain == "" && resp.PrivateKey == "" {
		c.Logger.Info("Node already registered with Toggle service")
		return result, nil
	}

	// We have certificate data, so parse the expires_on date
	var expiresOn time.Time
	if resp.ExpiresOn != "" {
		c.Logger.Debug("Received expires_on value", "expires_on", resp.ExpiresOn)
		expiresOn, err = time.Parse(time.RFC3339, resp.ExpiresOn)
		if err != nil {
			c.Logger.Warn("Failed to parse expires_on date", "error", err)
			// Use a default expiration time of 15 years if parsing fails
			expiresOn = time.Now().AddDate(15, 0, 0)
		}
	} else {
		// If no expiration provided, use 15 years default
		c.Logger.Debug("No expires_on value provided, using default expiration")
		expiresOn = time.Now().AddDate(15, 0, 0)
	}

	// Fill in the complete registration result
	result.Domain = resp.Domain
	result.Certificate = resp.Certificate
	result.PrivateKey = resp.PrivateKey
	result.ExpiresOn = expiresOn

	c.Logger.Info("Registration successful with Toggle service via gRPC",
		"domain", result.Domain, "expiresOn", result.ExpiresOn)

	return result, nil
}

// RegistrationResult contains the result of a registration request
type RegistrationResult struct {
	Message     string    `json:"message"`
	Domain      string    `json:"domain"`
	Certificate string    `json:"certificate"`
	PrivateKey  string    `json:"private_key"`
	ExpiresOn   time.Time `json:"expires_on"`
}

// removeProtocolPrefix removes the protocol prefix (http:// or https://) from a URL
func removeProtocolPrefix(url string) string {
	for _, prefix := range []string{"http://", "https://"} {
		if len(url) > len(prefix) && url[:len(prefix)] == prefix {
			return url[len(prefix):]
		}
	}
	return url
}
