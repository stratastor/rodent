// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/toggle/client"
	"github.com/stratastor/toggle-rodent-proto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func init() {
	// Use development config which has development.enabled: true
	// This must be set before any config loading happens
	os.Setenv("RODENT_CONFIG", "/home/rodent/.rodent/rodent.yml.dev")
}

const testEventsGRPCPort = 50101 // Use a different high port from other tests

// MockToggleServer is a mock implementation of the Toggle server for events testing
type MockToggleServer struct {
	proto.UnimplementedRodentServiceServer
	t             *testing.T
	receivedBatches []*proto.EventBatch
	batchesLock     sync.Mutex
	batchWaitGroup  *sync.WaitGroup
}

// NewMockToggleServer creates a new mock Toggle server for testing
func NewMockToggleServer(t *testing.T) *MockToggleServer {
	return &MockToggleServer{
		t:               t,
		receivedBatches: make([]*proto.EventBatch, 0),
		batchWaitGroup:  &sync.WaitGroup{},
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

// SendEvents implements the SendEvents RPC for testing
func (s *MockToggleServer) SendEvents(
	ctx context.Context,
	batch *proto.EventBatch,
) (*proto.EventBatchResponse, error) {
	// Verify JWT authentication
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &proto.EventBatchResponse{
			Success: false,
			Message: "No metadata found",
		}, nil
	}
	
	auth := md["authorization"]
	if len(auth) == 0 || len(auth[0]) < 7 || auth[0][:7] != "Bearer " {
		return &proto.EventBatchResponse{
			Success: false,
			Message: "Invalid authorization header",
		}, nil
	}

	s.t.Logf("Received event batch with %d events, batch ID: %s", 
		len(batch.Events), batch.BatchId)

	// Store the batch for verification
	s.batchesLock.Lock()
	s.receivedBatches = append(s.receivedBatches, batch)
	s.batchesLock.Unlock()

	// Log details of each event
	for i, event := range batch.Events {
		s.t.Logf("Event %d: Type=%s, Level=%s, Category=%s, Source=%s", 
			i, event.EventType, event.Level.String(), event.Category.String(), event.Source)
	}

	// Signal that we received a batch
	s.batchWaitGroup.Done()

	return &proto.EventBatchResponse{
		Success: true,
		Message: "Events received successfully",
	}, nil
}

// GetReceivedBatches returns all received event batches
func (s *MockToggleServer) GetReceivedBatches() []*proto.EventBatch {
	s.batchesLock.Lock()
	defer s.batchesLock.Unlock()
	
	// Return a copy to avoid race conditions
	batches := make([]*proto.EventBatch, len(s.receivedBatches))
	copy(batches, s.receivedBatches)
	return batches
}

// ExpectBatches sets up expectation for a certain number of batches
func (s *MockToggleServer) ExpectBatches(count int) {
	s.batchWaitGroup.Add(count)
}

// WaitForBatches waits for expected batches with timeout
func (s *MockToggleServer) WaitForBatches(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		s.batchWaitGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for event batches")
	}
}

// setupEventsGRPCTest creates a test environment for events gRPC testing
func setupEventsGRPCTest(t *testing.T) (*MockToggleServer, func()) {
	// Skip if not integration test
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping events gRPC integration test; set RUN_INTEGRATION_TESTS=true to run")
	}

	// Create mock server
	mockServer := NewMockToggleServer(t)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", testEventsGRPCPort))
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

// TestEventsGRPC_Integration runs the full events gRPC integration test
func TestEventsGRPC_Integration(t *testing.T) {
	// Setup mock Toggle server and test environment
	mockServer, cleanup := setupEventsGRPCTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Create a JWT for testing
	testJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0LW9yZyIsInBydiI6dHJ1ZX0.dTdm2rfiSvx6rZ5JyIQvXg4gCYjmKTCcRKWsJo63Oh4"

	// Create a logger for the client
	l, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test-events")
	require.NoError(t, err)

	// Initialize the gRPC client
	serverAddr := fmt.Sprintf("localhost:%d", testEventsGRPCPort)
	gClient, err := client.NewGRPCClient(l, testJWT, serverAddr)
	require.NoError(t, err)
	defer gClient.Close()

	// Test direct event client functionality first
	t.Run("DirectEventClient", func(t *testing.T) {
		testDirectEventClient(t, mockServer, gClient, ctx)
	})

	// Test event system initialization and emission
	t.Run("EventSystemInitialization", func(t *testing.T) {
		testEventSystemInitialization(t, mockServer, gClient, ctx, l)
	})
}

