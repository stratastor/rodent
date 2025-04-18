// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"testing"
)

func TestIsPrivateNetwork(t *testing.T) {
	tests := []struct {
		name     string
		jwt      string
		expected bool
		wantErr  bool
	}{
		{
			name:     "Private network token",
			jwt:      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicHJ2Ijp0cnVlLCJyaWQiOiIydnNUdzdjNFd2b21zQTllRGtQd3YyYk1CVzIiLCJzdWIiOiJjNzUwMGNjOC02M2UxLTRjMmItYWU4NS02MmFkOTA0YTdmNmIiLCJ0aWQiOiIydnNUdzdrbnRpV0ZMSWhDbHNSQW1yT2FZZUgifQ.34jPJR44u40hp_xrYjShVD7DoCOOQk_QKL7XfPLsTUs",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "Public network token (no prv claim)",
			jwt:      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicmlkIjoiMnZzVHc3YzRXdm9tc0E5ZURrUHd2MmJNQlcyIiwic3ViIjoiYzc1MDBjYzgtNjNlMS00YzJiLWFlODUtNjJhZDkwNGE3ZjZiIiwidGlkIjoiMnZzVHc3a250aVdGTEloQ2xzUkFtck9hWWVIIn0.QZ9jtRfjMwlPYYrVZ2J_xNNGGSTsGRhEm1oGdPDSkuY",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "Public network token (prv=false)",
			jwt:      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicHJ2IjpmYWxzZSwicmlkIjoiMnZzVHc3YzRXdm9tc0E5ZURrUHd2MmJNQlcyIiwic3ViIjoiYzc1MDBjYzgtNjNlMS00YzJiLWFlODUtNjJhZDkwNGE3ZjZiIiwidGlkIjoiMnZzVHc3a250aVdGTEloQ2xzUkFtck9hWWVIIn0.3wGgDqCMuF-JVtWtjbRZtfIRZLKh7_SYpjv-C42Oa7o",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "Invalid JWT format",
			jwt:      "not.a.valid.jwt",
			expected: false,
			wantErr:  true,
		},
		{
			name:     "Empty JWT",
			jwt:      "",
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsPrivateNetwork(tt.jwt)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsPrivateNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("IsPrivateNetwork() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseJWT(t *testing.T) {
	validJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicHJ2Ijp0cnVlLCJyaWQiOiIydnNUdzdjNFd2b21zQTllRGtQd3YyYk1CVzIiLCJzdWIiOiJjNzUwMGNjOC02M2UxLTRjMmItYWU4NS02MmFkOTA0YTdmNmIiLCJ0aWQiOiIydnNUdzdrbnRpV0ZMSWhDbHNSQW1yT2FZZUgifQ.34jPJR44u40hp_xrYjShVD7DoCOOQk_QKL7XfPLsTUs"
	
	parts := ParseJWT(validJWT)
	if len(parts) != 3 {
		t.Errorf("ParseJWT() returned %d parts, want 3", len(parts))
	}
	
	invalidJWT := "not.a.valid.jwt.token"
	parts = ParseJWT(invalidJWT)
	if parts != nil {
		t.Errorf("ParseJWT() with invalid JWT returned %v, want nil", parts)
	}
	
	emptyJWT := ""
	parts = ParseJWT(emptyJWT)
	if parts != nil {
		t.Errorf("ParseJWT() with empty JWT returned %v, want nil", parts)
	}
}

func TestDecodeJWTPayload(t *testing.T) {
	// Base64 encoded payload part from the sample JWT
	payload := "eyJleHAiOjIyMTgzMTg2MTcsImlhdCI6MTc0NDkzMzAxNywicHJ2Ijp0cnVlLCJyaWQiOiIydnNUdzdjNFd2b21zQTllRGtQd3YyYk1CVzIiLCJzdWIiOiJjNzUwMGNjOC02M2UxLTRjMmItYWU4NS02MmFkOTA0YTdmNmIiLCJ0aWQiOiIydnNUdzdrbnRpV0ZMSWhDbHNSQW1yT2FZZUgifQ"
	
	claims, err := DecodeJWTPayload(payload)
	if err != nil {
		t.Fatalf("DecodeJWTPayload() error = %v", err)
	}
	
	// Check specific claims
	if sub, ok := claims["sub"].(string); !ok || sub != "c7500cc8-63e1-4c2b-ae85-62ad904a7f6b" {
		t.Errorf("DecodeJWTPayload() 'sub' claim = %v, want 'c7500cc8-63e1-4c2b-ae85-62ad904a7f6b'", claims["sub"])
	}
	
	if prv, ok := claims["prv"].(bool); !ok || !prv {
		t.Errorf("DecodeJWTPayload() 'prv' claim = %v, want true", claims["prv"])
	}
	
	// Test with invalid base64
	_, err = DecodeJWTPayload("invalid-base64")
	if err == nil {
		t.Errorf("DecodeJWTPayload() with invalid base64 should return error")
	}
}

func TestRemoveProtocolPrefix(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "HTTP URL",
			url:      "http://example.com",
			expected: "example.com",
		},
		{
			name:     "HTTPS URL",
			url:      "https://example.com",
			expected: "example.com",
		},
		{
			name:     "No protocol",
			url:      "example.com",
			expected: "example.com",
		},
		{
			name:     "With port and path",
			url:      "https://example.com:8080/path",
			expected: "example.com:8080/path",
		},
		{
			name:     "Empty string",
			url:      "",
			expected: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeProtocolPrefix(tt.url)
			if got != tt.expected {
				t.Errorf("removeProtocolPrefix() = %v, want %v", got, tt.expected)
			}
		})
	}
}