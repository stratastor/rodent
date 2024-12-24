package dataset

import (
	"context"
	"testing"

	"github.com/stratastor/logger"
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
	poolCfg := pool.CreateConfig{
		Name: poolName,
		VDevSpec: []pool.VDevSpec{
			{
				Type:    "raidz",
				Devices: env.GetLoopDevices(),
			},
		},
	}

	err := poolMgr.Create(context.Background(), poolCfg)
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
			err := datasetMgr.Create(context.Background(), CreateConfig{
				Name: fsName,
				Type: "filesystem",
				Properties: map[string]string{
					"compression": "on",
					"quota":       "10M",
				},
			})
			if err != nil {
				t.Fatalf("failed to create filesystem: %v", err)
			}

			// Verify dataset exists and properties
			exists, err := datasetMgr.Exists(context.Background(), fsName)
			if err != nil {
				t.Fatalf("failed to check dataset: %v", err)
			}
			if !exists {
				t.Error("dataset does not exist")
			}

			prop, err := datasetMgr.GetProperty(context.Background(), fsName, "compression")
			if err != nil {
				t.Fatalf("failed to get property: %v", err)
			}
			if prop.Value != "on" {
				t.Errorf("property value = %v, want 'on'", prop.Value)
			}
		})

		t.Run("Snapshots", func(t *testing.T) {
			snapName := fsName + "@snap1"
			err := datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
				Dataset: fsName,
				Name:    "snap1",
			})
			if err != nil {
				t.Fatalf("failed to create snapshot: %v", err)
			}

			exists, err := datasetMgr.Exists(context.Background(), snapName)
			if err != nil {
				t.Fatalf("failed to check snapshot: %v", err)
			}
			if !exists {
				t.Error("snapshot does not exist")
			}
		})

		t.Run("Clones", func(t *testing.T) {
			cloneName := poolName + "/clone1"
			err := datasetMgr.Clone(context.Background(), CloneConfig{
				Snapshot: fsName + "@snap1",
				Name:     cloneName,
				Properties: map[string]string{
					"mountpoint": "/mnt/clone1",
				},
			})
			if err != nil {
				t.Fatalf("failed to create clone: %v", err)
			}

			exists, err := datasetMgr.Exists(context.Background(), cloneName)
			if err != nil {
				t.Fatalf("failed to check clone: %v", err)
			}
			if !exists {
				t.Error("clone does not exist")
			}
		})

		t.Run("Bookmarks", func(t *testing.T) {
			bookmarkName := fsName + "#mark1"
			err := datasetMgr.CreateBookmark(context.Background(), BookmarkConfig{
				Snapshot: fsName + "@snap1",
				Bookmark: bookmarkName,
			})
			if err != nil {
				t.Fatalf("failed to create bookmark: %v", err)
			}

			exists, err := datasetMgr.Exists(context.Background(), bookmarkName)
			if err != nil {
				t.Fatalf("failed to check bookmark: %v", err)
			}
			if !exists {
				t.Error("bookmark does not exist")
			}
		})
	})

	t.Run("Volumes", func(t *testing.T) {
		volName := poolName + "/vol1"
		err := datasetMgr.Create(context.Background(), CreateConfig{
			Name: volName,
			Type: "volume",
			Properties: map[string]string{
				"volsize":      "30M",
				"volblocksize": "8K",
			},
		})
		if err != nil {
			t.Fatalf("failed to create volume: %v", err)
		}

		exists, err := datasetMgr.Exists(context.Background(), volName)
		if err != nil {
			t.Fatalf("failed to check volume: %v", err)
		}
		if !exists {
			t.Error("volume does not exist")
		}

		prop, err := datasetMgr.GetProperty(context.Background(), volName, "volsize")
		if err != nil {
			t.Fatalf("failed to get property: %v", err)
		}
		if prop.Value != "31457280" {
			t.Errorf("property value = %v, want '31457280'", prop.Value)
		}
	})

	// Additional negative test case
	t.Run("VolumeWithoutSize", func(t *testing.T) {
		volName := poolName + "/vol2"
		err := datasetMgr.Create(context.Background(), CreateConfig{
			Name: volName,
			Type: "volume",
			Properties: map[string]string{
				"volblocksize": "8K",
				// Missing volsize property
			},
		})
		if err == nil {
			t.Error("expected error when creating volume without size")
		}
	})

	// Clean up test pool
	err = poolMgr.Destroy(context.Background(), poolName, true)
	if err != nil {
		t.Fatalf("failed to destroy pool: %v", err)
	}
	poolDestroyed = true
}
