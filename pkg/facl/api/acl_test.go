package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/facl"
)

// TODO: Fix failing test cases

func setupTestAPI(t *testing.T) (*gin.Engine, string, func()) {
	t.Helper()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "facl-api-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	f, err := os.Create(testFile)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.WriteString("test content")
	f.Close()

	// Create subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create logger
	log := common.Log

	// Create ACL manager
	manager := facl.NewACLManager(log, nil)

	// Create handler
	handler := NewACLHandler(manager, log)

	// Create router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Register routes
	aclGroup := router.Group("/api/v1/rodent/facl")
	handler.RegisterRoutes(aclGroup)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return router, tempDir, cleanup
}

func TestACLHandler_GetACL(t *testing.T) {
	router, tempDir, cleanup := setupTestAPI(t)
	defer cleanup()

	// Check if ACLs are supported
	_, err := os.Stat(facl.BinGetfacl)
	if err != nil {
		t.Skip("getfacl not available, skipping test")
	}

	testFile := filepath.Join(tempDir, "test.txt")
	urlPath := "/api/v1/rodent/facl" + testFile

	// Test GET request
	req, _ := http.NewRequest("GET", urlPath, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Verify response
	if resp.Code != http.StatusOK {
		t.Errorf("Wrong status code: got %v, want %v", resp.Code, http.StatusOK)
		t.Logf("Response: %s", resp.Body.String())
	}

	// Verify response content
	var result struct {
		Result facl.ACLListResult `json:"result"`
	}

	err = json.Unmarshal(resp.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Failed to parse response: %v", err)
		return
	}

	if result.Result.Path != testFile {
		t.Errorf("Wrong path in result: got %s, want %s", result.Result.Path, testFile)
	}

	if result.Result.Type != facl.ACLTypePOSIX {
		t.Errorf("Wrong ACL type: got %s, want %s", result.Result.Type, facl.ACLTypePOSIX)
	}

	if len(result.Result.Entries) == 0 {
		t.Error("Expected ACL entries in result, got none")
	}
}

func TestACLHandler_SetModifyRemoveACL(t *testing.T) {
	router, tempDir, cleanup := setupTestAPI(t)
	defer cleanup()

	// Check if ACLs are supported
	_, err := os.Stat(facl.BinSetfacl)
	if err != nil {
		t.Skip("setfacl not available, skipping test")
	}

	testFile := filepath.Join(tempDir, "test.txt")
	urlPath := "/api/v1/rodent/facl" + testFile

	// Test SET request (PUT)
	t.Run("SetACL", func(t *testing.T) {
		reqBody := struct {
			Type      facl.ACLType    `json:"type"`
			Entries   []facl.ACLEntry `json:"entries"`
			Recursive bool            `json:"recursive"`
		}{
			Type: facl.ACLTypePOSIX,
			Entries: []facl.ACLEntry{
				{
					Type:        facl.EntryUser,
					Principal:   "nobody",
					Permissions: []facl.PermissionType{facl.PermReadData, facl.PermExecute},
				},
			},
			Recursive: false,
		}

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PUT", urlPath, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("Wrong status code for SET: got %v, want %v", resp.Code, http.StatusOK)
			t.Logf("Response: %s", resp.Body.String())
			return
		}

		// Verify ACL was set
		getReq, _ := http.NewRequest("GET", urlPath, nil)
		getResp := httptest.NewRecorder()
		router.ServeHTTP(getResp, getReq)

		var result struct {
			Result facl.ACLListResult `json:"result"`
		}

		err = json.Unmarshal(getResp.Body.Bytes(), &result)
		if err != nil {
			t.Errorf("Failed to parse GET response: %v", err)
			return
		}

		// Look for our entry
		found := false
		for _, entry := range result.Result.Entries {
			if entry.Type == facl.EntryUser && entry.Principal == "nobody" {
				found = true
				perms := make(map[facl.PermissionType]bool)
				for _, p := range entry.Permissions {
					perms[p] = true
				}
				if !perms[facl.PermReadData] || !perms[facl.PermExecute] {
					t.Errorf("Wrong permissions: %v", entry.Permissions)
				}
				break
			}
		}

		if !found {
			t.Error("ACL entry not found after setting")
		}
	})

	// Test MODIFY request (PATCH)
	t.Run("ModifyACL", func(t *testing.T) {
		reqBody := struct {
			Type      facl.ACLType    `json:"type"`
			Entries   []facl.ACLEntry `json:"entries"`
			Recursive bool            `json:"recursive"`
		}{
			Type: facl.ACLTypePOSIX,
			Entries: []facl.ACLEntry{
				{
					Type:        facl.EntryUser,
					Principal:   "nobody",
					Permissions: []facl.PermissionType{facl.PermReadData, facl.PermWriteData},
				},
			},
			Recursive: false,
		}

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PATCH", urlPath, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("Wrong status code for MODIFY: got %v, want %v", resp.Code, http.StatusOK)
			t.Logf("Response: %s", resp.Body.String())
			return
		}

		// Verify ACL was modified
		getReq, _ := http.NewRequest("GET", urlPath, nil)
		getResp := httptest.NewRecorder()
		router.ServeHTTP(getResp, getReq)

		var result struct {
			Result facl.ACLListResult `json:"result"`
		}

		err = json.Unmarshal(getResp.Body.Bytes(), &result)
		if err != nil {
			t.Errorf("Failed to parse GET response: %v", err)
			return
		}

		// Look for modified entry
		found := false
		for _, entry := range result.Result.Entries {
			if entry.Type == facl.EntryUser && entry.Principal == "nobody" {
				found = true
				perms := make(map[facl.PermissionType]bool)
				for _, p := range entry.Permissions {
					perms[p] = true
				}
				if !perms[facl.PermReadData] || !perms[facl.PermWriteData] {
					t.Errorf("Wrong permissions after modify: %v", entry.Permissions)
				}
				break
			}
		}

		if !found {
			t.Error("Modified ACL entry not found")
		}
	})

	// Test REMOVE request (DELETE)
	t.Run("RemoveACL", func(t *testing.T) {
		reqBody := struct {
			Type           facl.ACLType    `json:"type"`
			Entries        []facl.ACLEntry `json:"entries"`
			Recursive      bool            `json:"recursive"`
			RemoveAllXattr bool            `json:"remove_all_xattr"`
		}{
			Type: facl.ACLTypePOSIX,
			Entries: []facl.ACLEntry{
				{
					Type:      facl.EntryUser,
					Principal: "nobody",
				},
			},
			Recursive:      false,
			RemoveAllXattr: false,
		}

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("DELETE", urlPath, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("Wrong status code for REMOVE: got %v, want %v", resp.Code, http.StatusOK)
			t.Logf("Response: %s", resp.Body.String())
			return
		}

		// Verify ACL was removed
		getReq, _ := http.NewRequest("GET", urlPath, nil)
		getResp := httptest.NewRecorder()
		router.ServeHTTP(getResp, getReq)

		var result struct {
			Result facl.ACLListResult `json:"result"`
		}

		err = json.Unmarshal(getResp.Body.Bytes(), &result)
		if err != nil {
			t.Errorf("Failed to parse GET response: %v", err)
			return
		}

		// Make sure the entry was removed
		for _, entry := range result.Result.Entries {
			if entry.Type == facl.EntryUser && entry.Principal == "nobody" {
				t.Error("ACL entry found after removal")
				break
			}
		}
	})
}

