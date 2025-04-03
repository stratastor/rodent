/*
 * Copyright 2024-2025 Raamsri Kumar <raam@tinkershack.in>
 * Copyright 2024-2025 The StrataSTOR Authors and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
)

// DatasetHandler provides HTTP endpoints for ZFS dataset operations.
// It implements the following features:
//   - Filesystem creation and management
//   - Volume creation and management
//   - Snapshot operations
//   - Clone operations
//   - Property management
//
// All operations use proper validation and error handling.
type DatasetHandler struct {
	manager *dataset.Manager
}

// PoolHandler provides HTTP endpoints for ZFS pool operations.
// It implements the following features:
//   - Pool creation and destruction
//   - Import/export operations
//   - Status and property management
//   - Device management (attach/detach/replace)
//   - Maintenance operations (scrub/resilver)
//
// All operations use proper validation and error handling.
type PoolHandler struct {
	manager *pool.Manager
}

// Response types match dataset package types
type Property = dataset.Property
type Dataset = dataset.Dataset
type SnapshotInfo = dataset.SnapshotInfo
