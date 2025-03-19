// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/services/manager"
)

// setupTestRouter creates a router and handler for real service testing
func setupRealServicesRouter(t *testing.T) (*gin.Engine, *ServiceHandler) {
	gin.SetMode(gin.TestMode)

	l, err := logger.New(logger.Config{LogLevel: "debug"})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	mgr, err := manager.NewServiceManager(l)
	if err != nil {
		t.Fatalf("Failed to create ServiceManager: %v", err)
	}

	handler := NewServiceHandler(mgr)
	router := gin.New()
	router.Use(gin.Recovery())

	return router, handler
}

func TestRouteRegistration(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	handler := NewServiceHandler(nil) // We just need the handler for route registration

	// Register routes
	group := router.Group("/api/services")
	handler.RegisterRoutes(group)

	// Check that all expected routes are registered
	routes := router.Routes()

	expectedRoutes := map[string]bool{
		"GET /api/services":                false,
		"GET /api/services/status":         false,
		"GET /api/services/:name/status":   false,
		"POST /api/services/:name/start":   false,
		"POST /api/services/:name/stop":    false,
		"POST /api/services/:name/restart": false,
	}

	for _, route := range routes {
		path := route.Method + " " + route.Path
		if _, exists := expectedRoutes[path]; exists {
			expectedRoutes[path] = true
		}
	}

	for route, found := range expectedRoutes {
		if !found {
			t.Errorf("Expected route %s was not registered", route)
		}
	}
}

