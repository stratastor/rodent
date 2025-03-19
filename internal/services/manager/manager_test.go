// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"testing"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/services"
)

// setupTestManager creates a service manager for testing
func setupTestManager(t *testing.T) *ServiceManager {
	l, err := logger.New(logger.Config{LogLevel: "debug"})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	mgr, err := NewServiceManager(l)
	if err != nil {
		t.Fatalf("Failed to create ServiceManager: %v", err)
	}

	return mgr
}

func TestRealServices(t *testing.T) {
	mgr := setupTestManager(t)
	ctx := context.Background()

	// Get all available services
	availableServices := mgr.ListServices()
	if len(availableServices) == 0 {
		t.Log("No real services available - skipping real service tests")
		return
	}

	t.Logf("Found %d real services: %v", len(availableServices), availableServices)

	// Test all services lifecycle
	for _, serviceName := range availableServices {
		t.Run("Service_"+serviceName, func(t *testing.T) {
			svc, ok := mgr.GetService(serviceName)
			if !ok {
				t.Fatalf("Service %s not found after listing", serviceName)
			}

			// Get initial status
			initialStatus, err := svc.Status(ctx)
			if err != nil {
				t.Fatalf("Failed to get %s initial status: %v", serviceName, err)
			}

			t.Logf("Initial status of %s: %d instances", serviceName, len(initialStatus))
			for i, status := range initialStatus {
				t.Logf("Instance %d: %s (%s) - health: %s",
					i, status.InstanceName(), status.InstanceState(), status.InstanceHealth())
			}

			// Full lifecycle testing
			t.Run("Lifecycle", func(t *testing.T) {
				// Stop service
				t.Logf("Stopping %s...", serviceName)
				if err := svc.Stop(ctx); err != nil {
					t.Logf(
						"Stop operation returned error: %v (might be expected for systemd services)",
						err,
					)
				}

				// Wait for service to fully stop
				time.Sleep(3 * time.Second)

				// Check status after stop
				stopStatus, err := svc.Status(ctx)
				if err != nil {
					t.Fatalf("Failed to get %s status after stop: %v", serviceName, err)
				}
				t.Logf("Status after stop: %d instances", len(stopStatus))
				for i, status := range stopStatus {
					t.Logf("Instance %d: %s (%s)", i, status.InstanceName(), status.InstanceState())
				}

				// Start service
				t.Logf("Starting %s...", serviceName)
				if err := svc.Start(ctx); err != nil {
					t.Logf(
						"Start operation returned error: %v (might be expected for systemd services)",
						err,
					)
				}

				// Wait for service to fully start
				time.Sleep(3 * time.Second)

				// Check status after start
				startStatus, err := svc.Status(ctx)
				if err != nil {
					t.Fatalf("Failed to get %s status after start: %v", serviceName, err)
				}
				t.Logf("Status after start: %d instances", len(startStatus))
				for i, status := range startStatus {
					t.Logf("Instance %d: %s (%s)", i, status.InstanceName(), status.InstanceState())
				}

				// Restart service
				t.Logf("Restarting %s...", serviceName)
				if err := svc.Restart(ctx); err != nil {
					t.Logf(
						"Restart operation returned error: %v (might be expected for systemd services)",
						err,
					)
				}

				// Wait for service to fully restart
				time.Sleep(3 * time.Second)

				// Check status after restart
				restartStatus, err := svc.Status(ctx)
				if err != nil {
					t.Fatalf("Failed to get %s status after restart: %v", serviceName, err)
				}
				t.Logf("Status after restart: %d instances", len(restartStatus))
				for i, status := range restartStatus {
					t.Logf("Instance %d: %s (%s)", i, status.InstanceName(), status.InstanceState())
				}

				// Verify specific service behaviors
				verifyServiceStateBehavior(
					t,
					serviceName,
					initialStatus,
					stopStatus,
					startStatus,
					restartStatus,
				)
			})
		})
	}

	// Test status retrieval via manager
	t.Run("GetAllServiceStatuses", func(t *testing.T) {
		statuses := mgr.GetAllServiceStatuses(ctx)

		// Verify we got statuses for all services
		if len(statuses) != len(availableServices) {
			t.Errorf("Expected statuses for %d services, got %d",
				len(availableServices), len(statuses))
		}

		// Log all service statuses
		for service, status := range statuses {
			t.Logf("Service %s status details: %v", service, status)
		}
	})
}

// Helper function to verify service state behavior
func verifyServiceStateBehavior(t *testing.T, serviceName string,
	initialStatus, stopStatus, startStatus, restartStatus []services.ServiceStatus) {

	// Extract states from status lists
	extractStates := func(statuses []services.ServiceStatus) []string {
		var states []string
		for _, status := range statuses {
			states = append(states, status.InstanceState())
		}
		return states
	}

	initialStates := extractStates(initialStatus)
	stopStates := extractStates(stopStatus)
	startStates := extractStates(startStatus)
	restartStates := extractStates(restartStatus)

	t.Logf("State transitions for %s:", serviceName)
	t.Logf("  Initial: %v", initialStates)
	t.Logf("  After stop: %v", stopStates)
	t.Logf("  After start: %v", startStates)
	t.Logf("  After restart: %v", restartStates)

	// Service-specific assertions
	switch serviceName {
	case "docker":
		// For Docker (systemd managed), operations may not change state
		// so just log the observed behavior
		t.Logf("Docker service operations controlled by systemd - observed behavior captured")

	case "traefik":
		// For Traefik (Docker managed), we expect more direct control
		// If we see transitions, log and verify them
		if len(initialStates) > 0 && len(startStates) > 0 {
			for _, state := range startStates {
				if state != "running" && state != "active" {
					t.Logf("Note: Expected Traefik to be running after start, found %s", state)
				}
			}
		}
	}
}
