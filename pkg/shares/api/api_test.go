// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/pkg/shares"
	"github.com/stratastor/rodent/pkg/shares/smb"
)

// TODO: Backup, cleanup config, and reload on exit
var (
	testFS string
	ts1    string // test share 1
	ts2    string // test share 2
)

// setupAPITest creates a test environment for API testing
func setupAPITest(t *testing.T) (*gin.Engine, *smb.Manager, *smb.ServiceManager, func()) {
	testFS = os.Getenv("RODENT_TEST_FS")

	// Skip if required env vars are not set
	if testFS == "" {
		t.Skip(
			"Required environment variables RODENT_TEST_USER, RODENT_TEST_USER_PASS, and RODENT_TEST_FS must be set",
		)
	}

	// Check if the test filesystem exists
	if _, err := os.Stat(testFS); os.IsNotExist(err) {
		t.Skipf("Test filesystem '%s' does not exist", testFS)
	}

	ts1 = "api-test-share-1-" + time.Now().Format("20060102150405")
	ts2 = "api-test-share-2-" + time.Now().Format("20060102150405")

	// Create temporary directory for share configs
	tempDir, err := os.MkdirTemp("", "rodent-smb-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create logger
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "api-test")
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create executor that doesn't actually execute commands
	executor := command.NewCommandExecutor(true)

	// Create SMB manager
	smbManager, err := smb.NewManager(log, executor, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create SMB manager: %v", err)
	}

	// Create SMB service manager
	smbService := smb.NewServiceManager(log)

	// Create API handler
	handler := NewSharesHandler(log, smbManager, smbService)

	// Create router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Register routes
	apiGroup := router.Group("/api/shares")
	handler.RegisterRoutes(apiGroup)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return router, smbManager, smbService, cleanup
}

