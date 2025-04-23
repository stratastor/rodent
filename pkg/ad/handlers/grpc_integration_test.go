// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/ad"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const testPort = 50098 // Use a different high port from the ZFS tests

// generateUniqueID generates a unique identifier for test resources
func generateUniqueID() string {
	// Create a random number and current timestamp to ensure uniqueness
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("test%d%d", time.Now().Unix(), r.Intn(10000))
}

// MockToggleServer is a mock implementation of the Toggle server
// It emulates Toggle by sending AD commands to the Rodent client
type MockToggleServer struct {
	proto.UnimplementedRodentServiceServer
	t                  *testing.T
	receivedResponses  map[string]*proto.CommandResponse
	responsesLock      sync.Mutex
	responseWaitGroups map[string]*sync.WaitGroup
	testUserCN         string // CN of the test user we create
	testGroupCN        string // CN of the test group we create
}

// NewMockToggleServer creates a new mock Toggle server for testing
func NewMockToggleServer(t *testing.T) *MockToggleServer {
	uniqueID := generateUniqueID()
	return &MockToggleServer{
		t:                  t,
		receivedResponses:  make(map[string]*proto.CommandResponse),
		responseWaitGroups: make(map[string]*sync.WaitGroup),
		testUserCN:         fmt.Sprintf("TestUser_%s", uniqueID),
		testGroupCN:        fmt.Sprintf("TestGroup_%s", uniqueID),
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
	// First, get initial lists of users and groups
	// -------------------------------------------------------------------------
	userListReqID := "test-user-list-req"
	groupListReqID := "test-group-list-req"

	// Create wait groups for these requests
	userWg := &sync.WaitGroup{}
	userWg.Add(1)
	s.responseWaitGroups[userListReqID] = userWg

	groupWg := &sync.WaitGroup{}
	groupWg.Add(1)
	s.responseWaitGroups[groupListReqID] = groupWg

	// Send user list command
	err := stream.Send(&proto.ToggleRequest{
		RequestId: userListReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdUserList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send user list command: %w", err)
	}

	// Send group list command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: groupListReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdGroupList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send group list command: %w", err)
	}

	// Wait for user list response
	if err := s.waitForResponse(userWg, "user list"); err != nil {
		return err
	}

	// Wait for group list response
	if err := s.waitForResponse(groupWg, "group list"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Test creating a user
	// -------------------------------------------------------------------------
	createUserReqID := "test-create-user-req"
	createUserWg := &sync.WaitGroup{}
	createUserWg.Add(1)
	s.responseWaitGroups[createUserReqID] = createUserWg

	// Generate unique SAM account name
	samAccountName := strings.ToLower(strings.ReplaceAll(s.testUserCN, " ", ""))

	// Create user request payload
	userCreatePayload, _ := json.Marshal(UserRequest{
		CN:             s.testUserCN,
		SAMAccountName: samAccountName,
		GivenName:      "Test",
		Surname:        "User",
		Description:    "Test user created via gRPC integration test",
		DisplayName:    s.testUserCN,
		Enabled:        true,
	})

	// Send create user command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: createUserReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdUserCreate,
				Payload:     userCreatePayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send create user command: %w", err)
	}

	// Wait for create user response
	if err := s.waitForResponse(createUserWg, "create user"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Test creating a group
	// -------------------------------------------------------------------------
	createGroupReqID := "test-create-group-req"
	createGroupWg := &sync.WaitGroup{}
	createGroupWg.Add(1)
	s.responseWaitGroups[createGroupReqID] = createGroupWg

	// Generate unique SAM account name for group
	groupSamAccountName := strings.ToLower(strings.ReplaceAll(s.testGroupCN, " ", ""))

	// Create group request payload
	groupCreatePayload, _ := json.Marshal(GroupRequest{
		CN:             s.testGroupCN,
		SAMAccountName: groupSamAccountName,
		Description:    "Test group created via gRPC integration test",
		DisplayName:    s.testGroupCN,
	})

	// Send create group command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: createGroupReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdGroupCreate,
				Payload:     groupCreatePayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send create group command: %w", err)
	}

	// Wait for create group response
	if err := s.waitForResponse(createGroupWg, "create group"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get the created user
	// -------------------------------------------------------------------------
	getUserReqID := "test-get-user-req"
	getUserWg := &sync.WaitGroup{}
	getUserWg.Add(1)
	s.responseWaitGroups[getUserReqID] = getUserWg

	// Create get user payload
	getUserPayload, _ := json.Marshal(map[string]string{
		"username": samAccountName,
	})

	// Send get user command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: getUserReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdUserGet,
				Payload:     getUserPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send get user command: %w", err)
	}

	// Wait for get user response
	if err := s.waitForResponse(getUserWg, "get user"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Test for a non-existent user to verify error handling
	// -------------------------------------------------------------------------
	errorReqID := "test-nonexistent-user-req"
	errorWg := &sync.WaitGroup{}
	errorWg.Add(1)
	s.responseWaitGroups[errorReqID] = errorWg

	// Create payload with non-existent username
	errorPayload, _ := json.Marshal(map[string]string{
		"username": "nonexistent-user-" + generateUniqueID(),
	})

	// Send command for non-existent user
	err = stream.Send(&proto.ToggleRequest{
		RequestId: errorReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: CmdUserGet,
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

// CleanupTestResources cleans up all test resources created during the test
func (s *MockToggleServer) CleanupTestResources(adClient *ad.ADClient) error {
	var errs []string

	// Delete test user if it was created
	if s.testUserCN != "" {
		if err := adClient.DeleteUser(s.testUserCN); err != nil {
			errs = append(errs, fmt.Sprintf("Failed to delete test user: %v", err))
		}
	}

	// Delete test group if it was created
	if s.testGroupCN != "" {
		if err := adClient.DeleteGroup(s.testGroupCN); err != nil {
			errs = append(errs, fmt.Sprintf("Failed to delete test group: %v", err))
		}
	}

	// Return any errors
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// SetupTestWithMockToggle sets up a test with our mock Toggle server
func SetupTestWithMockToggle(t *testing.T) (*MockToggleServer, *ad.ADClient, func()) {
	// Initialize AD client
	adClient, err := ad.New()
	if err != nil {
		t.Skipf("Skipping test - AD client initialization failed: %v", err)
	}

	// Create and register handlers
	adHandler := NewADHandlerWithClient(adClient)

	// Register AD handlers
	RegisterADGRPCHandlers(adHandler)

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

		// Clean up test resources
		if err := mockServer.CleanupTestResources(adClient); err != nil {
			t.Logf("Warning: cleanup of test resources failed: %v", err)
		}

		// Close AD client
		adHandler.Close()
	}

	return mockServer, adClient, cleanup
}

// TestADGRPCIntegration tests the full AD gRPC integration flow
func TestADGRPCIntegration(t *testing.T) {
	// Check if we should skip this test - it requires AD server connectivity
	cfg := config.GetConfig()
	if cfg.AD.AdminPassword == "" {
		t.Skip("Skipping AD gRPC integration test - no AD admin password configured")
	}

	// Setup mock Toggle server and test environment
	mockServer, _, cleanup := SetupTestWithMockToggle(t)
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

	// Verify the user list response
	userListResp := mockServer.GetCommandResponse("test-user-list-req")
	require.NotNil(t, userListResp, "Should have received a user list response")
	assert.True(t, userListResp.Success, "User list request should have succeeded")
	assert.NotEmpty(t, userListResp.Payload, "User list response should not be empty")

	// Verify the group list response
	groupListResp := mockServer.GetCommandResponse("test-group-list-req")
	require.NotNil(t, groupListResp, "Should have received a group list response")
	assert.True(t, groupListResp.Success, "Group list request should have succeeded")
	assert.NotEmpty(t, groupListResp.Payload, "Group list response should not be empty")

	// Verify user creation response
	createUserResp := mockServer.GetCommandResponse("test-create-user-req")
	require.NotNil(t, createUserResp, "Should have received a create user response")
	assert.True(t, createUserResp.Success,
		"User creation should have succeeded: %s", createUserResp.Message)

	// Verify group creation response
	createGroupResp := mockServer.GetCommandResponse("test-create-group-req")
	require.NotNil(t, createGroupResp, "Should have received a create group response")
	assert.True(t, createGroupResp.Success,
		"Group creation should have succeeded: %s", createGroupResp.Message)

	// Verify the get user response
	getUserResp := mockServer.GetCommandResponse("test-get-user-req")
	require.NotNil(t, getUserResp, "Should have received a get user response")
	assert.True(t, getUserResp.Success, "Get user request should have succeeded")
	assert.NotEmpty(t, getUserResp.Payload, "Get user response should not be empty")

	// Verify the error response for a non-existent user
	errorResp := mockServer.GetCommandResponse("test-nonexistent-user-req")
	require.NotNil(t, errorResp, "Should have received an error response")
	assert.False(t, errorResp.Success, "Response should indicate failure for non-existent user")
	assert.Contains(
		t,
		errorResp.Message,
		"not found",
		"Error message should indicate user not found",
	)
}
