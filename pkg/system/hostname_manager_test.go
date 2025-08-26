// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHostnameManager(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewHostnameManager(logger)
	
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.executor)
	t.Logf("Hostname manager created successfully")
}

func TestHostnameManager_GetHostname(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewHostnameManager(logger)
	ctx := context.Background()
	
	hostname, err := manager.GetHostname(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, hostname)
	
	t.Logf("Current hostname: %s", hostname)
	assert.NotEmpty(t, hostname)
}

func TestHostnameManager_GetHostnameInfo(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewHostnameManager(logger)
	ctx := context.Background()
	
	info, err := manager.GetHostnameInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	
	t.Logf("Hostname Info: Static=%s, Transient=%s, Pretty=%s", 
		info.Static, info.Transient, info.Pretty)
	
	assert.NotEmpty(t, info.Static)
}

func TestHostnameManager_ValidateHostname(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewHostnameManager(logger)
	
	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{
			name:     "valid hostname",
			hostname: "test-host",
			wantErr:  false,
		},
		{
			name:     "valid hostname with domain",
			hostname: "test-host.example.com",
			wantErr:  false,
		},
		{
			name:     "valid single char",
			hostname: "a",
			wantErr:  false,
		},
		{
			name:     "empty hostname",
			hostname: "",
			wantErr:  true,
		},
		{
			name:     "hostname with invalid chars",
			hostname: "test_host",
			wantErr:  true,
		},
		{
			name:     "hostname starting with hyphen",
			hostname: "-test",
			wantErr:  true,
		},
		{
			name:     "hostname ending with hyphen",
			hostname: "test-",
			wantErr:  true,
		},
		{
			name:     "hostname starting with dot",
			hostname: ".test",
			wantErr:  true,
		},
		{
			name:     "hostname ending with dot",
			hostname: "test.",
			wantErr:  true,
		},
		{
			name:     "purely numeric hostname",
			hostname: "123456",
			wantErr:  true,
		},
		{
			name:     "too long hostname",
			hostname: "a" + string(make([]byte, 300)), // Over 253 chars
			wantErr:  true,
		},
		{
			name:     "label too long",
			hostname: string(make([]byte, 70)) + ".com", // Over 63 chars per label
			wantErr:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateHostname(tt.hostname)
			t.Logf("Hostname: '%s', Valid: %v, Error: %v", tt.hostname, err == nil, err)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Integration test - only run when explicitly enabled
func TestHostnameManager_SetHostname_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	logger := createTestLogger(t)
	manager := NewHostnameManager(logger)
	ctx := context.Background()
	
	// Get original hostname first
	originalHostname, err := manager.GetHostname(ctx)
	require.NoError(t, err)
	t.Logf("Original hostname: %s", originalHostname)
	
	// Skip the test if we can't restore the hostname
	t.Cleanup(func() {
		if originalHostname != "" {
			restoreReq := SetHostnameRequest{Hostname: originalHostname}
			if err := manager.SetHostname(ctx, restoreReq); err != nil {
				t.Errorf("Failed to restore original hostname: %v", err)
			} else {
				t.Logf("Restored original hostname: %s", originalHostname)
			}
		}
	})
	
	// Test setting a new hostname
	testHostname := "test-system-hostname"
	request := SetHostnameRequest{Hostname: testHostname}
	
	t.Logf("Setting hostname to: %s", testHostname)
	err = manager.SetHostname(ctx, request)
	require.NoError(t, err)
	
	// Verify the change
	newHostname, err := manager.GetHostname(ctx)
	require.NoError(t, err)
	t.Logf("New hostname: %s", newHostname)
	assert.Equal(t, testHostname, newHostname)
}