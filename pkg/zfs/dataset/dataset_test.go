/*
 * Copyright 2024 Raamsri Kumar <raam@tinkershack.in> and The StrataSTOR Authors 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */package dataset

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/pool"
	"github.com/stratastor/rodent/pkg/zfs/testutil"
)

func TestDatasetOperations(t *testing.T) {
	// Setup test environment
	env := testutil.NewTestEnv(t, 3)
	defer env.Cleanup()

	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "debug"})
	poolMgr := pool.NewManager(executor)
	datasetMgr := NewManager(executor)

	// Create test pool
	poolName := testutil.GeneratePoolName()
	err := poolMgr.Create(context.Background(), pool.CreateConfig{
		Name: poolName,
		VDevSpec: []pool.VDevSpec{{
			Type:    "raidz",
			Devices: env.GetLoopDevices(),
		}},
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Track pool state for cleanup
	var poolDestroyed bool
	defer func() {
		if !poolDestroyed {
			poolMgr.Destroy(context.Background(), poolName, true)
		}
	}()

	t.Run("Filesystems", func(t *testing.T) {
		fsName := poolName + "/fs1"

		t.Run("Create", func(t *testing.T) {
			err := datasetMgr.CreateFilesystem(context.Background(), FilesystemConfig{
				NameConfig: NameConfig{
					Name: fsName,
				},
				Properties: map[string]string{
					"compression": "on",
					"quota":       "10M",
				},
				Parents:    true,
				DoNotMount: false,
			})
			if err != nil {
				t.Fatalf("failed to create filesystem: %v", err)
			}

			exists, err := datasetMgr.Exists(context.Background(), fsName)
			if err != nil {
				t.Fatalf("failed to check existence: %v", err)
			}
			if !exists {
				t.Fatal("filesystem not found")
			}
		})

		t.Run("Properties", func(t *testing.T) {
			// Get single property
			prop, err := datasetMgr.GetProperty(context.Background(), PropertyConfig{
				NameConfig: NameConfig{
					Name: fsName,
				},
				Property: "compression",
			})
			if err != nil {
				t.Fatalf("failed to get property: %v", err)
			}
			if prop.Datasets[fsName].Properties["compression"].Value != "on" {
				t.Errorf("unexpected compression value: %v",
					prop.Datasets[fsName].Properties["compression"].Value)
			}

			// List all properties
			props, err := datasetMgr.ListProperties(context.Background(), NameConfig{
				Name: fsName,
			})
			if err != nil {
				t.Fatalf("failed to list properties: %v", err)
			}
			if len(props.Datasets[fsName].Properties) == 0 {
				t.Error("no properties returned")
			}

			// Set property
			err = datasetMgr.SetProperty(context.Background(), SetPropertyConfig{
				PropertyConfig: PropertyConfig{
					NameConfig: NameConfig{
						Name: fsName,
					},
					Property: "atime",
				},

				Value: "off",
			})
			if err != nil {
				t.Fatalf("failed to set property: %v", err)
			}
		})

		t.Run("Snapshots", func(t *testing.T) {
			snap1 := "snap1"
			snapName := fsName + "@" + snap1

			// Create snapshot
			err := datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
				NameConfig: NameConfig{
					Name: fsName,
				},
				SnapName:  snap1,
				Recursive: false,
			})
			if err != nil {
				t.Fatalf("failed to create snapshot: %v", err)
			}

			// List snapshots
			snaps, err := datasetMgr.List(context.Background(), ListConfig{
				Type: "snapshot",
				Name: fsName,
			})
			if err != nil {
				t.Fatalf("failed to list snapshots: %v", err)
			}
			if len(snaps.Datasets) == 0 {
				t.Error("no snapshots found")
			}

			// Rollback
			err = datasetMgr.Rollback(context.Background(), RollbackConfig{
				NameConfig: NameConfig{
					Name: snapName,
				},
				DestroyRecent: true,
			})
			if err != nil {
				t.Fatalf("failed to rollback: %v", err)
			}
		})

		t.Run("Clones", func(t *testing.T) {
			cloneName := poolName + "/clone1"

			err := datasetMgr.Clone(context.Background(), CloneConfig{
				NameConfig: NameConfig{
					Name: fsName + "@" + "snap1",
				},
				CloneName: cloneName,
				Parents:   true,
			})
			if err != nil {
				t.Fatalf("failed to create clone: %v", err)
			}

			// Promote clone
			err = datasetMgr.PromoteClone(context.Background(), NameConfig{
				Name: cloneName,
			})
			if err != nil {
				t.Fatalf("failed to promote clone: %v", err)
			}
		})

		t.Run("Inherit", func(t *testing.T) {
			err := datasetMgr.InheritProperty(context.Background(), InheritConfig{
				NamesConfig: NamesConfig{
					Names: []string{fsName},
				},
				Property:  "compression",
				Recursive: true,
			})
			if err != nil {
				t.Fatalf("failed to inherit property: %v", err)
			}
		})

		t.Run("Mount", func(t *testing.T) {
			mountFS := poolName + "/mountfs"
			err := datasetMgr.CreateFilesystem(context.Background(), FilesystemConfig{
				NameConfig: NameConfig{
					Name: mountFS,
				},
				Properties: map[string]string{
					"compression": "on",
					"quota":       "10M",
				},
				Parents:    true,
				DoNotMount: true,
			})
			if err != nil {
				t.Fatalf("failed to create filesystem: %+v", err)
			}

			err = datasetMgr.Mount(context.Background(), MountConfig{
				NameConfig: NameConfig{
					Name: mountFS,
				},
				Force: true,
				// TODO: Check whether mount works with options and temp mount point
				// Options:        []string{"noatime"},
				// TempMountPoint: "/mnt/test-rodent/",
			})
			if err != nil {
				if rerr, ok := err.(*errors.RodentError); ok {
					t.Fatalf("failed to mount: %v\nOutput: %s\nCommand: %s",
						rerr,
						rerr.Metadata["output"],
						rerr.Metadata["command"])
				}
				t.Fatalf("failed to mount: %v", err)
			}

			err = datasetMgr.Unmount(context.Background(), UnmountConfig{
				NameConfig: NameConfig{
					Name: mountFS,
				},
				Force: true,
			})
			if err != nil {
				t.Fatalf("failed to unmount: %v", err)
			}
		})

		t.Run("Rename", func(t *testing.T) {
			newName := poolName + "/fs1_renamed"
			err := datasetMgr.Rename(context.Background(), RenameConfig{
				NameConfig: NameConfig{
					Name: fsName,
				},
				NewName: newName,
				Force:   true,
			})
			if err != nil {
				t.Fatalf("failed to rename: %v", err)
			}

			exists, err := datasetMgr.Exists(context.Background(), newName)
			if err != nil {
				t.Fatalf("failed to check renamed dataset: %v", err)
			}
			if !exists {
				t.Error("renamed dataset not found")
			}
		})

		t.Run("Destroy", func(t *testing.T) {
			err := datasetMgr.Destroy(context.Background(), DestroyConfig{
				NameConfig: NameConfig{
					Name: poolName + "/fs1_renamed",
				},
				RecursiveDestroyDependents: true,
				Force:                      true,
			})
			if err != nil {
				t.Fatalf("failed to destroy: %v", err)
			}
		})
	})

	t.Run("Volumes", func(t *testing.T) {
		// Basic volume creation
		t.Run("CreateVolume", func(t *testing.T) {
			volName := poolName + "/vol1"
			err := datasetMgr.CreateVolume(context.Background(), VolumeConfig{
				NameConfig: NameConfig{
					Name: volName,
				},
				Size: "30M",
				Properties: map[string]string{
					"compression": "on",
					"blocksize":   "8K",
				},
			})
			if err != nil {
				t.Fatalf("failed to create volume: %v", err)
			}

			// Verify volume exists
			exists, err := datasetMgr.Exists(context.Background(), volName)
			if err != nil {
				t.Fatalf("failed to check volume: %v", err)
			}
			if !exists {
				t.Error("volume does not exist")
			}

			// Verify properties
			props, err := datasetMgr.ListProperties(context.Background(), NameConfig{
				Name: volName,
			})
			if err != nil {
				t.Fatalf("failed to get properties: %v", err)
			}

			// Check volsize
			if props.Datasets[volName].Properties["volsize"].Value != "31457280" {
				t.Errorf("volsize = %v, want '31457280'",
					props.Datasets[volName].Properties["volsize"].Value)
			}

			// Check blocksize
			if props.Datasets[volName].Properties["volblocksize"].Value != "8192" {
				t.Errorf("blocksize = %v, want '8192'",
					props.Datasets[volName].Properties["volblocksize"].Value)
			}
		})

		// Sparse volume creation
		t.Run("CreateSparseVolume", func(t *testing.T) {
			volName := poolName + "/vol2"
			err := datasetMgr.CreateVolume(context.Background(), VolumeConfig{
				NameConfig: NameConfig{
					Name: volName,
				},
				Size:   "20M",
				Sparse: true,
			})
			if err != nil {
				t.Fatalf("failed to create sparse volume: %v", err)
			}

			// Verify reservation is not set
			props, err := datasetMgr.ListProperties(context.Background(), NameConfig{
				Name: volName,
			})
			if err != nil {
				t.Fatalf("failed to get properties: %v", err)
			}

			reservation := props.Datasets[volName].Properties["refreservation"].Value
			if reservation != "0" {
				t.Errorf("refreservation = %v, want '0'", reservation)
			}
		})

		// Volume with parent creation
		t.Run("CreateVolumeWithParent", func(t *testing.T) {
			volName := poolName + "/datasets/volumes/vol3"
			err := datasetMgr.CreateVolume(context.Background(), VolumeConfig{
				NameConfig: NameConfig{
					Name: volName,
				},
				Size:    "20M",
				Parents: true,
			})
			if err != nil {
				t.Fatalf("failed to create volume with parents: %v", err)
			}

			// Verify parent datasets were created
			parent := poolName + "/datasets/volumes"
			exists, err := datasetMgr.Exists(context.Background(), parent)
			if err != nil {
				t.Fatalf("failed to check parent: %v", err)
			}
			if !exists {
				t.Error("parent dataset not created")
			}
		})

	})

	t.Run("DiffOperations", func(t *testing.T) {
		// Create a test filesystem for diff operations
		diffFS := poolName + "/difftest"
		err := datasetMgr.CreateFilesystem(context.Background(), FilesystemConfig{
			NameConfig: NameConfig{
				Name: diffFS,
			},
			Properties: map[string]string{
				"compression": "on",
			},
		})
		if err != nil {
			t.Fatalf("failed to create filesystem: %v", err)
		}

		// Create initial snapshot
		snap1 := "snap1"
		err = datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
			NameConfig: NameConfig{Name: diffFS},
			SnapName:   snap1,
		})
		if err != nil {
			t.Fatalf("failed to create first snapshot: %v", err)
		}

		// Create test files
		testFile := "/" + diffFS + "/testfile"
		if err := os.WriteFile(testFile, []byte("test data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Create directory structure
		testDir := "/" + diffFS + "/dir1/subdir"
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}

		// Create symlink
		symlink := "/" + diffFS + "/symlink"
		if err := os.Symlink(testFile, symlink); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Create second snapshot
		snap2 := "snap2"
		err = datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
			NameConfig: NameConfig{Name: diffFS},
			SnapName:   snap2,
		})
		if err != nil {
			t.Fatalf("failed to create second snapshot: %v", err)
		}

		// Test various diff scenarios
		t.Run("SnapshotDiff", func(t *testing.T) {
			result, err := datasetMgr.Diff(context.Background(), DiffConfig{
				NamesConfig: NamesConfig{
					Names: []string{
						diffFS + "@" + snap1,
						diffFS + "@" + snap2,
					},
				},
			})
			if err != nil {
				t.Fatalf("failed to get diff: %v", err)
			}

			// Verify changes are detected
			var (
				foundFile    bool
				foundDir     bool
				foundSymlink bool
			)

			for _, change := range result.Changes {
				switch {
				case change.Path == testFile && change.ChangeType == "+" && change.FileType == "F":
					foundFile = true
				case change.Path == testDir && change.ChangeType == "+" && change.FileType == "/":
					foundDir = true
				case change.Path == symlink && change.ChangeType == "+" && change.FileType == "@":
					foundSymlink = true
				}
			}

			if !foundFile {
				t.Error("file change not detected")
			}
			if !foundDir {
				t.Error("directory change not detected")
			}
			if !foundSymlink {
				t.Error("symlink change not detected")
			}
		})

		t.Run("FileModification", func(t *testing.T) {
			// Modify test file
			if err := os.WriteFile(testFile, []byte("modified data"), 0644); err != nil {
				t.Fatalf("failed to modify test file: %v", err)
			}

			// Create third snapshot
			snap3 := "snap3"
			err = datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
				NameConfig: NameConfig{Name: diffFS},
				SnapName:   snap3,
			})
			if err != nil {
				t.Fatalf("failed to create third snapshot: %v", err)
			}

			// Check modification
			result, err := datasetMgr.Diff(context.Background(), DiffConfig{
				NamesConfig: NamesConfig{
					Names: []string{
						diffFS + "@" + snap2,
						diffFS + "@" + snap3,
					},
				},
			})
			if err != nil {
				t.Fatalf("failed to get diff: %v", err)
			}

			foundMod := false
			for _, change := range result.Changes {
				if change.Path == testFile && change.ChangeType == "M" {
					foundMod = true
					break
				}
			}
			if !foundMod {
				t.Error("file modification not detected")
			}
		})

		t.Run("RenameOperation", func(t *testing.T) {
			// Rename test file
			newPath := "/" + diffFS + "/renamed_file"
			if err := os.Rename(testFile, newPath); err != nil {
				t.Fatalf("failed to rename file: %v", err)
			}

			// Create fourth snapshot
			snap4 := "snap4"
			err = datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
				NameConfig: NameConfig{Name: diffFS},
				SnapName:   snap4,
			})
			if err != nil {
				t.Fatalf("failed to create fourth snapshot: %v", err)
			}

			// Check rename
			result, err := datasetMgr.Diff(context.Background(), DiffConfig{
				NamesConfig: NamesConfig{
					Names: []string{
						diffFS + "@" + "snap3",
						diffFS + "@" + snap4,
					},
				},
			})
			if err != nil {
				t.Fatalf("failed to get diff: %v", err)
			}

			foundRename := false
			for _, change := range result.Changes {
				if change.ChangeType == "R" &&
					change.Path == testFile &&
					change.NewPath == newPath {
					foundRename = true
					break
				}
			}
			if !foundRename {
				t.Error("file rename not detected")
			}
		})

		t.Run("ErrorCases", func(t *testing.T) {
			testCases := []struct {
				name    string
				config  DiffConfig
				wantErr bool
			}{
				{
					name: "missing names",
					config: DiffConfig{
						NamesConfig: NamesConfig{Names: []string{}},
					},
					wantErr: true,
				},
				{
					name: "single name",
					config: DiffConfig{
						NamesConfig: NamesConfig{
							Names: []string{diffFS + "@" + snap1},
						},
					},
					wantErr: true,
				},
				{
					name: "non-existent snapshot",
					config: DiffConfig{
						NamesConfig: NamesConfig{
							Names: []string{
								diffFS + "@" + snap1,
								diffFS + "@nonexistent",
							},
						},
					},
					wantErr: true,
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					_, err := datasetMgr.Diff(context.Background(), tc.config)
					if (err != nil) != tc.wantErr {
						t.Errorf("Diff() error = %v, wantErr %v", err, tc.wantErr)
					}
				})
			}
		})
	})

	t.Run("ShareOperations", func(t *testing.T) {
		hasNFS, hasSMB := testutil.CheckSharingServices(t)
		if !hasNFS && !hasSMB {
			t.Skip("No sharing services available")
		}
		setNFS := "off"
		setSMB := "off"
		if hasNFS {
			setNFS = "rw=192.168.1.0/24,async,root_squash,subtree_check"
		}
		if hasSMB {
			setSMB = "on"
		}

		// Create test filesystem
		shareFS := poolName + "/sharefs"
		err := datasetMgr.CreateFilesystem(context.Background(), FilesystemConfig{
			NameConfig: NameConfig{Name: shareFS},
			Properties: map[string]string{
				"sharenfs": setNFS,
				"sharesmb": setSMB,
			},
		})
		if err != nil {
			t.Fatalf("failed to create filesystem: %v", err)
		}

		// Test share dataset
		t.Run("ShareDataset", func(t *testing.T) {
			err := datasetMgr.Share(context.Background(), ShareConfig{
				Name:     shareFS,
				LoadKeys: true,
			})
			if err != nil {
				// This is expected as the shares are already shared via properties
				if strings.Contains(err.Error(), "already shared") {
					t.Skip("Filesystem already shared. Expected error")
				}
				t.Fatalf("failed to share dataset: %v", err)
			}
		})

		// Test share all datasets
		t.Run("ShareAll", func(t *testing.T) {
			err := datasetMgr.Share(context.Background(), ShareConfig{
				All: true,
			})
			if err != nil {
				t.Fatalf("failed to share all datasets: %v", err)
			}
		})

		// Test unshare dataset
		t.Run("UnshareDataset", func(t *testing.T) {
			err := datasetMgr.Unshare(context.Background(), UnshareConfig{
				Name: shareFS,
			})
			if err != nil {
				t.Fatalf("failed to unshare dataset: %v", err)
			}
		})

		// Test unshare all datasets
		t.Run("UnshareAll", func(t *testing.T) {
			err := datasetMgr.Unshare(context.Background(), UnshareConfig{
				All: true,
			})
			if err != nil {
				// If there are suspended pools, it will fail
				t.Logf("failed to unshare all datasets: %v", err)
			}
		})

		// Test error cases
		t.Run("ErrorCases", func(t *testing.T) {
			// Share without dataset name or -a
			err := datasetMgr.Share(context.Background(), ShareConfig{})
			if err == nil {
				t.Error("expected error when sharing without dataset name or -a")
			}

			// Unshare without dataset name or -a
			err = datasetMgr.Unshare(context.Background(), UnshareConfig{})
			if err == nil {
				t.Error("expected error when unsharing without dataset name or -a")
			}
		})
	})

	// Clean up
	err = poolMgr.Destroy(context.Background(), poolName, true)
	if err != nil {
		t.Fatalf("failed to destroy pool: %v", err)
	}
	poolDestroyed = true
}

