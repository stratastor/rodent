// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"strings"
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
		name          string
		jwt           string
		baseURL       string
		rpcAddr       string
		isGRPC        bool
		expectError   bool
		errorContains string
	}{
		{
			name:        "gRPC Registration",
			jwt:         "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicHJ2Ijp0cnVlLCJyaWQiOiIydnNUdzdjNFd2b21zQTllRGtQd3YyYk1CVzIiLCJzdWIiOiJjNzUwMGNjOC02M2UxLTRjMmItYWU4NS02MmFkOTA0YTdmNmIiLCJ0aWQiOiIydnNUdzdrbnRpV0ZMSWhDbHNSQW1yT2FZZUgifQ.34jPJR44u40hp_xrYjShVD7DoCOOQk_QKL7XfPLsTUs",
			baseURL:     "http://localhost:8142",
			rpcAddr:     "localhost:8242",
			isGRPC:      true,
			expectError: false,
		},
		{
			name:          "REST Registration",
			jwt:           "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicHJ2Ijp0cnVlLCJyaWQiOiIydnNUdzdjNFd2b21zQTllRGtQd3YyYk1CVzIiLCJzdWIiOiJjNzUwMGNjOC02M2UxLTRjMmItYWU4NS02MmFkOTA0YTdmNmIiLCJ0aWQiOiIydnNUdzdrbnRpV0ZMSWhDbHNSQW1yT2FZZUgifQ.34jPJR44u40hp_xrYjShVD7DoCOOQk_QKL7XfPLsTUs",
			baseURL:       "http://localhost:8142",
			rpcAddr:       "localhost:8242",
			isGRPC:        false,
			expectError:   true,
			errorContains: "Rodents with Private Tunnel token can't register with Strata Secure endpoints",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create client based on type
			var client ToggleClient
			var err error

			if tt.isGRPC {
				grpcClient, err := NewGRPCClient(testLogger, tt.jwt, tt.rpcAddr)
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

			// Check for expected errors
			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got success")
				}

				// Verify error message contains expected text
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf(
						"Error doesn't contain expected text.\nExpected to contain: %s\nActual error: %v",
						tt.errorContains,
						err,
					)
				} else {
					t.Logf("Got expected error: %v", err)
				}
				return
			}

			// If we don't expect an error but got one
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
