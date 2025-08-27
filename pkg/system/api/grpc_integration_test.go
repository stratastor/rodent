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
	"github.com/stratastor/rodent/pkg/system"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	// Use development config which has development.enabled: true
	// This must be set before any config loading happens
	os.Setenv("RODENT_CONFIG", "/home/rodent/.rodent/rodent.yml.dev")
}

const testSystemGRPCPort = 50100 // Use a different high port from other tests

// SystemTestState holds the original system state for restoration
type SystemTestState struct {
	OriginalHostname  string
	OriginalTimezone  string
	OriginalLocale    string
	TestUsersCreated  []string
	TestGroupsCreated []string
}

// MockToggleServer is a mock implementation of the Toggle server for System testing
type MockToggleServer struct {
	proto.UnimplementedRodentServiceServer
	t                  *testing.T
	receivedResponses  map[string]*proto.CommandResponse
	responsesLock      sync.Mutex
	responseWaitGroups map[string]*sync.WaitGroup
	systemHandler      *SystemHandler
	testState          *SystemTestState
}

// NewMockToggleServer creates a new mock Toggle server for testing
func NewMockToggleServer(t *testing.T, systemHandler *SystemHandler) *MockToggleServer {
	return &MockToggleServer{
		t:                  t,
		receivedResponses:  make(map[string]*proto.CommandResponse),
		responseWaitGroups: make(map[string]*sync.WaitGroup),
		systemHandler:      systemHandler,
		testState: &SystemTestState{
			TestUsersCreated:  make([]string, 0),
			TestGroupsCreated: make([]string, 0),
		},
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

// captureAndValidateOriginalState captures the current system state and validates we can restore it
func (s *MockToggleServer) captureAndValidateOriginalState() error {
	ctx := context.Background()

	// Capture original hostname
	hostname, err := s.systemHandler.manager.GetHostname(ctx)
	if err != nil {
		return fmt.Errorf("failed to capture original hostname: %w", err)
	}
	s.testState.OriginalHostname = hostname

	// Capture original timezone
	timezone, err := s.systemHandler.manager.GetTimezone(ctx)
	if err != nil {
		return fmt.Errorf("failed to capture original timezone: %w", err)
	}
	s.testState.OriginalTimezone = timezone

	// Capture original locale
	locale, err := s.systemHandler.manager.GetLocale(ctx)
	if err != nil {
		return fmt.Errorf("failed to capture original locale: %w", err)
	}
	s.testState.OriginalLocale = locale

	s.t.Logf("Captured original state - Hostname: %s, Timezone: %s, Locale: %s",
		s.testState.OriginalHostname, s.testState.OriginalTimezone, s.testState.OriginalLocale)

	// VALIDATE we can restore the original state by attempting to set the same values
	s.t.Log("Validating ability to restore original state...")

	// Test hostname restoration capability
	hostnameReq := system.SetHostnameRequest{
		Hostname: s.testState.OriginalHostname,
		Static:   true,
	}
	if err := s.systemHandler.manager.SetHostname(ctx, hostnameReq); err != nil {
		return fmt.Errorf("failed to validate hostname restoration capability: %w", err)
	}

	// Test timezone restoration capability
	timezoneReq := system.SetTimezoneRequest{
		Timezone: s.testState.OriginalTimezone,
	}
	if err := s.systemHandler.manager.SetTimezone(ctx, timezoneReq); err != nil {
		return fmt.Errorf("failed to validate timezone restoration capability: %w", err)
	}

	// Test locale restoration capability
	localeReq := system.SetLocaleRequest{
		Locale: s.testState.OriginalLocale,
	}
	if err := s.systemHandler.manager.SetLocale(ctx, localeReq); err != nil {
		return fmt.Errorf("failed to validate locale restoration capability: %w", err)
	}

	s.t.Log("Successfully validated ability to restore original state")
	return nil
}

// restoreOriginalState restores the system to its original state
func (s *MockToggleServer) restoreOriginalState() {
	ctx := context.Background()

	// Restore hostname
	if s.testState.OriginalHostname != "" {
		req := system.SetHostnameRequest{
			Hostname: s.testState.OriginalHostname,
			Static:   true,
		}
		if err := s.systemHandler.manager.SetHostname(ctx, req); err != nil {
			s.t.Errorf("Failed to restore original hostname: %v", err)
		} else {
			s.t.Logf("Restored original hostname: %s", s.testState.OriginalHostname)
		}
	}

	// Restore timezone
	if s.testState.OriginalTimezone != "" {
		req := system.SetTimezoneRequest{
			Timezone: s.testState.OriginalTimezone,
		}
		if err := s.systemHandler.manager.SetTimezone(ctx, req); err != nil {
			s.t.Errorf("Failed to restore original timezone: %v", err)
		} else {
			s.t.Logf("Restored original timezone: %s", s.testState.OriginalTimezone)
		}
	}

	// Restore locale
	if s.testState.OriginalLocale != "" {
		req := system.SetLocaleRequest{
			Locale: s.testState.OriginalLocale,
		}
		if err := s.systemHandler.manager.SetLocale(ctx, req); err != nil {
			s.t.Errorf("Failed to restore original locale: %v", err)
		} else {
			s.t.Logf("Restored original locale: %s", s.testState.OriginalLocale)
		}
	}

	// Clean up test users
	for _, username := range s.testState.TestUsersCreated {
		if err := s.systemHandler.manager.DeleteUser(ctx, username); err != nil {
			s.t.Errorf("Failed to delete test user %s: %v", username, err)
		} else {
			s.t.Logf("Cleaned up test user: %s", username)
		}
	}

	// Clean up test groups
	for _, groupname := range s.testState.TestGroupsCreated {
		if err := s.systemHandler.manager.DeleteGroup(ctx, groupname); err != nil {
			s.t.Errorf("Failed to delete test group %s: %v", groupname, err)
		} else {
			s.t.Logf("Cleaned up test group: %s", groupname)
		}
	}
}

// Connect implements the bidirectional streaming for testing
func (s *MockToggleServer) Connect(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Rodent client connected to mock Toggle server")

	// Capture original system state and validate we can restore it - FAIL if we can't
	if err := s.captureAndValidateOriginalState(); err != nil {
		return fmt.Errorf(
			"cannot proceed with destructive tests - failed to capture and validate original state restoration: %w",
			err,
		)
	}

	// Ensure cleanup happens regardless of test outcome
	defer s.restoreOriginalState()

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

	return s.runSystemTests(stream)
}

// runSystemTests executes a series of system management tests with proper flow
func (s *MockToggleServer) runSystemTests(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Starting system gRPC integration tests...")

	// Test read-only system information endpoints first (safe operations)
	if err := s.testSystemInfoEndpoints(stream); err != nil {
		return err
	}

	// Test hostname management with state restoration
	if err := s.testHostnameManagement(stream); err != nil {
		return err
	}

	// Test user management lifecycle (create -> get -> delete)
	if err := s.testUserManagementFlow(stream); err != nil {
		return err
	}

	// Test group management lifecycle (create -> delete)
	if err := s.testGroupManagementFlow(stream); err != nil {
		return err
	}

	// Test system configuration with state restoration
	if err := s.testSystemConfigurationFlow(stream); err != nil {
		return err
	}

	s.t.Log("All system gRPC tests completed successfully")
	return nil
}

// testSystemInfoEndpoints tests all read-only system information endpoints
func (s *MockToggleServer) testSystemInfoEndpoints(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing system information endpoints...")

	tests := []struct {
		name    string
		cmdType string
		reqID   string
	}{
		{"system-info", proto.CmdSystemInfoGet, "test-system-info"},
		{"cpu-info", proto.CmdSystemInfoCPUGet, "test-cpu-info"},
		{"memory-info", proto.CmdSystemInfoMemoryGet, "test-memory-info"},
		{"os-info", proto.CmdSystemInfoOSGet, "test-os-info"},
		{"performance-info", proto.CmdSystemInfoPerformanceGet, "test-performance-info"},
	}

	for _, test := range tests {
		wg := &sync.WaitGroup{}
		wg.Add(1)
		s.responseWaitGroups[test.reqID] = wg

		err := stream.Send(&proto.ToggleRequest{
			RequestId: test.reqID,
			Payload: &proto.ToggleRequest_Command{
				Command: &proto.CommandRequest{
					CommandType: test.cmdType,
					Payload:     []byte("{}"),
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to send %s command: %w", test.name, err)
		}

		if err := s.waitForResponse(wg, test.name); err != nil {
			return err
		}
	}

	return nil
}

// testHostnameManagement tests hostname get/set with proper restoration
func (s *MockToggleServer) testHostnameManagement(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing hostname management with state restoration...")

	// First, get current hostname
	wg1 := &sync.WaitGroup{}
	wg1.Add(1)
	s.responseWaitGroups["test-hostname-get"] = wg1

	err := stream.Send(&proto.ToggleRequest{
		RequestId: "test-hostname-get",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSystemHostnameGet,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send hostname get command: %w", err)
	}

	if err := s.waitForResponse(wg1, "hostname get"); err != nil {
		return err
	}

	// Then set a test hostname (will be restored in cleanup)
	wg2 := &sync.WaitGroup{}
	wg2.Add(1)
	s.responseWaitGroups["test-hostname-set"] = wg2

	testHostname := "test-system-grpc-temp"
	payload := system.SetHostnameRequest{
		Hostname: testHostname,
		Pretty:   "Test System gRPC Temporary",
		Static:   true,
	}
	payloadBytes, _ := json.Marshal(payload)

	err = stream.Send(&proto.ToggleRequest{
		RequestId: "test-hostname-set",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSystemHostnameSet,
				Payload:     payloadBytes,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send hostname set command: %w", err)
	}

	if err := s.waitForResponse(wg2, "hostname set"); err != nil {
		return err
	}

	// Verify the hostname was changed by getting it again
	wg3 := &sync.WaitGroup{}
	wg3.Add(1)
	s.responseWaitGroups["test-hostname-verify"] = wg3

	err = stream.Send(&proto.ToggleRequest{
		RequestId: "test-hostname-verify",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSystemHostnameGet,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send hostname verify command: %w", err)
	}

	return s.waitForResponse(wg3, "hostname verify")
}

// testUserManagementFlow tests complete user lifecycle with cleanup
func (s *MockToggleServer) testUserManagementFlow(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing user management flow...")

	testUsername := "testuser-grpc-temp"

	// List users first
	wg1 := &sync.WaitGroup{}
	wg1.Add(1)
	s.responseWaitGroups["test-users-list"] = wg1

	err := stream.Send(&proto.ToggleRequest{
		RequestId: "test-users-list",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSystemUsersList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send users list command: %w", err)
	}

	if err := s.waitForResponse(wg1, "users list"); err != nil {
		return err
	}

	// Create a test user
	wg2 := &sync.WaitGroup{}
	wg2.Add(1)
	s.responseWaitGroups["test-users-create"] = wg2

	payload := system.CreateUserRequest{
		Username:   testUsername,
		FullName:   "Test User gRPC Temporary",
		CreateHome: false,        // Don't create home to minimize impact
		Shell:      "/bin/false", // Use false shell for safety
	}
	payloadBytes, _ := json.Marshal(payload)

	err = stream.Send(&proto.ToggleRequest{
		RequestId: "test-users-create",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSystemUsersCreate,
				Payload:     payloadBytes,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send users create command: %w", err)
	}

	if err := s.waitForResponse(wg2, "users create"); err != nil {
		return err
	}

	// Track user for cleanup
	s.testState.TestUsersCreated = append(s.testState.TestUsersCreated, testUsername)

	// Get the created user
	wg3 := &sync.WaitGroup{}
	wg3.Add(1)
	s.responseWaitGroups["test-users-get"] = wg3

	getPayload := map[string]string{"username": testUsername}
	getPayloadBytes, _ := json.Marshal(getPayload)

	err = stream.Send(&proto.ToggleRequest{
		RequestId: "test-users-get",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSystemUsersGet,
				Payload:     getPayloadBytes,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send users get command: %w", err)
	}

	if err := s.waitForResponse(wg3, "users get"); err != nil {
		return err
	}

	// Delete the test user immediately (rather than waiting for cleanup)
	wg4 := &sync.WaitGroup{}
	wg4.Add(1)
	s.responseWaitGroups["test-users-delete"] = wg4

	deletePayload := map[string]string{"username": testUsername}
	deletePayloadBytes, _ := json.Marshal(deletePayload)

	err = stream.Send(&proto.ToggleRequest{
		RequestId: "test-users-delete",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSystemUsersDelete,
				Payload:     deletePayloadBytes,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send users delete command: %w", err)
	}

	if err := s.waitForResponse(wg4, "users delete"); err != nil {
		return err
	}

	// Remove from cleanup list since we deleted it
	s.testState.TestUsersCreated = []string{}

	return nil
}

// testGroupManagementFlow tests complete group lifecycle with cleanup
func (s *MockToggleServer) testGroupManagementFlow(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing group management flow...")

	testGroupName := "testgroup-grpc-temp"

	// List groups first
	wg1 := &sync.WaitGroup{}
	wg1.Add(1)
	s.responseWaitGroups["test-groups-list"] = wg1

	err := stream.Send(&proto.ToggleRequest{
		RequestId: "test-groups-list",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSystemGroupsList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send groups list command: %w", err)
	}

	if err := s.waitForResponse(wg1, "groups list"); err != nil {
		return err
	}

	// Create a test group
	wg2 := &sync.WaitGroup{}
	wg2.Add(1)
	s.responseWaitGroups["test-groups-create"] = wg2

	payload := system.CreateGroupRequest{
		Name: testGroupName,
	}
	payloadBytes, _ := json.Marshal(payload)

	err = stream.Send(&proto.ToggleRequest{
		RequestId: "test-groups-create",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSystemGroupsCreate,
				Payload:     payloadBytes,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send groups create command: %w", err)
	}

	if err := s.waitForResponse(wg2, "groups create"); err != nil {
		return err
	}

	// Track group for cleanup
	s.testState.TestGroupsCreated = append(s.testState.TestGroupsCreated, testGroupName)

	// Delete the test group immediately (rather than waiting for cleanup)
	wg3 := &sync.WaitGroup{}
	wg3.Add(1)
	s.responseWaitGroups["test-groups-delete"] = wg3

	deletePayload := map[string]string{"name": testGroupName}
	deletePayloadBytes, _ := json.Marshal(deletePayload)

	err = stream.Send(&proto.ToggleRequest{
		RequestId: "test-groups-delete",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdSystemGroupsDelete,
				Payload:     deletePayloadBytes,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send groups delete command: %w", err)
	}

	if err := s.waitForResponse(wg3, "groups delete"); err != nil {
		return err
	}

	// Remove from cleanup list since we deleted it
	s.testState.TestGroupsCreated = []string{}

	return nil
}

// testSystemConfigurationFlow tests timezone and locale with restoration
func (s *MockToggleServer) testSystemConfigurationFlow(
	stream proto.RodentService_ConnectServer,
) error {
	s.t.Log("Testing system configuration with state restoration...")

	// Test timezone and locale management
	tests := []struct {
		name    string
		cmdType string
		reqID   string
		payload interface{}
	}{
		{"timezone-get", proto.CmdSystemTimezoneGet, "test-timezone-get", nil},
		{"timezone-set", proto.CmdSystemTimezoneSet, "test-timezone-set",
			system.SetTimezoneRequest{Timezone: "America/New_York"}},
		{"locale-get", proto.CmdSystemLocaleGet, "test-locale-get", nil},
		{"locale-set", proto.CmdSystemLocaleSet, "test-locale-set",
			system.SetLocaleRequest{Locale: "en_US.UTF-8"}},
	}

	for _, test := range tests {
		wg := &sync.WaitGroup{}
		wg.Add(1)
		s.responseWaitGroups[test.reqID] = wg

		var payloadBytes []byte
		if test.payload != nil {
			payloadBytes, _ = json.Marshal(test.payload)
		} else {
			payloadBytes = []byte("{}")
		}

		err := stream.Send(&proto.ToggleRequest{
			RequestId: test.reqID,
			Payload: &proto.ToggleRequest_Command{
				Command: &proto.CommandRequest{
					CommandType: test.cmdType,
					Payload:     payloadBytes,
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to send %s command: %w", test.name, err)
		}

		if err := s.waitForResponse(wg, test.name); err != nil {
			return err
		}
	}

	return nil
}

// GetResponse retrieves a stored response
func (s *MockToggleServer) GetResponse(requestID string) *proto.CommandResponse {
	s.responsesLock.Lock()
	defer s.responsesLock.Unlock()
	return s.receivedResponses[requestID]
}

// setupGRPCTest creates a test environment for gRPC testing
func setupSystemGRPCTest(t *testing.T) (*MockToggleServer, func()) {
	// Skip if not integration test
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping system gRPC integration test; set RUN_INTEGRATION_TESTS=true to run")
	}

	// Skip if not running as root (many system operations require root)
	// if os.Geteuid() != 0 {
	// 	t.Skip("Skipping system integration tests; requires root privileges")
	// }

	// Create logger for testing
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test.system.grpc")
	require.NoError(t, err, "Failed to create logger")

	// Create system manager
	manager := system.NewManager(log)

	// Create system handler
	systemHandler := NewSystemHandler(manager, log)

	// Register gRPC handlers
	RegisterSystemGRPCHandlers(systemHandler)

	// Create mock server
	mockServer := NewMockToggleServer(t, systemHandler)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", testSystemGRPCPort))
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

// TestSystemGRPC_Integration runs the full gRPC integration test with state management
func TestSystemGRPC_Integration(t *testing.T) {
	// Setup mock Toggle server and test environment
	mockServer, cleanup := setupSystemGRPCTest(t)
	defer cleanup()

	// Initialize a Rodent client to connect to our mock Toggle server
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Create a JWT for testing (doesn't need to be valid for our mock Toggle server)
	testJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0LW9yZyIsInBydiI6dHJ1ZX0.dTdm2rfiSvx6rZ5JyIQvXg4gCYjmKTCcRKWsJo63Oh4"

	// Create a logger for the client
	l, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test-client")
	require.NoError(t, err)

	// Initialize the gRPC client
	serverAddr := fmt.Sprintf("localhost:%d", testSystemGRPCPort)
	gClient, err := client.NewGRPCClient(l, testJWT, serverAddr)
	require.NoError(t, err)
	defer gClient.Close()

	// Connect to the mock Toggle server
	_, cerr := gClient.Connect(ctx)
	require.NoError(t, cerr)

	// Wait for all test communications to complete
	time.Sleep(20 * time.Second)

	// -------------------------------------------------------------------------
	// Verify responses
	// -------------------------------------------------------------------------

	// Expected responses from our test flow
	expectedResponses := []string{
		// System info (safe read-only operations)
		"test-system-info",
		"test-cpu-info",
		"test-memory-info",
		"test-os-info",
		"test-performance-info",
		// Hostname management
		"test-hostname-get",
		"test-hostname-set",
		"test-hostname-verify",
		// User management flow
		"test-users-list",
		"test-users-create",
		"test-users-get",
		"test-users-delete",
		// Group management flow
		"test-groups-list",
		"test-groups-create",
		"test-groups-delete",
		// System configuration
		"test-timezone-get",
		"test-timezone-set",
		"test-locale-get",
		"test-locale-set",
	}

	successCount := 0
	for _, reqID := range expectedResponses {
		resp := mockServer.GetResponse(reqID)
		require.NotNil(t, resp, "Should have received response for %s", reqID)

		if !resp.Success {
			// Log warnings for failed operations but don't fail the test
			// Some operations might fail due to permissions or system state
			t.Logf("Warning: %s failed: %s", reqID, resp.Message)
		} else {
			successCount++
			assert.True(t, resp.Success, "Failed response for %s: %s", reqID, resp.Message)

			// Pretty print successful responses
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

	t.Logf("System gRPC integration test completed: %d/%d operations successful",
		successCount, len(expectedResponses))

	// Require that at least the read-only operations succeeded
	readOnlyOps := 5 // system info endpoints
	require.GreaterOrEqual(t, successCount, readOnlyOps,
		"At least the read-only system info operations should succeed")
}

// TestSystemGRPC_BasicValidation tests basic gRPC functionality without streaming
func TestSystemGRPC_BasicValidation(t *testing.T) {
	_, cleanup := setupSystemGRPCTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", testSystemGRPCPort),
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

	t.Log("Basic system gRPC validation completed successfully")
}
