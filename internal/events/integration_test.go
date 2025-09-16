// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

// integration_test.go - Structured Event System Integration Tests
//
// This file contains integration tests for the new structured event system that uses:
// - Predefined event type constants (eventsconstants.StorageTransferStarted, etc.)
// - Strongly-typed protobuf payloads (eventspb.StorageTransferPayload, etc.)
// - Structured emission functions (EmitStorageTransfer, EmitIdentityUser, etc.)
// - Schema-driven event definitions from toggle-rodent-proto repository
//
// These tests validate the new event architecture that provides:
// - Type safety and schema validation
// - Centralized event definitions preventing Rodent/Toggle dissonance
// - All 8 event categories (SYSTEM, STORAGE, NETWORK, SECURITY, SERVICE, IDENTITY, ACCESS, SHARING)
// - Structured metadata using standardized keys (eventsconstants.MetaComponent, etc.)
//
// Key differences from legacy events (integration_legacy_test.go):
// - Uses protobuf-defined payload structures instead of map[string]interface{}
// - Event types are constants defined in toggle-rodent-proto
// - Metadata follows standardized schema
// - Supports new categories (IDENTITY, ACCESS, SHARING)
// - Prevents double JSON marshaling through emitTypedEvent()
//
// This represents the target architecture for all future event implementations.

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/toggle/client"
	eventsconstants "github.com/stratastor/toggle-rodent-proto/go/events"
	"github.com/stratastor/toggle-rodent-proto/proto"
	eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func init() {
	// Use development config which has development.enabled: true
	// This must be set before any config loading happens
	os.Setenv("RODENT_CONFIG", "/home/rodent/.rodent/rodent.yml.dev")
}

const testEventsGRPCPortUpdated = 50103 // Use a different high port from other tests

// MockToggleServerUpdated is a mock implementation of the Toggle server for updated events testing
type MockToggleServerUpdated struct {
	proto.UnimplementedRodentServiceServer
	t               *testing.T
	receivedBatches []*proto.EventBatch
	batchesLock     sync.Mutex
	batchWaitGroup  *sync.WaitGroup
}

// NewMockToggleServerUpdated creates a new mock Toggle server for testing
func NewMockToggleServerUpdated(t *testing.T) *MockToggleServerUpdated {
	return &MockToggleServerUpdated{
		t:               t,
		receivedBatches: make([]*proto.EventBatch, 0),
		batchWaitGroup:  &sync.WaitGroup{},
	}
}

// Register is a simple implementation that just returns success
func (s *MockToggleServerUpdated) Register(
	ctx context.Context,
	req *proto.RegisterRequest,
) (*proto.RegisterResponse, error) {
	return &proto.RegisterResponse{
		Success: true,
		Message: "Test registration successful",
	}, nil
}

// SendEvents implements the SendEvents RPC for testing
func (s *MockToggleServerUpdated) SendEvents(
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

	s.t.Logf("Received updated event batch with %d events, batch ID: %s",
		len(batch.Events), batch.BatchId)

	// Store the batch for verification
	s.batchesLock.Lock()
	s.receivedBatches = append(s.receivedBatches, batch)
	s.batchesLock.Unlock()

	// Log details of each event
	for i, event := range batch.Events {
		s.t.Logf("Event %d: Type=%s, Level=%s, Category=%s, Source=%s",
			i, event.EventType, event.Level.String(), event.Category.String(), event.Source)

		// Log structured payload if available
		if len(event.Payload) > 0 {
			var payload map[string]interface{}
			if err := json.Unmarshal(event.Payload, &payload); err == nil {
				s.t.Logf("  Structured Payload: %+v", payload)
			}
		}

		// Log metadata
		if len(event.Metadata) > 0 {
			s.t.Logf("  Metadata: %+v", event.Metadata)
		}
	}

	// Signal that we received a batch
	s.batchWaitGroup.Done()

	return &proto.EventBatchResponse{
		Success: true,
		Message: "Events received successfully",
	}, nil
}

// GetReceivedBatches returns all received event batches
func (s *MockToggleServerUpdated) GetReceivedBatches() []*proto.EventBatch {
	s.batchesLock.Lock()
	defer s.batchesLock.Unlock()

	// Return a copy to avoid race conditions
	batches := make([]*proto.EventBatch, len(s.receivedBatches))
	copy(batches, s.receivedBatches)
	return batches
}

