// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/services/docker"
	"github.com/stratastor/rodent/internal/services/traefik"
)

// NewDockerClient creates a new Docker service client
func NewDockerClient(logger logger.Logger) (*docker.Client, error) {
	return docker.NewClient(logger)
}

// NewTraefikClient creates a new Traefik service client
func NewTraefikClient(logger logger.Logger) (*traefik.Client, error) {
	return traefik.NewClient(logger)
}

// CertificateData contains information about a TLS certificate
type CertificateData = traefik.CertificateData