// testDirectEventClient tests the event client directly
func testDirectEventClient(t *testing.T, mockServer *MockToggleServer, gClient *client.GRPCClient, ctx context.Context) {
	// Create event client with test config
	config := DefaultEventConfig()
	config.BatchSize = 2          // Small batch size for quick testing
	config.BatchTimeout = 1 * time.Second

	// Create a logger for the event client
	eventLogger, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test-event-client")
	require.NoError(t, err)

	eventClient := NewEventClient(gClient.GetProtoClient(), gClient.GetJWT(), config, eventLogger)

	// Create test events
	events := []*Event{
		{
			ID:        "test-event-1",
			Type:      "test.event.one",
			Level:     LevelInfo,
			Category:  CategorySystem,
			Source:    "integration-test",
			Timestamp: time.Now(),
			Payload:   []byte(`{"key": "value1"}`),
			Metadata:  map[string]string{"test": "metadata1"},
		},
		{
			ID:        "test-event-2", 
			Type:      "test.event.two",
			Level:     LevelWarn,
			Category:  CategorySecurity,
			Source:    "integration-test",
			Timestamp: time.Now(),
			Payload:   []byte(`{"key": "value2"}`),
			Metadata:  map[string]string{"test": "metadata2"},
		},
	}

	// Set expectation for 1 batch
	mockServer.ExpectBatches(1)

	// Send batch
	err = eventClient.SendBatch(ctx, events)
	require.NoError(t, err, "Failed to send event batch")

	// Wait for batch to be received
	err = mockServer.WaitForBatches(5 * time.Second)
	require.NoError(t, err, "Failed to receive event batch")

	// Verify received events
	batches := mockServer.GetReceivedBatches()
	require.Len(t, batches, 1, "Should have received exactly 1 batch")

	batch := batches[0]
	require.Len(t, batch.Events, 2, "Batch should contain 2 events")

	// Verify first event
	event1 := batch.Events[0]
	assert.Equal(t, "test.event.one", event1.EventType)
	assert.Equal(t, proto.EventLevel_EVENT_LEVEL_INFO, event1.Level)
	assert.Equal(t, proto.EventCategory_EVENT_CATEGORY_SYSTEM, event1.Category)
	assert.Equal(t, "integration-test", event1.Source)
	assert.Equal(t, "test-event-1", event1.EventId)
	assert.Equal(t, `{"key": "value1"}`, string(event1.Payload))

	// Verify second event
	event2 := batch.Events[1]
	assert.Equal(t, "test.event.two", event2.EventType)
	assert.Equal(t, proto.EventLevel_EVENT_LEVEL_WARN, event2.Level)
	assert.Equal(t, proto.EventCategory_EVENT_CATEGORY_SECURITY, event2.Category)
	assert.Equal(t, "integration-test", event2.Source)
	assert.Equal(t, "test-event-2", event2.EventId)
	assert.Equal(t, `{"key": "value2"}`, string(event2.Payload))

	t.Log("Direct event client test completed successfully")
}