// ExpectBatches sets up expectation for a certain number of batches
func (s *MockToggleServerUpdated) ExpectBatches(count int) {
	s.batchWaitGroup.Add(count)
}

// WaitForBatches waits for expected batches with timeout
func (s *MockToggleServerUpdated) WaitForBatches(timeout time.Duration) error {
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

// setupEventsGRPCTestUpdated creates a test environment for updated events gRPC testing
func setupEventsGRPCTestUpdated(t *testing.T) (*MockToggleServerUpdated, func()) {
	// Skip if not integration test
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip(
			"Skipping updated events gRPC integration test; set RUN_INTEGRATION_TESTS=true to run",
		)
	}

	// Create mock server
	mockServer := NewMockToggleServerUpdated(t)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", testEventsGRPCPortUpdated))
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

// TestEventsGRPCUpdated_Integration runs the full updated events gRPC integration test
func TestEventsGRPCUpdated_Integration(t *testing.T) {
	// Setup mock Toggle server and test environment
	mockServer, cleanup := setupEventsGRPCTestUpdated(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Create a JWT for testing
	testJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0LW9yZyIsInBydiI6dHJ1ZX0.dTdm2rfiSvx6rZ5JyIQvXg4gCYjmKTCcRKWsJo63Oh4"

	// Create a logger for the client
	l, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test-events-updated")
	require.NoError(t, err)

	// Initialize the gRPC client
	serverAddr := fmt.Sprintf("localhost:%d", testEventsGRPCPortUpdated)
	gClient, err := client.NewGRPCClient(l, testJWT, serverAddr)
	require.NoError(t, err)
	defer gClient.Close()

	// Test direct event client functionality first
	t.Run("DirectEventClientUpdated", func(t *testing.T) {
		testDirectEventClientUpdated(t, mockServer, gClient, ctx)
	})

	// Test updated event system initialization and emission
	t.Run("EventSystemInitializationUpdated", func(t *testing.T) {
		testEventSystemInitializationUpdated(t, mockServer, gClient, ctx, l)
	})

	// Test disk flush functionality
	t.Run("DiskFlushFunctionalityUpdated", func(t *testing.T) {
		testDiskFlushFunctionalityUpdated(t, l)
	})
}

// testDirectEventClientUpdated tests the event client directly with structured events
func testDirectEventClientUpdated(
	t *testing.T,
	mockServer *MockToggleServerUpdated,
	gClient *client.GRPCClient,
	ctx context.Context,
) {
	// Create event client with test config
	config := DefaultEventConfig()
	config.BatchSize = 2 // Small batch size for quick testing
	config.BatchTimeout = 1 * time.Second

	// Create a logger for the event client
	eventLogger, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test-event-client-updated")
	require.NoError(t, err)

	eventClient := NewEventClient(gClient.GetProtoClient(), gClient.GetJWT(), config, eventLogger)

	// Create test events with structured payloads
	event1Payload := &eventspb.StorageTransferPayload{
		TransferId:     "test-transfer-1",
		Operation:      "send",
		SourceSnapshot: "tank/test@snap1",
		TargetDataset:  "backup/test",
		Status:         "started",
		RemoteHost:     "backup-server",
	}
	event1PayloadBytes, _ := json.Marshal(event1Payload)

	event2Payload := &eventspb.SecurityKeyPayload{
		KeyId:       "test-key-1",
		KeyType:     "ed25519",
		Algorithm:   "Ed25519",
		KeySize:     256,
		Fingerprint: "SHA256:testfingerprint",
		CreatedBy:   "integration-test",
		Purpose:     "ssh",
	}
	event2PayloadBytes, _ := json.Marshal(event2Payload)

	events := []*Event{
		{
			ID:        "test-event-updated-1",
			Type:      eventsconstants.StorageTransferStarted,
			Level:     LevelInfo,
			Category:  CategoryStorage,
			Source:    "integration-test",
			Timestamp: time.Now(),
			Payload:   event1PayloadBytes,
			Metadata: map[string]string{
				eventsconstants.MetaComponent:  "zfs-transfer",
				eventsconstants.MetaAction:     "start",
				eventsconstants.MetaTransferID: "test-transfer-1",
			},
		},
		{
			ID:        "test-event-updated-2",
			Type:      eventsconstants.SecurityKeyGenerated,
			Level:     LevelInfo,
			Category:  CategorySecurity,
			Source:    "integration-test",
			Timestamp: time.Now(),
			Payload:   event2PayloadBytes,
			Metadata: map[string]string{
				eventsconstants.MetaComponent: "ssh-key-manager",
				eventsconstants.MetaAction:    "generate",
				eventsconstants.MetaUser:      "integration-test",
			},
		},
	}

	// Set expectation for 1 batch
	mockServer.ExpectBatches(1)

	// Send batch
	err = eventClient.SendBatch(ctx, events)
	require.NoError(t, err, "Failed to send updated event batch")

	// Wait for batch to be received
	err = mockServer.WaitForBatches(5 * time.Second)
	require.NoError(t, err, "Failed to receive updated event batch")

	// Verify received events
	batches := mockServer.GetReceivedBatches()
	require.Len(t, batches, 1, "Should have received exactly 1 batch")

	batch := batches[0]
	require.Len(t, batch.Events, 2, "Batch should contain 2 events")

	// Verify first event (Storage Transfer)
	event1 := batch.Events[0]
	assert.Equal(t, eventsconstants.StorageTransferStarted, event1.EventType)
	assert.Equal(t, proto.EventLevel_EVENT_LEVEL_INFO, event1.Level)
	assert.Equal(t, proto.EventCategory_EVENT_CATEGORY_STORAGE, event1.Category)
	assert.Equal(t, "integration-test", event1.Source)
	assert.Equal(t, "test-event-updated-1", event1.EventId)

	// Verify structured payload can be unmarshaled
	var receivedPayload1 eventspb.StorageTransferPayload
	err = json.Unmarshal(event1.Payload, &receivedPayload1)
	require.NoError(t, err, "Should be able to unmarshal storage transfer payload")
	assert.Equal(t, "test-transfer-1", receivedPayload1.TransferId)
	assert.Equal(t, "send", receivedPayload1.Operation)

	// Verify metadata
	assert.Equal(t, "zfs-transfer", event1.Metadata[eventsconstants.MetaComponent])
	assert.Equal(t, "test-transfer-1", event1.Metadata[eventsconstants.MetaTransferID])

	// Verify second event (Security Key)
	event2 := batch.Events[1]
	assert.Equal(t, eventsconstants.SecurityKeyGenerated, event2.EventType)
	assert.Equal(t, proto.EventLevel_EVENT_LEVEL_INFO, event2.Level)
	assert.Equal(t, proto.EventCategory_EVENT_CATEGORY_SECURITY, event2.Category)
	assert.Equal(t, "integration-test", event2.Source)
	assert.Equal(t, "test-event-updated-2", event2.EventId)

	// Verify structured payload
	var receivedPayload2 eventspb.SecurityKeyPayload
	err = json.Unmarshal(event2.Payload, &receivedPayload2)
	require.NoError(t, err, "Should be able to unmarshal security key payload")
	assert.Equal(t, "test-key-1", receivedPayload2.KeyId)
	assert.Equal(t, "ed25519", receivedPayload2.KeyType)

	t.Log("Direct event client updated test completed successfully")
}

// testEventSystemInitializationUpdated tests full event system initialization and emission with structured events
func testEventSystemInitializationUpdated(
	t *testing.T,
	mockServer *MockToggleServerUpdated,
	gClient *client.GRPCClient,
	ctx context.Context,
	l logger.Logger,
) {
	// Initialize event system with the gRPC client
	err := InitializeWithClient(ctx, gClient, l)
	require.NoError(t, err, "Failed to initialize updated event system")

	// Verify system is initialized
	assert.True(t, IsInitialized(), "Updated event system should be initialized")

	// Get stats to verify initialization
	stats := GetStats()
	assert.True(t, stats["initialized"].(bool), "Stats should show initialized")

	// Set expectation for events (we'll emit several events)
	mockServer.ExpectBatches(1) // We expect 1 batch due to small test batch size

	// Emit various types of events using structured payloads

	// System Event
	systemPayload := &eventspb.SystemStartupPayload{
		Version:         "test-1.0.0",
		BootTimeSeconds: time.Now().Unix(),
		Hostname:        "test-host",
		ServicesStarted: []string{"test-service"},
	}
	EmitSystemStartup(systemPayload, map[string]string{
		eventsconstants.MetaComponent: "test-system",
		eventsconstants.MetaAction:    "startup",
	})

	// Storage Event
	storagePayload := &eventspb.StorageTransferPayload{
		TransferId:       "test-transfer-warn",
		Operation:        "receive",
		SourceSnapshot:   "remote/dataset@snap",
		TargetDataset:    "local/dataset",
		Status:           "warning",
		BytesTransferred: 1024,
		RemoteHost:       "remote-host",
	}
	EmitStorageTransfer(
		eventsconstants.StorageTransferStarted,
		LevelWarn,
		storagePayload,
		map[string]string{
			eventsconstants.MetaComponent:  "zfs-transfer",
			eventsconstants.MetaAction:     "start",
			eventsconstants.MetaTransferID: "test-transfer-warn",
		},
	)

	// Network Event
	networkPayload := &eventspb.NetworkConnectionPayload{
		SourceIp:        "192.168.1.100",
		DestinationIp:   "192.168.1.1",
		SourcePort:      12345,
		DestinationPort: 80,
		Protocol:        "tcp",
		ConnectionType:  "tcp",
		Status:          "failed",
		DurationMs:      5000,
	}
	EmitNetworkConnection(
		eventsconstants.NetworkConnectionFailed,
		LevelError,
		networkPayload,
		map[string]string{
			eventsconstants.MetaComponent: "netmage",
			eventsconstants.MetaAction:    "connect",
			eventsconstants.MetaInterface: "eth0",
		},
	)

	// Security Event
	securityPayload := &eventspb.SecurityAuthPayload{
		Username:     "test-user",
		IpAddress:    "192.168.1.200",
		Method:       "password",
		Reason:       "invalid_credentials",
		Resource:     "ssh",
		AttemptCount: 3,
	}
	EmitSecurityAuth(
		eventsconstants.SecurityAuthFailed,
		LevelCritical,
		securityPayload,
		map[string]string{
			eventsconstants.MetaComponent: "auth-manager",
			eventsconstants.MetaAction:    "authenticate",
			eventsconstants.MetaUser:      "test-user",
		},
	)

	// Service Event
	servicePayload := &eventspb.ServiceStatusPayload{
		ServiceName:      "test-service",
		Status:           "running",
		Version:          "1.0.0",
		Pid:              9999,
		UptimeSeconds:    300,
		EnabledAtStartup: true,
	}
	EmitServiceStatus(eventsconstants.ServiceStarted, LevelInfo, servicePayload, map[string]string{
		eventsconstants.MetaComponent: "service-manager",
		eventsconstants.MetaAction:    "start",
		eventsconstants.MetaService:   "test-service",
	})

	// Identity Event (new category)
	identityPayload := &eventspb.IdentityUserPayload{
		Username:    "test-ad-user",
		DisplayName: "Test AD User",
		Email:       "testuser@test.local",
		Groups:      []string{"users", "developers"},
		Enabled:     true,
		Domain:      "test.local",
	}
	EmitIdentityUser(
		eventsconstants.IdentityUserCreated,
		LevelInfo,
		identityPayload,
		map[string]string{
			eventsconstants.MetaComponent: "ad-manager",
			eventsconstants.MetaAction:    "create",
			eventsconstants.MetaUser:      "test-ad-user",
			eventsconstants.MetaDomain:    "test.local",
		},
	)

	// Generic emit for backward compatibility testing
	Emit(
		"generic.test.event",
		LevelInfo,
		CategoryService,
		"test-generic",
		"Generic event",
		map[string]string{"type": "generic"},
	)

	// Wait a bit for async processing
	time.Sleep(2 * time.Second)

	// Wait for batch to be received (batch timeout is 30s, so wait 35s)
	err = mockServer.WaitForBatches(35 * time.Second)
	require.NoError(t, err, "Failed to receive updated event batches")

	// Verify received events
	batches := mockServer.GetReceivedBatches()
	require.GreaterOrEqual(t, len(batches), 1, "Should have received at least 1 batch")

	// Count total events across all batches
	totalEvents := 0
	for _, batch := range batches {
		totalEvents += len(batch.Events)
	}
	assert.GreaterOrEqual(t, totalEvents, 7, "Should have received at least 7 events")

	// Verify event types and categories are correct
	eventTypesSeen := make(map[string]bool)
	categoriesSeen := make(map[proto.EventCategory]bool)

	for _, batch := range batches {
		for _, event := range batch.Events {
			eventTypesSeen[event.EventType] = true
			categoriesSeen[event.Category] = true

			// Verify structured payloads can be parsed
			if len(event.Payload) > 0 {
				// Try to unmarshal as object first, then as string
				var payload map[string]interface{}
				err := json.Unmarshal(event.Payload, &payload)
				if err != nil {
					// If object unmarshal fails, try string (for backward compatibility events)
					var stringPayload string
					err2 := json.Unmarshal(event.Payload, &stringPayload)
					assert.NoError(t, err2, "Payload should be valid JSON (object or string)")
				}
			}
		}
	}

	// Check we saw the expected event types
	expectedTypes := []string{
		eventsconstants.SystemStartup,
		eventsconstants.StorageTransferStarted,
		eventsconstants.NetworkConnectionFailed,
		eventsconstants.SecurityAuthFailed,
		eventsconstants.ServiceStarted,
		eventsconstants.IdentityUserCreated,
		"generic.test.event",
	}

	for _, expectedType := range expectedTypes {
		assert.True(
			t,
			eventTypesSeen[expectedType],
			"Should have seen event type: %s",
			expectedType,
		)
	}

	// Check we saw the expected categories (including new ones)
	expectedCategories := []proto.EventCategory{
		proto.EventCategory_EVENT_CATEGORY_SYSTEM,
		proto.EventCategory_EVENT_CATEGORY_STORAGE,
		proto.EventCategory_EVENT_CATEGORY_NETWORK,
		proto.EventCategory_EVENT_CATEGORY_SECURITY,
		proto.EventCategory_EVENT_CATEGORY_SERVICE,
		proto.EventCategory_EVENT_CATEGORY_IDENTITY, // New category
	}

	for _, expectedCategory := range expectedCategories {
		assert.True(
			t,
			categoriesSeen[expectedCategory],
			"Should have seen category: %s",
			expectedCategory,
		)
	}

	// Test shutdown
	err = Shutdown(ctx)
	require.NoError(t, err, "Failed to shutdown updated event system")

	assert.False(
		t,
		IsInitialized(),
		"Updated event system should not be initialized after shutdown",
	)

	t.Log("Updated event system initialization test completed successfully")
}

// testDiskFlushFunctionalityUpdated tests disk flush behavior with structured events
func testDiskFlushFunctionalityUpdated(t *testing.T, l logger.Logger) {
	// Clean up any existing test events directory
	tempEventsDir := filepath.Join(os.TempDir(), "rodent-test-events-updated")
	defer os.RemoveAll(tempEventsDir)

	// Create a custom config for testing with low flush threshold
	testConfig := DefaultEventConfig()
	testConfig.FlushThreshold = 3 // Flush after 3 events for faster testing
	testConfig.BufferSize = 6     // Small buffer for testing

	// Create event buffer with test config
	buffer := NewEventBuffer(testConfig, l)

	// Override the events directory to our temp directory
	buffer.eventsDir = tempEventsDir

	// Test: Structured events flush to disk properly
	t.Run("StructuredEventsDiskFlush", func(t *testing.T) {
		// Ensure directory doesn't exist initially
		os.RemoveAll(tempEventsDir)

		// Create structured events with different payload types
		events := []*Event{
			{
				ID:        "flush-test-1",
				Type:      eventsconstants.StorageTransferCompleted,
				Level:     LevelInfo,
				Category:  CategoryStorage,
				Source:    "test",
				Timestamp: time.Now(),
				Payload: func() []byte {
					payload := &eventspb.StorageTransferPayload{
						TransferId:       "flush-transfer-1",
						Operation:        "send",
						Status:           "completed",
						DurationSeconds:  120,
						BytesTransferred: 1024 * 1024,
					}
					data, _ := json.Marshal(payload)
					return data
				}(),
				Metadata: map[string]string{
					eventsconstants.MetaComponent:  "zfs-transfer",
					eventsconstants.MetaAction:     "complete",
					eventsconstants.MetaTransferID: "flush-transfer-1",
				},
			},
			{
				ID:        "flush-test-2",
				Type:      eventsconstants.IdentityUserCreated,
				Level:     LevelInfo,
				Category:  CategoryIdentity,
				Source:    "test",
				Timestamp: time.Now(),
				Payload: func() []byte {
					payload := &eventspb.IdentityUserPayload{
						Username:    "flushuser",
						DisplayName: "Flush Test User",
						Email:       "flush@test.local",
						Enabled:     true,
						Domain:      "test.local",
					}
					data, _ := json.Marshal(payload)
					return data
				}(),
				Metadata: map[string]string{
					eventsconstants.MetaComponent: "ad-manager",
					eventsconstants.MetaAction:    "create",
					eventsconstants.MetaUser:      "flushuser",
					eventsconstants.MetaDomain:    "test.local",
				},
			},
		}

		// Add events - need to add enough to trigger flush
		// With FlushThreshold=3, we need to add 3 events to trigger flush
		for _, event := range events {
			err := buffer.Add(event)
			assert.NoError(t, err)
		}

		// Add the trigger event (3rd event) to reach the flush threshold
		triggerEvent := &Event{
			ID:        "flush-trigger",
			Type:      eventsconstants.SystemStartup,
			Level:     LevelInfo,
			Category:  CategorySystem,
			Source:    "test",
			Timestamp: time.Now(),
			Payload: func() []byte {
				payload := &eventspb.SystemStartupPayload{
					Hostname:        "flush-test-host",
					BootTimeSeconds: time.Now().Unix(),
				}
				data, _ := json.Marshal(payload)
				return data
			}(),
			Metadata: map[string]string{
				eventsconstants.MetaComponent: "system",
				eventsconstants.MetaAction:    "startup",
			},
		}

		err := buffer.Add(triggerEvent)
		assert.NoError(t, err)

		// Now we have 3 events in buffer. Add one more to trigger flush.
		finalEvent := &Event{
			ID:        "flush-final",
			Type:      eventsconstants.ServiceStarted,
			Level:     LevelInfo,
			Category:  CategoryService,
			Source:    "test",
			Timestamp: time.Now(),
			Payload: func() []byte {
				payload := &eventspb.ServiceStatusPayload{
					ServiceName: "test-service",
					Status:      "running",
					Pid:         12345,
				}
				data, _ := json.Marshal(payload)
				return data
			}(),
			Metadata: map[string]string{
				eventsconstants.MetaComponent: "service-manager",
				eventsconstants.MetaAction:    "start",
			},
		}

		err = buffer.Add(finalEvent)
		assert.NoError(t, err)

		// Directory should exist after flush
		_, err = os.Stat(tempEventsDir)
		assert.NoError(t, err, "Events directory should be created during flush")

		// Verify a file was created
		files, err := os.ReadDir(tempEventsDir)
		assert.NoError(t, err)
		assert.Len(t, files, 1, "Should have one flush file")

		// Safety check for index access to prevent panic
		if len(files) == 0 {
			t.Fatal("No files found in events directory")
			return
		}

		// Verify the file contains properly structured events
		filePath := filepath.Join(tempEventsDir, files[0].Name())
		fileContent, err := os.ReadFile(filePath)
		assert.NoError(t, err)

		// Parse as proto events array
		var protoEvents []*proto.Event
		err = json.Unmarshal(fileContent, &protoEvents)
		assert.NoError(t, err, "File should contain valid JSON array of proto events")

		// Should have flushed exactly FlushThreshold events
		assert.Len(
			t,
			protoEvents,
			testConfig.FlushThreshold,
			"Should have flushed exactly %d events",
			testConfig.FlushThreshold,
		)

		// Buffer should have 1 event remaining (the final event that triggered the flush)
		assert.Equal(t, 1, buffer.Size(), "Buffer should have 1 event after flush")

		// Verify structured payloads in flushed events
		foundTransfer := false
		foundIdentity := false
		foundSystem := false

		for _, event := range protoEvents {
			switch event.EventType {
			case eventsconstants.StorageTransferCompleted:
				foundTransfer = true
				var payload eventspb.StorageTransferPayload
				err := json.Unmarshal(event.Payload, &payload)
				assert.NoError(t, err, "Should be able to parse storage transfer payload")
				assert.Equal(t, "flush-transfer-1", payload.TransferId)
			case eventsconstants.IdentityUserCreated:
				foundIdentity = true
				var payload eventspb.IdentityUserPayload
				err := json.Unmarshal(event.Payload, &payload)
				assert.NoError(t, err, "Should be able to parse identity user payload")
				assert.Equal(t, "flushuser", payload.Username)
			case eventsconstants.SystemStartup:
				foundSystem = true
				var payload eventspb.SystemStartupPayload
				err := json.Unmarshal(event.Payload, &payload)
				assert.NoError(t, err, "Should be able to parse system startup payload")
				assert.Equal(t, "flush-test-host", payload.Hostname)
			}
		}

		assert.True(t, foundTransfer, "Should have flushed storage transfer event")
		assert.True(t, foundIdentity, "Should have flushed identity user event")
		assert.True(t, foundSystem, "Should have flushed system startup event")

		t.Logf(
			"Successfully flushed %d structured events to disk: %s",
			len(protoEvents),
			files[0].Name(),
		)
	})
}
