// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestLogger(t *testing.T) logger.Logger {
	testLogger, err := logger.New(logger.Config{LogLevel: "debug"})
	require.NoError(t, err)
	return testLogger
}

func TestNewManager(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewManager(logger)
	
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.infoCollector)
	assert.NotNil(t, manager.hostnameManager)
	assert.NotNil(t, manager.userManager)
	assert.NotNil(t, manager.powerManager)
	t.Logf("Manager created successfully with all components initialized")
}

func TestManager_GetSystemInfo(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewManager(logger)
	ctx := context.Background()
	
	info, err := manager.GetSystemInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	
	t.Logf("System Info: Hostname=%s, OS=%s, Kernel=%s, CPU_Count=%d, Memory_Total=%d, Uptime=%s",
		info.Hostname, info.OS.Name, info.OS.KernelRelease, 
		info.Hardware.CPU.ProcessorCount, info.Hardware.Memory.Total, info.Uptime)
	
	// Basic validations
	assert.NotEmpty(t, info.Hostname)
	assert.NotEmpty(t, info.OS.Name)
	assert.NotEmpty(t, info.OS.KernelRelease)
	assert.Greater(t, info.Hardware.CPU.ProcessorCount, 0)
	assert.Greater(t, info.Hardware.Memory.Total, uint64(0))
	assert.Greater(t, info.Uptime, time.Duration(0))
}

func TestManager_GetOSInfo(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewManager(logger)
	ctx := context.Background()
	
	osInfo, err := manager.GetOSInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, osInfo)
	
	t.Logf("OS Info: Name=%s, Version=%s, Kernel=%s, Arch=%s",
		osInfo.Name, osInfo.Version, osInfo.KernelRelease, osInfo.Architecture)
	
	assert.NotEmpty(t, osInfo.Name)
	assert.NotEmpty(t, osInfo.KernelRelease)
	assert.NotEmpty(t, osInfo.Architecture)
}

func TestManager_GetHardwareInfo(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewManager(logger)
	ctx := context.Background()
	
	hwInfo, err := manager.GetHardwareInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, hwInfo)
	
	t.Logf("Hardware Info: CPU_Model=%s, CPU_Count=%d, Memory_Total=%d, Memory_Available=%d",
		hwInfo.CPU.ModelName, hwInfo.CPU.ProcessorCount, hwInfo.Memory.Total, hwInfo.Memory.Available)
	
	assert.Greater(t, hwInfo.CPU.ProcessorCount, 0)
	assert.Greater(t, hwInfo.Memory.Total, uint64(0))
	assert.NotEmpty(t, hwInfo.CPU.ModelName)
}

func TestManager_GetPerformanceInfo(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewManager(logger)
	ctx := context.Background()
	
	perfInfo, err := manager.GetPerformanceInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, perfInfo)
	
	t.Logf("Performance Info: CPU_Usage=%.2f%%, Memory_Usage=%.2f%%, Load_Avg=[%.2f, %.2f, %.2f]",
		perfInfo.CPUUsage.Total, perfInfo.CPUUsage.Total, 
		perfInfo.LoadAverage.Load1, perfInfo.LoadAverage.Load5, perfInfo.LoadAverage.Load15)
	
	assert.GreaterOrEqual(t, perfInfo.CPUUsage.Total, 0.0)
	assert.LessOrEqual(t, perfInfo.CPUUsage.Total, 100.0)
	assert.GreaterOrEqual(t, perfInfo.LoadAverage.Load1, 0.0)
	assert.GreaterOrEqual(t, perfInfo.LoadAverage.Load5, 0.0)
}

func TestManager_GetSystemHealth(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewManager(logger)
	ctx := context.Background()
	
	health, err := manager.GetSystemHealth(ctx)
	require.NoError(t, err)
	require.NotNil(t, health)
	
	t.Logf("System Health: %+v", health)
	
	assert.Equal(t, "healthy", health["status"])
	assert.Contains(t, health, "timestamp")
	assert.Contains(t, health, "uptime")
	assert.Contains(t, health, "hostname")
}

func TestManager_ValidateSystemOperation(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewManager(logger)
	
	tests := []struct {
		name      string
		operation string
		params    map[string]interface{}
		wantErr   bool
	}{
		{
			name:      "valid shutdown",
			operation: "shutdown",
			params:    map[string]interface{}{},
			wantErr:   false,
		},
		{
			name:      "valid reboot",
			operation: "reboot",
			params:    map[string]interface{}{},
			wantErr:   false,
		},
		{
			name:      "valid hostname set",
			operation: "set_hostname",
			params:    map[string]interface{}{"hostname": "test-host"},
			wantErr:   false,
		},
		{
			name:      "invalid hostname set - empty",
			operation: "set_hostname",
			params:    map[string]interface{}{"hostname": ""},
			wantErr:   true,
		},
		{
			name:      "invalid hostname set - wrong type",
			operation: "set_hostname",
			params:    map[string]interface{}{"hostname": 123},
			wantErr:   true,
		},
		{
			name:      "unknown operation",
			operation: "invalid_op",
			params:    map[string]interface{}{},
			wantErr:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateSystemOperation(tt.operation, tt.params)
			t.Logf("Operation: %s, Params: %+v, Error: %v", tt.operation, tt.params, err)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}