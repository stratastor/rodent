// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// EventBuffer manages the in-memory event buffer with disk spillover
type EventBuffer struct {
	events   []*Event  // Dynamic slice with pre-allocated capacity
	mu       sync.RWMutex
	config   *EventConfig
	logger   logger.Logger
	eventsDir string
}

// NewEventBuffer creates a new event buffer
func NewEventBuffer(cfg *EventConfig, l logger.Logger) *EventBuffer {
	return &EventBuffer{
		events:    make([]*Event, 0, cfg.BufferSize), // Pre-allocate capacity
		config:    cfg,
		logger:    l,
		eventsDir: config.GetEventsDir(),
	}
}

// Add adds an event to the buffer
func (eb *EventBuffer) Add(event *Event) error {
	// Apply filtering
	if !eb.config.ShouldProcess(event) {
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
	
	eb.logger.Debug("Event added to buffer", 
		"event_id", event.ID,
		"event_type", event.Type, 
		"buffer_size", len(eb.events))
	
	return nil
}

// GetBatch gets up to batchSize events from the buffer
// Returns events and whether there are more events available
func (eb *EventBuffer) GetBatch(batchSize int) ([]*Event, bool) {
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
	batch := make([]*Event, end)
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

// GetAll gets all events from the buffer (for shutdown)
func (eb *EventBuffer) GetAll() []*Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if len(eb.events) == 0 {
		return nil
	}

	// Copy all events
	all := make([]*Event, len(eb.events))
	copy(all, eb.events)

	// Clear buffer
	eb.events = eb.events[:0] // Reset slice but keep capacity

	eb.logger.Debug("Retrieved all events from buffer", "count", len(all))
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

	// Convert to proto format for consistent schema
	protoEvents := make([]*proto.Event, len(eb.events))
	for i, event := range eb.events {
		protoEvents[i] = event.ToProtoEvent()
	}

	// Generate UUID7 filename for natural time ordering
	filename := common.UUID7() + ".json"
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

	// Write proto events as JSON (same schema as gRPC)
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty print for debugging ease
	
	if err := encoder.Encode(protoEvents); err != nil {
		return fmt.Errorf("failed to encode events to JSON: %w", err)
	}

	eb.logger.Info("Flushed events to disk", 
		"count", len(eb.events),
		"file", filename)

	// Clear the buffer but keep capacity
	eb.events = eb.events[:0]
	
	return nil
}

// ShouldProcess determines if an event should be processed based on filters
func (c *EventConfig) ShouldProcess(event *Event) bool {
	// Check level filter using slices.Contains
	if !slices.Contains(c.EnabledLevels, event.Level) {
		return false
	}
	
	// Check category filter using slices.Contains
	return slices.Contains(c.EnabledCategories, event.Category)
}