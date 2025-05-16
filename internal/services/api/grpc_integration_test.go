// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/services/manager"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const testPort = 50098 // Use a different high port from other tests

// MockToggleServer is a mock implementation of the Toggle server
// for testing service management operations
type MockToggleServer struct {
	proto.UnimplementedRodentServiceServer
	t                  *testing.T
	receivedResponses  map[string]*proto.CommandResponse
	responsesLock      sync.Mutex
	responseWaitGroups map[string]*sync.WaitGroup
	serviceHandler     *ServiceHandler
}

// NewMockToggleServer creates a new mock Toggle server for testing
func NewMockToggleServer(t *testing.T, serviceHandler *ServiceHandler) *MockToggleServer {
	return &MockToggleServer{
		t:                  t,
		receivedResponses:  make(map[string]*proto.CommandResponse),
		responseWaitGroups: make(map[string]*sync.WaitGroup),
		serviceHandler:     serviceHandler,
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
	// List all services
	// -------------------------------------------------------------------------
	listServicesReqID := "test-services-list-req"
	listWg := &sync.WaitGroup{}
	listWg.Add(1)
	s.responseWaitGroups[listServicesReqID] = listWg

	// Send list services command
	err := stream.Send(&proto.ToggleRequest{
		RequestId: listServicesReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdServicesList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send list services command: %w", err)
	}

	// Wait for list services response
	if err := s.waitForResponse(listWg, "list services"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get statuses of all services
	// -------------------------------------------------------------------------
	allStatusesReqID := "test-services-statuses-req"
	allStatusesWg := &sync.WaitGroup{}
	allStatusesWg.Add(1)
	s.responseWaitGroups[allStatusesReqID] = allStatusesWg

	// Send get all statuses command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: allStatusesReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdServicesStatuses,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send all service statuses command: %w", err)
	}

	// Wait for all statuses response
	if err := s.waitForResponse(allStatusesWg, "all service statuses"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get the available service names from the list response
	// -------------------------------------------------------------------------
	s.responsesLock.Lock()
	listResp := s.receivedResponses[listServicesReqID]
	s.responsesLock.Unlock()

	// Ensure we got a valid response
	if listResp == nil || !listResp.Success {
		return fmt.Errorf("failed to get service list")
	}

	// Parse the list response to get service names
	var listPayload struct {
		Services []string `json:"services"`
	}
	if err := json.Unmarshal(listResp.Payload, &listPayload); err != nil {
		return fmt.Errorf("failed to parse service list response: %w", err)
	}

	// If no services are available, we can't test further
	if len(listPayload.Services) == 0 {
		s.t.Log("No services available for testing")
		return nil
	}

	// Choose a service to test with - prefer samba if available
	testService := listPayload.Services[0]
	for _, service := range listPayload.Services {
		if service == "samba" {
			testService = service
			break
		}
	}
	s.t.Logf("Using service '%s' for testing", testService)

	// -------------------------------------------------------------------------
	// Get status of one service
	// -------------------------------------------------------------------------
	statusReqID := "test-service-status-req"
	statusWg := &sync.WaitGroup{}
	statusWg.Add(1)
	s.responseWaitGroups[statusReqID] = statusWg

	// Create status payload
	statusPayload, _ := json.Marshal(map[string]string{
		"name": testService,
	})

	// Send status command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: statusReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdServiceStatus,
				Payload:     statusPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send service status command: %w", err)
	}

	// Wait for status response
	if err := s.waitForResponse(statusWg, "service status"); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Get startup status (is-enabled) of the service
	// -------------------------------------------------------------------------
	enabledReqID := "test-service-is-enabled-req"
	enabledWg := &sync.WaitGroup{}
	enabledWg.Add(1)
	s.responseWaitGroups[enabledReqID] = enabledWg

	// Create is-enabled payload
	enabledPayload, _ := json.Marshal(map[string]string{
		"name": testService,
	})

	// Send is-enabled command
	err = stream.Send(&proto.ToggleRequest{
		RequestId: enabledReqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdServiceIsEnabled,
				Payload:     enabledPayload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send service is-enabled command: %w", err)
	}

	// Wait for is-enabled response
	if err := s.waitForResponse(enabledWg, "service is-enabled"); err != nil {
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
	// Skip if not integration test
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping services gRPC integration test; set RUN_INTEGRATION_TESTS=true to run")
	}

	// Create a logger
	l, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test.services")
	require.NoError(t, err)

	// Create service manager
	serviceManager, err := manager.NewServiceManager(l)
	if err != nil {
		t.Fatalf("Failed to create service manager: %v", err)
	}

	// Create service handler
	serviceHandler := NewServiceHandler(serviceManager)

	// Register gRPC handlers
	RegisterServiceGRPCHandlers(serviceHandler)

	// Create and start mock Toggle server
	mockServer := NewMockToggleServer(t, serviceHandler)
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

		// Clean up any resources
		if err := serviceHandler.Close(); err != nil {
			t.Logf("Error cleaning up service handler: %v", err)
		}
	}

	return mockServer, cleanup
}

// TestServicesGRPCIntegration tests the services gRPC integration flow
func TestServicesGRPCIntegration(t *testing.T) {
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

	// Verify service list response
	listResp := mockServer.GetCommandResponse("test-services-list-req")
	require.NotNil(t, listResp, "Should have received a service list response")
	assert.True(t, listResp.Success, "Service list request should have succeeded")
	assert.NotEmpty(t, listResp.Payload, "Service list response should not be empty")

	// Verify service statuses response
	statusesResp := mockServer.GetCommandResponse("test-services-statuses-req")
	require.NotNil(t, statusesResp, "Should have received a service statuses response")
	assert.True(t, statusesResp.Success, "Service statuses request should have succeeded")
	assert.NotEmpty(t, statusesResp.Payload, "Service statuses response should not be empty")

	// Verify single service status response if we were able to test it
	statusResp := mockServer.GetCommandResponse("test-service-status-req")
	if statusResp != nil {
		assert.True(t, statusResp.Success, "Service status request should have succeeded")
		assert.NotEmpty(t, statusResp.Payload, "Service status response should not be empty")

		// Parse the response using a generic map to handle different status structures
		var statusData map[string]interface{}
		err := json.Unmarshal(statusResp.Payload, &statusData)
		require.NoError(t, err)

		// Verify presence of name field
		assert.Contains(t, statusData, "name", "Status response should include service name")
		assert.NotEmpty(t, statusData["name"], "Service name should not be empty")

		// Status might be a string or an array (for multi-service like samba)
		assert.Contains(t, statusData, "status", "Status response should include status field")
		assert.NotNil(t, statusData["status"], "Status should not be nil")
	}

	// Verify service is-enabled response if we were able to test it
	enabledResp := mockServer.GetCommandResponse("test-service-is-enabled-req")
	if enabledResp != nil {
		assert.True(
			t,
			enabledResp.Success,
			"Service is-enabled request should have succeeded or returned appropriate error",
		)

		// The enabled response might contain an error for services that don't support
		// startup management, which is also a valid test case
		if enabledResp.Success {
			var enabledData struct {
				Name    string `json:"name"`
				Enabled bool   `json:"enabled"`
			}
			err := json.Unmarshal(enabledResp.Payload, &enabledData)
			require.NoError(t, err)
			assert.NotEmpty(t, enabledData.Name, "Enabled response should include service name")
		}
	}
	t.Logf("Test completed successfully, received %d responses", len(mockServer.receivedResponses))
}
