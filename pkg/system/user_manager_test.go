// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUserManager(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewUserManager(logger)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.executor)
	t.Logf("User manager created successfully")
}

func TestUserManager_GetUsers(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewUserManager(logger)
	ctx := context.Background()

	users, err := manager.GetUsers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, users)

	t.Logf("Found %d users", len(users))

	// Should have at least root user
	foundRoot := false
	for _, user := range users {
		t.Logf("User: %s (UID: %d, GID: %d, Home: %s, Shell: %s)",
			user.Username, user.UID, user.GID, user.HomeDir, user.Shell)
		if user.Username == "root" {
			foundRoot = true
			assert.Equal(t, 0, user.UID)
		}
	}
	assert.True(t, foundRoot, "Should find root user")
}

func TestUserManager_GetUser(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewUserManager(logger)
	ctx := context.Background()

	// Test getting root user
	user, err := manager.GetUser(ctx, "root")
	require.NoError(t, err)
	require.NotNil(t, user)

	t.Logf("Root user: UID=%d, GID=%d, Home=%s, Shell=%s, Locked=%v",
		user.UID, user.GID, user.HomeDir, user.Shell, user.Locked)

	assert.Equal(t, "root", user.Username)
	assert.Equal(t, 0, user.UID)
	assert.NotEmpty(t, user.HomeDir)
	assert.NotEmpty(t, user.Shell)
}

func TestUserManager_GetUser_NonExistent(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewUserManager(logger)
	ctx := context.Background()

	user, err := manager.GetUser(ctx, "nonexistentuser12345")
	assert.Error(t, err)
	assert.Nil(t, user)
	t.Logf("Expected error for non-existent user: %v", err)
}

func TestUserManager_GetGroups(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewUserManager(logger)
	ctx := context.Background()

	groups, err := manager.GetGroups(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, groups)

	t.Logf("Found %d groups", len(groups))

	// Should have at least root group
	foundRoot := false
	for _, group := range groups {
		t.Logf("Group: %s (GID: %d, Members: %v)",
			group.Name, group.GID, group.Members)
		if group.Name == "root" {
			foundRoot = true
			assert.Equal(t, 0, group.GID)
		}
	}
	assert.True(t, foundRoot, "Should find root group")
}

func TestUserManager_ValidateCreateUserRequest(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewUserManager(logger)

	tests := []struct {
		name    string
		request CreateUserRequest
		wantErr bool
	}{
		{
			name: "valid user request",
			request: CreateUserRequest{
				Username: "testuser",
				FullName: "Test User",
				Shell:    "/bin/bash",
			},
			wantErr: false,
		},
		{
			name: "empty username",
			request: CreateUserRequest{
				Username: "",
			},
			wantErr: true,
		},
		{
			name: "valid username with underscore",
			request: CreateUserRequest{
				Username: "test_user",
			},
			wantErr: false,
		},
		{
			name: "invalid username with uppercase",
			request: CreateUserRequest{
				Username: "TestUser",
			},
			wantErr: true,
		},
		{
			name: "invalid username starting with number",
			request: CreateUserRequest{
				Username: "1testuser",
			},
			wantErr: true,
		},
		{
			name: "username too long",
			request: CreateUserRequest{
				Username: "averylongusernamethatexceedsthemaximumlengthallowed",
			},
			wantErr: true,
		},
		{
			name: "invalid shell",
			request: CreateUserRequest{
				Username: "testuser",
				Shell:    "/nonexistent/shell",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateCreateUserRequest(tt.request)
			t.Logf("Request: %+v, Valid: %v, Error: %v", tt.request, err == nil, err)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUserManager_ValidateCreateGroupRequest(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewUserManager(logger)

	tests := []struct {
		name    string
		request CreateGroupRequest
		wantErr bool
	}{
		{
			name: "valid group request",
			request: CreateGroupRequest{
				Name: "testgroup",
			},
			wantErr: false,
		},
		{
			name: "empty group name",
			request: CreateGroupRequest{
				Name: "",
			},
			wantErr: true,
		},
		{
			name: "valid group name with underscore",
			request: CreateGroupRequest{
				Name: "test_group",
			},
			wantErr: false,
		},
		{
			name: "invalid group name with uppercase",
			request: CreateGroupRequest{
				Name: "TestGroup",
			},
			wantErr: true,
		},
		{
			name: "invalid group name starting with number",
			request: CreateGroupRequest{
				Name: "1testgroup",
			},
			wantErr: true,
		},
		{
			name: "group name too long",
			request: CreateGroupRequest{
				Name: "averylonggroupnamethatexceedsthemaximumlengthallowed",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateCreateGroupRequest(tt.request)
			t.Logf("Request: %+v, Valid: %v, Error: %v", tt.request, err == nil, err)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Integration tests - only run when explicitly enabled
func TestUserManager_CreateDeleteUser_Integration(t *testing.T) {
	// Skip if not root or not running integration tests
	// if os.Getuid() != 0 {
	// 	t.Skip("Skipping integration test - requires root privileges")
	// }
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set RUN_INTEGRATION_TESTS=true to run")
	}

	logger := createTestLogger(t)
	manager := NewUserManager(logger)
	ctx := context.Background()

	testUsername := "testuser12345"

	// Clean up in case test failed before
	t.Cleanup(func() {
		_ = manager.DeleteUser(ctx, testUsername)
		t.Logf("Cleanup: attempted to delete user %s", testUsername)
	})

	// Create user
	createReq := CreateUserRequest{
		Username: testUsername,
		FullName: "Test User",
		Shell:    "/bin/bash",
		Password: "testpassword123",
	}

	t.Logf("Creating user: %s", testUsername)
	err := manager.CreateUser(ctx, createReq)
	require.NoError(t, err)

	// Verify user was created
	user, err := manager.GetUser(ctx, testUsername)
	require.NoError(t, err)
	require.NotNil(t, user)

	t.Logf("Created user: %+v", user)
	assert.Equal(t, testUsername, user.Username)
	assert.NotEmpty(t, user.HomeDir)

	// Delete user
	t.Logf("Deleting user: %s", testUsername)
	err = manager.DeleteUser(ctx, testUsername)
	require.NoError(t, err)

	// Verify user was deleted
	user, err = manager.GetUser(ctx, testUsername)
	assert.Error(t, err)
	assert.Nil(t, user)
	t.Logf("User successfully deleted")
}

func TestUserManager_ProtectedUsersDeletion(t *testing.T) {
	logger := createTestLogger(t)
	manager := NewUserManager(logger)
	ctx := context.Background()

	protectedUsers := []string{"ubuntu", "rodent", "strata", "root"}

	for _, username := range protectedUsers {
		t.Run("protected_user_"+username, func(t *testing.T) {
			err := manager.DeleteUser(ctx, username)
			assert.Error(t, err)
			t.Logf("Protected user %s cannot be deleted: %v", username, err)
		})
	}
}
