// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"testing"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
)

func TestRegistration(t *testing.T) {
	// This test performs a live registration with the Toggle service
	// Skip it by default to avoid side effects during regular testing
	if testing.Short() {
		t.Skip("Skipping live registration test in short mode")
	}

	// Create a test logger
	testLogger, err := logger.NewTag(
		config.NewLoggerConfig(config.GetConfig()),
		"toggle_registration_test",
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	// Test both REST and gRPC clients
	tests := []struct {
		name     string
		jwt      string
		baseURL  string
		isGRPC   bool
	}{
		{
			name:    "gRPC Registration",
			jwt:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicHJ2Ijp0cnVlLCJyaWQiOiIydnNUdzdjNFd2b21zQTllRGtQd3YyYk1CVzIiLCJzdWIiOiJjNzUwMGNjOC02M2UxLTRjMmItYWU4NS02MmFkOTA0YTdmNmIiLCJ0aWQiOiIydnNUdzdrbnRpV0ZMSWhDbHNSQW1yT2FZZUgifQ.34jPJR44u40hp_xrYjShVD7DoCOOQk_QKL7XfPLsTUs",
			baseURL: "localhost:8242",
			isGRPC:  true,
		},
		{
			name:    "REST Registration",
			jwt:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicmlkIjoiMnZzVHc3YzRXdm9tc0E5ZURrUHd2MmJNQlcyIiwic3ViIjoiYzc1MDBjYzgtNjNlMS00YzJiLWFlODUtNjJhZDkwNGE3ZjZiIiwidGlkIjoiMnZzVHc3a250aVdGTEloQ2xzUkFtck9hWWVIIn0.QZ9jtRfjMwlPYYrVZ2J_xNNGGSTsGRhEm1oGdPDSkuY",
			baseURL: "http://localhost:8142",
			isGRPC:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create client based on type
			var client ToggleClient
			var err error

			if tt.isGRPC {
				grpcClient, err := NewGRPCClient(testLogger, tt.jwt, tt.baseURL)
				if err != nil {
					t.Fatalf("Failed to create gRPC client: %v", err)
				}
				defer grpcClient.Close()
				client = grpcClient
			} else {
				restClient, err := NewClient(testLogger, tt.jwt, tt.baseURL)
				if err != nil {
					t.Fatalf("Failed to create REST client: %v", err)
				}
				client = NewClientAdapter(restClient)
			}

			// Try to register
			ctx := context.Background()
			result, err := client.Register(ctx)
			if err != nil {
				t.Fatalf("Registration failed: %v", err)
			}

			// Validate basic registration result
			if result == nil {
				t.Fatalf("Registration returned nil result")
			}

			t.Logf("Registration successful: %s", result.Message)

			// For private network (gRPC), we don't expect certificates
			if tt.isGRPC {
				if result.Certificate != "" && result.PrivateKey != "" {
					t.Logf("Unexpected certificate data for private network node")
				}
			}
		})
	}
}