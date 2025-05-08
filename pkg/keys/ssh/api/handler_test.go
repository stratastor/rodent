// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/keys/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateRandomID() string {
	// Generate a random UUID
	id := uuid.New().String()
	// Remove hyphens and take first 12 chars for shorter ID
	id = strings.ReplaceAll(id, "-", "")[:12]
	return fmt.Sprintf("peer-%s", id)
}

func setupTestAPI(t *testing.T) (*gin.Engine, *SSHKeyHandler, string, func()) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "ssh-key-api-test-")
	require.NoError(t, err)

	// Set environment variable to use this directory
	oldDirPath := os.Getenv("RODENT_SSH_DIR_PATH")
	os.Setenv("RODENT_SSH_DIR_PATH", tempDir)

	// Setup Gin in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create logger and handler
	testLogger, err := logger.New(logger.Config{LogLevel: "debug"})
	require.NoError(t, err)

	handler, err := NewSSHKeyHandler(testLogger)
	require.NoError(t, err)

	// Register routes
	group := router.Group("/api/v1/keys/ssh")
	handler.RegisterRoutes(group)

	// Return cleanup function
	cleanup := func() {
		os.RemoveAll(tempDir)
		os.Setenv("RODENT_SSH_DIR_PATH", oldDirPath)
	}

	return router, handler, tempDir, cleanup
}

func TestGenerateKeyPairAPI(t *testing.T) {
	router, _, _, cleanup := setupTestAPI(t)
	defer cleanup()

	// Generate random peering ID
	peeringID := generateRandomID()

	// Create request payload
	payload := ssh.GenerateKeyPairRequest{
		PeeringID: peeringID,
		Type:      ssh.KeyPairTypeED25519,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest("POST", "/api/v1/keys/ssh/keypair", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response ssh.GenerateKeyPairResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Validate response
	assert.Equal(t, peeringID, response.PeeringID)
	assert.Equal(t, ssh.KeyPairTypeED25519, response.Type)
	assert.NotEmpty(t, response.PublicKey)
	assert.NotEmpty(t, response.PrivateKeyPath)
	assert.NotEmpty(t, response.PublicKeyPath)

	// Now try to remove it via API
	delreq := httptest.NewRequest("DELETE", "/api/v1/keys/ssh/keypair/"+peeringID, nil)
	delWrite := httptest.NewRecorder()
	router.ServeHTTP(delWrite, delreq)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRemoveKeyPairAPI(t *testing.T) {
	router, handler, _, cleanup := setupTestAPI(t)
	defer cleanup()

	// Generate a random peering ID
	peeringID := generateRandomID()

	// First create a key pair directly using the manager
	_, err := handler.manager.GenerateKeyPair(context.TODO(), peeringID, ssh.KeyPairTypeED25519)
	require.NoError(t, err)

	// Now try to remove it via API
	req := httptest.NewRequest("DELETE", "/api/v1/keys/ssh/keypair/"+peeringID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify it's gone by checking directly with the manager
	has, err := handler.manager.HasKeyPair(peeringID)
	require.NoError(t, err)
	assert.False(t, has)
}
