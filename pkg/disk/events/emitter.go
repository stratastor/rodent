// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"time"

	"github.com/google/uuid"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/events"
	"github.com/stratastor/rodent/pkg/disk/types"
	eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
)

// Emitter emits disk-related events
type Emitter struct {
	logger   logger.Logger
	eventBus *events.EventBus
}

// NewEmitter creates a new disk event emitter
func NewEmitter(l logger.Logger, eventBus *events.EventBus) *Emitter {
	return &Emitter{
		logger:   l,
		eventBus: eventBus,
	}
}

// EmitDiskDiscovered emits a disk discovery event
func (e *Emitter) EmitDiskDiscovered(disk *types.PhysicalDisk) {
	var temperature int32
	var powerOnHours uint64
	if disk.SMARTInfo != nil {
		temperature = int32(disk.SMARTInfo.Temperature)
		powerOnHours = disk.SMARTInfo.PowerOnHours
	}

	payload := &eventspb.StorageDiskPayload{
		DeviceId:      disk.DeviceID,
		DevicePath:    disk.DevicePath,
		Serial:        disk.Serial,
		Model:         disk.Model,
		Vendor:        disk.Vendor,
		SizeBytes:     int64(disk.SizeBytes),
		State:         string(disk.State),
		Health:        string(disk.Health),
		Temperature:   temperature,
		PowerOnHours:  powerOnHours,
		InterfaceType: string(disk.Interface),
		DeviceType:    string(disk.Type),
		Operation:     eventspb.StorageDiskPayload_STORAGE_DISK_OPERATION_DISCOVERED,
	}

	e.emitDiskEvent(eventspb.EventLevel_EVENT_LEVEL_INFO, payload, map[string]string{
		"device_id":   disk.DeviceID,
		"device_path": disk.DevicePath,
		"model":       disk.Model,
	})
}

// EmitDiskHealthChanged emits a disk health change event
func (e *Emitter) EmitDiskHealthChanged(disk *types.PhysicalDisk, oldHealth, newHealth types.HealthStatus) {
	level := eventspb.EventLevel_EVENT_LEVEL_INFO
	switch newHealth {
	case types.HealthCritical:
		level = eventspb.EventLevel_EVENT_LEVEL_CRITICAL
	case types.HealthWarning:
		level = eventspb.EventLevel_EVENT_LEVEL_WARN
	}

	var temperature int32
	var powerOnHours uint64
	if disk.SMARTInfo != nil {
		temperature = int32(disk.SMARTInfo.Temperature)
		powerOnHours = disk.SMARTInfo.PowerOnHours
	}

	payload := &eventspb.StorageDiskPayload{
		DeviceId:      disk.DeviceID,
		DevicePath:    disk.DevicePath,
		Serial:        disk.Serial,
		Model:         disk.Model,
		State:         string(disk.State),
		Health:        string(newHealth),
		Temperature:   temperature,
		PowerOnHours:  powerOnHours,
		Operation:     eventspb.StorageDiskPayload_STORAGE_DISK_OPERATION_HEALTH_CHANGED,
	}

	e.emitDiskEvent(level, payload, map[string]string{
		"device_id":  disk.DeviceID,
		"old_health": string(oldHealth),
		"new_health": string(newHealth),
	})
}

// EmitDiskStateChanged emits a disk state change event
func (e *Emitter) EmitDiskStateChanged(disk *types.PhysicalDisk, oldState, newState types.DiskState) {
	level := eventspb.EventLevel_EVENT_LEVEL_INFO
	switch newState {
	case types.DiskStateFaulted:
		level = eventspb.EventLevel_EVENT_LEVEL_ERROR
	case types.DiskStateDegraded:
		level = eventspb.EventLevel_EVENT_LEVEL_WARN
	}

	payload := &eventspb.StorageDiskPayload{
		DeviceId:   disk.DeviceID,
		DevicePath: disk.DevicePath,
		Serial:     disk.Serial,
		Model:      disk.Model,
		State:      string(newState),
		Health:     string(disk.Health),
		Operation:  eventspb.StorageDiskPayload_STORAGE_DISK_OPERATION_STATE_CHANGED,
	}

	e.emitDiskEvent(level, payload, map[string]string{
		"device_id": disk.DeviceID,
		"old_state": string(oldState),
		"new_state": string(newState),
	})
}

// EmitDiskRemoved emits a disk removal event
func (e *Emitter) EmitDiskRemoved(disk *types.PhysicalDisk) {
	payload := &eventspb.StorageDiskPayload{
		DeviceId:   disk.DeviceID,
		DevicePath: disk.DevicePath,
		Serial:     disk.Serial,
		Model:      disk.Model,
		State:      string(disk.State),
		Operation:  eventspb.StorageDiskPayload_STORAGE_DISK_OPERATION_REMOVED,
	}

	e.emitDiskEvent(eventspb.EventLevel_EVENT_LEVEL_WARN, payload, map[string]string{
		"device_id":   disk.DeviceID,
		"device_path": disk.DevicePath,
	})
}

// EmitProbeStarted emits a probe start event
func (e *Emitter) EmitProbeStarted(execution *types.ProbeExecution, devicePath string) {
	payload := &eventspb.StorageDiskProbePayload{
		ProbeId:         execution.ID,
		DeviceId:        execution.DeviceID,
		DevicePath:      devicePath,
		ProbeType:       string(execution.Type),
		Status:          string(execution.Status),
		PercentComplete: int32(execution.PercentComplete),
		Operation:       eventspb.StorageDiskProbePayload_STORAGE_DISK_PROBE_OPERATION_STARTED,
	}

	e.emitProbeEvent(eventspb.EventLevel_EVENT_LEVEL_DEBUG, payload, map[string]string{
		"probe_id":   execution.ID,
		"device_id":  execution.DeviceID,
		"probe_type": string(execution.Type),
	})
}

