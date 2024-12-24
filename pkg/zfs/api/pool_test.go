package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/pool"
	"github.com/stratastor/rodent/pkg/zfs/testutil"
)

func TestPoolAPI(t *testing.T) {
	// Setup test environment
	env := testutil.NewTestEnv(t, 3)
	defer env.Cleanup()

	// Setup API server
	gin.SetMode(gin.TestMode)
	router := gin.New()

	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "debug"})
	poolManager := pool.NewManager(executor)
	poolHandler := NewPoolHandler(poolManager)

	poolHandler.RegisterRoutes(router.Group("/api/v1"))

	// Test pool creation
	t.Run("CreatePool", func(t *testing.T) {
		reqBody := pool.CreateConfig{
			Name: "tank",
			VDevSpec: []pool.VDevSpec{
				{
					Type:    "raidz",
					Devices: env.GetLoopDevices(),
				},
			},
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("failed to marshal request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/pools", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			var apiError errors.RodentError
			if err := json.NewDecoder(rec.Body).Decode(&apiError); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, apiError.Message)
		}
	})

	// Test property operations
	t.Run("Properties", func(t *testing.T) {
		// Test setting property
		reqBody := map[string]string{"value": "test pool"}
		body, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("failed to marshal request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPut, "/api/v1/pools/tank/properties/comment", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			var apiError errors.RodentError
			if err := json.NewDecoder(rec.Body).Decode(&apiError); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, apiError.Message)
		}

		// Test getting property
		req = httptest.NewRequest(http.MethodGet, "/api/v1/pools/tank/properties/comment", nil)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			var apiError errors.RodentError
			if err := json.NewDecoder(rec.Body).Decode(&apiError); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, apiError.Message)
		}

		var prop pool.Property
		if err := json.NewDecoder(rec.Body).Decode(&prop); err != nil {
			t.Fatalf("failed to decode property response: %v", err)
		}

		if prop.Value != "test pool" {
			t.Errorf("property value = %v, want 'test pool'", prop.Value)
		}
	})

	// Test pool destruction
	t.Run("DestroyPool", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/pools/tank", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			var apiError errors.RodentError
			if err := json.NewDecoder(rec.Body).Decode(&apiError); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, apiError.Message)
		}
	})
}
