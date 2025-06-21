/*
 * Copyright 2024-2025 Raamsri Kumar <raam@tinkershack.in>
 * Copyright 2024-2025 The StrataSTOR Authors and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dataset

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/pool"
	"github.com/stratastor/rodent/pkg/zfs/testutil"
)

func TestDataTransferOperations(t *testing.T) {
	logLevel := os.Getenv("RODENT_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "debug"
	}

	// Setup test environment
	env1 := testutil.NewTestEnv(t, 3)
	defer env1.Cleanup()

	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: logLevel})
	poolMgr := pool.NewManager(executor)
	datasetMgr := NewManager(executor)

	// Create source pool
	srcPoolName := testutil.GeneratePoolName()
	srcPoolCfg := pool.CreateConfig{
		Name: srcPoolName,
		VDevSpec: []pool.VDevSpec{{
			Type:    "raidz",
			Devices: env1.GetLoopDevices(),
		}},
	}

	err := poolMgr.Create(context.Background(), srcPoolCfg)
	if err != nil {
		t.Fatalf("failed to create source pool: %v", err)
	}

	// Track pool state for cleanup
	var srcPoolDestroyed bool
	defer func() {
		if !srcPoolDestroyed {
			poolMgr.Destroy(context.Background(), srcPoolName, true)
		}
	}()

	// Create destination pool
	env2 := testutil.NewTestEnv(t, 3)
	defer env2.Cleanup()

	dstPoolName := testutil.GeneratePoolName()
	err = poolMgr.Create(context.Background(), pool.CreateConfig{
		Name: dstPoolName,
		VDevSpec: []pool.VDevSpec{{
			Type:    "raidz",
			Devices: env2.GetLoopDevices(),
		}},
	})
	if err != nil {
		t.Fatalf("failed to create destination pool: %v", err)
	}

	var dstPoolDestroyed bool
	defer func() {
		if !dstPoolDestroyed {
			poolMgr.Destroy(context.Background(), dstPoolName, true)
		}
	}()

	t.Run("BasicTransfer", func(t *testing.T) {
		// Create source filesystem with data
		srcName := srcPoolName + "/fs1"
		_, err := datasetMgr.CreateFilesystem(context.Background(), FilesystemConfig{
			NameConfig: NameConfig{Name: srcName},
			Properties: map[string]string{
				"compression": "on",
				"quota":       "100M",
			},
		})
		if err != nil {
			t.Fatalf("failed to create source filesystem: %v", err)
		}

		// Create test data
		testFile := "/" + srcName + "/testfile"
		if err := os.WriteFile(testFile, []byte("test data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Create snapshot
		snapName := srcName + "@snap1"
		err = datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
			NameConfig: NameConfig{Name: srcName},
			SnapName:   "snap1",
			Recursive:  false,
		})
		if err != nil {
			t.Fatalf("failed to create snapshot: %v", err)
		}

		// Test transfer
		dstName := dstPoolName + "/received"
		err = datasetMgr.SendReceive(context.Background(),
			SendConfig{
				Snapshot:   snapName,
				Properties: true,
				Parsable:   true,
				Compressed: true,
				LogLevel:   logLevel,
			},
			ReceiveConfig{
				Target:    dstName,
				Force:     true,
				Resumable: true,
			},
		)
		if err != nil {
			if rerr, ok := err.(*errors.RodentError); ok {
				t.Fatalf("failed remote transfer: %v\nOutput: %s\nCommand: %s",
					rerr,
					rerr.Metadata["output"],
					rerr.Metadata["command"])
			}
			t.Fatalf("failed remote transfer: %v", err)
		}

		// Verify transfer
		exists, err := datasetMgr.Exists(context.Background(), dstName+"@snap1")
		if err != nil {
			t.Fatalf("failed to check destination: %v", err)
		}
		if !exists {
			t.Fatal("destination snapshot does not exist")
		}

		// Verify properties
		props, err := datasetMgr.ListProperties(context.Background(), NameConfig{Name: dstName})
		if err != nil {
			t.Fatalf("failed to get properties: %v", err)
		}
		if props.Datasets[dstName].Properties["compression"].Value != "on" {
			t.Error("compression property not transferred")
		}

		// Verify data
		dstFile := "/" + dstName + "/testfile"
		data, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatalf("failed to read test file: %v", err)
		}
		if string(data) != "test data" {
			t.Error("data mismatch in transferred file")
		}
	})

	t.Run("IncrementalTransfer", func(t *testing.T) {
		// Create source filesystem
		srcFs := srcPoolName + "/fs2"
		_, err := datasetMgr.CreateFilesystem(context.Background(), FilesystemConfig{
			NameConfig: NameConfig{Name: srcFs},
			Properties: map[string]string{
				"compression": "on",
			},
		})
		if err != nil {
			t.Fatalf("failed to create source filesystem: %v", err)
		}

		// Create initial snapshot and data
		snap1 := srcFs + "@snap1"
		err = datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
			NameConfig: NameConfig{Name: srcFs},
			SnapName:   "snap1",
		})
		if err != nil {
			t.Fatalf("failed to create first snapshot: %v", err)
		}

		// Initial transfer
		dstFs := dstPoolName + "/fs2"
		err = datasetMgr.SendReceive(context.Background(),
			SendConfig{
				Snapshot:   snap1,
				Properties: true,
				LogLevel:   logLevel,
			},
			ReceiveConfig{
				Target:     dstFs,
				Force:      true,
				Properties: map[string]string{"compression": "on"},
			},
		)
		if err != nil {
			t.Fatalf("failed initial transfer: %v", err)
		}

		// Create new data and second snapshot
		testFile := "/" + srcFs + "/testfile2"
		if err := os.WriteFile(testFile, []byte("incremental data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		snap2 := srcFs + "@snap2"
		err = datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
			NameConfig: NameConfig{Name: srcFs},
			SnapName:   "snap2",
		})
		if err != nil {
			t.Fatalf("failed to create second snapshot: %v", err)
		}

		// Incremental transfer
		err = datasetMgr.SendReceive(context.Background(),
			SendConfig{
				Snapshot:     snap2,
				FromSnapshot: snap1,
				Properties:   true,
				LogLevel:     logLevel,
			},
			ReceiveConfig{
				Target: dstFs,
				Force:  true,
			},
		)
		if err != nil {
			t.Fatalf("failed incremental transfer: %v", err)
		}

		// Verify both snapshots exist on destination
		for _, snap := range []string{snap1, snap2} {
			exists, err := datasetMgr.Exists(context.Background(),
				dstFs+"@"+strings.Split(snap, "@")[1])
			if err != nil {
				t.Fatalf("failed to check snapshot %s: %v", snap, err)
			}
			if !exists {
				t.Errorf("snapshot %s does not exist on destination", snap)
			}
		}
	})

	t.Run("ResumeTransfer", func(t *testing.T) {
		// Create source filesystem with large data
		srcFs := srcPoolName + "/fs3"
		_, err := datasetMgr.CreateFilesystem(context.Background(), FilesystemConfig{
			NameConfig: NameConfig{Name: srcFs},
			Properties: map[string]string{
				"compression": "off", // Disable compression for larger transfer
			},
		})
		if err != nil {
			t.Fatalf("failed to create source filesystem: %v", err)
		}

		// Create large test data (100MB)
		testFile := "/" + srcFs + "/largefile"
		f, err := os.Create(testFile)
		if err != nil {
			t.Fatalf("failed to create large file: %v", err)
		}
		if err := f.Truncate(100 * 1024 * 1024); err != nil {
			t.Fatalf("failed to create large file: %v", err)
		}
		f.Close()

		// Create snapshot
		snapName := srcFs + "@snap1"
		err = datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
			NameConfig: NameConfig{Name: srcFs},
			SnapName:   "snap1",
		})
		if err != nil {
			t.Fatalf("failed to create snapshot: %v", err)
		}

		// TODO: Cancel transfer immediately to simulate failure. Send a larger dir to test resumable transfer.

		// Start resumable receive
		dstFs := dstPoolName + "/fs3"
		err = datasetMgr.SendReceive(context.Background(),
			SendConfig{
				Snapshot:   snapName,
				Properties: true,
				Parsable:   true,
				LogLevel:   logLevel,
			},
			ReceiveConfig{
				Target:    dstFs,
				Force:     true,
				Resumable: true,
			},
		)
		if err != nil {
			if rerr, ok := err.(*errors.RodentError); ok {
				t.Fatalf("failed remote transfer: %v\nOutput: %s\nCommand: %s",
					rerr,
					rerr.Metadata["output"],
					rerr.Metadata["command"])
			}
			t.Fatalf("failed remote transfer: %v", err)
		}

		// Get resume token
		token, err := datasetMgr.GetResumeToken(context.Background(), NameConfig{
			Name: dstFs,
		})
		if err != nil {
			t.Fatalf("failed to get resume token: %v", err)
		}
		if token == "" {
			t.Log("no resume token available")
		}

		// Resume transfer
		err = datasetMgr.SendReceive(context.Background(),
			SendConfig{
				ResumeToken: token,
				Parsable:    true,
				LogLevel:    logLevel,
			},
			ReceiveConfig{
				Target:    dstFs,
				Force:     true,
				Resumable: true,
			},
		)
		if err != nil {
			t.Fatalf("failed to resume transfer: %v", err)
		}
	})

	t.Run("RemoteTransfer", func(t *testing.T) {
		if os.Getenv("RODENT_REMOTE_TEST") == "" {
			t.Skip("Skipping remote transfer test (set RODENT_REMOTE_TEST=1 to enable)")
		}

		// Create source filesystem
		srcFs := srcPoolName + "/fs4"
		_, err := datasetMgr.CreateFilesystem(context.Background(), FilesystemConfig{
			NameConfig: NameConfig{Name: srcFs},
			Properties: map[string]string{
				"compression": "on",
			},
		})
		if err != nil {
			t.Fatalf("failed to create source filesystem: %v", err)
		}

		// Create test data
		testFile := "/" + srcFs + "/testfile"
		if err := os.WriteFile(testFile, []byte("remote test data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Create snapshot
		snapName := srcFs + "@snap1"
		err = datasetMgr.CreateSnapshot(context.Background(), SnapshotConfig{
			NameConfig: NameConfig{Name: srcFs},
			SnapName:   "snap1",
		})
		if err != nil {
			t.Fatalf("failed to create snapshot: %v", err)
		}

		// Configure remote connection
		remoteConfig := RemoteConfig{
			Host:             os.Getenv("RODENT_REMOTE_HOST"),
			User:             os.Getenv("RODENT_REMOTE_USER"),
			Port:             22,
			PrivateKey:       os.Getenv("RODENT_REMOTE_KEY"),
			SkipHostKeyCheck: true,
		}

		// Remote transfer
		err = datasetMgr.SendReceive(context.Background(),
			SendConfig{
				Snapshot:   snapName,
				Properties: true,
				Parsable:   true,
				Compressed: true,
				LogLevel:   logLevel,
			},
			ReceiveConfig{
				Target:       os.Getenv("RODENT_REMOTE_DATASET"),
				Force:        true,
				Resumable:    true,
				RemoteConfig: remoteConfig,
			},
		)
		if err != nil {
			if rerr, ok := err.(*errors.RodentError); ok {
				t.Fatalf("failed remote transfer: %v\nOutput: %s\nCommand: %s",
					rerr,
					rerr.Metadata["output"],
					rerr.Metadata["command"])
			}
			t.Fatalf("failed remote transfer: %v", err)
		}
	})

	// Clean up
	for _, name := range []string{srcPoolName, dstPoolName} {
		if err := poolMgr.Destroy(context.Background(), name, true); err != nil {
			t.Errorf("failed to destroy pool %s: %v", name, err)
		}
	}
	srcPoolDestroyed = true
	dstPoolDestroyed = true
}
