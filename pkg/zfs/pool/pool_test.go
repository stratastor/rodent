package pool

import (
	"context"
	"testing"

	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/testutil"
)

func TestPoolOperations(t *testing.T) {
	// Setup test environment
	env := testutil.NewTestEnv(t, 3) // 3 disks for RAIDZ
	defer env.Cleanup()

	// Create pool manager
	executor := command.NewCommandExecutor(true)
	manager := NewManager(executor)

	// Test pool creation
	t.Run("CreatePool", func(t *testing.T) {
		cfg := CreateConfig{
			Name: "tank",
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
			t.Fatalf("failed to create pool: %v", err)
		}

		// Verify pool exists
		status, err := manager.Status(context.Background(), "tank")
		if err != nil {
			t.Fatalf("failed to get pool status: %v", err)
		}
		if status.Pools["tank"].State != "ONLINE" {
			t.Errorf("pool state = %s, want ONLINE", status.Pools["tank"].State)
		}
	})

	// Test property operations
	t.Run("Properties", func(t *testing.T) {
		// Test setting property
		err := manager.SetProperty(context.Background(), "tank", "comment", "test pool")
		if err != nil {
			t.Fatalf("failed to set property: %v", err)
		}

		// Test getting property
		prop, err := manager.GetProperty(context.Background(), "tank", "comment")
		if err != nil {
			t.Fatalf("failed to get property: %v", err)
		}
		if prop.Value != "test pool" {
			t.Errorf("property value = %v, want 'test pool'", prop.Value)
		}
	})

	// Test pool destruction
	t.Run("DestroyPool", func(t *testing.T) {
		err := manager.Destroy(context.Background(), "tank", true)
		if err != nil {
			t.Fatalf("failed to destroy pool: %v", err)
		}
	})
}
