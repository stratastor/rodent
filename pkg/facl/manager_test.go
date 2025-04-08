package facl

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/errors"
)

// TODO: Fix failing test cases

// setupTestDir creates a temporary directory structure for testing
func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "facl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create subdirectories
	subDirs := []string{"dir1", "dir2", "dir1/subdir1"}
	for _, dir := range subDirs {
		err := os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		if err != nil {
			os.RemoveAll(tempDir)
			t.Fatalf("Failed to create subdirectory %s: %v", dir, err)
		}
	}

	// Create some test files
	files := []string{
		"file1.txt",
		"dir1/file2.txt",
		"dir2/file3.txt",
		"dir1/subdir1/file4.txt",
	}

	for _, file := range files {
		f, err := os.Create(filepath.Join(tempDir, file))
		if err != nil {
			os.RemoveAll(tempDir)
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
		f.WriteString("test content")
		f.Close()
	}

	// Return cleanup function
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// isACLSupported checks if the filesystem supports ACLs
func isACLSupported(t *testing.T, path string) bool {
	t.Helper()

	// Check if setfacl and getfacl are available
	_, err := os.Stat(BinSetfacl)
	if err != nil {
		t.Logf("%s not found, skipping test", BinSetfacl)
		return false
	}

	_, err = os.Stat(BinGetfacl)
	if err != nil {
		t.Logf("%s not found, skipping test", BinGetfacl)
		return false
	}

	// Try to set a simple ACL
	ctx := context.Background()
	log := common.Log
	_, err = command.ExecCommand(ctx, log, BinSetfacl, "-m", "u:root:rwx", path)
	if err != nil {
		t.Logf("ACLs not supported on test directory: %v", err)
		return false
	}

	return true
}

func TestACLManager_GetACL(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Skip if ACLs not supported
	if !isACLSupported(t, tempDir) {
		t.Skip("ACLs not supported on test filesystem")
	}

	// Create ACL manager
	log := common.Log
	manager := NewACLManager(log, nil)

	// Test cases
	testCases := []struct {
		name      string
		path      string
		recursive bool
		wantErr   bool
	}{
		{
			name:      "Root directory",
			path:      tempDir,
			recursive: false,
			wantErr:   false,
		},
		{
			name:      "Root directory recursive",
			path:      tempDir,
			recursive: true,
			wantErr:   false,
		},
		{
			name:      "Subdirectory",
			path:      filepath.Join(tempDir, "dir1"),
			recursive: false,
			wantErr:   false,
		},
		{
			name:      "File",
			path:      filepath.Join(tempDir, "file1.txt"),
			recursive: false,
			wantErr:   false,
		},
		{
			name:      "Non-existent path",
			path:      filepath.Join(tempDir, "nonexistent"),
			recursive: false,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := manager.GetACL(context.Background(), ACLListConfig{
				Path:      tc.path,
				Recursive: tc.recursive,
			})

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Basic validation
			if result.Path != tc.path {
				t.Errorf("Wrong path in result: got %s, want %s", result.Path, tc.path)
			}

			if result.Type != ACLTypePOSIX {
				t.Errorf("Wrong ACL type: got %s, want %s", result.Type, ACLTypePOSIX)
			}

			// Validate entries (at least some base entries should be present)
			if len(result.Entries) == 0 {
				t.Error("Expected at least some ACL entries, got none")
			}

			// If recursive, check that we have children
			if tc.recursive && os.FileInfo.IsDir(func() os.FileInfo {
				info, _ := os.Stat(tc.path)
				return info
			}()) {
				if len(result.Children) == 0 {
					t.Error("Expected children for recursive listing, got none")
				}
			}
		})
	}
}

