// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestLogger(t *testing.T) logger.Logger {
	testLogger, err := logger.New(logger.Config{LogLevel: "debug"})
	require.NoError(t, err)
	return testLogger
}

func setupTestHandler(t *testing.T) (*SystemHandler, *gin.Engine) {
	// Create test logger
	testLogger := createTestLogger(t)
	
	// Create system manager
	systemManager := system.NewManager(testLogger)
	
	// Create handler
	handler := NewSystemHandler(systemManager, testLogger)
	
	// Create Gin engine in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Register routes
	v1 := router.Group("/api/v1/system")
	handler.RegisterRoutes(v1)
	
	return handler, router
}

func TestSystemHandler_GetSystemInfo(t *testing.T) {
	handler, router := setupTestHandler(t)
	_ = handler // Avoid unused variable
	
	req, err := http.NewRequest("GET", "/api/v1/system/info", nil)
	require.NoError(t, err)
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	t.Logf("Response Status: %d", w.Code)
	t.Logf("Response Body: %s", w.Body.String())
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.True(t, response.Success)
	assert.NotNil(t, response.Result)
}

func TestSystemHandler_GetOSInfo(t *testing.T) {
	handler, router := setupTestHandler(t)
	_ = handler
	
	req, err := http.NewRequest("GET", "/api/v1/system/info/os", nil)
	require.NoError(t, err)
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	t.Logf("Response Status: %d", w.Code)
	t.Logf("Response Body: %s", w.Body.String())
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.True(t, response.Success)
	assert.NotNil(t, response.Result)
}

func TestSystemHandler_GetHostname(t *testing.T) {
	handler, router := setupTestHandler(t)
	_ = handler
	
	req, err := http.NewRequest("GET", "/api/v1/system/hostname", nil)
	require.NoError(t, err)
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	t.Logf("Response Status: %d", w.Code)
	t.Logf("Response Body: %s", w.Body.String())
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.True(t, response.Success)
	assert.NotNil(t, response.Result)
	
	// Should contain hostname
	result, ok := response.Result.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, result, "hostname")
	t.Logf("Hostname: %v", result["hostname"])
}

func TestSystemHandler_SetHostname_ValidationError(t *testing.T) {
	handler, router := setupTestHandler(t)
	_ = handler
	
	// Test with invalid hostname
	reqBody := system.SetHostnameRequest{
		Hostname: "", // Invalid - empty
	}
	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)
	
	req, err := http.NewRequest("PUT", "/api/v1/system/hostname", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	t.Logf("Response Status: %d", w.Code)
	t.Logf("Response Body: %s", w.Body.String())
	
	assert.NotEqual(t, http.StatusOK, w.Code) // Should return validation error
	
	var response APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
}

func TestSystemHandler_GetUsers(t *testing.T) {
	handler, router := setupTestHandler(t)
	_ = handler
	
	req, err := http.NewRequest("GET", "/api/v1/system/users", nil)
	require.NoError(t, err)
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	t.Logf("Response Status: %d", w.Code)
	t.Logf("Response Body: %s", w.Body.String())
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.True(t, response.Success)
	assert.NotNil(t, response.Result)
	
	// Should contain users and count
	result, ok := response.Result.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, result, "users")
	assert.Contains(t, result, "count")
	t.Logf("User count: %v", result["count"])
}

func TestSystemHandler_GetUser(t *testing.T) {
	handler, router := setupTestHandler(t)
	_ = handler
	
	// Test getting root user
	req, err := http.NewRequest("GET", "/api/v1/system/users/root", nil)
	require.NoError(t, err)
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	t.Logf("Response Status: %d", w.Code)
	t.Logf("Response Body: %s", w.Body.String())
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.True(t, response.Success)
	assert.NotNil(t, response.Result)
}

func TestSystemHandler_GetUser_NotFound(t *testing.T) {
	handler, router := setupTestHandler(t)
	_ = handler
	
	// Test getting non-existent user
	req, err := http.NewRequest("GET", "/api/v1/system/users/nonexistentuser12345", nil)
	require.NoError(t, err)
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	t.Logf("Response Status: %d", w.Code)
	t.Logf("Response Body: %s", w.Body.String())
	
	assert.NotEqual(t, http.StatusOK, w.Code) // Should return error
	
	var response APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
}

func TestSystemHandler_GetPowerStatus(t *testing.T) {
	handler, router := setupTestHandler(t)
	_ = handler
	
	req, err := http.NewRequest("GET", "/api/v1/system/power/status", nil)
	require.NoError(t, err)
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	t.Logf("Response Status: %d", w.Code)
	t.Logf("Response Body: %s", w.Body.String())
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.True(t, response.Success)
	assert.NotNil(t, response.Result)
}