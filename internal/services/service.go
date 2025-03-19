// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
)

// Service represents a generic service interface
type Service interface {
	// Name returns the name of the service
	Name() string

	// Status returns the current status of the service
	Status(ctx context.Context) (string, error)

	// Start starts the service
	Start(ctx context.Context) error

	// Stop stops the service
	Stop(ctx context.Context) error

	// Restart restarts the service
	Restart(ctx context.Context) error
}
