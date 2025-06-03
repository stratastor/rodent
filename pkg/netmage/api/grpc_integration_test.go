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
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/netmage"
	"github.com/stratastor/rodent/pkg/netmage/types"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const testGRPCPort = 50099 // Use a different high port from other tests

// MockToggleServer is a mock implementation of the Toggle server for netmage testing
type MockToggleServer struct {
	proto.UnimplementedRodentServiceServer
	t                  *testing.T
	receivedResponses  map[string]*proto.CommandResponse
	responsesLock      sync.Mutex
	responseWaitGroups map[string]*sync.WaitGroup
	networkHandler     *NetworkHandler
}

// NewMockToggleServer creates a new mock Toggle server for testing
func NewMockToggleServer(t *testing.T, networkHandler *NetworkHandler) *MockToggleServer {
	return &MockToggleServer{
		t:                  t,
		receivedResponses:  make(map[string]*proto.CommandResponse),
		responseWaitGroups: make(map[string]*sync.WaitGroup),
		networkHandler:     networkHandler,
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
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for %s response", description)
	}
}

// Connect implements the bidirectional streaming for testing
func (s *MockToggleServer) Connect(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Rodent client connected to mock Toggle server")

	// Process incoming responses from Rodent in a separate goroutine
	go func() {
		for {
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

	return s.runNetworkTests(stream)
}

// runNetworkTests executes a series of network management tests
func (s *MockToggleServer) runNetworkTests(stream proto.RodentService_ConnectServer) error {
	// Test system info
	if err := s.testSystemInfo(stream); err != nil {
		return err
	}

	// Test interface listing
	if err := s.testListInterfaces(stream); err != nil {
		return err
	}

	// Test IP validation
	if err := s.testIPValidation(stream); err != nil {
		return err
	}

	// Test netplan config
	if err := s.testNetplanConfig(stream); err != nil {
		return err
	}

	s.t.Log("All network gRPC tests completed successfully")
	return nil
}

// testSystemInfo tests the system information endpoint
func (s *MockToggleServer) testSystemInfo(stream proto.RodentService_ConnectServer) error {
	reqID := "test-system-info"
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s.responseWaitGroups[reqID] = wg

	err := stream.Send(&proto.ToggleRequest{
		RequestId: reqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdNetworkSystemInfo,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send system info command: %w", err)
	}

	return s.waitForResponse(wg, "system info")
}

// testListInterfaces tests the interface listing endpoint
func (s *MockToggleServer) testListInterfaces(stream proto.RodentService_ConnectServer) error {
	reqID := "test-interfaces-list"
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s.responseWaitGroups[reqID] = wg

	err := stream.Send(&proto.ToggleRequest{
		RequestId: reqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdNetworkInterfacesList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send interfaces list command: %w", err)
	}

	return s.waitForResponse(wg, "interfaces list")
}

// testIPValidation tests the IP validation endpoint
func (s *MockToggleServer) testIPValidation(stream proto.RodentService_ConnectServer) error {
	reqID := "test-ip-validation"
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s.responseWaitGroups[reqID] = wg

	payload := map[string]string{"address": "192.168.1.1"}
	payloadBytes, _ := json.Marshal(payload)

	err := stream.Send(&proto.ToggleRequest{
		RequestId: reqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdNetworkValidateIP,
				Payload:     payloadBytes,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send IP validation command: %w", err)
	}

	return s.waitForResponse(wg, "IP validation")
}

// testNetplanConfig tests the netplan configuration endpoint
func (s *MockToggleServer) testNetplanConfig(stream proto.RodentService_ConnectServer) error {
	reqID := "test-netplan-config"
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s.responseWaitGroups[reqID] = wg

	err := stream.Send(&proto.ToggleRequest{
		RequestId: reqID,
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdNetworkNetplanGetConfig,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send netplan config command: %w", err)
	}

	return s.waitForResponse(wg, "netplan config")
}

// GetResponse retrieves a stored response
func (s *MockToggleServer) GetResponse(requestID string) *proto.CommandResponse {
	s.responsesLock.Lock()
	defer s.responsesLock.Unlock()
	return s.receivedResponses[requestID]
}

// setupGRPCTest creates a test environment for gRPC testing
func setupGRPCTest(t *testing.T) (*MockToggleServer, func()) {
	// Skip tests if running in environments where network commands might not work
	if os.Getenv("SKIP_NETMAGE_TESTS") == "true" {
		t.Skip("Netmage tests skipped via SKIP_NETMAGE_TESTS environment variable")
	}

	// Skip if not integration test
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping netmage gRPC integration test; set RUN_INTEGRATION_TESTS=true to run")
	}

	// Create logger for testing
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test.netmage.grpc")
	require.NoError(t, err, "Failed to create logger")

	// Create netmage manager
	ctx := context.Background()
	manager, err := netmage.NewManager(ctx, log, types.RendererNetworkd)
	require.NoError(t, err, "Failed to create netmage manager")

	// Create network handler
	networkHandler := NewNetworkHandler(manager, log)

	// Register gRPC handlers
	RegisterNetworkGRPCHandlers(networkHandler)

	// Create mock server
	mockServer := NewMockToggleServer(t, networkHandler)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", testGRPCPort))
	require.NoError(t, err, "Failed to listen on port")

	grpcServer := grpc.NewServer()
	proto.RegisterRodentServiceServer(grpcServer, mockServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("gRPC server error: %v", err)
		}
	}()

	cleanup := func() {
		grpcServer.GracefulStop()
	}

	// Wait a moment for server to be ready
	time.Sleep(100 * time.Millisecond)

	return mockServer, cleanup
}

// TestNetworkGRPC_Integration runs the full gRPC integration test
func TestNetworkGRPC_Integration(t *testing.T) {
	// Setup mock Toggle server and test environment
	mockServer, cleanup := setupGRPCTest(t)
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
	serverAddr := fmt.Sprintf("localhost:%d", testGRPCPort)
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

	// Validate that all expected responses were received
	responses := []string{
		"test-system-info",
		"test-interfaces-list", 
		"test-ip-validation",
		"test-netplan-config",
	}

	for _, reqID := range responses {
		resp := mockServer.GetResponse(reqID)
		require.NotNil(t, resp, "Should have received response for %s", reqID)
		assert.True(t, resp.Success, "Failed response for %s: %s", reqID, resp.Message)
		
		// Pretty print the JSON response payload
		if len(resp.Payload) > 0 {
			var prettyJSON map[string]interface{}
			if err := json.Unmarshal(resp.Payload, &prettyJSON); err == nil {
				prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
				t.Logf("Response for %s:\n%s", reqID, string(prettyBytes))
			} else {
				t.Logf("Response payload for %s (raw): %s", reqID, string(resp.Payload))
			}
		}
	}
}

// TestNetworkGRPC_BasicValidation tests basic gRPC functionality without streaming
func TestNetworkGRPC_BasicValidation(t *testing.T) {
	_, cleanup := setupGRPCTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", testGRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "Failed to connect to gRPC server")
	defer conn.Close()

	client := proto.NewRodentServiceClient(conn)

	// Test registration first
	regResp, err := client.Register(ctx, &proto.RegisterRequest{
		SystemInfo: &proto.SystemInfo{
			CpuUsage:    50.0,
			MemoryUsage: 60.0,
			DiskUsage:   70.0,
		},
	})
	require.NoError(t, err, "Failed to register")
	assert.True(t, regResp.Success, "Registration failed: %s", regResp.Message)

	// The integration test will run through the Connect method
	// which is tested in TestNetworkGRPC_Integration
	t.Log("Basic gRPC validation completed successfully")
}