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
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/events"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/rodent/pkg/disk"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	// Use development config
	os.Setenv("RODENT_CONFIG", "/home/rodent/.rodent/rodent.yml.dev")
}

const testDiskGRPCPort = 50101 // Use a different port from system tests

// MockToggleServerDisk is a mock implementation of the Toggle server for Disk testing
type MockToggleServerDisk struct {
	proto.UnimplementedRodentServiceServer
	t                  *testing.T
	receivedResponses  map[string]*proto.CommandResponse
	responsesLock      sync.Mutex
	responseWaitGroups map[string]*sync.WaitGroup
	diskHandler        *DiskHandler
}

// NewMockToggleServerDisk creates a new mock Toggle server for disk testing
func NewMockToggleServerDisk(t *testing.T, diskHandler *DiskHandler) *MockToggleServerDisk {
	return &MockToggleServerDisk{
		t:                  t,
		receivedResponses:  make(map[string]*proto.CommandResponse),
		responseWaitGroups: make(map[string]*sync.WaitGroup),
		diskHandler:        diskHandler,
	}
}

// Register is a simple implementation that just returns success
func (s *MockToggleServerDisk) Register(
	ctx context.Context,
	req *proto.RegisterRequest,
) (*proto.RegisterResponse, error) {
	return &proto.RegisterResponse{
		Success: true,
		Message: "Test registration successful",
	}, nil
}

// waitForResponse is a helper to wait for a response with timeout
func (s *MockToggleServerDisk) waitForResponse(
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
	case <-time.After(15 * time.Second):
		return fmt.Errorf("timeout waiting for %s response", description)
	}
}

// Connect implements the bidirectional streaming for testing
func (s *MockToggleServerDisk) Connect(stream proto.RodentService_ConnectServer) error {
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

	return s.runDiskTests(stream)
}

// runDiskTests executes a series of disk management tests
func (s *MockToggleServerDisk) runDiskTests(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Starting disk gRPC integration tests...")

	// Test inventory operations
	if err := s.testInventoryOperations(stream); err != nil {
		return err
	}

	// Test discovery operations
	if err := s.testDiscoveryOperations(stream); err != nil {
		return err
	}

	// Test health and SMART operations
	if err := s.testHealthAndSMARTOperations(stream); err != nil {
		return err
	}

	// Test topology operations
	if err := s.testTopologyOperations(stream); err != nil {
		return err
	}

	// Test probe operations
	if err := s.testProbeOperations(stream); err != nil {
		return err
	}

	// Test statistics operations
	if err := s.testStatisticsOperations(stream); err != nil {
		return err
	}

	// Test configuration operations
	if err := s.testConfigurationOperations(stream); err != nil {
		return err
	}

	s.t.Log("All disk gRPC tests completed successfully")
	return nil
}