// EmitProbeCompleted emits a probe completion event
func (e *Emitter) EmitProbeCompleted(execution *types.ProbeExecution, devicePath string) {
	level := eventspb.EventLevel_EVENT_LEVEL_INFO
	if execution.Status == types.ProbeStatusFailed {
		level = eventspb.EventLevel_EVENT_LEVEL_ERROR
	}

	var duration int64
	if execution.StartedAt != nil && execution.CompletedAt != nil {
		duration = int64(execution.CompletedAt.Sub(*execution.StartedAt).Seconds())
	}

	result := string(execution.Result)

	payload := &eventspb.StorageDiskProbePayload{
		ProbeId:         execution.ID,
		DeviceId:        execution.DeviceID,
		DevicePath:      devicePath,
		ProbeType:       string(execution.Type),
		Status:          string(execution.Status),
		PercentComplete: 100,
		Result:          result,
		DurationSeconds: duration,
		ErrorMessage:    execution.ErrorMessage,
		Operation:       eventspb.StorageDiskProbePayload_STORAGE_DISK_PROBE_OPERATION_COMPLETED,
	}

	e.emitProbeEvent(level, payload, map[string]string{
		"probe_id":   execution.ID,
		"device_id":  execution.DeviceID,
		"probe_type": string(execution.Type),
		"status":     string(execution.Status),
	})
}

// EmitProbeProgress emits a probe progress event
func (e *Emitter) EmitProbeProgress(execution *types.ProbeExecution, devicePath string) {
	payload := &eventspb.StorageDiskProbePayload{
		ProbeId:         execution.ID,
		DeviceId:        execution.DeviceID,
		DevicePath:      devicePath,
		ProbeType:       string(execution.Type),
		Status:          string(execution.Status),
		PercentComplete: int32(execution.PercentComplete),
		Operation:       eventspb.StorageDiskProbePayload_STORAGE_DISK_PROBE_OPERATION_PROGRESS,
	}

	e.emitProbeEvent(eventspb.EventLevel_EVENT_LEVEL_DEBUG, payload, map[string]string{
		"probe_id":  execution.ID,
		"device_id": execution.DeviceID,
	})
}

// EmitProbeConflict emits a probe conflict event
func (e *Emitter) EmitProbeConflict(execution *types.ProbeExecution, devicePath, conflictReason string) {
	payload := &eventspb.StorageDiskProbePayload{
		ProbeId:        execution.ID,
		DeviceId:       execution.DeviceID,
		DevicePath:     devicePath,
		ProbeType:      string(execution.Type),
		Status:         string(types.ProbeStatusScheduled),
		ConflictReason: conflictReason,
		Operation:      eventspb.StorageDiskProbePayload_STORAGE_DISK_PROBE_OPERATION_CONFLICTED,
	}

	e.emitProbeEvent(eventspb.EventLevel_EVENT_LEVEL_WARN, payload, map[string]string{
		"probe_id":        execution.ID,
		"device_id":       execution.DeviceID,
		"conflict_reason": conflictReason,
	})
}

// safeEmit safely emits an event, checking if eventBus is initialized
func (e *Emitter) safeEmit(event *eventspb.Event) {
	if e.eventBus == nil {
		e.logger.Debug("event bus not initialized, event not emitted",
			"event_id", event.EventId,
			"category", event.Category.String(),
			"source", event.Source)
		return
	}

	e.eventBus.EmitStructuredEvent(event)
}

// emitDiskEvent emits a disk event
func (e *Emitter) emitDiskEvent(
	level eventspb.EventLevel,
	payload *eventspb.StorageDiskPayload,
	metadata map[string]string,
) {
	event := &eventspb.Event{
		EventId:   uuid.New().String(),
		Level:     level,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_STORAGE,
		Source:    "rodent.disk-manager",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_StorageEvent{
			StorageEvent: &eventspb.StorageEvent{
				EventType: &eventspb.StorageEvent_DiskEvent{
					DiskEvent: payload,
				},
			},
		},
	}

	e.safeEmit(event)
	e.logger.Debug("disk event emitted",
		"event_id", event.EventId,
		"operation", payload.Operation.String(),
		"device_id", payload.DeviceId,
		"level", level.String())
}

// emitProbeEvent emits a probe event
func (e *Emitter) emitProbeEvent(
	level eventspb.EventLevel,
	payload *eventspb.StorageDiskProbePayload,
	metadata map[string]string,
) {
	event := &eventspb.Event{
		EventId:   uuid.New().String(),
		Level:     level,
		Category:  eventspb.EventCategory_EVENT_CATEGORY_STORAGE,
		Source:    "rodent.disk-manager.probe",
		Timestamp: time.Now().UnixMilli(),
		Metadata:  metadata,
		EventPayload: &eventspb.Event_StorageEvent{
			StorageEvent: &eventspb.StorageEvent{
				EventType: &eventspb.StorageEvent_DiskProbeEvent{
					DiskProbeEvent: payload,
				},
			},
		},
	}

	e.safeEmit(event)
	e.logger.Debug("probe event emitted",
		"event_id", event.EventId,
		"operation", payload.Operation.String(),
		"probe_id", payload.ProbeId,
		"level", level.String())
}
