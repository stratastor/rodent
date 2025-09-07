// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"fmt"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// EventClient handles sending event batches to Toggle via gRPC
type EventClient struct {
	grpcClient proto.RodentServiceClient
	config     *EventConfig
	logger     logger.Logger
}

// NewEventClient creates a new event client
func NewEventClient(grpcClient proto.RodentServiceClient, cfg *EventConfig, l logger.Logger) *EventClient {
	return &EventClient{
		grpcClient: grpcClient,
		config:     cfg,
		logger:     l,
	}
}

// SendBatch sends a batch of events to Toggle
func (ec *EventClient) SendBatch(ctx context.Context, events []*Event) error {
	if len(events) == 0 {
		return nil
	}

	// Convert to proto events
	protoEvents := make([]*proto.Event, len(events))
	for i, event := range events {
		protoEvents[i] = event.ToProtoEvent()
	}

	// Create batch
	batch := &proto.EventBatch{
		Events:         protoEvents,
		BatchTimestamp: time.Now().UnixMilli(),
		BatchId:        common.UUID7(),
	}

	// Send with retries
	return ec.sendWithRetry(ctx, batch)
}

// sendWithRetry sends the batch with exponential backoff retry
func (ec *EventClient) sendWithRetry(ctx context.Context, batch *proto.EventBatch) error {
	var lastErr error
	
	for attempt := 0; attempt < ec.config.MaxRetryAttempts; attempt++ {
		// Create timeout context for each attempt
		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		
		// Try to send
		resp, err := ec.grpcClient.SendEvents(attemptCtx, batch)
		cancel()
		
		if err == nil {
			// Check response
			if resp.Success {
				ec.logger.Debug("Successfully sent event batch",
					"batch_id", batch.BatchId,
					"event_count", len(batch.Events),
					"attempt", attempt+1)
				return nil
			} else {
				// Toggle received but had processing issues - not our problem
				ec.logger.Warn("Toggle reported processing issues but received batch",
					"batch_id", batch.BatchId,
					"message", resp.Message)
				return nil
			}
		}

		lastErr = err
		ec.logger.Warn("Failed to send event batch, will retry",
			"batch_id", batch.BatchId,
			"attempt", attempt+1,
			"max_attempts", ec.config.MaxRetryAttempts,
			"error", err)

		// Wait before retry (exponential backoff)
		if attempt < ec.config.MaxRetryAttempts-1 {
			backoff := ec.config.RetryBackoffBase * time.Duration(1<<attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("failed to send event batch after %d attempts: %w", 
		ec.config.MaxRetryAttempts, lastErr)
}