// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"github.com/stratastor/rodent/pkg/errors"
)

// ============================================================================
// Metadata Operations
// ============================================================================

// SetDiskTags sets custom tags for a disk
func (m *Manager) SetDiskTags(deviceID string, tags map[string]string) error {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	disk, exists := m.deviceCache[deviceID]
	if !exists {
		return errors.New(errors.DiskNotFound, "device not found").
			WithMetadata("device_id", deviceID)
	}

	if disk.Tags == nil {
		disk.Tags = make(map[string]string)
	}

	// Merge tags
	for k, v := range tags {
		disk.Tags[k] = v
	}

	m.stateManager.SaveDebounced()
	m.logger.Info("disk tags updated", "device_id", deviceID, "tags", tags)

	return nil
}

// DeleteDiskTags deletes specific tags from a disk
func (m *Manager) DeleteDiskTags(deviceID string, tagKeys []string) error {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	disk, exists := m.deviceCache[deviceID]
	if !exists {
		return errors.New(errors.DiskNotFound, "device not found").
			WithMetadata("device_id", deviceID)
	}

	if disk.Tags != nil {
		for _, key := range tagKeys {
			delete(disk.Tags, key)
		}
	}

	m.stateManager.SaveDebounced()
	m.logger.Info("disk tags deleted", "device_id", deviceID, "keys", tagKeys)

	return nil
}

// SetDiskNotes sets notes for a disk
func (m *Manager) SetDiskNotes(deviceID string, notes string) error {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	disk, exists := m.deviceCache[deviceID]
	if !exists {
		return errors.New(errors.DiskNotFound, "device not found").
			WithMetadata("device_id", deviceID)
	}

	disk.Notes = notes

	m.stateManager.SaveDebounced()
	m.logger.Info("disk notes updated", "device_id", deviceID)

	return nil
}