// testEventSystemInitialization tests full event system initialization and emission
func testEventSystemInitialization(t *testing.T, mockServer *MockToggleServer, gClient *client.GRPCClient, ctx context.Context, l logger.Logger) {
	// Initialize event system with the gRPC client
	err := InitializeWithClient(ctx, gClient, l)
	require.NoError(t, err, "Failed to initialize event system")

	// Verify system is initialized
	assert.True(t, IsInitialized(), "Event system should be initialized")

	// Get stats to verify initialization
	stats := GetStats()
	assert.True(t, stats["initialized"].(bool), "Stats should show initialized")

	// Set expectation for events (we'll emit several events)
	mockServer.ExpectBatches(1) // We expect 1 batch due to small test batch size

	// Emit various types of events
	EmitSystemEvent("system.test.info", LevelInfo, map[string]string{"action": "test"}, nil)
	EmitStorageEvent("storage.test.warn", LevelWarn, "test-storage", map[string]any{"size": 1024}, map[string]string{"pool": "test"})
	EmitNetworkEvent("network.test.error", LevelError, "test-network", "Connection failed", map[string]string{"interface": "eth0"})
	EmitSecurityEvent("security.test.critical", LevelCritical, "test-security", map[string]bool{"authenticated": false}, nil)
	EmitServiceEvent("service.test.info", LevelInfo, "test-service", "Service started", map[string]string{"version": "1.0"})

	// Generic emit
	Emit("generic.test.event", LevelInfo, CategoryService, "test-generic", "Generic event", map[string]string{"type": "generic"})

	// Wait a bit for async processing
	time.Sleep(2 * time.Second)

	// Wait for batch to be received (batch timeout is 30s, so wait 35s)
	err = mockServer.WaitForBatches(35 * time.Second)
	require.NoError(t, err, "Failed to receive event batches")

	// Verify received events
	batches := mockServer.GetReceivedBatches()
	require.GreaterOrEqual(t, len(batches), 1, "Should have received at least 1 batch")

	// Count total events across all batches
	totalEvents := 0
	for _, batch := range batches {
		totalEvents += len(batch.Events)
	}
	assert.GreaterOrEqual(t, totalEvents, 6, "Should have received at least 6 events")

	// Verify event types and categories are correct
	eventTypesSeen := make(map[string]bool)
	categoriesSeen := make(map[proto.EventCategory]bool)

	for _, batch := range batches {
		for _, event := range batch.Events {
			eventTypesSeen[event.EventType] = true
			categoriesSeen[event.Category] = true
		}
	}

	// Check we saw the expected event types
	expectedTypes := []string{
		"system.test.info",
		"storage.test.warn", 
		"network.test.error",
		"security.test.critical",
		"service.test.info",
		"generic.test.event",
	}

	for _, expectedType := range expectedTypes {
		assert.True(t, eventTypesSeen[expectedType], "Should have seen event type: %s", expectedType)
	}

	// Check we saw the expected categories
	expectedCategories := []proto.EventCategory{
		proto.EventCategory_EVENT_CATEGORY_SYSTEM,
		proto.EventCategory_EVENT_CATEGORY_STORAGE,
		proto.EventCategory_EVENT_CATEGORY_NETWORK,
		proto.EventCategory_EVENT_CATEGORY_SECURITY,
		proto.EventCategory_EVENT_CATEGORY_SERVICE,
	}

	for _, expectedCategory := range expectedCategories {
		assert.True(t, categoriesSeen[expectedCategory], "Should have seen category: %s", expectedCategory)
	}

	// Test shutdown
	err = Shutdown(ctx)
	require.NoError(t, err, "Failed to shutdown event system")

	assert.False(t, IsInitialized(), "Event system should not be initialized after shutdown")

	t.Log("Event system initialization test completed successfully")
}

// TestEventsGRPC_BasicValidation tests basic gRPC functionality
func TestEventsGRPC_BasicValidation(t *testing.T) {
	mockServer, cleanup := setupEventsGRPCTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", testEventsGRPCPort),
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

	// Set expectation for 1 batch
	mockServer.ExpectBatches(1)

	// Test SendEvents with JWT
	testJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0LW9yZyIsInBydiI6dHJ1ZX0.dTdm2rfiSvx6rZ5JyIQvXg4gCYjmKTCcRKWsJo63Oh4"
	md := metadata.Pairs("authorization", "Bearer "+testJWT)
	authCtx := metadata.NewOutgoingContext(ctx, md)

	// Create test event
	event := &proto.Event{
		EventId:   "test-validation-event",
		EventType: "validation.test",
		Level:     proto.EventLevel_EVENT_LEVEL_INFO,
		Category:  proto.EventCategory_EVENT_CATEGORY_SYSTEM,
		Source:    "validation-test",
		Timestamp: time.Now().UnixMilli(),
		Payload:   []byte(`{"validation": true}`),
		Metadata:  map[string]string{"test": "validation"},
	}

	batch := &proto.EventBatch{
		Events:         []*proto.Event{event},
		BatchTimestamp: time.Now().UnixMilli(),
		BatchId:        "test-validation-batch",
	}

	// Send events
	eventsResp, err := client.SendEvents(authCtx, batch)
	require.NoError(t, err, "Failed to send events")
	assert.True(t, eventsResp.Success, "SendEvents failed: %s", eventsResp.Message)

	// Wait for batch to be received
	err = mockServer.WaitForBatches(5 * time.Second)
	require.NoError(t, err, "Failed to receive expected batch")

	t.Log("Basic events gRPC validation completed successfully")
}

