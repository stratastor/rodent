// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
	"github.com/stratastor/rodent/pkg/zfs/testutil"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const testPort = 50099 // Use a high port number for testing

// MockToggleServer is a mock implementation of the Toggle server
// It emulates Toggle by sending ZFS commands to the Rodent client
type MockToggleServer struct {
	proto.UnimplementedRodentServiceServer
	t                  *testing.T
	poolName           string
	receivedResponses  map[string]*proto.CommandResponse
	responsesLock      sync.Mutex
	responseWaitGroups map[string]*sync.WaitGroup
}

// NewMockToggleServer creates a new mock Toggle server for testing
func NewMockToggleServer(t *testing.T, poolName string) *MockToggleServer {
	return &MockToggleServer{
		t:                  t,
		poolName:           poolName,
		receivedResponses:  make(map[string]*proto.CommandResponse),
		responseWaitGroups: make(map[string]*sync.WaitGroup),
	}
}

// Register is a simple implementation that just returns success
func (s *MockToggleServer) Register(
	ctx context.Context,
	req *proto.RegisterRequest,
) (*proto.RegisterResponse, error) {
	return &proto.RegisterResponse{
		Success: true,
		Message: "Test registration successful",
	}, nil
}

// Connect is the core streaming method where we test the integration
func (s *MockToggleServer) Connect(stream proto.RodentService_ConnectServer) error {
	// This will be called when Rodent connects to our mock Toggle server
	s.t.Log("Rodent client connected to mock Toggle server")

	// Process incoming responses from Rodent in a separate goroutine
	go func() {
		for {
			// Receive response from Rodent
			resp, err := stream.Recv()
			if err != nil {
				s.t.Logf("Error receiving from Rodent: %v", err)
				return
			}

			// Process command responses
			if cmdResp := resp.GetCommandResponse(); cmdResp != nil {
				s.t.Logf("Received command response for request ID: %s", cmdResp.RequestId)
				// Store response for validation in test
				s.responsesLock.Lock()
				s.receivedResponses[cmdResp.RequestId] = cmdResp
				s.responsesLock.Unlock()

				// Signal that the response has been received
				if wg, exists := s.responseWaitGroups[cmdResp.RequestId]; exists {
					wg.Done()
				}
			}
		}
	}()

	// Send ZFS commands to test Rodent's handlers
	// We'll test basic pool list first
	poolListReqID := "test-pool-list-req"

	// Create wait group for this request
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s.responseWaitGroups[poolListReqID] = wg

	// Send pool list command
	err := stream.Send(&proto.ToggleRequest{
		RequestId: poolListReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdPoolList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send pool list command: %w", err)
	}

	// Wait for response with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.t.Log("Received pool list response")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for pool list response")
	}

	// Test for a pool that doesn't exist to check error handling
	errorReqID := "test-nonexistent-pool-req"
	wg = &sync.WaitGroup{}
	wg.Add(1)
	s.responseWaitGroups[errorReqID] = wg

	// Create payload with non-existent pool name
	errorPayload, _ := json.Marshal(map[string]string{
		"name": "non-existent-pool",
	})

	// Send command for non-existent pool
	err = stream.Send(&proto.ToggleRequest{
		RequestId: errorReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdPoolStatus,
				Payload:     errorPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send error test command: %w", err)
	}

	// Wait for response with timeout
	done = make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.t.Log("Received error test response")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for error test response")
	}

	// Keep the connection open for a bit more
	time.Sleep(1 * time.Second)
	return nil
}

// GetCommandResponse retrieves a received command response by request ID
func (s *MockToggleServer) GetCommandResponse(requestID string) *proto.CommandResponse {
	s.responsesLock.Lock()
	defer s.responsesLock.Unlock()
	return s.receivedResponses[requestID]
}

