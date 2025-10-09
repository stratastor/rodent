// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/internal/constants"
	"github.com/stratastor/rodent/internal/toggle/client"
	eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
)

// TYPE-SAFE STRUCTURED EVENT EMISSION FUNCTIONS

// System Events

// EmitSystemStartup emits a system startup event with auto-populated fields
// startupType: "initial_startup", "restart", "reconnect"
func EmitSystemStartup(startupType string) {
	cfg := config.GetConfig()
	jwt := cfg.Toggle.JWT

	// Extract rodent_id and org_id from JWT
	rodentID, _ := client.ExtractRodentIDFromJWT(jwt)
	orgID, _ := client.ExtractSubFromJWT(jwt)

	// Get hostname
	hostname, _ := os.Hostname()

	payload := &eventspb.SystemStartupPayload{
		RodentId:       rodentID,
		OrganizationId: orgID,
		StartupTime:    time.Now().Format(time.RFC3339),
		StartupType:    startupType,
		Services:       []string{"rodent-controller"},
		Version:        constants.RodentVersion,
		SystemInfo: map[string]string{
			"os":       runtime.GOOS,
			"arch":     runtime.GOARCH,
			"hostname": hostname,
		},
	}

	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     eventspb.EventLevel_EVENT_LEVEL_INFO,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_SYSTEM,
		Source:    "rodent.system",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  map[string]string{},
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

// EmitSystemRegistration emits a registration event with auto-populated fields
// registrationType: "new_registration", "renewal", "reconnect"
func EmitSystemRegistration(
	registrationType string,
	toggleDomain string,
	certExpires time.Time,
	isPrivateNetwork bool,
) {
	cfg := config.GetConfig()
	jwt := cfg.Toggle.JWT

	// Extract rodent_id and org_id from JWT
	rodentID, _ := client.ExtractRodentIDFromJWT(jwt)
	orgID, _ := client.ExtractSubFromJWT(jwt)

	certExpiresStr := ""
	if !certExpires.IsZero() {
		certExpiresStr = certExpires.Format(time.RFC3339)
	}

	payload := &eventspb.SystemRegistrationPayload{
		RodentId:           rodentID,
		OrganizationId:     orgID,
		RegisteredAt:       time.Now().Format(time.RFC3339),
		ToggleDomain:       toggleDomain,
		CertificateExpires: certExpiresStr,
		IsPrivateNetwork:   isPrivateNetwork,
		RegistrationType:   registrationType,
	}

	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     eventspb.EventLevel_EVENT_LEVEL_INFO,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_SYSTEM,
		Source:    "rodent.system",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  map[string]string{},
		EventPayload: &eventspb.Event_SystemEvent{
			SystemEvent: &eventspb.SystemEvent{
				EventType: &eventspb.SystemEvent_Registration{
					Registration: payload,
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

func EmitServiceConfigChange(
	serviceName string,
	configPath string,
	status string,
	metadata map[string]string,
) {
	payload := &eventspb.SystemConfigChangePayload{
		ConfigSection: serviceName,
		ChangedKeys:   []string{configPath},
		Operation:     eventspb.SystemConfigChangePayload_SYSTEM_CONFIG_OPERATION_UPDATED,
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["service_name"] = serviceName
	metadata["config_path"] = configPath
	metadata["status"] = status

	EmitSystemConfigChange(payload, metadata)
}

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

// Deprecated: EmitStorageTransfer is replaced by EmitDataTransfer
// Use EmitDataTransfer with DataTransferTransferPayload for comprehensive transfer events

// Data Transfer Events

func EmitDataTransfer(
	level eventspb.EventLevel,
	payload *eventspb.DataTransferTransferPayload,
	metadata map[string]string,
) {
	event := &eventspb.Event{
		EventId:   generateEventID(),
		Level:     level,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_DATA_TRANSFER,
		Source:    "rodent.zfs-transfer-manager",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_DataTransferEvent{
			DataTransferEvent: &eventspb.DataTransferEvent{
				EventType: &eventspb.DataTransferEvent_TransferEvent{
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
	if GlobalEventBus != nil {
		GlobalEventBus.EmitStructuredEvent(event)
		common.Log.Debug("Event emitted successfully",
			"event_id", event.EventId,
			"category", event.Category.String(),
			"source", event.Source,
			"level", event.Level.String())
	} else {
		// Log warning if event bus is not initialized - helps diagnose missing events
		// This can happen if events are emitted before Initialize() is called
		common.Log.Debug("Event dropped - globalEventBus not initialized",
			"category", event.Category.String(),
			"source", event.Source,
			"level", event.Level.String())
	}
}
