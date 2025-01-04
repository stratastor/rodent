package dataset

import (
	"context"
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

	// Clean up
	err = poolMgr.Destroy(context.Background(), poolName, true)
	if err != nil {
		t.Fatalf("failed to destroy pool: %v", err)
	}
	poolDestroyed = true
}

// 	// Additional negative test case
// 	t.Run("VolumeWithoutSize", func(t *testing.T) {
// 		volName := poolName + "/vol2"
// 		err := datasetMgr.Create(context.Background(), CreateConfig{
// 			Name: volName,
// 			Type: "volume",
// 			Properties: map[string]string{
// 				"volblocksize": "8K",
// 				// Missing volsize property
// 			},
// 		})
// 		if err == nil {
// 			t.Error("expected error when creating volume without size")
// 		}
// 	})
