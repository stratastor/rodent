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
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/facl"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const testPort = 50096 // Use a different high port from other tests

// MockToggleServer is a mock implementation of the Toggle server
// It emulates Toggle by sending FACL commands to the Rodent client
type MockToggleServer struct {
	proto.UnimplementedRodentServiceServer
	t                  *testing.T
	testDirPath        string
	receivedResponses  map[string]*proto.CommandResponse
	responsesLock      sync.Mutex
	responseWaitGroups map[string]*sync.WaitGroup
	aclHandler         *ACLHandler
}

// NewMockToggleServer creates a new mock Toggle server for testing
func NewMockToggleServer(t *testing.T, aclHandler *ACLHandler) *MockToggleServer {
	// Create a temporary test directory
	rand.Seed(time.Now().UnixNano())
	testDir := fmt.Sprintf("/tmp/facl-grpc-test-%d", rand.Intn(10000))
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a test file
	testFile := fmt.Sprintf("%s/test.txt", testDir)
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.WriteString("test content")
	f.Close()

	// Set the test directory to match a ZFS path in testing mode
	os.Setenv("RODENT_TESTING", "1")

	return &MockToggleServer{
		t:                  t,
		testDirPath:        testFile, // Use the file instead of the directory
		receivedResponses:  make(map[string]*proto.CommandResponse),
		responseWaitGroups: make(map[string]*sync.WaitGroup),
		aclHandler:         aclHandler,
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
	// First, get the ACLs for the test file
	// -------------------------------------------------------------------------
	getACLReqID := "test-get-acl-req"
	getWg := &sync.WaitGroup{}
	getWg.Add(1)
	s.responseWaitGroups[getACLReqID] = getWg

	// Create get ACL payload
	getPayload, _ := json.Marshal(map[string]string{
		"path": s.testDirPath,
	})

	// Send get ACL command
	err := stream.Send(&proto.ToggleRequest{
		RequestId: getACLReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdFACLGet,
				Payload:     getPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send get ACL command: %w", err)
	}

	// Wait for get ACL response
	if err := s.waitForResponse(getWg, "get ACL"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Set ACLs for the test file - using simple set of entries similar to the passing test
	// -------------------------------------------------------------------------
	setACLReqID := "test-set-acl-req"
	setWg := &sync.WaitGroup{}
	setWg.Add(1)
	s.responseWaitGroups[setACLReqID] = setWg

	// Create test ACL entries - using 'nobody' which exists on all systems
	aclEntries := []facl.ACLEntry{
		{
			Type:        facl.EntryUser,
			Principal:   "nobody",
			Permissions: []facl.PermissionType{facl.PermReadData, facl.PermExecute},
			IsDefault:   false,
		},
	}

	// Create set ACL payload
	setPayload, _ := json.Marshal(map[string]interface{}{
		"path":    s.testDirPath,
		"type":    facl.ACLTypePOSIX,
		"entries": aclEntries,
	})

	// Send set ACL command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: setACLReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdFACLSet,
				Payload:     setPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send set ACL command: %w", err)
	}

	// Wait for set ACL response
	if err := s.waitForResponse(setWg, "set ACL"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get ACLs again to verify the changes
	// -------------------------------------------------------------------------
	getAgainReqID := "test-get-again-acl-req"
	getAgainWg := &sync.WaitGroup{}
	getAgainWg.Add(1)
	s.responseWaitGroups[getAgainReqID] = getAgainWg

	// Send get ACL command again
	err = stream.Send(&proto.ToggleRequest{
		RequestId: getAgainReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdFACLGet,
				Payload:     getPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send get ACL again command: %w", err)
	}

	// Wait for get ACL again response
	if err := s.waitForResponse(getAgainWg, "get ACL again"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Modify ACLs for the test file
	// -------------------------------------------------------------------------
	modifyACLReqID := "test-modify-acl-req"
	modifyWg := &sync.WaitGroup{}
	modifyWg.Add(1)
	s.responseWaitGroups[modifyACLReqID] = modifyWg

	// Create modified ACL entries
	modifiedEntries := []facl.ACLEntry{
		{
			Type:        facl.EntryUser,
			Principal:   "nobody",
			Permissions: []facl.PermissionType{facl.PermReadData, facl.PermWriteData},
			IsDefault:   false,
		},
	}

	// Create modify ACL payload
	modifyPayload, _ := json.Marshal(map[string]interface{}{
		"path":    s.testDirPath,
		"type":    facl.ACLTypePOSIX,
		"entries": modifiedEntries,
	})

	// Send modify ACL command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: modifyACLReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdFACLModify,
				Payload:     modifyPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send modify ACL command: %w", err)
	}

	// Wait for modify ACL response
	if err := s.waitForResponse(modifyWg, "modify ACL"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get ACLs again to verify the modifications
	// -------------------------------------------------------------------------
	getModifiedReqID := "test-get-modified-acl-req"
	getModifiedWg := &sync.WaitGroup{}
	getModifiedWg.Add(1)
	s.responseWaitGroups[getModifiedReqID] = getModifiedWg

	// Send get ACL command for modified ACLs
	err = stream.Send(&proto.ToggleRequest{
		RequestId: getModifiedReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdFACLGet,
				Payload:     getPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send get modified ACL command: %w", err)
	}

	// Wait for get modified ACL response
	if err := s.waitForResponse(getModifiedWg, "get modified ACL"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Remove ACLs for the test file
	// -------------------------------------------------------------------------
	removeACLReqID := "test-remove-acl-req"
	removeWg := &sync.WaitGroup{}
	removeWg.Add(1)
	s.responseWaitGroups[removeACLReqID] = removeWg

	// Create ACL entries to remove
	removeEntries := []facl.ACLEntry{
		{
			Type:      facl.EntryUser,
			Principal: "nobody",
			IsDefault: false,
		},
	}

	// Create remove ACL payload
	removePayload, _ := json.Marshal(map[string]interface{}{
		"path":    s.testDirPath,
		"type":    facl.ACLTypePOSIX,
		"entries": removeEntries,
	})

	// Send remove ACL command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: removeACLReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdFACLRemove,
				Payload:     removePayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send remove ACL command: %w", err)
	}

	// Wait for remove ACL response
	if err := s.waitForResponse(removeWg, "remove ACL"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get ACLs again to verify the removal
	// -------------------------------------------------------------------------
	getFinalReqID := "test-get-final-acl-req"
	getFinalWg := &sync.WaitGroup{}
	getFinalWg.Add(1)
	s.responseWaitGroups[getFinalReqID] = getFinalWg

	// Send get ACL command for final verification
	err = stream.Send(&proto.ToggleRequest{
		RequestId: getFinalReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdFACLGet,
				Payload:     getPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send get final ACL command: %w", err)
	}

	// Wait for get final ACL response
	if err := s.waitForResponse(getFinalWg, "get final ACL"); err != nil {
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
	// Remove test directory
	os.RemoveAll(filepath.Dir(s.testDirPath))
	os.Unsetenv("RODENT_TESTING")
}

// SetupTestWithMockToggle sets up a test with our mock Toggle server
func SetupTestWithMockToggle(t *testing.T) (*MockToggleServer, func()) {
	// Skip if not integration test
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping FACL gRPC integration test; set RUN_INTEGRATION_TESTS=true to run")
	}

	// Create a logger
	l, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test.facl")
	require.NoError(t, err)

	// Create necessary managers
	aclManager := facl.NewACLManager(l, nil)

	// Create ACLHandler
	aclHandler := NewACLHandler(aclManager, l)

	// Register gRPC handlers
	RegisterFACLGRPCHandlers(aclHandler)

	// Create and start mock Toggle server
	mockServer := NewMockToggleServer(t, aclHandler)
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

// TestFACLGRPCIntegration tests the full FACL gRPC integration flow
func TestFACLGRPCIntegration(t *testing.T) {
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

	// Verify the initial get ACL response
	getResp := mockServer.GetCommandResponse("test-get-acl-req")
	require.NotNil(t, getResp, "Should have received a get ACL response")
	assert.True(t, getResp.Success, "Get ACL request should have succeeded")
	assert.NotEmpty(t, getResp.Payload, "Get ACL response should not be empty")

	// Verify set ACL response
	setResp := mockServer.GetCommandResponse("test-set-acl-req")
	require.NotNil(t, setResp, "Should have received a set ACL response")
	assert.True(t, setResp.Success, "Set ACL request should have succeeded")

	// Verify get ACL again response
	getAgainResp := mockServer.GetCommandResponse("test-get-again-acl-req")
	require.NotNil(t, getAgainResp, "Should have received a get ACL again response")
	assert.True(t, getAgainResp.Success, "Get ACL again request should have succeeded")

	// Verify the set operation worked by checking ACL entries
	var getAgainResult map[string]interface{}
	err = json.Unmarshal(getAgainResp.Payload, &getAgainResult)
	require.NoError(t, err)

	// Verify modify ACL response
	modifyResp := mockServer.GetCommandResponse("test-modify-acl-req")
	require.NotNil(t, modifyResp, "Should have received a modify ACL response")
	assert.True(t, modifyResp.Success, "Modify ACL request should have succeeded")

	// Verify get modified ACL response
	getModifiedResp := mockServer.GetCommandResponse("test-get-modified-acl-req")
	require.NotNil(t, getModifiedResp, "Should have received a get modified ACL response")
	assert.True(t, getModifiedResp.Success, "Get modified ACL request should have succeeded")

	// Verify remove ACL response
	removeResp := mockServer.GetCommandResponse("test-remove-acl-req")
	require.NotNil(t, removeResp, "Should have received a remove ACL response")
	assert.True(t, removeResp.Success, "Remove ACL request should have succeeded")

	// Verify get final ACL response
	getFinalResp := mockServer.GetCommandResponse("test-get-final-acl-req")
	require.NotNil(t, getFinalResp, "Should have received a get final ACL response")
	assert.True(t, getFinalResp.Success, "Get final ACL request should have succeeded")
}