func TestACLManager_SetAndModifyACL(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Skip if ACLs not supported
	if !isACLSupported(t, tempDir) {
		t.Skip("ACLs not supported on test filesystem")
	}

	// Create ACL manager
	log := common.Log
	manager := NewACLManager(log, nil)

	// Test path
	testPath := filepath.Join(tempDir, "file1.txt")

	// Initial ACL set
	t.Run("SetACL", func(t *testing.T) {
		err := manager.SetACL(context.Background(), ACLConfig{
			Path: testPath,
			Type: ACLTypePOSIX,
			Entries: []ACLEntry{
				{
					Type:        EntryUser,
					Principal:   "nobody",
					Permissions: []PermissionType{PermReadData},
				},
				{
					Type:        EntryGroup,
					Principal:   "nogroup",
					Permissions: []PermissionType{PermReadData, PermWriteData},
				},
			},
		})

		if err != nil {
			t.Errorf("Failed to set ACL: %v", err)
			return
		}

		// Verify the ACL was set
		result, err := manager.GetACL(context.Background(), ACLListConfig{
			Path: testPath,
		})

		if err != nil {
			t.Errorf("Failed to get ACL after setting: %v", err)
			return
		}

		// Look for our entries
		foundUser := false
		foundGroup := false

		for _, entry := range result.Entries {
			if entry.Type == EntryUser && entry.Principal == "nobody" {
				foundUser = true
				if len(entry.Permissions) != 1 || entry.Permissions[0] != PermReadData {
					t.Errorf("Wrong permissions for user nobody: %v", entry.Permissions)
				}
			}
			if entry.Type == EntryGroup && entry.Principal == "nogroup" {
				foundGroup = true
				perms := make(map[PermissionType]bool)
				for _, p := range entry.Permissions {
					perms[p] = true
				}
				if !perms[PermReadData] || !perms[PermWriteData] {
					t.Errorf("Wrong permissions for group nogroup: %v", entry.Permissions)
				}
			}
		}

		if !foundUser {
			t.Error("User entry not found after setting")
		}
		if !foundGroup {
			t.Error("Group entry not found after setting")
		}
	})

	// Modify ACL
	t.Run("ModifyACL", func(t *testing.T) {
		err := manager.ModifyACL(context.Background(), ACLConfig{
			Path: testPath,
			Type: ACLTypePOSIX,
			Entries: []ACLEntry{
				{
					Type:        EntryUser,
					Principal:   "nobody",
					Permissions: []PermissionType{PermReadData, PermExecute},
				},
			},
		})

		if err != nil {
			t.Errorf("Failed to modify ACL: %v", err)
			return
		}

		// Verify the ACL was modified
		result, err := manager.GetACL(context.Background(), ACLListConfig{
			Path: testPath,
		})

		if err != nil {
			t.Errorf("Failed to get ACL after modification: %v", err)
			return
		}

		// Look for modified entry
		foundModified := false
		for _, entry := range result.Entries {
			if entry.Type == EntryUser && entry.Principal == "nobody" {
				foundModified = true
				perms := make(map[PermissionType]bool)
				for _, p := range entry.Permissions {
					perms[p] = true
				}
				if !perms[PermReadData] || !perms[PermExecute] {
					t.Errorf("Wrong permissions after modification: %v", entry.Permissions)
				}
			}
		}

		if !foundModified {
			t.Error("Modified entry not found")
		}
	})
}