func TestPermissionOperations(t *testing.T) {
	// Setup test environment
	env := testutil.NewTestEnv(t, 3)
	defer env.Cleanup()

	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "debug"})
	poolMgr := pool.NewManager(executor)
	datasetMgr := NewManager(executor)

	// Create test user
	testUser := "zfstest"
	if err := exec.Command("sudo", "useradd", "-m", testUser).Run(); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	// Clean up test user
	defer func() {
		if err := exec.Command("sudo", "userdel", "-r", testUser).Run(); err != nil {
			t.Logf("failed to remove test user: %v", err)
		}
	}()

	// Create test pool
	poolName := testutil.GeneratePoolName()
	err := poolMgr.Create(context.Background(), pool.CreateConfig{
		Name: poolName,
		VDevSpec: []pool.VDevSpec{{
			Type:    "raidz",
			Devices: env.GetLoopDevices(),
		}},
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Track pool state for cleanup
	var poolDestroyed bool
	defer func() {
		if !poolDestroyed {
			poolMgr.Destroy(context.Background(), poolName, true)
		}
	}()

	fsName := poolName + "/fs1"
	err = datasetMgr.CreateFilesystem(context.Background(), FilesystemConfig{
		NameConfig: NameConfig{Name: fsName},
	})
	if err != nil {
		t.Fatalf("failed to create filesystem: %v", err)
	}

	t.Run("PermissionSetOperations", func(t *testing.T) {
		// Create permission set
		err := datasetMgr.Allow(context.Background(), AllowConfig{
			NameConfig: NameConfig{Name: fsName},
			SetName:    "@testset",
			Permissions: []string{
				"create", "destroy", "mount", "snapshot",
			},
		})
		if err != nil {
			t.Fatalf("failed to create permission set: %v", err)
		}

		// Verify permission set
		result, err := datasetMgr.ListPermissions(context.Background(), NameConfig{
			Name: fsName,
		})
		if err != nil {
			t.Fatalf("failed to list permissions: %v", err)
		}

		if perms, ok := result.PermissionSets["@testset"]; !ok {
			t.Error("permission set not found")
		} else {
			expected := []string{"create", "destroy", "mount", "snapshot"}
			for _, p := range expected {
				found := false
				for _, actual := range perms {
					if actual == p {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("permission %s not found in set", p)
				}
			}
		}
	})

	t.Run("UserPermissionOperations", func(t *testing.T) {
		// Add negative test case for non-existent user
		t.Run("NonExistentUser", func(t *testing.T) {
			err := datasetMgr.Allow(context.Background(), AllowConfig{
				NameConfig:  NameConfig{Name: fsName},
				Users:       []string{"nonexistentuser"},
				Permissions: []string{"@testset"},
			})
			if err == nil {
				t.Error("expected error for non-existent user")
			} else {
				// Check if it's the right kind of error
				if rerr, ok := err.(*errors.RodentError); !ok ||
					rerr.Code != errors.ZFSCommandFailed {
					t.Errorf("expected ZFSCommandFailed, got %v", err)
				}
				// Also verify the error message mentions invalid user
				if !strings.Contains(err.Error(), "invalid user") {
					t.Errorf("error message should mention invalid user, got: %v", err)
				}
			}
		})

		// Grant user permissions
		err := datasetMgr.Allow(context.Background(), AllowConfig{
			NameConfig: NameConfig{Name: fsName},
			Users:      []string{testUser},
			Permissions: []string{
				"@testset", "rollback",
			},
			Local:      true,
			Descendent: true,
		})
		if err != nil {
			t.Fatalf("failed to grant user permissions: %v", err)
		}

		// Verify permissions
		result, err := datasetMgr.ListPermissions(context.Background(), NameConfig{
			Name: fsName,
		})
		if err != nil {
			t.Fatalf("failed to list permissions: %v", err)
		}

		if perms, ok := result.LocalDescendent["user "+testUser]; !ok {
			t.Error("user permissions not found")
		} else {
			expected := []string{"@testset", "rollback"}
			for _, p := range expected {
				found := false
				for _, actual := range perms {
					if actual == p {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("permission %s not found for user", p)
				}
			}
		}
	})

	t.Run("CreateTimePermissions", func(t *testing.T) {
		// Set create time permissions
		err := datasetMgr.Allow(context.Background(), AllowConfig{
			NameConfig:  NameConfig{Name: fsName},
			Create:      true,
			Permissions: []string{"destroy"},
		})
		if err != nil {
			t.Fatalf("failed to set create time permissions: %v", err)
		}

		// Verify permissions
		result, err := datasetMgr.ListPermissions(context.Background(), NameConfig{
			Name: fsName,
		})
		if err != nil {
			t.Fatalf("failed to list permissions: %v", err)
		}

		found := false
		for _, p := range result.CreateTime {
			if p == "destroy" {
				found = true
				break
			}
		}
		if !found {
			t.Error("create time permission not found")
		}
	})

	t.Run("UnallowOperations", func(t *testing.T) {
		// Remove permission set
		err := datasetMgr.Unallow(context.Background(), UnallowConfig{
			NameConfig: NameConfig{Name: fsName},
			SetName:    "@testset",
		})
		if err != nil {
			t.Fatalf("failed to remove permission set: %v", err)
		}

		// Remove user permissions
		err = datasetMgr.Unallow(context.Background(), UnallowConfig{
			NameConfig: NameConfig{Name: fsName},
			Users:      []string{"testuser"},
			Local:      true,
			Descendent: true,
		})
		if err != nil {
			t.Fatalf("failed to remove user permissions: %v", err)
		}

		// Verify permissions were removed
		result, err := datasetMgr.ListPermissions(context.Background(), NameConfig{
			Name: fsName,
		})
		if err != nil {
			t.Fatalf("failed to list permissions: %v", err)
		}

		if _, ok := result.PermissionSets["@testset"]; ok {
			t.Error("permission set still exists")
		}
		if _, ok := result.LocalDescendent["user testuser"]; ok {
			t.Error("user permissions still exist")
		}
	})

	t.Run("ErrorCases", func(t *testing.T) {
		testCases := []struct {
			name    string
			config  AllowConfig
			wantErr bool
		}{
			{
				name: "invalid permission set name",
				config: AllowConfig{
					NameConfig:  NameConfig{Name: fsName},
					SetName:     "testset", // Missing @ prefix
					Permissions: []string{"create"},
				},
				wantErr: true,
			},
			{
				name: "mutually exclusive flags",
				config: AllowConfig{
					NameConfig: NameConfig{Name: fsName},
					Users:      []string{"user1"},
					Groups:     []string{"group1"},
					Everyone:   true,
				},
				wantErr: true,
			},
			{
				name: "invalid permission name",
				config: AllowConfig{
					NameConfig:  NameConfig{Name: fsName},
					Users:       []string{"testuser"},
					Permissions: []string{"invalid_perm"},
				},
				wantErr: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := datasetMgr.Allow(context.Background(), tc.config)
				if (err != nil) != tc.wantErr {
					t.Errorf("Allow() error = %v, wantErr %v", err, tc.wantErr)
				}
			})
		}
	})

	// Clean up
	err = poolMgr.Destroy(context.Background(), poolName, true)
	if err != nil {
		t.Fatalf("failed to destroy pool: %v", err)
	}
	poolDestroyed = true
}
