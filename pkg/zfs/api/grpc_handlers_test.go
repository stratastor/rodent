// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
	"github.com/stratastor/rodent/pkg/zfs/testutil"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestGRPC sets up test environment for gRPC handler tests
func setupTestGRPC(t *testing.T) (*PoolHandler, *DatasetHandler, string, func()) {
	// Setup test environment
	env := testutil.NewTestEnv(t, 3)
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "debug"})
	poolMgr := pool.NewManager(executor)
	datasetMgr := dataset.NewManager(executor)
	transferMgr, err := dataset.NewTransferManager(logger.Config{LogLevel: "debug"})
	if err != nil {
		t.Fatalf("failed to create dataset transfer manager: %v", err)
	}

	// Create test pool
	poolName := testutil.GeneratePoolName()
	err = poolMgr.Create(context.Background(), pool.CreateConfig{
		Name: poolName,
		VDevSpec: []pool.VDevSpec{
			{
				Type:    "raidz",
				Devices: env.GetLoopDevices(),
			},
		},
	})
	require.NoError(t, err)

	// Create handlers
	poolHandler := NewPoolHandler(poolMgr)
	datasetHandler, err := NewDatasetHandler(datasetMgr, transferMgr)
	if err != nil {
		t.Fatalf("failed to create dataset handler: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		// Destroy test pool and clean up
		_ = poolMgr.Destroy(context.Background(), poolName, true)
		env.Cleanup()
	}

	return poolHandler, datasetHandler, poolName, cleanup
}

// TestPoolListGRPC tests the pool list gRPC handler
func TestPoolListGRPC(t *testing.T) {
	// Setup
	poolHandler, _, poolName, cleanup := setupTestGRPC(t)
	defer cleanup()

	// Create request
	request := &proto.ToggleRequest{
		RequestId: "test-request-id",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdPoolList,
				Payload:     []byte("{}"),
			},
		},
	}

	// Call handler directly
	handler := handlePoolList(poolHandler)
	response, err := handler(request, request.GetCommand())

	// Assertions
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "test-request-id", response.RequestId)

	// Parse payload
	var result map[string]interface{}
	err = json.Unmarshal(response.Payload, &result)
	require.NoError(t, err)
	t.Logf("Result: %v", result)

	// Check if our pool is in the list
	poolsData, ok := result["result"].(map[string]interface{})
	require.True(t, ok, "Expected 'result' field to be an object")

	pools, ok := poolsData["pools"].(map[string]interface{})
	require.True(t, ok, "Expected 'pools' field to be an object")

	_, ok = pools[poolName]
	assert.True(t, ok, "Expected to find our test pool in the response")
}

// TestPoolStatusGRPC tests the pool status gRPC handler
func TestPoolStatusGRPC(t *testing.T) {
	// Setup
	poolHandler, _, poolName, cleanup := setupTestGRPC(t)
	defer cleanup()

	// Create payload
	payload, err := json.Marshal(map[string]string{
		"name": poolName,
	})
	require.NoError(t, err)

	// Create request
	request := &proto.ToggleRequest{
		RequestId: "test-request-id",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdPoolStatus,
				Payload:     payload,
			},
		},
	}

	// Call handler directly
	handler := handlePoolStatus(poolHandler)
	response, err := handler(request, request.GetCommand())

	// Assertions
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "test-request-id", response.RequestId)

	// Parse payload
	var result map[string]interface{}
	err = json.Unmarshal(response.Payload, &result)
	require.NoError(t, err)

	// Check if our pool status is in the response
	poolsData, ok := result["result"].(map[string]interface{})
	require.True(t, ok, "Expected 'result' field to be an object")

	// Pool status might not be directly accessible by name since it's a structure
	// Just check that the response contains valid data
	assert.NotEmpty(t, poolsData, "Expected pool status data in response")
}

// TestDatasetListGRPC tests the dataset list gRPC handler
func TestDatasetListGRPC(t *testing.T) {
	// Setup
	_, datasetHandler, poolName, cleanup := setupTestGRPC(t)
	defer cleanup()

	// Create a test filesystem
	fsName := poolName + "/testfs"
	createFS := dataset.FilesystemConfig{
		NameConfig: dataset.NameConfig{
			Name: fsName,
		},
		Properties: map[string]string{
			"mountpoint": "none",
		},
	}

	_, err := datasetHandler.manager.CreateFilesystem(context.Background(), createFS)
	require.NoError(t, err)

	// Create payload for listing datasets
	payload, err := json.Marshal(dataset.ListConfig{
		Type: "filesystem",
	})
	require.NoError(t, err)

	// Create request
	request := &proto.ToggleRequest{
		RequestId: "test-request-id",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdDatasetList,
				Payload:     payload,
			},
		},
	}

	// Call handler directly
	handler := handleDatasetList(datasetHandler)
	response, err := handler(request, request.GetCommand())

	// Assertions
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "test-request-id", response.RequestId)

	// Parse payload
	var respResult map[string]interface{}
	err = json.Unmarshal(response.Payload, &respResult)
	require.NoError(t, err)

	// Check if our filesystem is in the list
	listResult, ok := respResult["result"].(map[string]interface{})
	require.True(t, ok, "Expected 'result' field to be an object")

	datasets, ok := listResult["datasets"].(map[string]interface{})
	require.True(t, ok, "Expected 'datasets' field to be an object")

	// Check that our test filesystem is in the list
	found := false
	for key := range datasets {
		if key == fsName {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected to find our test filesystem in the response")
}

// TestErrorHandlingGRPC tests error handling in gRPC handlers
func TestErrorHandlingGRPC(t *testing.T) {
	// Setup
	poolHandler, _, _, cleanup := setupTestGRPC(t)
	defer cleanup()

	// Create payload with non-existent pool
	payload, err := json.Marshal(map[string]string{
		"name": "non-existent-pool",
	})
	require.NoError(t, err)

	// Create request
	request := &proto.ToggleRequest{
		RequestId: "test-request-id",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdPoolStatus,
				Payload:     payload,
			},
		},
	}

	// Call handler directly
	handler := handlePoolStatus(poolHandler)
	response, err := handler(request, request.GetCommand())
	t.Logf("Error: %+v", err)
	t.Logf("Response: %+v", response)

	// Assertions for error case
	require.Error(t, err)
	assert.Nil(t, response)

	// Test the error response creation
	errorResponse, err := successResponse("test-request-id", "Error message", nil)
	require.NoError(t, err)
	assert.Equal(t, "test-request-id", errorResponse.RequestId)
	assert.True(t, errorResponse.Success)
}