// SetupTestWithMockToggle sets up a test with our mock Toggle server
func SetupTestWithMockToggle(t *testing.T) (*MockToggleServer, string, func()) {
	// Set up test environment
	env := testutil.NewTestEnv(t, 3)
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "debug"})
	poolMgr := pool.NewManager(executor)
	datasetMgr := dataset.NewManager(executor)
	transferMgr, err := dataset.NewTransferManager(logger.Config{LogLevel: "debug"})
	require.NoError(t, err, "failed to create dataset transfer manager")

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

	// Create and register handlers
	poolHandler := NewPoolHandler(poolMgr)
	datasetHandler, err := NewDatasetHandler(datasetMgr, transferMgr)
	if err != nil {
		t.Fatalf("failed to create dataset handler: %v", err)
	}

	// Clear any existing handlers and register ZFS handlers
	// client.ClearCommandHandlers()
	RegisterZFSGRPCHandlers(poolHandler, datasetHandler)

	// Create and start mock Toggle server
	mockServer := NewMockToggleServer(t, poolName)
	grpcServer := grpc.NewServer()
	proto.RegisterRodentServiceServer(grpcServer, mockServer)

	// Start listening on the test port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", testPort))
	require.NoError(t, err)

	// Start gRPC server in a goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("Failed to serve: %v", err)
		}
	}()

	// Create cleanup function
	cleanup := func() {
		grpcServer.Stop()
		// Destroy test pool and clean up
		_ = poolMgr.Destroy(context.Background(), poolName, true)
		env.Cleanup()
	}

	return mockServer, poolName, cleanup
}

// TestGRPCIntegration tests the full gRPC integration flow
func TestGRPCIntegration(t *testing.T) {
	// Skip in normal tests - this requires a full setup
	// t.Skip("Skipping integration test that requires a full gRPC setup")

	// Setup mock Toggle server and test environment
	mockServer, poolName, cleanup := SetupTestWithMockToggle(t)
	defer cleanup()

	// Initialize a Rodent client to connect to our mock Toggle server
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create a JWT for testing (doesn't need to be valid for testing)
	testJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0LW9yZyIsInBydiI6dHJ1ZX0.dTdm2rfiSvx6rZ5JyIQvXg4gCYjmKTCcRKWsJo63Oh4"

	// Create a logger for the client
	l, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test-client")
	require.NoError(t, err)

	// Initialize the gRPC client
	serverAddr := fmt.Sprintf("localhost:%d", testPort)
	gClient, err := client.NewGRPCClient(l, testJWT, serverAddr)
	require.NoError(t, err)
	defer gClient.Close()

	// Connect to the mock Toggle server
	_, cerr := gClient.Connect(ctx)
	require.NoError(t, cerr)

	// Wait for all test communications to complete
	time.Sleep(10 * time.Second)

	// Verify the pool list response
	poolListResp := mockServer.GetCommandResponse("test-pool-list-req")
	require.NotNil(t, poolListResp, "Should have received a pool list response")

	// Check if the response contains our test pool
	var result map[string]interface{}
	err = json.Unmarshal(poolListResp.Payload, &result)
	require.NoError(t, err)

	poolsData, ok := result["result"].(map[string]interface{})
	require.True(t, ok, "Expected 'result' field to be an object")

	pools, ok := poolsData["pools"].(map[string]interface{})
	require.True(t, ok, "Expected 'pools' field to be an object")

	_, ok = pools[poolName]
	assert.True(t, ok, "Expected to find our test pool in the response")

	// Verify the error response
	errorResp := mockServer.GetCommandResponse("test-nonexistent-pool-req")
	require.NotNil(t, errorResp, "Should have received an error response")

	// Check error properties
	assert.False(t, errorResp.Success, "Response should indicate failure")
	assert.NotNil(t, errorResp.Error, "Response should include error details")

	// Convert error to RodentError for validation
	if errorResp.Error != nil {
		rodentErr := errors.FromProto(errorResp.Error)
		assert.Equal(t, errors.DomainZFS, rodentErr.Domain, "Error should be from ZFS domain")
	}
}
