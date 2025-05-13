// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"time"
)

// CertificateData contains information about a TLS certificate
type CertificateData struct {
	Domain      string    `yaml:"domain"`
	Certificate string    `yaml:"certificate"`
	PrivateKey  string    `yaml:"privateKey"`
	ExpiresOn   time.Time `yaml:"expiresOn"`
}

// CertificateInstaller represents services that can install TLS certificates
type CertificateInstaller interface {
	InstallCertificate(ctx context.Context, data CertificateData) error
}

// Service represents a generic service interface
type Service interface {
	// Name returns the name of the service
	Name() string

	// Status returns the current status of the service
	Status(ctx context.Context) ([]ServiceStatus, error)

	// Start starts the service
	Start(ctx context.Context) error

	// Stop stops the service
	Stop(ctx context.Context) error

	// Restart restarts the service
	Restart(ctx context.Context) error
}

// StartupService represents a service that can be enabled/disabled at system startup
type StartupService interface {
	Service

	// EnableAtStartup enables the service to start automatically at system boot
	EnableAtStartup(ctx context.Context) error

	// DisableAtStartup disables the service from starting automatically at system boot
	DisableAtStartup(ctx context.Context) error

	// IsEnabledAtStartup checks if the service is enabled to start at system boot
	IsEnabledAtStartup(ctx context.Context) (bool, error)
}

type ServiceStatus interface {
	InstanceGist() string
	InstanceName() string
	InstanceService() string
	InstanceStatus() string
	InstanceHealth() string
	InstanceState() string
}