func TestACLHandler_ErrorCases(t *testing.T) {
	router, tempDir, cleanup := setupTestAPI(t)
	defer cleanup()

	// Test invalid paths
	t.Run("InvalidPath", func(t *testing.T) {
		invalidPaths := []string{
			"/api/v1/rodent/facl/path/with/|/pipe",
			"/api/v1/rodent/facl/path/with/>/redirect",
			"/api/v1/rodent/facl/path/with/</redirect",
			"/api/v1/rodent/facl/path/with/;/semicolon",
		}

		for _, path := range invalidPaths {
			req, _ := http.NewRequest("GET", path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Errorf("Expected bad request for path %s: got %v, want %v",
					path, resp.Code, http.StatusBadRequest)
			}
		}
	})

	// Test missing or invalid request body
	t.Run("InvalidRequestBody", func(t *testing.T) {
		validPath := "/api/v1/rodent/facl" + filepath.Join(tempDir, "test.txt")

		// Empty body
		req, _ := http.NewRequest("PUT", validPath, bytes.NewBuffer([]byte{}))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("Expected bad request for empty body: got %v, want %v",
				resp.Code, http.StatusBadRequest)
		}

		// Invalid JSON
		req, _ = http.NewRequest("PUT", validPath, bytes.NewBuffer([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		resp = httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("Expected bad request for invalid JSON: got %v, want %v",
				resp.Code, http.StatusBadRequest)
		}

		// Missing required fields
		reqBody := struct {
			Type    facl.ACLType    `json:"type"`
			Entries []facl.ACLEntry `json:"entries"`
		}{
			Type:    "", // Missing required type
			Entries: []facl.ACLEntry{},
		}

		body, _ := json.Marshal(reqBody)
		req, _ = http.NewRequest("PUT", validPath, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp = httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("Expected bad request for missing fields: got %v, want %v",
				resp.Code, http.StatusBadRequest)
		}
	})

	// Test non-existent path
	t.Run("NonExistentPath", func(t *testing.T) {
		nonExistentPath := "/api/v1/rodent/facl" + filepath.Join(tempDir, "does-not-exist.txt")

		req, _ := http.NewRequest("GET", nonExistentPath, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		// Should return an error status
		if resp.Code == http.StatusOK {
			t.Errorf("Expected error status for non-existent path, got %v", resp.Code)
		}
	})
}