// TestSharesAPI tests all share API endpoints
func TestSharesAPI(t *testing.T) {
	router, manager, _, cleanup := setupAPITest(t)
	defer cleanup()

	// Create test share for subsequent tests
	createTestShare := func(t *testing.T, name, path string) {
		shareConfig := smb.NewSMBShareConfig(name, path)
		err := manager.CreateShare(context.Background(), shareConfig)
		if err != nil {
			t.Fatalf("Failed to create test share: %v", err)
		}
	}

	// Case 1: Test listShares endpoint
	t.Run("ListShares", func(t *testing.T) {
		// Create test share first
		createTestShare(t, ts1, testFS)

		req, _ := http.NewRequest("GET", "/api/shares/smb", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
			t.Errorf("Response: %s", w.Body.String())
		}

		var response struct {
			Shares []shares.ShareConfig `json:"shares"`
			Count  int                  `json:"count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Count == 0 {
			t.Error("Expected at least one share, got 0")
		}

		// Verify the share is in the list
		found := false
		for _, share := range response.Shares {
			if share.Name == ts1 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Test share not found in response")
		}
	})

	// Case 2: Test getShare endpoint
	t.Run("GetShare", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/shares/smb/"+ts1, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
			t.Errorf("Response: %s", w.Body.String())
		}

		var response shares.ShareConfig
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Name != ts1 {
			t.Errorf("Expected share name '%s', got '%s'", ts1, response.Name)
		}
		if response.Path != testFS {
			t.Errorf("Expected share path '%s', got '%s'", testFS, response.Path)
		}
	})

	// Case 3: Test createSMBShare endpoint
	t.Run("CreateSMBShare", func(t *testing.T) {
		shareConfig := struct {
			Name        string            `json:"name"`
			Path        string            `json:"path"`
			Description string            `json:"description"`
			Tags        map[string]string `json:"tags"`
		}{
			Name:        ts2,
			Path:        testFS,
			Description: "Test share 2",
			Tags: map[string]string{
				"purpose": "testing",
			},
		}

		body, _ := json.Marshal(shareConfig)
		req, _ := http.NewRequest("POST", "/api/shares/smb", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status Created, got %v", w.Code)
			t.Errorf("Response: %s", w.Body.String())
		}

		var response struct {
			Message string `json:"message"`
			Name    string `json:"name"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Name != ts2 {
			t.Errorf("Expected share name '%s', got '%s'", ts2, response.Name)
		}

		// Verify the share was created
		exists, err := manager.Exists(context.Background(), ts2)
		if err != nil {
			t.Fatalf("Failed to check if share exists: %v", err)
		}
		if !exists {
			t.Error("Share was not created")
		}
	})

	// Case 4: Test updateSMBShare endpoint
	t.Run("UpdateSMBShare", func(t *testing.T) {
		// Get the current share config
		smbShare, err := manager.GetSMBShare(context.Background(), ts1)
		if err != nil {
			t.Fatalf("Failed to get share config: %v", err)
		}

		// Update description
		smbShare.Description = "Updated test share 1"

		body, _ := json.Marshal(smbShare)
		req, _ := http.NewRequest("PUT", "/api/shares/smb/"+ts1, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
			t.Errorf("Response: %s", w.Body.String())
		}

		// Verify the share was updated
		updatedShare, err := manager.GetSMBShare(context.Background(), ts1)
		if err != nil {
			t.Fatalf("Failed to get updated share config: %v", err)
		}
		if updatedShare.Description != "Updated test share 1" {
			t.Errorf("Share description not updated: expected 'Updated test share 1', got '%s'",
				updatedShare.Description)
		}
	})

	// Case 5: Test getSMBShare endpoint
	t.Run("GetSMBShare", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/shares/smb/"+ts1, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
			t.Errorf("Response: %s", w.Body.String())
		}

		var response smb.SMBShareConfig
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Name != ts1 {
			t.Errorf("Expected share name '%s', got '%s'", ts1, response.Name)
		}
		if response.Path != testFS {
			t.Errorf("Expected share path '%s', got '%s'", testFS, response.Path)
		}
		if response.Description != "Updated test share 1" {
			t.Errorf("Expected share description 'Updated test share 1', got '%s'",
				response.Description)
		}
	})

	// Case 6: Test bulkUpdateSMBShares endpoint
	t.Run("BulkUpdateSMBShares", func(t *testing.T) {
		bulkConfig := smb.SMBBulkUpdateConfig{
			All: true,
			Parameters: map[string]string{
				"hide dot files": "yes",
				"case sensitive": "no",
			},
		}

		body, _ := json.Marshal(bulkConfig)
		req, _ := http.NewRequest("PUT", "/api/shares/smb/bulk-update", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
			t.Errorf("Response: %s", w.Body.String())
		}

		var response struct {
			Message string                    `json:"message"`
			Results []smb.SMBBulkUpdateResult `json:"results"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if len(response.Results) == 0 {
			t.Error("Expected bulk update results, got none")
		}

		// Verify parameters were updated
		share1, err := manager.GetSMBShare(context.Background(), ts1)
		if err != nil {
			t.Fatalf("Failed to get share config: %v", err)
		}
		if share1.CustomParameters["hide dot files"] != "yes" {
			t.Errorf("Parameter 'hide dot files' not updated: got '%s'",
				share1.CustomParameters["hide dot files"])
		}
	})

	// Case 7: Test listSMBShares endpoint
	t.Run("ListSMBShares", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/shares/smb", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
			t.Errorf("Response: %s", w.Body.String())
		}

		var response struct {
			Shares []shares.ShareConfig `json:"shares"`
			Count  int                  `json:"count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Count < 2 {
			t.Errorf("Expected at least 2 shares, got %d", response.Count)
		}
	})

	// Test global config endpoints
	// t.Run("GlobalConfig", func(t *testing.T) {
	// 	// First, update global config
	// 	globalConfig := smb.SMBGlobalConfig{
	// 		WorkGroup:    "TESTGROUP",
	// 		SecurityMode: "user",
	// 		ServerString: "Test Server",
	// 		LogLevel:     "1",
	// 	}

	// 	body, _ := json.Marshal(globalConfig)
	// 	req, _ := http.NewRequest("PUT", "/api/shares/smb/global", bytes.NewBuffer(body))
	// 	req.Header.Set("Content-Type", "application/json")
	// 	w := httptest.NewRecorder()
	// 	router.ServeHTTP(w, req)

	// 	if w.Code != http.StatusOK {
	// 		t.Errorf("Expected status OK, got %v", w.Code)
	// 		t.Errorf("Response: %s", w.Body.String())
	// 	}

	// 	// Then get global config and verify it was updated
	// 	req, _ = http.NewRequest("GET", "/api/shares/smb/global", nil)
	// 	w = httptest.NewRecorder()
	// 	router.ServeHTTP(w, req)

	// 	if w.Code != http.StatusOK {
	// 		t.Errorf("Expected status OK, got %v", w.Code)
	// 		t.Errorf("Response: %s", w.Body.String())
	// 	}

	// 	var response smb.SMBGlobalConfig
	// 	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
	// 		t.Fatalf("Failed to unmarshal response: %v", err)
	// 	}

	// 	if response.WorkGroup != "TESTGROUP" {
	// 		t.Errorf("Expected workgroup 'TESTGROUP', got '%s'", response.WorkGroup)
	// 	}
	// 	if response.SecurityMode != "user" {
	// 		t.Errorf("Expected security mode 'user', got '%s'", response.SecurityMode)
	// 	}
	// })

	// Case 9: Test deleteShare endpoint
	t.Run("DeleteShare", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/shares/smb/"+ts1, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("Expected status No Content, got %v", w.Code)
			t.Errorf("Response: %s", w.Body.String())
		}

		// Verify the share was deleted
		exists, err := manager.Exists(context.Background(), ts1)
		if err != nil {
			t.Fatalf("Failed to check if share exists: %v", err)
		}
		if exists {
			t.Error("Share was not deleted")
		}
	})

	// Case 10: Test error handling with non-existent share
	t.Run("ErrorHandling", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/shares/smb/nonexistent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			t.Error("Expected error status for non-existent share, got OK")
		}

		var response struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal error response: %v", err)
		}

		if response.Error.Message == "" {
			t.Error("Expected error message, got empty string")
		}
	})
}

// TestServiceAPI tests the service API endpoints
func TestServiceAPI(t *testing.T) {
	router, _, _, cleanup := setupAPITest(t)
	defer cleanup()

	t.Run("GetServiceStatus", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/shares/smb/service/status", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// We're just checking that the endpoint works, not the actual status
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
			t.Errorf("Response: %s", w.Body.String())
		}
	})

	t.Run("ServiceOperations", func(t *testing.T) {
		operations := []string{"start", "stop", "restart", "reload"}

		for _, op := range operations {
			t.Run(op, func(t *testing.T) {
				// Sleep for 5 seconds to avoid overwhelming the service
				time.Sleep(5 * time.Second)
				req, _ := http.NewRequest("POST", "/api/shares/smb/service/"+op, nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Expected status OK, got %v", w.Code)
					t.Errorf("Response: %s", w.Body.String())
				}

				var response struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if !strings.Contains(response.Message, op) {
					t.Errorf("Expected message to contain '%s', got '%s'", op, response.Message)
				}
			})
		}
	})
}