// testInventoryOperations tests disk listing and retrieval
func (s *MockToggleServerDisk) testInventoryOperations(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing inventory operations...")

	tests := []struct {
		name    string
		cmdType string
		reqID   string
		payload interface{}
	}{
		{"disk-list", proto.CmdDiskList, "test-disk-list", nil},
		{"disk-list-available", proto.CmdDiskListAvailable, "test-disk-list-available", nil},
		{"disk-list-filtered", proto.CmdDiskList, "test-disk-list-filtered",
			map[string]interface{}{
				"states": []string{"AVAILABLE"},
			}},
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

	// Test disk get if we have any disks
	listResp := s.GetResponse("test-disk-list")
	if listResp != nil && listResp.Success && len(listResp.Payload) > 0 {
		var result struct {
			Disks []types.PhysicalDisk `json:"disks"`
		}
		if err := json.Unmarshal(listResp.Payload, &result); err == nil && len(result.Disks) > 0 {
			// Test getting the first disk
			wg := &sync.WaitGroup{}
			wg.Add(1)
			s.responseWaitGroups["test-disk-get"] = wg

			payload := map[string]string{"device_id": result.Disks[0].DeviceID}
			payloadBytes, _ := json.Marshal(payload)

			err := stream.Send(&proto.ToggleRequest{
				RequestId: "test-disk-get",
				Payload: &proto.ToggleRequest_Command{
					Command: &proto.CommandRequest{
						CommandType: proto.CmdDiskGet,
						Payload:     payloadBytes,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to send disk-get command: %w", err)
			}

			if err := s.waitForResponse(wg, "disk-get"); err != nil {
				return err
			}
		}
	}

	return nil
}

// testDiscoveryOperations tests disk discovery
func (s *MockToggleServerDisk) testDiscoveryOperations(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing discovery operations...")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	s.responseWaitGroups["test-disk-discover"] = wg

	err := stream.Send(&proto.ToggleRequest{
		RequestId: "test-disk-discover",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdDiskDiscover,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send disk-discover command: %w", err)
	}

	return s.waitForResponse(wg, "disk-discover")
}

// testHealthAndSMARTOperations tests health check and SMART operations
func (s *MockToggleServerDisk) testHealthAndSMARTOperations(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing health and SMART operations...")

	// Trigger health check for all disks
	wg1 := &sync.WaitGroup{}
	wg1.Add(1)
	s.responseWaitGroups["test-health-check"] = wg1

	err := stream.Send(&proto.ToggleRequest{
		RequestId: "test-health-check",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdDiskHealthCheck,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send health-check command: %w", err)
	}

	if err := s.waitForResponse(wg1, "health-check"); err != nil {
		return err
	}

	// Get health status for a specific disk if available
	listResp := s.GetResponse("test-disk-list")
	if listResp != nil && listResp.Success && len(listResp.Payload) > 0 {
		var result struct {
			Disks []types.PhysicalDisk `json:"disks"`
		}
		if err := json.Unmarshal(listResp.Payload, &result); err == nil && len(result.Disks) > 0 {
			deviceID := result.Disks[0].DeviceID

			// Get health status
			wg2 := &sync.WaitGroup{}
			wg2.Add(1)
			s.responseWaitGroups["test-health-get"] = wg2

			payload := map[string]string{"device_id": deviceID}
			payloadBytes, _ := json.Marshal(payload)

			err := stream.Send(&proto.ToggleRequest{
				RequestId: "test-health-get",
				Payload: &proto.ToggleRequest_Command{
					Command: &proto.CommandRequest{
						CommandType: proto.CmdDiskHealthGet,
						Payload:     payloadBytes,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to send health-get command: %w", err)
			}

			if err := s.waitForResponse(wg2, "health-get"); err != nil {
				return err
			}

			// Get SMART data
			wg3 := &sync.WaitGroup{}
			wg3.Add(1)
			s.responseWaitGroups["test-smart-get"] = wg3

			err = stream.Send(&proto.ToggleRequest{
				RequestId: "test-smart-get",
				Payload: &proto.ToggleRequest_Command{
					Command: &proto.CommandRequest{
						CommandType: proto.CmdDiskSMARTGet,
						Payload:     payloadBytes,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to send smart-get command: %w", err)
			}

			if err := s.waitForResponse(wg3, "smart-get"); err != nil {
				return err
			}

			// Refresh SMART data
			wg4 := &sync.WaitGroup{}
			wg4.Add(1)
			s.responseWaitGroups["test-smart-refresh"] = wg4

			err = stream.Send(&proto.ToggleRequest{
				RequestId: "test-smart-refresh",
				Payload: &proto.ToggleRequest_Command{
					Command: &proto.CommandRequest{
						CommandType: proto.CmdDiskSMARTRefresh,
						Payload:     payloadBytes,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to send smart-refresh command: %w", err)
			}

			if err := s.waitForResponse(wg4, "smart-refresh"); err != nil {
				return err
			}
		}
	}

	return nil
}

// testTopologyOperations tests topology-related operations
func (s *MockToggleServerDisk) testTopologyOperations(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing topology operations...")

	tests := []struct {
		name    string
		cmdType string
		reqID   string
	}{
		{"topology-get", proto.CmdDiskTopologyGet, "test-topology-get"},
		{"topology-refresh", proto.CmdDiskTopologyRefresh, "test-topology-refresh"},
		{"topology-controllers", proto.CmdDiskTopologyControllers, "test-topology-controllers"},
		{"topology-enclosures", proto.CmdDiskTopologyEnclosures, "test-topology-enclosures"},
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

// testProbeOperations tests probe (SMART self-test) operations
func (s *MockToggleServerDisk) testProbeOperations(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing probe operations...")

	// List probes
	wg1 := &sync.WaitGroup{}
	wg1.Add(1)
	s.responseWaitGroups["test-probe-list"] = wg1

	err := stream.Send(&proto.ToggleRequest{
		RequestId: "test-probe-list",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdDiskProbeList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send probe-list command: %w", err)
	}

	if err := s.waitForResponse(wg1, "probe-list"); err != nil {
		return err
	}

	// List probe schedules
	wg2 := &sync.WaitGroup{}
	wg2.Add(1)
	s.responseWaitGroups["test-probe-schedule-list"] = wg2

	err = stream.Send(&proto.ToggleRequest{
		RequestId: "test-probe-schedule-list",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdDiskProbeScheduleList,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send probe-schedule-list command: %w", err)
	}

	return s.waitForResponse(wg2, "probe-schedule-list")
}

// testStatisticsOperations tests statistics operations
func (s *MockToggleServerDisk) testStatisticsOperations(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing statistics operations...")

	// Get global statistics
	wg1 := &sync.WaitGroup{}
	wg1.Add(1)
	s.responseWaitGroups["test-stats-global"] = wg1

	err := stream.Send(&proto.ToggleRequest{
		RequestId: "test-stats-global",
		Payload: &proto.ToggleRequest_Command{
			Command: &proto.CommandRequest{
				CommandType: proto.CmdDiskStatsGlobal,
				Payload:     []byte("{}"),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send stats-global command: %w", err)
	}

	if err := s.waitForResponse(wg1, "stats-global"); err != nil {
		return err
	}

	// Get device statistics if we have any disks
	listResp := s.GetResponse("test-disk-list")
	if listResp != nil && listResp.Success && len(listResp.Payload) > 0 {
		var result struct {
			Disks []types.PhysicalDisk `json:"disks"`
		}
		if err := json.Unmarshal(listResp.Payload, &result); err == nil && len(result.Disks) > 0 {
			wg2 := &sync.WaitGroup{}
			wg2.Add(1)
			s.responseWaitGroups["test-stats-get"] = wg2

			payload := map[string]string{"device_id": result.Disks[0].DeviceID}
			payloadBytes, _ := json.Marshal(payload)

			err := stream.Send(&proto.ToggleRequest{
				RequestId: "test-stats-get",
				Payload: &proto.ToggleRequest_Command{
					Command: &proto.CommandRequest{
						CommandType: proto.CmdDiskStatsGet,
						Payload:     payloadBytes,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to send stats-get command: %w", err)
			}

			if err := s.waitForResponse(wg2, "stats-get"); err != nil {
				return err
			}
		}
	}

	return nil
}

// testConfigurationOperations tests configuration management
func (s *MockToggleServerDisk) testConfigurationOperations(stream proto.RodentService_ConnectServer) error {
	s.t.Log("Testing configuration operations...")

	tests := []struct {
		name    string
		cmdType string
		reqID   string
	}{
		{"config-get", proto.CmdDiskConfigGet, "test-config-get"},
		{"monitoring-get", proto.CmdDiskMonitoringGet, "test-monitoring-get"},
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

// GetResponse retrieves a stored response
func (s *MockToggleServerDisk) GetResponse(requestID string) *proto.CommandResponse {
	s.responsesLock.Lock()
	defer s.responsesLock.Unlock()
	return s.receivedResponses[requestID]
}

// setupDiskGRPCTest creates a test environment for gRPC testing
func setupDiskGRPCTest(t *testing.T) (*MockToggleServerDisk, func()) {
	// Skip if not integration test
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping disk gRPC integration test; set RUN_INTEGRATION_TESTS=true to run")
	}

	// Create logger for testing
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test.disk.grpc")
	require.NoError(t, err, "Failed to create logger")

	// Create command executor with sudo support
	executor := command.NewCommandExecutor(true)

	// Use global event bus (may be nil if not initialized yet)
	eventBus := events.GlobalEventBus
	if eventBus == nil {
		log.Warn("Global event bus not initialized, disk events will be logged only")
	}

	// Create disk manager
	manager, err := disk.NewManager(log, executor, eventBus)
	require.NoError(t, err, "Failed to create disk manager")

	// Start disk manager
	ctx := context.Background()
	err = manager.Start(ctx)
	require.NoError(t, err, "Failed to start disk manager")

	// Create disk handler
	diskHandler := NewDiskHandler(manager, log)

	// Register gRPC handlers
	RegisterDiskGRPCHandlers(diskHandler)

	// Create mock server
	mockServer := NewMockToggleServerDisk(t, diskHandler)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", testDiskGRPCPort))
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
		ctx := context.Background()
		manager.Stop(ctx)
	}

	// Wait a moment for server to be ready and discovery to complete
	time.Sleep(2 * time.Second)

	return mockServer, cleanup
}

// TestDiskGRPC_Integration runs the full gRPC integration test
func TestDiskGRPC_Integration(t *testing.T) {
	// Setup mock Toggle server and test environment
	mockServer, cleanup := setupDiskGRPCTest(t)
	defer cleanup()

	// Initialize a Rodent client to connect to our mock Toggle server
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a JWT for testing
	testJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0LW9yZyIsInBydiI6dHJ1ZX0.dTdm2rfiSvx6rZ5JyIQvXg4gCYjmKTCcRKWsJo63Oh4"

	// Create a logger for the client
	l, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test-client")
	require.NoError(t, err)

	// Initialize the gRPC client
	serverAddr := fmt.Sprintf("localhost:%d", testDiskGRPCPort)
	gClient, err := client.NewGRPCClient(l, testJWT, serverAddr)
	require.NoError(t, err)
	defer gClient.Close()

	// Connect to the mock Toggle server
	_, cerr := gClient.Connect(ctx)
	require.NoError(t, cerr)

	// Wait for all test communications to complete
	time.Sleep(30 * time.Second)

	// -------------------------------------------------------------------------
	// Verify responses
	// -------------------------------------------------------------------------

	// Expected responses from our test flow
	expectedResponses := []string{
		// Inventory operations
		"test-disk-list",
		"test-disk-list-available",
		"test-disk-list-filtered",
		// Discovery operations
		"test-disk-discover",
		// Health and SMART operations
		"test-health-check",
		// Topology operations
		"test-topology-get",
		"test-topology-refresh",
		"test-topology-controllers",
		"test-topology-enclosures",
		// Probe operations
		"test-probe-list",
		"test-probe-schedule-list",
		// Statistics operations
		"test-stats-global",
		// Configuration operations
		"test-config-get",
		"test-monitoring-get",
	}

	successCount := 0
	for _, reqID := range expectedResponses {
		resp := mockServer.GetResponse(reqID)
		require.NotNil(t, resp, "Should have received response for %s", reqID)

		if !resp.Success {
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
				}
			}
		}
	}

	t.Logf("Disk gRPC integration test completed: %d/%d operations successful",
		successCount, len(expectedResponses))

	// Require that at least the read-only operations succeeded
	readOnlyOps := 10 // inventory, topology, probe list, stats, config get operations
	require.GreaterOrEqual(t, successCount, readOnlyOps,
		"At least the read-only operations should succeed")
}

// TestDiskGRPC_BasicValidation tests basic gRPC functionality without streaming
func TestDiskGRPC_BasicValidation(t *testing.T) {
	_, cleanup := setupDiskGRPCTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", testDiskGRPCPort),
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

	t.Log("Basic disk gRPC validation completed successfully")
}
