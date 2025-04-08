package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/pkg/facl"
	"github.com/stratastor/rodent/pkg/shares/smb"
)

// Setup test environment
func setupTestAPI(t *testing.T) (*gin.Engine, *SharesHandler, string, func()) {
	t.Helper()

	// Create temporary directory for configurations
	tempDir, err := os.MkdirTemp("", "shares-api-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create logger
	logConfig := logger.LogConfig{
		Level: "debug",
	}
	log, err := logger.New(logConfig)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create command executor
	executor := command.NewCommandExecutor()

	// Mock ACLManager
	aclManager := facl.NewMockACLManager(log)

	// Override configuration directory for testing
	smb.SharesConfigDir = filepath.Join(tempDir, "shares.d")
	os.MkdirAll(smb.SharesConfigDir, 0755)

	// Create SMB manager
	smbManager, err := smb.NewManager(log, executor, aclManager)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create SMB manager: %v", err)
	}

	// Create SMB service manager (mocked for tests)
	smbService := &MockSMBServiceManager{
		logger: log,
		status: "inactive",
	}

	// Create shares handler
	sharesHandler := NewSharesHandler(log, smbManager, smbService)

	// Create router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Register routes
	apiGroup := router.Group("/api/v1/rodent")
	sharesHandler.RegisterRoutes(apiGroup)

	cleanup := func() {
		// Clean up temporary directory
		os.RemoveAll(tempDir)
	}

	return router, sharesHandler, tempDir, cleanup
}

// MockSMBServiceManager simulates an SMB service for testing
type MockSMBServiceManager struct {
	logger logger.Logger
	status string
}

func (m *MockSMBServiceManager) Start(ctx context.Context) error {
	m.status = "active"
	return nil
}

func (m *MockSMBServiceManager) Stop(ctx context.Context) error {
	m.status = "inactive"
	return nil
}

func (m *MockSMBServiceManager) Restart(ctx context.Context) error {
	m.status = "active"
	return nil
}

func (m *MockSMBServiceManager) Status(ctx context.Context) (string, error) {
	return m.status, nil
}

func (m *MockSMBServiceManager) ReloadConfig(ctx context.Context) error {
	return nil
}

