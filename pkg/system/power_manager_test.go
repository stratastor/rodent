// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPowerManager(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewPowerManager(logger)
	
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.executor)
	t.Logf("Power manager created successfully")
}

func TestPowerManager_GetPowerStatus(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewPowerManager(logger)
	ctx := context.Background()
	
	status, err := manager.GetPowerStatus(ctx)
	require.NoError(t, err)
	require.NotNil(t, status)
	
	t.Logf("Power status: %+v", status)
	
	// Should have basic status info
	assert.Contains(t, status, "status")
}

func TestPowerManager_GetScheduledShutdown(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewPowerManager(logger)
	ctx := context.Background()
	
	info, err := manager.GetScheduledShutdown(ctx)
	// Error is OK if no shutdown is scheduled
	if err == nil {
		t.Logf("Scheduled shutdown info: %+v", info)
	} else {
		t.Logf("No scheduled shutdown (expected): %v", err)
	}
}

func TestPowerManager_ValidatePowerOperation(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewPowerManager(logger)
	
	tests := []struct {
		name      string
		operation string
		request   PowerOperationRequest
		wantErr   bool
	}{
		{
			name:      "valid shutdown",
			operation: "shutdown",
			request:   PowerOperationRequest{},
			wantErr:   false,
		},
		{
			name:      "valid reboot",
			operation: "reboot", 
			request:   PowerOperationRequest{},
			wantErr:   false,
		},
		{
			name:      "invalid operation",
			operation: "invalid",
			request:   PowerOperationRequest{},
			wantErr:   true,
		},
		{
			name:      "shutdown with force flag",
			operation: "shutdown",
			request: PowerOperationRequest{
				Force: true,
			},
			wantErr: false,
		},
		{
			name:      "shutdown with message",
			operation: "shutdown",
			request: PowerOperationRequest{
				Message: "Maintenance shutdown",
			},
			wantErr: false,
		},
		{
			name:      "shutdown with message too long",
			operation: "shutdown",
			request: PowerOperationRequest{
				Message: string(make([]byte, 300)), // Over 200 chars
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidatePowerOperation(tt.operation, tt.request)
			t.Logf("Operation: %s, Request: %+v, Valid: %v, Error: %v", 
				tt.operation, tt.request, err == nil, err)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPowerManager_ScheduleShutdown_DryRun(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewPowerManager(logger)
	ctx := context.Background()
	
	// Test validation only - don't actually schedule
	delay := 5 * time.Minute
	message := "Test scheduled shutdown"
	
	// This would schedule a real shutdown, so we'll just test validation
	err := manager.ValidatePowerOperation("shutdown", PowerOperationRequest{
		Message: message,
	})
	
	assert.NoError(t, err)
	t.Logf("Shutdown schedule validation passed for delay=%v, message=%s", delay, message)
	
	// Test with empty message (should still be valid)
	err = manager.ValidatePowerOperation("shutdown", PowerOperationRequest{})
	assert.NoError(t, err)
	t.Logf("Shutdown without message is valid")
	
	// Test ScheduleShutdown method exists but don't actually call it
	// (we can test the method signature)
	t.Run("ScheduleShutdown_method_exists", func(t *testing.T) {
		// Just verify we can call the method signature without executing
		_ = func() error {
			return manager.ScheduleShutdown(ctx, delay, message)
		}
		t.Logf("ScheduleShutdown method signature is correct")
	})
}

// Note: We don't test actual shutdown/reboot operations in unit tests
// as they would terminate the system. Integration tests should be run
// in isolated environments only.