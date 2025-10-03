// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/toggle-rodent-proto/proto"
	eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
	pbproto "google.golang.org/protobuf/proto"
)

// EventBuffer manages the in-memory event buffer with disk spillover
type EventBuffer struct {
	events   []*eventspb.Event  // Dynamic slice with pre-allocated capacity - structured events only
	mu       sync.RWMutex
	config   *EventConfig
	logger   logger.Logger
	eventsDir string
}

// NewEventBuffer creates a new event buffer
func NewEventBuffer(cfg *EventConfig, l logger.Logger) *EventBuffer {
	return &EventBuffer{
		events:    make([]*eventspb.Event, 0, cfg.BufferSize), // Pre-allocate capacity
		config:    cfg,
		logger:    l,
		eventsDir: config.GetEventsDir(),
	}
}

// AddStructured adds a structured event to the buffer
func (eb *EventBuffer) AddStructured(event *eventspb.Event) error {
	// Apply filtering
	if !eb.config.ShouldProcessStructured(event) {
		return nil // Event filtered out
	}

	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Check if we need to flush to disk first (O(1) operation)
	if len(eb.events) >= eb.config.FlushThreshold {
		if err := eb.flushToDiskLocked(); err != nil {
			eb.logger.Error("Failed to flush events to disk", "error", err)
			// Continue anyway - don't block event addition
		}
	}

	// Add event to buffer
	eb.events = append(eb.events, event)
	
	eb.logger.Debug("Structured event added to buffer",
		"event_id", event.EventId,
		"event_category", event.Category.String(),
		"buffer_size", len(eb.events))
	
	return nil
}

// GetBatchStructured gets up to batchSize structured events from the buffer
// Returns events and whether there are more events available
func (eb *EventBuffer) GetBatchStructured(batchSize int) ([]*eventspb.Event, bool) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if len(eb.events) == 0 {
		return nil, false
	}

	// Determine batch size
	end := batchSize
	if end > len(eb.events) {
		end = len(eb.events)
	}

	// Copy events for the batch
	batch := make([]*eventspb.Event, end)
	copy(batch, eb.events[:end])

	// Remove batched events from buffer
	eb.events = eb.events[end:]

	hasMore := len(eb.events) > 0
	
	eb.logger.Debug("Retrieved event batch", 
		"batch_size", len(batch),
		"remaining_events", len(eb.events),
		"has_more", hasMore)

	return batch, hasMore
}

// GetAllStructured gets all structured events from the buffer (for shutdown)
func (eb *EventBuffer) GetAllStructured() []*eventspb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if len(eb.events) == 0 {
		return nil
	}

	// Copy all events
	all := make([]*eventspb.Event, len(eb.events))
	copy(all, eb.events)

	// Clear buffer
	eb.events = eb.events[:0] // Reset slice but keep capacity

	eb.logger.Debug("Retrieved all structured events from buffer", "count", len(all))
	return all
}

// Size returns the current buffer size
func (eb *EventBuffer) Size() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.events)
}

// Flush flushes current buffer to disk and clears it
func (eb *EventBuffer) Flush() error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	return eb.flushToDiskLocked()
}

// flushToDiskLocked flushes events to disk using same schema as gRPC (caller must hold lock)
func (eb *EventBuffer) flushToDiskLocked() error {
	if len(eb.events) == 0 {
		return nil
	}

	// Events are already in protobuf format (eventspb.Event)
	// Create proto.EventBatch for disk storage
	eventBatch := &proto.EventBatch{
		Events:         eb.events,  // Our events are already eventspb.Event
		BatchTimestamp: time.Now().UnixMilli(),
		BatchId:        common.UUID7(),
	}

	// Generate UUID7 filename for natural time ordering
	filename := common.UUID7() + ".pb"
	filepath := filepath.Join(eb.eventsDir, filename)

	// Create the events directory if it doesn't exist
	if err := os.MkdirAll(eb.eventsDir, 0755); err != nil {
		return fmt.Errorf("failed to create events directory: %w", err)
	}

	// Open file for writing
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create event log file: %w", err)
	}
	defer file.Close()

	// Serialize EventBatch as protobuf binary
	binaryData, err := pbproto.Marshal(eventBatch)
	if err != nil {
		return fmt.Errorf("failed to marshal event batch to protobuf: %w", err)
	}

	if _, err := file.Write(binaryData); err != nil {
		return fmt.Errorf("failed to write protobuf data to disk: %w", err)
	}

	eb.logger.Info("Flushed structured events to disk as protobuf binary",
		"count", len(eb.events),
		"file", filename)

	// Clear the buffer but keep capacity
	eb.events = eb.events[:0]
	
	return nil
}

// ShouldProcessStructured determines if a structured event should be processed based on filters
func (c *EventConfig) ShouldProcessStructured(event *eventspb.Event) bool {
	// Check level filter
	if !slices.Contains(c.EnabledLevels, event.Level) {
		return false
	}

	// Check category filter
	return slices.Contains(c.EnabledCategories, event.Category)
}