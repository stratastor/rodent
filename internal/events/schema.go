// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"time"

	"github.com/google/uuid"
	eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
)

// TYPE-SAFE STRUCTURED EVENT EMISSION FUNCTIONS

// System Events

func EmitSystemStartup(payload *eventspb.SystemStartupPayload, metadata map[string]string) {
	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     eventspb.EventLevel_EVENT_LEVEL_INFO,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_SYSTEM,
		Source:    "system",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_SystemEvent{
			SystemEvent: &eventspb.SystemEvent{
				EventType: &eventspb.SystemEvent_Startup{
					Startup: payload,
				},
			},
		},
	}
	emitStructuredEvent(event)
}

func EmitSystemShutdown(payload *eventspb.SystemShutdownPayload, metadata map[string]string) {
	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     eventspb.EventLevel_EVENT_LEVEL_INFO,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_SYSTEM,
		Source:    "system",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_SystemEvent{
			SystemEvent: &eventspb.SystemEvent{
				EventType: &eventspb.SystemEvent_Shutdown{
					Shutdown: payload,
				},
			},
		},
	}
	emitStructuredEvent(event)
}

func EmitSystemConfigChange(
	payload *eventspb.SystemConfigChangePayload,
	metadata map[string]string,
) {
	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     eventspb.EventLevel_EVENT_LEVEL_INFO,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_SYSTEM,
		Source:    "system",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_SystemEvent{
			SystemEvent: &eventspb.SystemEvent{
				EventType: &eventspb.SystemEvent_ConfigChanged{
					ConfigChanged: payload,
				},
			},
		},
	}
	emitStructuredEvent(event)
}

func EmitSystemUser(
	level eventspb.EventLevel,
	payload *eventspb.SystemUserPayload,
	metadata map[string]string,
) {
	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     level,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_SYSTEM,
		Source:    "system-user-manager",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_SystemEvent{
			SystemEvent: &eventspb.SystemEvent{
				EventType: &eventspb.SystemEvent_UserEvent{
					UserEvent: payload,
				},
			},
		},
	}
	emitStructuredEvent(event)
}

// Service Events

func EmitServiceStatus(
	level eventspb.EventLevel,
	payload *eventspb.ServiceStatusPayload,
	metadata map[string]string,
) {
	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     level,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_SERVICE,
		Source:    "service-manager",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_ServiceEvent{
			ServiceEvent: &eventspb.ServiceEvent{
				EventType: &eventspb.ServiceEvent_StatusEvent{
					StatusEvent: payload,
				},
			},
		},
	}
	emitStructuredEvent(event)
}

// Storage Events

func EmitStoragePool(
	level eventspb.EventLevel,
	payload *eventspb.StoragePoolPayload,
	metadata map[string]string,
) {
	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     level,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_STORAGE,
		Source:    "zfs-pool-manager",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_StorageEvent{
			StorageEvent: &eventspb.StorageEvent{
				EventType: &eventspb.StorageEvent_PoolEvent{
					PoolEvent: payload,
				},
			},
		},
	}
	emitStructuredEvent(event)
}

func EmitStorageDataset(
	level eventspb.EventLevel,
	payload *eventspb.StorageDatasetPayload,
	metadata map[string]string,
) {
	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     level,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_STORAGE,
		Source:    "zfs-dataset-manager",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_StorageEvent{
			StorageEvent: &eventspb.StorageEvent{
				EventType: &eventspb.StorageEvent_DatasetEvent{
					DatasetEvent: payload,
				},
			},
		},
	}
	emitStructuredEvent(event)
}

func EmitStorageTransfer(
	level eventspb.EventLevel,
	payload *eventspb.StorageTransferPayload,
	metadata map[string]string,
) {
	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     level,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_STORAGE,
		Source:    "zfs-transfer-manager",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_StorageEvent{
			StorageEvent: &eventspb.StorageEvent{
				EventType: &eventspb.StorageEvent_TransferEvent{
					TransferEvent: payload,
				},
			},
		},
	}
	emitStructuredEvent(event)
}

// Add more emission functions for other categories as needed...

// HELPER FUNCTIONS

// generateEventID creates a UUID for event identification
func generateEventID() string {
	return uuid.New().String()
}

// emitStructuredEvent sends the structured event through the event system
func emitStructuredEvent(event *eventspb.Event) {
	// Ensure metadata is not nil
	if event.Metadata == nil {
		event.Metadata = make(map[string]string)
	}

	// Send directly to the global event bus
	if globalEventBus != nil {
		globalEventBus.EmitStructuredEvent(event)
	}
}