func TestACLManager_RemoveACL(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Skip if ACLs not supported
	if !isACLSupported(t, tempDir) {
		t.Skip("ACLs not supported on test filesystem")
	}

	// Create ACL manager
	log := common.Log
	manager := NewACLManager(log, nil)

	// Test path
	testPath := filepath.Join(tempDir, "file1.txt")

	// First set some ACLs
	err := manager.SetACL(context.Background(), ACLConfig{
		Path: testPath,
		Type: ACLTypePOSIX,
		Entries: []ACLEntry{
			{
				Type:        EntryUser,
				Principal:   "nobody",
				Permissions: []PermissionType{PermReadData, PermWriteData},
			},
			{
				Type:        EntryGroup,
				Principal:   "nogroup",
				Permissions: []PermissionType{PermReadData, PermExecute},
			},
		},
	})

	if err != nil {
		t.Fatalf("Failed to set up ACLs for removal test: %v", err)
	}

	// Now remove specific ACL
	t.Run("RemoveSpecificACL", func(t *testing.T) {
		err := manager.RemoveACL(context.Background(), ACLRemoveConfig{
			ACLConfig: ACLConfig{
				Path: testPath,
				Type: ACLTypePOSIX,
				Entries: []ACLEntry{
					{
						Type:      EntryUser,
						Principal: "nobody",
					},
				},
			},
		})

		if err != nil {
			t.Errorf("Failed to remove ACL: %v", err)
			return
		}

		// Verify the ACL was removed
		result, err := manager.GetACL(context.Background(), ACLListConfig{
			Path: testPath,
		})

		if err != nil {
			t.Errorf("Failed to get ACL after removal: %v", err)
			return
		}

		// Make sure nobody entry is gone
		for _, entry := range result.Entries {
			if entry.Type == EntryUser && entry.Principal == "nobody" {
				t.Error("User entry found after removal")
			}
		}
	})

	// Test removing all ACLs
	t.Run("RemoveAllACLs", func(t *testing.T) {
		// First make sure we still have at least the group entry
		beforeResult, err := manager.GetACL(context.Background(), ACLListConfig{
			Path: testPath,
		})
		if err != nil {
			t.Fatalf("Failed to get ACLs before removal: %v", err)
		}

		foundBefore := false
		for _, entry := range beforeResult.Entries {
			if entry.Type == EntryGroup && entry.Principal == "nogroup" {
				foundBefore = true
				break
			}
		}

		if !foundBefore {
			t.Fatal("Expected to find group entry before removal")
		}

		// Now remove all ACLs
		err = manager.RemoveACL(context.Background(), ACLRemoveConfig{
			ACLConfig: ACLConfig{
				Path: testPath,
				Type: ACLTypePOSIX,
			},
			RemoveAllXattr: true,
		})

		if err != nil {
			t.Errorf("Failed to remove all ACLs: %v", err)
			return
		}

		// Verify ACLs were removed
		afterResult, err := manager.GetACL(context.Background(), ACLListConfig{
			Path: testPath,
		})
		if err != nil {
			t.Errorf("Failed to get ACLs after removal: %v", err)
			return
		}

		// Should only have base entries
		for _, entry := range afterResult.Entries {
			if (entry.Type == EntryUser && entry.Principal != "") ||
				(entry.Type == EntryGroup && entry.Principal != "") {
				t.Errorf("Found named entry after removing all ACLs: %+v", entry)
			}
		}
	})
}

func TestACLManager_DefaultACLs(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Skip if ACLs not supported
	if !isACLSupported(t, tempDir) {
		t.Skip("ACLs not supported on test filesystem")
	}

	// Create ACL manager
	log := common.Log
	manager := NewACLManager(log, nil)

	// Test path - must be a directory for default ACLs
	testPath := filepath.Join(tempDir, "dir1")

	// Set default ACLs
	err := manager.SetACL(context.Background(), ACLConfig{
		Path: testPath,
		Type: ACLTypePOSIX,
		Entries: []ACLEntry{
			{
				Type:        EntryUser,
				Principal:   "nobody",
				Permissions: []PermissionType{PermReadData, PermExecute},
				IsDefault:   true,
			},
		},
	})

	if err != nil {
		t.Fatalf("Failed to set default ACL: %v", err)
	}

	// Verify default ACL was set
	result, err := manager.GetACL(context.Background(), ACLListConfig{
		Path: testPath,
	})

	if err != nil {
		t.Fatalf("Failed to get ACL after setting default: %v", err)
	}

	// Look for default entry
	foundDefault := false
	for _, entry := range result.Entries {
		if entry.IsDefault && entry.Type == EntryUser && entry.Principal == "nobody" {
			foundDefault = true
			perms := make(map[PermissionType]bool)
			for _, p := range entry.Permissions {
				perms[p] = true
			}
			if !perms[PermReadData] || !perms[PermExecute] {
				t.Errorf("Wrong permissions for default ACL: %v", entry.Permissions)
			}
		}
	}

	if !foundDefault {
		t.Error("Default entry not found")
	}

	// Remove default ACLs
	err = manager.RemoveACL(context.Background(), ACLRemoveConfig{
		ACLConfig: ACLConfig{
			Path: testPath,
			Type: ACLTypePOSIX,
		},
		RemoveDefault: true,
	})

	if err != nil {
		t.Fatalf("Failed to remove default ACLs: %v", err)
	}

	// Verify default ACLs were removed
	result, err = manager.GetACL(context.Background(), ACLListConfig{
		Path: testPath,
	})

	if err != nil {
		t.Fatalf("Failed to get ACL after removing default: %v", err)
	}

	// Should not find any default entries
	for _, entry := range result.Entries {
		if entry.IsDefault {
			t.Errorf("Found default entry after removal: %+v", entry)
		}
	}
}