func TestRealServices(t *testing.T) {
	router, handler := setupRealServicesRouter(t)

	// Register routes
	group := router.Group("/services")
	handler.RegisterRoutes(group)

	// Test listing services
	t.Run("ListServices", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/services", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}

		var response struct {
			Services []string `json:"services"`
		}

		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		// Log services found
		t.Logf("Found services: %v", response.Services)

		// Skip remaining tests if no services available
		if len(response.Services) == 0 {
			t.Skip("No services available - skipping remaining tests")
		}
	})

	// Test getting all service statuses
	t.Run("GetAllServiceStatuses", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/services/status", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
			return
		}

		var response struct {
			Statuses map[string]interface{} `json:"statuses"`
		}

		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		// Log all service statuses
		for service, status := range response.Statuses {
			t.Logf("Service %s status: %v", service, status)

			// Verify status has expected structure based on type
			switch status := status.(type) {
			case []interface{}:
				t.Logf("Service %s has %d instances", service, len(status))
				validateInstancesFormat(t, service, status)

			case map[string]interface{}:
				if errorMsg, hasError := status["error"]; hasError {
					t.Logf("Service %s has error: %v", service, errorMsg)
				} else {
					t.Logf("Service %s has status info: %v", service, status)
				}

			default:
				t.Errorf("Unexpected status type for service %s: %T", service, status)
			}
		}
	})

	// Test specific services if available
	testSpecificService := func(serviceName string) {
		t.Run("Service_"+serviceName, func(t *testing.T) {
			// Check if service exists
			req, _ := http.NewRequest("GET", "/services/"+serviceName+"/status", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusNotFound {
				t.Skipf("Service %s not available - skipping test", serviceName)
				return
			}

			// Get initial status
			initialStatus := getServiceStatus(t, router, serviceName)
			t.Logf("Initial status of %s: %v", serviceName, initialStatus)

			// Full lifecycle testing
			t.Run("Lifecycle", func(t *testing.T) {
				// Stop service
				stopResponse := performServiceOperation(t, router, serviceName, "stop")
				t.Logf("Stop response for %s: %v", serviceName, stopResponse)

				// Wait for service to fully stop
				time.Sleep(2 * time.Second)

				// Check status after stop
				stopStatus := getServiceStatus(t, router, serviceName)
				t.Logf("Status after stop for %s: %v", serviceName, stopStatus)

				// Start service
				startResponse := performServiceOperation(t, router, serviceName, "start")
				t.Logf("Start response for %s: %v", serviceName, startResponse)

				// Wait for service to fully start
				time.Sleep(2 * time.Second)

				// Check status after start
				startStatus := getServiceStatus(t, router, serviceName)
				t.Logf("Status after start for %s: %v", serviceName, startStatus)

				// Restart service
				restartResponse := performServiceOperation(t, router, serviceName, "restart")
				t.Logf("Restart response for %s: %v", serviceName, restartResponse)

				// Wait for service to fully restart
				time.Sleep(2 * time.Second)

				// Check status after restart
				restartStatus := getServiceStatus(t, router, serviceName)
				t.Logf("Status after restart for %s: %v", serviceName, restartStatus)

				// Verify state transitions
				verifyStateTransitions(
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

	// Test Docker service if available
	testSpecificService("docker")

	// Test Traefik service if available
	testSpecificService("traefik")

	// Test non-existent service (should return 404)
	t.Run("NonExistentService", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/services/nonexistentservice/status", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status NotFound for non-existent service, got %v", w.Code)
		}
	})

	// Test invalid operations
	t.Run("InvalidInputs", func(t *testing.T) {
		// Test with very long service name
		t.Run("VeryLongServiceName", func(t *testing.T) {
			veryLongName := strings.Repeat("a", 1000)
			req, _ := http.NewRequest("POST", "/services/"+veryLongName+"/start", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should handle this without crashing
			if w.Code == http.StatusOK {
				t.Error("Expected error for very long service name, got OK")
			}
		})

		// Test with empty service name
		t.Run("EmptyServiceName", func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/services//start", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return an error
			if w.Code == http.StatusOK {
				t.Error("Expected error for empty service name, got OK")
			}
		})
	})
}

// Helper function to validate instances format
func validateInstancesFormat(t *testing.T, serviceName string, instances []interface{}) {
	for i, inst := range instances {
		instance, ok := inst.(map[string]interface{})
		if !ok {
			t.Errorf("Service %s instance %d is not a map: %T", serviceName, i, inst)
			continue
		}

		// Verify required fields
		requiredFields := []string{"name", "service", "state", "status"}
		for _, field := range requiredFields {
			if _, exists := instance[field]; !exists {
				t.Errorf("Service %s instance %d missing required field: %s",
					serviceName, i, field)
			}
		}
	}
}

// Helper function to get a service's status
func getServiceStatus(t *testing.T, router *gin.Engine, serviceName string) map[string]interface{} {
	req, _ := http.NewRequest("GET", "/services/"+serviceName+"/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// If service not found or error, return empty status
	if w.Code != http.StatusOK {
		t.Logf("Warning: Could not get status for %s: %d - %s",
			serviceName, w.Code, w.Body.String())
		return map[string]interface{}{
			"error": fmt.Sprintf("HTTP %d", w.Code),
		}
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal status response: %v", err)
	}

	return response
}

// Helper function to perform service operations
func performServiceOperation(
	t *testing.T,
	router *gin.Engine,
	serviceName, operation string,
) map[string]interface{} {
	req, _ := http.NewRequest("POST", "/services/"+serviceName+"/"+operation, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal operation response: %v", err)
	}

	// Log response code and body for diagnostics
	t.Logf("%s operation on %s returned code %d",
		operation, serviceName, w.Code)

	return response
}

// Helper function to verify state transitions
func verifyStateTransitions(t *testing.T, serviceName string,
	initialStatus, stopStatus, startStatus, restartStatus map[string]interface{}) {

	// Extract instance states based on service
	extractStates := func(status map[string]interface{}) []string {
		var states []string
		if statusArr, ok := status["status"].([]interface{}); ok {
			for _, instance := range statusArr {
				if instanceMap, ok := instance.(map[string]interface{}); ok {
					if state, ok := instanceMap["state"].(string); ok {
						states = append(states, state)
					}
				}
			}
		}
		return states
	}

	initialStates := extractStates(initialStatus)
	stopStates := extractStates(stopStatus)
	startStates := extractStates(startStatus)
	restartStates := extractStates(restartStatus)

	// Log all state transitions for analysis
	t.Logf("State transitions for %s:", serviceName)
	t.Logf("  Initial: %v", initialStates)
	t.Logf("  After stop: %v", stopStates)
	t.Logf("  After start: %v", startStates)
	t.Logf("  After restart: %v", restartStates)

	// Service-specific assertions
	switch serviceName {
	case "docker":
		// For docker, we expect all instances to be stopped after a stop command
		if len(initialStates) > 0 && len(stopStates) > 0 {
			for _, state := range stopStates {
				if state == "running" {
					t.Errorf("Expected Docker to be stopped, but found running instance")
				}
			}

			// And all to be running after a start command
			for _, state := range startStates {
				if state != "running" {
					t.Errorf("Expected Docker to be running after start, but found %s", state)
				}
			}
		}

	case "traefik":
		// Similar checks for Traefik
		if len(initialStates) > 0 && len(stopStates) > 0 {
			for _, state := range stopStates {
				if state == "running" {
					t.Errorf("Expected Traefik to be stopped, but found running instance")
				}
			}

			// Check that services are running after start
			if len(startStates) == 0 {
				t.Errorf("Expected Traefik instances after start, but found none")
			} else {
				for _, state := range startStates {
					if state != "running" {
						t.Errorf("Expected Traefik to be running after start, but found %s", state)
					}
				}
			}
		}
	}
}
