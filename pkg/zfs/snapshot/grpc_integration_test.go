// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/toggle/client"
	zfsCommand "github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const testPort = 50099 // Use a different high port from the other tests

// Environment variable for test dataset name
const envTestDatasetName = "RODENT_TEST_FS_NAME"

// Default test dataset name if environment variable is not set
const defaultTestDatasetName = "tank/test"

// getTestDatasetName retrieves the dataset name from environment variable or uses default
func getTestDatasetName() string {
	if dataset := os.Getenv(envTestDatasetName); dataset != "" {
		return dataset
	}
	return defaultTestDatasetName
}

// generateUniqueID generates a unique identifier for test resources
func generateUniqueID() string {
	// Create a random number and current timestamp to ensure uniqueness
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("test%d%d", time.Now().Unix(), r.Intn(10000))
}

// MockToggleServer is a mock implementation of the Toggle server
// It emulates Toggle by sending auto-snapshot commands to the Rodent client
type MockToggleServer struct {
	proto.UnimplementedRodentServiceServer
	t                  *testing.T
	receivedResponses  map[string]*proto.CommandResponse
	responsesLock      sync.Mutex
	responseWaitGroups map[string]*sync.WaitGroup
	testPolicyID       string // ID of the test policy we create
}

// NewMockToggleServer creates a new mock Toggle server for testing
func NewMockToggleServer(t *testing.T) *MockToggleServer {
	return &MockToggleServer{
		t:                  t,
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

// waitForResponse is a helper to wait for a response with timeout
func (s *MockToggleServer) waitForResponse(
	wg *sync.WaitGroup,
	description string,
) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.t.Logf("Received %s response", description)
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for %s response", description)
	}
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
				s.t.Logf("Received command response for request ID: %s, success: %v",
					cmdResp.RequestId, cmdResp.Success)
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

	// -------------------------------------------------------------------------
	// First, get initial list of policies
	// -------------------------------------------------------------------------
	policyListReqID := "test-policy-list-req"

	// Create wait group for this request
	policyWg := &sync.WaitGroup{}
	policyWg.Add(1)
	s.responseWaitGroups[policyListReqID] = policyWg

	// Send policy list command
	err := stream.Send(&proto.ToggleRequest{
		RequestId: policyListReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdPoliciesAutosnapList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send policy list command: %w", err)
	}

	// Wait for policy list response
	if err := s.waitForResponse(policyWg, "policy list"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Test creating a snapshot policy
	// -------------------------------------------------------------------------
	createPolicyReqID := "test-create-policy-req"
	createPolicyWg := &sync.WaitGroup{}
	createPolicyWg.Add(1)
	s.responseWaitGroups[createPolicyReqID] = createPolicyWg

	// Generate a unique name for the test policy
	uniqueID := generateUniqueID()
	policyName := fmt.Sprintf("TestPolicy_%s", uniqueID)

	// Get the dataset name from environment variable
	datasetName := getTestDatasetName()
	s.t.Logf("Using dataset %s for test", datasetName)

	// Create policy request payload
	hourlySchedule := ScheduleSpec{
		Type:     ScheduleTypeHourly,
		Interval: 1,
		Enabled:  true,
	}

	policyCreatePayload, _ := json.Marshal(EditPolicyParams{
		Name:            policyName,
		Description:     "Test policy created via gRPC integration test",
		Dataset:         datasetName,
		Schedules:       []ScheduleSpec{hourlySchedule},
		Recursive:       true,
		SnapNamePattern: fmt.Sprintf("autotest-%s-%%Y%%m%%d-%%H%%M%%S", uniqueID),
		RetentionPolicy: RetentionPolicy{
			Count: 5,
		},
		Enabled: true,
	})

	// Send create policy command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: createPolicyReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdPoliciesAutosnapCreate,
				Payload:     policyCreatePayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send create policy command: %w", err)
	}

	// Wait for create policy response
	if err := s.waitForResponse(createPolicyWg, "create policy"); err != nil {
		return err
	}

	// Extract the policy ID from the response for later use
	createResp := s.GetCommandResponse(createPolicyReqID)
	if createResp != nil && createResp.Success {
		var policyResp SnapshotPolicy
		if err := json.Unmarshal(createResp.Payload, &policyResp); err == nil {
			s.testPolicyID = policyResp.ID
			s.t.Logf("Created test policy with ID: %s", s.testPolicyID)
		}
	}

	// -------------------------------------------------------------------------
	// Get the created policy by ID
	// -------------------------------------------------------------------------
	getPolicyReqID := "test-get-policy-req"
	getPolicyWg := &sync.WaitGroup{}
	getPolicyWg.Add(1)
	s.responseWaitGroups[getPolicyReqID] = getPolicyWg

	// Create get policy payload
	getPolicyPayload, _ := json.Marshal(map[string]string{
		"id": s.testPolicyID,
	})

	// Send get policy command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: getPolicyReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdPoliciesAutosnapGet,
				Payload:     getPolicyPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send get policy command: %w", err)
	}

	// Wait for get policy response
	if err := s.waitForResponse(getPolicyWg, "get policy"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Update the created policy
	// -------------------------------------------------------------------------
	updatePolicyReqID := "test-update-policy-req"
	updatePolicyWg := &sync.WaitGroup{}
	updatePolicyWg.Add(1)
	s.responseWaitGroups[updatePolicyReqID] = updatePolicyWg

	// Get the policy we just created to update
	getResp := s.GetCommandResponse(getPolicyReqID)
	if getResp == nil || !getResp.Success {
		return fmt.Errorf("failed to get policy for update")
	}

	var policy SnapshotPolicy
	if err := json.Unmarshal(getResp.Payload, &policy); err != nil {
		return fmt.Errorf("failed to unmarshal policy: %w", err)
	}

	// Update policy
	updateParams := EditPolicyParams{
		ID:              policy.ID,
		Name:            policy.Name,
		Description:     "Updated test policy via gRPC integration test",
		Dataset:         policy.Dataset,
		Schedules:       policy.Schedules,
		Recursive:       policy.Recursive,
		SnapNamePattern: policy.SnapNamePattern,
		RetentionPolicy: policy.RetentionPolicy,
		Enabled:         policy.Enabled,
	}

	updatePolicyPayload, _ := json.Marshal(updateParams)

	// Send update policy command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: updatePolicyReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdPoliciesAutosnapUpdate,
				Payload:     updatePolicyPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send update policy command: %w", err)
	}

	// Wait for update policy response
	if err := s.waitForResponse(updatePolicyWg, "update policy"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Run the policy manually
	// -------------------------------------------------------------------------
	runPolicyReqID := "test-run-policy-req"
	runPolicyWg := &sync.WaitGroup{}
	runPolicyWg.Add(1)
	s.responseWaitGroups[runPolicyReqID] = runPolicyWg

	// Create run policy payload
	runPolicyPayload, _ := json.Marshal(RunPolicyParams{
		ID:            s.testPolicyID,
		ScheduleIndex: 0,
		DryRun:        true, // Use dry run to avoid actually creating snapshots in test
	})

	// Send run policy command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: runPolicyReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdPoliciesAutosnapRun,
				Payload:     runPolicyPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send run policy command: %w", err)
	}

	// Wait for run policy response
	if err := s.waitForResponse(runPolicyWg, "run policy"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Test for a non-existent policy to verify error handling
	// -------------------------------------------------------------------------
	errorReqID := "test-nonexistent-policy-req"
	errorWg := &sync.WaitGroup{}
	errorWg.Add(1)
	s.responseWaitGroups[errorReqID] = errorWg

	// Create payload with non-existent policy ID
	errorPayload, _ := json.Marshal(map[string]string{
		"id": "nonexistent-policy-" + generateUniqueID(),
	})

	// Send command for non-existent policy
	err = stream.Send(&proto.ToggleRequest{
		RequestId: errorReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdPoliciesAutosnapGet,
				Payload:     errorPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send error test command: %w", err)
	}

	// Wait for error response
	if err := s.waitForResponse(errorWg, "error test"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Delete the policy
	// -------------------------------------------------------------------------
	deletePolicyReqID := "test-delete-policy-req"
	deletePolicyWg := &sync.WaitGroup{}
	deletePolicyWg.Add(1)
	s.responseWaitGroups[deletePolicyReqID] = deletePolicyWg

	// Create delete policy payload
	deletePolicyPayload, _ := json.Marshal(map[string]interface{}{
		"id":               s.testPolicyID,
		"remove_snapshots": true,
	})

	// Send delete policy command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: deletePolicyReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdPoliciesAutosnapDelete,
				Payload:     deletePolicyPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send delete policy command: %w", err)
	}

	// Wait for delete policy response
	if err := s.waitForResponse(deletePolicyWg, "delete policy"); err != nil {
		return err
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
func SetupTestWithMockToggle(t *testing.T) (*MockToggleServer, func()) {
	// Initialize dataset manager with temp config for testing
	// Create command executor with sudo support
	executor := zfsCommand.NewCommandExecutor(true, config.NewLoggerConfig(config.GetConfig()))

	dsManager := dataset.NewManager(executor)

	// Create and register handlers
	snapshotHandler, err := NewGRPCHandler(dsManager)
	if err != nil {
		t.Skipf("Skipping test - snapshot handler initialization failed: %v", err)
	}

	// Register snapshot handlers
	RegisterAutosnapshotGRPCHandlers(snapshotHandler)

	// Create and start mock Toggle server
	mockServer := NewMockToggleServer(t)
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
		// Stop the gRPC server
		grpcServer.Stop()

		// Stop the snapshot manager
		if err := snapshotHandler.StopManager(); err != nil {
			t.Logf("Warning: failed to stop snapshot manager: %v", err)
		}
	}

	return mockServer, cleanup
}

// TestAutosnapshotGRPCIntegration tests the full auto-snapshot gRPC integration flow
func TestAutosnapshotGRPCIntegration(t *testing.T) {
	// Skip in short mode as this is an integration test
	if testing.Short() {
		t.Skip("Skipping auto-snapshot gRPC integration test in short mode")
	}

	// Skip if test dataset is not configured
	datasetName := getTestDatasetName()
	if datasetName == defaultTestDatasetName {
		t.Logf(
			"Using default dataset name %s. Set %s environment variable to use a specific dataset.",
			defaultTestDatasetName,
			envTestDatasetName,
		)
	}

	// Setup mock Toggle server and test environment
	mockServer, cleanup := SetupTestWithMockToggle(t)
	defer cleanup()

	// Initialize a Rodent client to connect to our mock Toggle server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a JWT for testing (doesn't need to be valid for our mock Toggle server)
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

	// -------------------------------------------------------------------------
	// Verify responses
	// -------------------------------------------------------------------------

	// Verify the policy list response
	policyListResp := mockServer.GetCommandResponse("test-policy-list-req")
	require.NotNil(t, policyListResp, "Should have received a policy list response")
	assert.True(t, policyListResp.Success, "Policy list request should have succeeded")

	// Verify policy creation response
	createPolicyResp := mockServer.GetCommandResponse("test-create-policy-req")
	require.NotNil(t, createPolicyResp, "Should have received a create policy response")
	assert.True(t, createPolicyResp.Success,
		"Policy creation should have succeeded: %s", createPolicyResp.Message)

	// Verify the policy has been created correctly
	var createdPolicy SnapshotPolicy
	require.NoError(t, json.Unmarshal(createPolicyResp.Payload, &createdPolicy))
	assert.NotEmpty(t, createdPolicy.ID, "Created policy should have an ID")
	assert.Contains(
		t,
		createdPolicy.Name,
		"TestPolicy_",
		"Created policy should have expected name prefix",
	)

	// Verify the get policy response
	getPolicyResp := mockServer.GetCommandResponse("test-get-policy-req")
	require.NotNil(t, getPolicyResp, "Should have received a get policy response")
	assert.True(t, getPolicyResp.Success, "Get policy request should have succeeded")
	assert.NotEmpty(t, getPolicyResp.Payload, "Get policy response should not be empty")

	// Verify the policy update response
	updatePolicyResp := mockServer.GetCommandResponse("test-update-policy-req")
	require.NotNil(t, updatePolicyResp, "Should have received an update policy response")
	assert.True(t, updatePolicyResp.Success, "Policy update should have succeeded")

	// Verify updated policy has new description
	var updatedPolicy SnapshotPolicy
	require.NoError(t, json.Unmarshal(updatePolicyResp.Payload, &updatedPolicy))
	assert.Equal(t, "Updated test policy via gRPC integration test", updatedPolicy.Description,
		"Policy description should have been updated")

	// Verify the run policy response (dry run)
	runPolicyResp := mockServer.GetCommandResponse("test-run-policy-req")
	require.NotNil(t, runPolicyResp, "Should have received a run policy response")
	assert.True(t, runPolicyResp.Success, "Run policy request should have succeeded")

	// Verify the error response for a non-existent policy
	errorResp := mockServer.GetCommandResponse("test-nonexistent-policy-req")
	require.NotNil(t, errorResp, "Should have received an error response")
	assert.False(t, errorResp.Success, "Response should indicate failure for non-existent policy")
	assert.Contains(
		t,
		errorResp.Message,
		"not found",
		"Error message should indicate policy not found",
	)

	// Verify the delete policy response
	deletePolicyResp := mockServer.GetCommandResponse("test-delete-policy-req")
	require.NotNil(t, deletePolicyResp, "Should have received a delete policy response")
	assert.True(t, deletePolicyResp.Success, "Policy deletion should have succeeded")
}
