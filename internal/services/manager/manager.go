// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/services"
	"github.com/stratastor/rodent/internal/services/clients"
)

// ServiceManager manages all services
type ServiceManager struct {
	logger   logger.Logger
	services map[string]services.Service
	mu       sync.RWMutex
}

// NewServiceManager creates a new service manager
func NewServiceManager(logger logger.Logger) (*ServiceManager, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	mgr := &ServiceManager{
		logger:   logger,
		services: make(map[string]services.Service),
	}

	// Initialize all available services
	if err := mgr.initializeServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}

	return mgr, nil
}

// initializeServices initializes all available service clients
func (m *ServiceManager) initializeServices() error {
	// Create Docker client
	dockerClient, err := clients.NewDockerClient(m.logger)
	if err != nil {
		m.logger.Warn("Failed to initialize Docker client", "err", err)
		// Continue without Docker client
	} else {
		m.registerService(dockerClient)
	}

	// Create Traefik client
	traefikClient, err := clients.NewTraefikClient(m.logger)
	if err != nil {
		m.logger.Warn("Failed to initialize Traefik client", "err", err)
		// Continue without Traefik client
	} else {
		m.registerService(traefikClient)
	}

	// Create AD DC client
	addcClient, err := clients.NewADDCClient(m.logger)
	if err != nil {
		m.logger.Warn("Failed to initialize AD DC client", "err", err)
		// Continue without AD DC client
	} else {
		m.registerService(addcClient)
	}

	// Create Samba client
	sambaClient, err := clients.NewSambaClient(m.logger)
	if err != nil {
		m.logger.Warn("Failed to initialize Samba client", "err", err)
		// Continue without Samba client
	} else {
		m.registerService(sambaClient)
	}

	// TODO: Add other services (NFS, etc.)

	return nil
}

// registerService registers a service with the manager
func (m *ServiceManager) registerService(svc services.Service) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services[svc.Name()] = svc
}

// GetService returns a service by name
func (m *ServiceManager) GetService(name string) (services.Service, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	svc, ok := m.services[name]
	return svc, ok
}

// ListServices returns a list of all available service names
func (m *ServiceManager) ListServices() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var names []string
	for name := range m.services {
		names = append(names, name)
	}
	return names
}

// GetAllServiceStatuses returns the status of all services
func (m *ServiceManager) GetAllServiceStatuses(ctx context.Context) map[string]interface{} {
	m.mu.RLock()
	services := make(map[string]services.Service)
	for name, svc := range m.services {
		services[name] = svc
	}
	m.mu.RUnlock()

	statuses := make(map[string]interface{})
	for name, svc := range services {
		statusList, err := svc.Status(ctx)
		if err != nil {
			statuses[name] = map[string]string{
				"error": err.Error(),
			}
		} else if len(statusList) == 0 {
			statuses[name] = map[string]string{
				"state": "unknown",
			}
		} else {
			// For multiple instances, return detailed information
			instanceStatuses := make([]map[string]string, 0, len(statusList))
			for _, status := range statusList {
				instanceStatuses = append(instanceStatuses, map[string]string{
					"name":    status.InstanceName(),
					"service": status.InstanceService(),
					"state":   status.InstanceState(),
					"status":  status.InstanceStatus(),
					"health":  status.InstanceHealth(),
				})
			}
			statuses[name] = instanceStatuses
		}
	}

	return statuses
}

// StartService starts a service by name
func (m *ServiceManager) StartService(ctx context.Context, name string) error {
	svc, ok := m.GetService(name)
	if !ok {
		return fmt.Errorf("service %q not found", name)
	}

	return svc.Start(ctx)
}

// StopService stops a service by name
func (m *ServiceManager) StopService(ctx context.Context, name string) error {
	svc, ok := m.GetService(name)
	if !ok {
		return fmt.Errorf("service %q not found", name)
	}

	return svc.Stop(ctx)
}

// RestartService restarts a service by name
func (m *ServiceManager) RestartService(ctx context.Context, name string) error {
	svc, ok := m.GetService(name)
	if !ok {
		return fmt.Errorf("service %q not found", name)
	}

	return svc.Restart(ctx)
}

// Close cleans up resources used by the service manager
func (m *ServiceManager) Close() error {
	return nil
}
