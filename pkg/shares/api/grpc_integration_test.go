// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

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
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/facl"
	"github.com/stratastor/rodent/pkg/shares/smb"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const testPort = 50097 // Use a different high port from other tests

// MockToggleServer is a mock implementation of the Toggle server
// It emulates Toggle by sending SMB share commands to the Rodent client
type MockToggleServer struct {
	proto.UnimplementedRodentServiceServer
	t                  *testing.T
	testShareName      string
	testSharePath      string
	receivedResponses  map[string]*proto.CommandResponse
	responsesLock      sync.Mutex
	responseWaitGroups map[string]*sync.WaitGroup
	sharesHandler      *SharesHandler
}

// NewMockToggleServer creates a new mock Toggle server for testing
func NewMockToggleServer(t *testing.T, sharesHandler *SharesHandler) *MockToggleServer {
	// Create a temporary test directory
	testPath := "/tmp/test-smb-share-grpc"
	err := os.MkdirAll(testPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Generate a random share name to avoid conflicts with other test runs
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomSuffix := fmt.Sprintf("%d", r.Intn(10000))
	testShareName := fmt.Sprintf("test-smb-share-grpc-%s", randomSuffix)

	return &MockToggleServer{
		t:                  t,
		testShareName:      testShareName,
		testSharePath:      testPath,
		receivedResponses:  make(map[string]*proto.CommandResponse),
		responseWaitGroups: make(map[string]*sync.WaitGroup),
		sharesHandler:      sharesHandler,
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
	// First, get the list of shares
	// -------------------------------------------------------------------------
	listSharesReqID := "test-shares-list-req"
	listWg := &sync.WaitGroup{}
	listWg.Add(1)
	s.responseWaitGroups[listSharesReqID] = listWg

	// Send share list command
	err := stream.Send(&proto.ToggleRequest{
		RequestId: listSharesReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSharesSMBList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send share list command: %w", err)
	}

	// Wait for share list response
	if err := s.waitForResponse(listWg, "share list"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Test creating a new SMB share
	// -------------------------------------------------------------------------
	createShareReqID := "test-share-create-req"
	createWg := &sync.WaitGroup{}
	createWg.Add(1)
	s.responseWaitGroups[createShareReqID] = createWg

	// Create share payload
	shareConfig := smb.SMBShareConfig{
		Name:        s.testShareName,
		Path:        s.testSharePath,
		Description: "Test share created via gRPC integration test",
		ReadOnly:    true,
		Browsable:   true,
		GuestOk:     false,
	}
	sharePayload, _ := json.Marshal(shareConfig)

	// Send create share command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: createShareReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSharesSMBCreate,
				Payload:     sharePayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send create share command: %w", err)
	}

	// Wait for create share response
	if err := s.waitForResponse(createWg, "create share"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get the created share
	// -------------------------------------------------------------------------
	getShareReqID := "test-share-get-req"
	getWg := &sync.WaitGroup{}
	getWg.Add(1)
	s.responseWaitGroups[getShareReqID] = getWg

	// Create get share payload
	getPayload, _ := json.Marshal(map[string]string{
		"name": s.testShareName,
	})

	// Send get share command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: getShareReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSharesSMBGet,
				Payload:     getPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send get share command: %w", err)
	}

	// Wait for get share response
	if err := s.waitForResponse(getWg, "get share"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Update the share
	// -------------------------------------------------------------------------
	updateShareReqID := "test-share-update-req"
	updateWg := &sync.WaitGroup{}
	updateWg.Add(1)
	s.responseWaitGroups[updateShareReqID] = updateWg

	// Update share payload
	updatedShareConfig := smb.SMBShareConfig{
		Name:        s.testShareName,
		Path:        s.testSharePath,
		Description: "Updated test share via gRPC",
		ReadOnly:    false,
		Browsable:   true,
		GuestOk:     true,
	}
	updatePayload, _ := json.Marshal(updatedShareConfig)

	// Send update share command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: updateShareReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSharesSMBUpdate,
				Payload:     updatePayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send update share command: %w", err)
	}

	// Wait for update share response
	if err := s.waitForResponse(updateWg, "update share"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get the share stats
	// -------------------------------------------------------------------------
	statsReqID := "test-share-stats-req"
	statsWg := &sync.WaitGroup{}
	statsWg.Add(1)
	s.responseWaitGroups[statsReqID] = statsWg

	// Create stats payload
	statsPayload, _ := json.Marshal(map[string]interface{}{
		"name":     s.testShareName,
		"detailed": true,
	})

	// Send stats command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: statsReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSharesSMBStats,
				Payload:     statsPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send share stats command: %w", err)
	}

	// Wait for stats response
	if err := s.waitForResponse(statsWg, "share stats"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get the global config
	// -------------------------------------------------------------------------
	globalReqID := "test-global-config-req"
	globalWg := &sync.WaitGroup{}
	globalWg.Add(1)
	s.responseWaitGroups[globalReqID] = globalWg

	// Send global config command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: globalReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSharesSMBGlobalGet,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send global config command: %w", err)
	}

	// Wait for global config response
	if err := s.waitForResponse(globalWg, "global config"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get the service status
	// -------------------------------------------------------------------------
	statusReqID := "test-service-status-req"
	statusWg := &sync.WaitGroup{}
	statusWg.Add(1)
	s.responseWaitGroups[statusReqID] = statusWg

	// Send service status command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: statusReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSharesSMBServiceStatus,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send service status command: %w", err)
	}

	// Wait for service status response
	if err := s.waitForResponse(statusWg, "service status"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Delete the share
	// -------------------------------------------------------------------------
	deleteReqID := "test-share-delete-req"
	deleteWg := &sync.WaitGroup{}
	deleteWg.Add(1)
	s.responseWaitGroups[deleteReqID] = deleteWg

	// Create delete payload
	deletePayload, _ := json.Marshal(map[string]string{
		"name": s.testShareName,
	})

	// Send delete command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: deleteReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSharesSMBDelete,
				Payload:     deletePayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send delete share command: %w", err)
	}

	// Wait for delete response
	if err := s.waitForResponse(deleteWg, "delete share"); err != nil {
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

// CleanupTestResources removes test resources created during the test
func (s *MockToggleServer) CleanupTestResources() {
	// Delete the test share if it exists
	ctx := context.Background()
	_, err := s.sharesHandler.smbManager.GetSMBShare(ctx, s.testShareName)
	if err == nil {
		if err := s.sharesHandler.smbManager.DeleteShare(ctx, s.testShareName); err != nil {
			s.t.Logf("Failed to delete test share: %v", err)
		}
	}

	// Remove test directory
	os.RemoveAll(s.testSharePath)
}

// SetupTestWithMockToggle sets up a test with our mock Toggle server
func SetupTestWithMockToggle(t *testing.T) (*MockToggleServer, func()) {
	// Skip if not integration test
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping SMB gRPC integration test; set RUN_INTEGRATION_TESTS=true to run")
	}

	// Create a logger
	l, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test.shares")
	require.NoError(t, err)

	// Create necessary managers
	executor := command.NewCommandExecutor(true)
	aclManager := facl.NewACLManager(l, nil)
	smbManager, err := smb.NewManager(l, executor, aclManager)
	if err != nil {
		t.Skipf("Skipping test - SMB manager initialization failed: %v", err)
	}
	smbService := smb.NewServiceManager(l)

	// Create SharesHandler
	sharesHandler := NewSharesHandler(l, smbManager, smbService)

	// Register gRPC handlers
	RegisterSharesGRPCHandlers(sharesHandler)

	// Create and start mock Toggle server
	mockServer := NewMockToggleServer(t, sharesHandler)
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

		// Clean up test resources
		mockServer.CleanupTestResources()
	}

	return mockServer, cleanup
}

// TestSharesGRPCIntegration tests the full SMB shares gRPC integration flow
func TestSharesGRPCIntegration(t *testing.T) {
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

	// Verify the share list response
	listResp := mockServer.GetCommandResponse("test-shares-list-req")
	require.NotNil(t, listResp, "Should have received a share list response")
	assert.True(t, listResp.Success, "Share list request should have succeeded")
	assert.NotEmpty(t, listResp.Payload, "Share list response should not be empty")

	// Verify share creation response
	createResp := mockServer.GetCommandResponse("test-share-create-req")
	require.NotNil(t, createResp, "Should have received a create share response")
	assert.True(t, createResp.Success,
		"Share creation should have succeeded: %s", createResp.Message)

	// Verify the get share response
	getResp := mockServer.GetCommandResponse("test-share-get-req")
	require.NotNil(t, getResp, "Should have received a get share response")
	assert.True(t, getResp.Success, "Get share request should have succeeded")
	assert.NotEmpty(t, getResp.Payload, "Get share response should not be empty")

	// Verify share was correctly created with expected values
	var shareConfig smb.SMBShareConfig
	err = json.Unmarshal(getResp.Payload, &shareConfig)
	require.NoError(t, err)
	assert.Equal(t, mockServer.testShareName, shareConfig.Name)
	assert.Equal(t, mockServer.testSharePath, shareConfig.Path)
	assert.Equal(t, true, shareConfig.ReadOnly)
	assert.Equal(t, false, shareConfig.GuestOk)

	// Verify update response
	updateResp := mockServer.GetCommandResponse("test-share-update-req")
	require.NotNil(t, updateResp, "Should have received an update share response")
	assert.True(t, updateResp.Success,
		"Share update should have succeeded: %s", updateResp.Message)

	// Verify stats response
	statsResp := mockServer.GetCommandResponse("test-share-stats-req")
	require.NotNil(t, statsResp, "Should have received a share stats response")
	assert.True(t, statsResp.Success, "Share stats request should have succeeded")
	assert.NotEmpty(t, statsResp.Payload, "Share stats response should not be empty")

	// Verify global config response
	globalResp := mockServer.GetCommandResponse("test-global-config-req")
	require.NotNil(t, globalResp, "Should have received a global config response")
	assert.True(t, globalResp.Success, "Global config request should have succeeded")
	assert.NotEmpty(t, globalResp.Payload, "Global config response should not be empty")

	// Verify service status response
	statusResp := mockServer.GetCommandResponse("test-service-status-req")
	require.NotNil(t, statusResp, "Should have received a service status response")
	assert.True(t, statusResp.Success, "Service status request should have succeeded")
	assert.NotEmpty(t, statusResp.Payload, "Service status response should not be empty")

	// Verify delete response
	deleteResp := mockServer.GetCommandResponse("test-share-delete-req")
	require.NotNil(t, deleteResp, "Should have received a delete share response")
	assert.True(t, deleteResp.Success,
		"Share deletion should have succeeded: %s", deleteResp.Message)
}
