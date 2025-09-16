// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	eventsconstants "github.com/stratastor/toggle-rodent-proto/go/events"
	eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
)

// Typed event emission functions with structured payloads

// System Events
func EmitSystemStartup(payload *eventspb.SystemStartupPayload, metadata map[string]string) {
	emitTypedEvent(
		eventsconstants.SystemStartup,
		LevelInfo,
		CategorySystem,
		"system",
		payload,
		metadata,
	)
}

func EmitSystemShutdown(metadata map[string]string) {
	emitTypedEvent(
		eventsconstants.SystemShutdown,
		LevelInfo,
		CategorySystem,
		"system",
		nil,
		metadata,
	)
}

func EmitSystemConfigChange(
	payload *eventspb.SystemConfigChangePayload,
	metadata map[string]string,
) {
	emitTypedEvent(
		eventsconstants.SystemConfigChanged,
		LevelInfo,
		CategorySystem,
		"system",
		payload,
		metadata,
	)
}

func EmitSystemUser(
	eventType string,
	level EventLevel,
	payload *eventspb.SystemUserPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategorySystem, "system-user-manager", payload, metadata)
}

// Storage Events
func EmitStoragePool(
	eventType string,
	level EventLevel,
	payload *eventspb.StoragePoolPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryStorage, "zfs-pool-manager", payload, metadata)
}

func EmitStorageDataset(
	eventType string,
	level EventLevel,
	payload *eventspb.StorageDatasetPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryStorage, "zfs-dataset-manager", payload, metadata)
}

func EmitStorageSnapshot(
	eventType string,
	level EventLevel,
	payload *eventspb.StorageSnapshotPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryStorage, "zfs-snapshot-manager", payload, metadata)
}

func EmitStorageTransfer(
	eventType string,
	level EventLevel,
	payload *eventspb.StorageTransferPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryStorage, "zfs-transfer-manager", payload, metadata)
}

// Network Events
func EmitNetworkInterface(
	eventType string,
	level EventLevel,
	payload *eventspb.NetworkInterfacePayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryNetwork, "netmage", payload, metadata)
}

func EmitNetworkConnection(
	eventType string,
	level EventLevel,
	payload *eventspb.NetworkConnectionPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryNetwork, "netmage", payload, metadata)
}

// Security Events
func EmitSecurityAuth(
	eventType string,
	level EventLevel,
	payload *eventspb.SecurityAuthPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategorySecurity, "auth-manager", payload, metadata)
}

func EmitSecurityKey(
	eventType string,
	level EventLevel,
	payload *eventspb.SecurityKeyPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategorySecurity, "key-manager", payload, metadata)
}

func EmitSecurityCertificate(
	eventType string,
	level EventLevel,
	payload *eventspb.SecurityCertificatePayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategorySecurity, "cert-manager", payload, metadata)
}

// Service Events
func EmitServiceStatus(
	eventType string,
	level EventLevel,
	payload *eventspb.ServiceStatusPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryService, "service-manager", payload, metadata)
}

// Identity Events (AD/LDAP)
func EmitIdentityUser(
	eventType string,
	level EventLevel,
	payload *eventspb.IdentityUserPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryIdentity, "ad-manager", payload, metadata)
}

func EmitIdentityGroup(
	eventType string,
	level EventLevel,
	payload *eventspb.IdentityGroupPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryIdentity, "ad-manager", payload, metadata)
}

func EmitIdentityComputer(
	eventType string,
	level EventLevel,
	payload *eventspb.IdentityComputerPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryIdentity, "ad-manager", payload, metadata)
}

// Access Control Events
func EmitAccessACL(
	eventType string,
	level EventLevel,
	payload *eventspb.AccessACLPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryAccess, "facl-manager", payload, metadata)
}

func EmitAccessPermission(
	eventType string,
	level EventLevel,
	payload *eventspb.AccessPermissionPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryAccess, "facl-manager", payload, metadata)
}

// File Sharing Events
func EmitSharingShare(
	eventType string,
	level EventLevel,
	payload *eventspb.SharingSharePayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategorySharing, "share-manager", payload, metadata)
}

func EmitSharingConnection(
	eventType string,
	level EventLevel,
	payload *eventspb.SharingConnectionPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategorySharing, "share-manager", payload, metadata)
}

func EmitSharingFileAccess(
	eventType string,
	level EventLevel,
	payload *eventspb.SharingFileAccessPayload,
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategorySharing, "share-manager", payload, metadata)
}

// Helper function for typed event emission
func emitTypedEvent(
	eventType string,
	level EventLevel,
	category EventCategory,
	source string,
	payload interface{},
	metadata map[string]string,
) {
	// Ensure metadata is not nil
	if metadata == nil {
		metadata = make(map[string]string)
	}

	// Pass payload directly to emitEvent to avoid double JSON marshaling
	emitEvent(eventType, level, category, source, payload, metadata)
}

// Convenience functions for backward compatibility and quick migration

// EmitSystemEventTyped emits a system event with structured payload
func EmitSystemEventTyped(
	eventType string,
	level EventLevel,
	payload interface{},
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategorySystem, "system", payload, metadata)
}

// EmitStorageEventTyped emits a storage event with structured payload
func EmitStorageEventTyped(
	eventType string,
	level EventLevel,
	source string,
	payload interface{},
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryStorage, source, payload, metadata)
}

// EmitNetworkEventTyped emits a network event with structured payload
func EmitNetworkEventTyped(
	eventType string,
	level EventLevel,
	source string,
	payload interface{},
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryNetwork, source, payload, metadata)
}

// EmitSecurityEventTyped emits a security event with structured payload
func EmitSecurityEventTyped(
	eventType string,
	level EventLevel,
	source string,
	payload interface{},
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategorySecurity, source, payload, metadata)
}

// EmitServiceEventTyped emits a service event with structured payload
func EmitServiceEventTyped(
	eventType string,
	level EventLevel,
	source string,
	payload interface{},
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryService, source, payload, metadata)
}

// EmitIdentityEventTyped emits an identity event with structured payload
func EmitIdentityEventTyped(
	eventType string,
	level EventLevel,
	source string,
	payload interface{},
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryIdentity, source, payload, metadata)
}

// EmitAccessEventTyped emits an access control event with structured payload
func EmitAccessEventTyped(
	eventType string,
	level EventLevel,
	source string,
	payload interface{},
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategoryAccess, source, payload, metadata)
}

// EmitSharingEventTyped emits a file sharing event with structured payload
func EmitSharingEventTyped(
	eventType string,
	level EventLevel,
	source string,
	payload interface{},
	metadata map[string]string,
) {
	emitTypedEvent(eventType, level, CategorySharing, source, payload, metadata)
}