// Test cases for SMB share operations
func TestSMBShareOperations(t *testing.T) {
	router, _, tempDir, cleanup := setupTestAPI(t)
	defer cleanup()

	// Create test directory
	testSharePath := filepath.Join(tempDir, "testshare")
	if err := os.MkdirAll(testSharePath, 0755); err != nil {
		t.Fatalf("Failed to create test share directory: %v", err)
	}

	// Test creating an SMB share
	t.Run("CreateSMBShare", func(t *testing.T) {
		smbConfig := smb.SMBShareConfig{
			Name:        "testshare",
			Description: "Test share",
			Path:        testSharePath,
			Enabled:     true,
			ReadOnly:    false,
			Browsable:   true,
			GuestOk:     false,
			ValidUsers:  []string{"testuser"},
		}

		body, _ := json.Marshal(smbConfig)
		req, _ := http.NewRequest("POST", "/api/v1/rodent/shares/smb", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status code %d, got %d", http.StatusCreated, w.Code)
			t.Errorf("Response body: %s", w.Body.String())
		}
	})

	// Test listing SMB shares
	t.Run("ListSMBShares", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/rodent/shares/smb", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
			t.Errorf("Response body: %s", w.Body.String())
		}

		var response struct {
			Shares []map[string]interface{} `json:"shares"`
			Count  int                      `json:"count"`
		}

		err := json.Unmarshal(w.Body.Bytes(), &response)
		if err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if response.Count != 1 {
			t.Errorf("Expected 1 share, got %d", response.Count)
		}

		if len(response.Shares) != 1 {
			t.Errorf("Expected 1 share, got %d", len(response.Shares))
		}

		if response.Shares[0]["name"] != "testshare" {
			t.Errorf("Expected share name 'testshare', got '%s'", response.Shares[0]["name"])
		}
	})

	// Test getting an SMB share
	t.Run("GetSMBShare", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/rodent/shares/smb/testshare", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
			t.Errorf("Response body: %s", w.Body.String())
		}

		var share smb.SMBShareConfig
		err := json.Unmarshal(w.Body.Bytes(), &share)
		if err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if share.Name != "testshare" {
			t.Errorf("Expected share name 'testshare', got '%s'", share.Name)
		}

		if share.Path != testSharePath {
			t.Errorf("Expected share path '%s', got '%s'", testSharePath, share.Path)
		}
	})

	// Test updating an SMB share
	t.Run("UpdateSMBShare", func(t *testing.T) {
		smbConfig := smb.SMBShareConfig{
			Name:        "testshare",
			Description: "Updated test share",
			Path:        testSharePath,
			Enabled:     true,
			ReadOnly:    true,
			Browsable:   true,
			GuestOk:     false,
			ValidUsers:  []string{"testuser", "anotheruser"},
		}

		body, _ := json.Marshal(smbConfig)
		req, _ := http.NewRequest(
			"PUT",
			"/api/v1/rodent/shares/smb/testshare",
			bytes.NewBuffer(body),
		)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
			t.Errorf("Response body: %s", w.Body.String())
		}

		// Verify the update
		req, _ = http.NewRequest("GET", "/api/v1/rodent/shares/smb/testshare", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var share smb.SMBShareConfig
		err := json.Unmarshal(w.Body.Bytes(), &share)
		if err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if share.Description != "Updated test share" {
			t.Errorf("Expected share description 'Updated test share', got '%s'", share.Description)
		}

		if !share.ReadOnly {
			t.Errorf("Expected share to be read-only after update")
		}

		if len(share.ValidUsers) != 2 {
			t.Errorf("Expected 2 valid users, got %d", len(share.ValidUsers))
		}
	})

	// Test bulk update
	t.Run("BulkUpdateSMBShares", func(t *testing.T) {
		bulkConfig := smb.SMBBulkUpdateConfig{
			All: true,
			Parameters: map[string]string{
				"map acl inherit": "yes",
				"inherit acls":    "yes",
			},
		}

		body, _ := json.Marshal(bulkConfig)
		req, _ := http.NewRequest(
			"PUT",
			"/api/v1/rodent/shares/smb/bulk-update",
			bytes.NewBuffer(body),
		)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
			t.Errorf("Response body: %s", w.Body.String())
		}

		var response struct {
			Message string                    `json:"message"`
			Results []smb.SMBBulkUpdateResult `json:"results"`
		}

		err := json.Unmarshal(w.Body.Bytes(), &response)
		if err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if len(response.Results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(response.Results))
		}

		if !response.Results[0].Success {
			t.Errorf("Expected bulk update to succeed, got error: %s", response.Results[0].Error)
		}

		// Verify the update
		req, _ = http.NewRequest("GET", "/api/v1/rodent/shares/smb/testshare", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var share smb.SMBShareConfig
		err = json.Unmarshal(w.Body.Bytes(), &share)
		if err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if share.CustomParameters["map acl inherit"] != "yes" {
			t.Errorf("Expected custom parameter 'map acl inherit' to be 'yes'")
		}

		if share.CustomParameters["inherit acls"] != "yes" {
			t.Errorf("Expected custom parameter 'inherit acls' to be 'yes'")
		}
	})

	// Test service operations
	t.Run("ServiceOperations", func(t *testing.T) {
		// Check initial status
		req, _ := http.NewRequest("GET", "/api/v1/rodent/shares/smb/service/status", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Start service
		req, _ = http.NewRequest("POST", "/api/v1/rodent/shares/smb/service/start", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Verify status changed
		req, _ = http.NewRequest("GET", "/api/v1/rodent/shares/smb/service/status", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var status smb.SMBServiceStatus
		err := json.Unmarshal(w.Body.Bytes(), &status)
		if err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if !status.Running {
			t.Errorf("Expected service to be running after start")
		}

		// Stop service
		req, _ = http.NewRequest("POST", "/api/v1/rodent/shares/smb/service/stop", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		}

		// Verify status changed
		req, _ = http.NewRequest("GET", "/api/v1/rodent/shares/smb/service/status", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		err = json.Unmarshal(w.Body.Bytes(), &status)
		if err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if status.Running {
			t.Errorf("Expected service to be stopped after stop")
		}
	})

	// Test deleting an SMB share
	t.Run("DeleteSMBShare", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/rodent/shares/smb/testshare", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("Expected status code %d, got %d", http.StatusNoContent, w.Code)
			t.Errorf("Response body: %s", w.Body.String())
		}

		// Verify the share is gone
		req, _ = http.NewRequest("GET", "/api/v1/rodent/shares/smb/testshare", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}

// Test path validation
func TestPathValidation(t *testing.T) {
	testCases := []struct {
		name        string
		path        string
		expectError bool
	}{
		{"ValidPath", "/valid/path", false},
		{"EmptyPath", "", true},
		{"RelativePath", "relative/path", true},
		{"PathTraversal", "/valid/../path", true},
		{"PathTraversalHidden", "/valid/..%2fpath", true},
		{"NonPrintableChars", "/valid/path\x00", true},
		{"InvalidChars", "/valid/path;command", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePath(tc.path)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for path '%s', got none", tc.path)
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error for path '%s', got: %v", tc.path, err)
			}
		})
	}
}
