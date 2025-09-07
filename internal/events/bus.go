// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// EventBus coordinates event processing, buffering, and transmission
type EventBus struct {
	buffer     *EventBuffer
	client     *EventClient
	config     *EventConfig
	logger     logger.Logger
	
	// Processing channels
	eventChan   chan *Event
	stopChan    chan struct{}
	shutdownChan chan struct{}
	
	// Synchronization
	wg          sync.WaitGroup
	mu          sync.RWMutex
	isShutdown  bool
}

// NewEventBus creates a new event bus
func NewEventBus(grpcClient proto.RodentServiceClient, jwt string, cfg *EventConfig, l logger.Logger) *EventBus {
	return &EventBus{
		buffer:       NewEventBuffer(cfg, l),
		client:       NewEventClient(grpcClient, jwt, cfg, l),
		config:       cfg,
		logger:       l,
		eventChan:    make(chan *Event, 1000), // Buffer for async event processing
		stopChan:     make(chan struct{}),
		shutdownChan: make(chan struct{}),
	}
}

// Start starts the event bus processing
func (eb *EventBus) Start(ctx context.Context) error {
	eb.mu.Lock()
	if eb.isShutdown {
		eb.mu.Unlock()
		return nil
	}
	eb.mu.Unlock()

	// Start event processor
	eb.wg.Add(1)
	go eb.processEvents(ctx)

	// Start batch sender
	eb.wg.Add(1)
	go eb.processBatches(ctx)

	eb.logger.Info("Event bus started",
		"buffer_size", eb.config.BufferSize,
		"flush_threshold", eb.config.FlushThreshold,
		"batch_size", eb.config.BatchSize,
		"batch_timeout", eb.config.BatchTimeout)

	return nil
}

// Emit emits an event (non-blocking)
func (eb *EventBus) Emit(eventType string, level EventLevel, category EventCategory, source string, payload []byte, metadata map[string]string) {
	eb.mu.RLock()
	if eb.isShutdown {
		eb.mu.RUnlock()
		return
	}
	eb.mu.RUnlock()

	event := &Event{
		ID:        common.UUID7(),
		Type:      eventType,
		Level:     level,
		Category:  category,
		Source:    source,
		Timestamp: time.Now(),
		Payload:   payload,
		Metadata:  metadata,
	}

	select {
	case eb.eventChan <- event:
		// Event queued successfully
	default:
		// Channel full - log warning but don't block
		eb.logger.Warn("Event channel full, dropping event",
			"event_type", eventType,
			"event_id", event.ID)
	}
}

// processEvents processes incoming events and adds them to buffer
func (eb *EventBus) processEvents(ctx context.Context) {
	defer eb.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-eb.stopChan:
			return
		case event := <-eb.eventChan:
			if err := eb.buffer.Add(event); err != nil {
				eb.logger.Error("Failed to add event to buffer",
					"event_id", event.ID,
					"event_type", event.Type,
					"error", err)
			}
			
			// Check if buffer reached batch size - send immediately
			if eb.buffer.Size() >= eb.config.BatchSize {
				eb.sendBatchIfReady(ctx, false)
			}
		}
	}
}

// processBatches handles periodic batch sending based on timeout
func (eb *EventBus) processBatches(ctx context.Context) {
	defer eb.wg.Done()

	ticker := time.NewTicker(eb.config.BatchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-eb.stopChan:
			return
		case <-ticker.C:
			// Time-based batch sending - force send even if < BatchSize events
			eb.logger.Debug("Batch timeout ticker fired, sending batch", 
				"timeout", eb.config.BatchTimeout,
				"buffer_size", eb.buffer.Size())
			eb.sendBatchIfReady(ctx, true)
		}
	}
}

// sendBatchIfReady sends a batch if conditions are met
func (eb *EventBus) sendBatchIfReady(ctx context.Context, force bool) {
	bufferSize := eb.buffer.Size()
	
	if bufferSize == 0 {
		return // Nothing to send
	}
	
	// Only send partial batches when forced (timeout or shutdown)
	if !force && bufferSize < eb.config.BatchSize {
		return
	}

	// Get batch from buffer
	events, hasMore := eb.buffer.GetBatch(eb.config.BatchSize)
	if len(events) == 0 {
		return
	}

	// Send batch
	if err := eb.client.SendBatch(ctx, events); err != nil {
		eb.logger.Error("Failed to send event batch",
			"batch_size", len(events),
			"error", err)
		// TODO: Consider re-queuing failed events
	} else {
		eb.logger.Debug("Successfully sent event batch",
			"batch_size", len(events),
			"buffer_remaining", eb.buffer.Size())
	}

	// If there are more events and we have enough for another full batch, send immediately
	if hasMore && eb.buffer.Size() >= eb.config.BatchSize {
		eb.sendBatchIfReady(ctx, false)
	}
}

// Shutdown gracefully shuts down the event bus
func (eb *EventBus) Shutdown(ctx context.Context) error {
	eb.mu.Lock()
	if eb.isShutdown {
		eb.mu.Unlock()
		return nil
	}
	eb.isShutdown = true
	eb.mu.Unlock()

	eb.logger.Info("Shutting down event bus...")

	// Stop accepting new events
	close(eb.stopChan)

	// Process remaining events in channel
	for {
		select {
		case event := <-eb.eventChan:
			if err := eb.buffer.Add(event); err != nil {
				eb.logger.Error("Failed to add event to buffer during shutdown",
					"event_id", event.ID, "error", err)
			}
		default:
			goto processingDone
		}
	}

processingDone:
	// Send all remaining events in buffer
	eb.logger.Debug("Shutdown: processing remaining events in buffer", 
		"buffer_size", eb.buffer.Size())
	for {
		events, hasMore := eb.buffer.GetBatch(eb.config.BatchSize)
		if len(events) == 0 {
			break
		}

		eb.logger.Debug("Shutdown: sending batch", 
			"batch_size", len(events), "has_more", hasMore)
		if err := eb.client.SendBatch(ctx, events); err != nil {
			eb.logger.Error("Failed to send events during shutdown",
				"batch_size", len(events), "error", err)
			// Continue trying to send other batches
		}

		if !hasMore {
			break
		}
	}

	// Flush any remaining events to disk
	if err := eb.buffer.Flush(); err != nil {
		eb.logger.Error("Failed to flush remaining events to disk", "error", err)
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		eb.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		eb.logger.Info("Event bus shutdown complete")
	case <-ctx.Done():
		eb.logger.Warn("Event bus shutdown timed out")
		return ctx.Err()
	}

	return nil
}

// GetStats returns current event bus statistics
func (eb *EventBus) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"buffer_size":       eb.buffer.Size(),
		"max_buffer_size":   eb.config.BufferSize,
		"flush_threshold":   eb.config.FlushThreshold,
		"batch_size":        eb.config.BatchSize,
		"pending_events":    len(eb.eventChan),
		"max_pending":       cap(eb.eventChan),
		"is_shutdown":       eb.isShutdown,
	}
}