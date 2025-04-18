// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"testing"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
)

func TestNewToggleClient(t *testing.T) {
	// Create a test logger
	testLogger, err := logger.NewTag(
		config.NewLoggerConfig(config.GetConfig()),
		"toggle_client_test",
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	tests := []struct {
		name      string
		jwt       string
		baseURL   string
		rpcAddr   string
		wantType  string
		expectErr bool
	}{
		{
			name:     "Private network JWT (prv=true)",
			jwt:      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicHJ2Ijp0cnVlLCJyaWQiOiIydnNUdzdjNFd2b21zQTllRGtQd3YyYk1CVzIiLCJzdWIiOiJjNzUwMGNjOC02M2UxLTRjMmItYWU4NS02MmFkOTA0YTdmNmIiLCJ0aWQiOiIydnNUdzdrbnRpV0ZMSWhDbHNSQW1yT2FZZUgifQ.34jPJR44u40hp_xrYjShVD7DoCOOQk_QKL7XfPLsTUs",
			baseURL:  "localhost:8142",
			rpcAddr:  "localhost:8242",
			wantType: "*client.GRPCClient",
			// gRPC server is running on port 8242
			expectErr: false,
		},
		{
			name:      "Public network JWT (no prv claim)",
			jwt:       "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicmlkIjoiMnZzVHc3YzRXdm9tc0E5ZURrUHd2MmJNQlcyIiwic3ViIjoiYzc1MDBjYzgtNjNlMS00YzJiLWFlODUtNjJhZDkwNGE3ZjZiIiwidGlkIjoiMnZzVHc3a250aVdGTEloQ2xzUkFtck9hWWVIIn0.QZ9jtRfjMwlPYYrVZ2J_xNNGGSTsGRhEm1oGdPDSkuY",
			baseURL:   "http://localhost:8142",
			rpcAddr:   "localhost:8242",
			wantType:  "*client.RESTClientAdapter",
			expectErr: false,
		},
		{
			name:      "Invalid JWT",
			jwt:       "invalid.jwt.token",
			baseURL:   "http://localhost:8142",
			rpcAddr:   "localhost:8242",
			wantType:  "*client.RESTClientAdapter", // Default to REST on error
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewToggleClient(testLogger, tt.jwt, tt.baseURL, tt.rpcAddr)

			if (err != nil) != tt.expectErr {
				t.Errorf("NewToggleClient() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if err != nil {
				return
			}

			// Check the type of client returned
			gotType := getClientType(client)
			if gotType != tt.wantType {
				t.Errorf("NewToggleClient() returned %s, want %s", gotType, tt.wantType)
			}
		})
	}
}

// Helper function to determine the type of client
func getClientType(client ToggleClient) string {
	switch client.(type) {
	case *RESTClientAdapter:
		return "*client.RESTClientAdapter"
	case *GRPCClient:
		return "*client.GRPCClient"
	default:
		return "unknown"
	}
}