func TestACLManager_ErrorCases(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Skip if ACLs not supported
	if !isACLSupported(t, tempDir) {
		t.Skip("ACLs not supported on test filesystem")
	}

	// Create ACL manager
	log := common.Log
	manager := NewACLManager(log, nil)

	// Test cases for validation errors
	testCases := []struct {
		name    string
		config  ACLConfig
		wantErr errors.ErrorCode
	}{
		{
			name: "Empty path",
			config: ACLConfig{
				Path: "",
				Type: ACLTypePOSIX,
				Entries: []ACLEntry{
					{
						Type:        EntryUser,
						Principal:   "nobody",
						Permissions: []PermissionType{PermReadData},
					},
				},
			},
			wantErr: errors.FACLInvalidInput,
		},
		{
			name: "Invalid path characters",
			config: ACLConfig{
				Path: filepath.Join(tempDir, "file1.txt; rm -rf /"),
				Type: ACLTypePOSIX,
				Entries: []ACLEntry{
					{
						Type:        EntryUser,
						Principal:   "nobody",
						Permissions: []PermissionType{PermReadData},
					},
				},
			},
			wantErr: errors.FACLInvalidInput,
		},
		{
			name: "Non-existent path",
			config: ACLConfig{
				Path: filepath.Join(tempDir, "non-existent-file"),
				Type: ACLTypePOSIX,
				Entries: []ACLEntry{
					{
						Type:        EntryUser,
						Principal:   "nobody",
						Permissions: []PermissionType{PermReadData},
					},
				},
			},
			wantErr: errors.FACLPathNotFound,
		},
		{
			name: "No entries",
			config: ACLConfig{
				Path:    filepath.Join(tempDir, "file1.txt"),
				Type:    ACLTypePOSIX,
				Entries: []ACLEntry{},
			},
			wantErr: errors.FACLInvalidInput,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name+"_SetACL", func(t *testing.T) {
			err := manager.SetACL(context.Background(), tc.config)

			if err == nil {
				t.Errorf("Expected error %v but got none", tc.wantErr)
				return
			}

			errCode, ok := errors.GetCode(err)
			if !ok {
				t.Errorf("Expected error code %v but got none", tc.wantErr)
				return
			}

			// Check error code
			if errCode != tc.wantErr {
				t.Errorf("Wrong error code: got %v, want %v", errCode, tc.wantErr)
			}
		})

		t.Run(tc.name+"_ModifyACL", func(t *testing.T) {
			err := manager.ModifyACL(context.Background(), tc.config)

			if err == nil {
				t.Errorf("Expected error %v but got none", tc.wantErr)
				return
			}

			errCode, ok := errors.GetCode(err)
			if !ok {
				t.Errorf("Expected error code %v but got none", tc.wantErr)
				return
			}
			// Check error code
			if errCode != tc.wantErr {
				t.Errorf("Wrong error code: got %v, want %v", errCode, tc.wantErr)
			}
		})
	}

	// Test removal error cases
	removeTestCases := []struct {
		name    string
		config  ACLRemoveConfig
		wantErr errors.ErrorCode
	}{
		{
			name: "Empty path",
			config: ACLRemoveConfig{
				ACLConfig: ACLConfig{
					Path: "",
					Type: ACLTypePOSIX,
					Entries: []ACLEntry{
						{
							Type:      EntryUser,
							Principal: "nobody",
						},
					},
				},
			},
			wantErr: errors.FACLInvalidInput,
		},
		{
			name: "No entries to remove",
			config: ACLRemoveConfig{
				ACLConfig: ACLConfig{
					Path:    filepath.Join(tempDir, "file1.txt"),
					Type:    ACLTypePOSIX,
					Entries: []ACLEntry{},
				},
			},
			wantErr: errors.FACLInvalidInput,
		},
	}

	for _, tc := range removeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			err := manager.RemoveACL(context.Background(), tc.config)

			if err == nil {
				t.Errorf("Expected error %v but got none", tc.wantErr)
				return
			}

			errCode, ok := errors.GetCode(err)
			if !ok {
				t.Errorf("Expected error code %v but got none", tc.wantErr)
				return
			}
			// Check error code
			if errCode != tc.wantErr {
				t.Errorf("Wrong error code: got %v, want %v", errCode, tc.wantErr)
			}
		})
	}
}
