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

package pool

import (
	"context"
	"strings"
	"testing"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/testutil"
)

func TestPoolOperations(t *testing.T) {
	// Setup test environment
	env := testutil.NewTestEnv(t, 3)
	defer env.Cleanup() // Only cleanup devices

	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "debug"})
	manager := NewManager(executor)
	poolName := testutil.GeneratePoolName()

	// Track pool state for cleanup
	var poolCreated bool
	var poolDestroyed bool
	defer func() {
		// Only attempt to destroy if pool was created but not destroyed
		if poolCreated && !poolDestroyed {
			if err := manager.Destroy(context.Background(), poolName, true); err != nil {
				t.Logf("cleanup: failed to destroy pool: %v", err)
			}
		}
	}()

	t.Run("CreatePool", func(t *testing.T) {
		cfg := CreateConfig{
			Name: poolName,
			VDevSpec: []VDevSpec{
				{
					Type:    "raidz",
					Devices: env.GetLoopDevices(),
				},
			},
			Properties: map[string]string{
				"ashift": "12",
			},
		}

		err := manager.Create(context.Background(), cfg)
		if err != nil {
			t.Fatalf("failed to create pool: %v\nDevices: %v", err, env.GetLoopDevices())
		}
		poolCreated = true // Mark pool as created

		// Verify pool exists
		status, err := manager.Status(context.Background(), poolName)
		if err != nil {
			t.Fatalf("failed to get pool status: %v", err)
		}
		if pool, ok := status.Pools[poolName]; !ok || pool.State != "ONLINE" {
			t.Errorf("pool state = %s, want ONLINE", pool.State)
		}

		t.Run("Properties", func(t *testing.T) {
			err := manager.SetProperty(context.Background(), poolName, "comment", "test pool")
			if err != nil {
				t.Fatalf("failed to set property: %v", err)
			}

			prop, err := manager.GetProperty(context.Background(), poolName, "comment")
			if err != nil {
				t.Fatalf("failed to get property: %v", err)
			}
			// Compare without quotes since they're added by ZFS
			if strings.Trim(prop.Value.(string), "'") != "test pool" {
				t.Errorf("property value = %v, want 'test pool'", prop.Value)
			}
		})

		// Only run destroy test if pool exists
		// Only run destroy test if previous tests passed
		if !t.Failed() {
			t.Run("DestroyPool", func(t *testing.T) {
				// Ensure we're destroying the right pool
				if poolName == "" {
					t.Fatal("pool name is empty")
				}

				err := manager.Destroy(context.Background(), poolName, true)
				if err != nil {
					t.Fatalf("failed to destroy pool: %v", err)
				}
				poolDestroyed = true // Mark as destroyed to prevent second destroy attempt
			})
		}
	})
}
