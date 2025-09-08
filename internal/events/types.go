// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"time"

	"github.com/stratastor/rodent/config"
	"github.com/stratastor/toggle-rodent-proto/proto"
)

// EventLevel maps to proto.EventLevel
type EventLevel int32

const (
	LevelUnspecified EventLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelCritical
)

// EventCategory maps to proto.EventCategory  
type EventCategory int32

const (
	CategoryUnspecified EventCategory = iota
	CategorySystem
	CategoryStorage
	CategoryNetwork
	CategorySecurity
	CategoryService
)

// Event represents an internal event before conversion to proto
type Event struct {
	ID        string
	Type      string
	Level     EventLevel
	Category  EventCategory
	Source    string
	Timestamp time.Time
	Payload   []byte
	Metadata  map[string]string
}

// ToProtoEvent converts internal Event to proto.Event
func (e *Event) ToProtoEvent() *proto.Event {
	return &proto.Event{
		EventId:   e.ID,
		EventType: e.Type,
		Level:     proto.EventLevel(e.Level),
		Category:  proto.EventCategory(e.Category),
		Source:    e.Source,
		Timestamp: e.Timestamp.UnixMilli(),
		Payload:   e.Payload,
		Metadata:  e.Metadata,
	}
}

// EventConfig represents event system configuration
type EventConfig struct {
	// Buffer configuration
	BufferSize     int `json:"buffer_size"`     // Max events in memory buffer (default: 20000)
	FlushThreshold int `json:"flush_threshold"` // Flush to disk threshold (default: 18000)
	
	// Batching configuration (memory → network)
	BatchSize    int           `json:"batch_size"`    // Max events per batch (default: 100)
	BatchTimeout time.Duration `json:"batch_timeout"` // Max time to wait for batch (default: 30s)
	
	// Filtering configuration (API configurable)
	EnabledLevels     []EventLevel    `json:"enabled_levels"`     // Levels to process
	EnabledCategories []EventCategory `json:"enabled_categories"` // Categories to process
	
	// Performance settings
	MaxFileSize      int64         `json:"max_file_size"`      // Max size per event log file (default: 10MB)
	MaxRetryAttempts int           `json:"max_retry_attempts"` // Max retry attempts for failed sends (default: 3)
	RetryBackoffBase time.Duration `json:"retry_backoff_base"` // Base backoff for retries (default: 1s)
}

// DefaultEventConfig returns default configuration
func DefaultEventConfig() *EventConfig {
	return &EventConfig{
		BufferSize:        20000,
		FlushThreshold:    18000,
		BatchSize:         100,
		BatchTimeout:      30 * time.Second,
		EnabledLevels:     []EventLevel{LevelInfo, LevelWarn, LevelError, LevelCritical},
		EnabledCategories: []EventCategory{CategorySystem, CategoryStorage, CategoryNetwork, CategorySecurity, CategoryService},
		MaxFileSize:       10 * 1024 * 1024, // 10MB
		MaxRetryAttempts:  3,
		RetryBackoffBase:  1 * time.Second,
	}
}

// GetEventConfig creates EventConfig from main config with defaults and profiles
func GetEventConfig() *EventConfig {
	cfg := config.GetConfig()
	eventConfig := DefaultEventConfig()
	
	// Apply profile presets first
	switch cfg.Events.Profile {
	case "default", "":
		// Keep the DefaultEventConfig() values - no changes needed
	case "high-throughput":
		eventConfig.BufferSize = 50000
		eventConfig.FlushThreshold = 45000
		eventConfig.BatchSize = 500
		eventConfig.BatchTimeout = 60 * time.Second
	case "low-latency":
		eventConfig.BufferSize = 5000
		eventConfig.FlushThreshold = 4000
		eventConfig.BatchSize = 50
		eventConfig.BatchTimeout = 5 * time.Second
	case "minimal":
		eventConfig.BufferSize = 2000
		eventConfig.FlushThreshold = 1800
		eventConfig.BatchSize = 25
		eventConfig.EnabledLevels = []EventLevel{LevelError, LevelCritical}
	}
	
	// Apply specific overrides after profile
	if cfg.Events.BufferSize != nil && *cfg.Events.BufferSize > 0 {
		eventConfig.BufferSize = *cfg.Events.BufferSize
	}
	if cfg.Events.FlushThreshold != nil && *cfg.Events.FlushThreshold > 0 {
		eventConfig.FlushThreshold = *cfg.Events.FlushThreshold
	}
	if cfg.Events.BatchSize != nil && *cfg.Events.BatchSize > 0 {
		eventConfig.BatchSize = *cfg.Events.BatchSize
	}
	if cfg.Events.BatchTimeout != nil && *cfg.Events.BatchTimeout > 0 {
		eventConfig.BatchTimeout = time.Duration(*cfg.Events.BatchTimeout) * time.Second
	}
	if cfg.Events.MaxFileSize != nil && *cfg.Events.MaxFileSize > 0 {
		eventConfig.MaxFileSize = *cfg.Events.MaxFileSize
	}
	
	return eventConfig
}

