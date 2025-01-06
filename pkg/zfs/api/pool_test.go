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

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/pool"
	"github.com/stratastor/rodent/pkg/zfs/testutil"
)

func setupPoolTestRouter(t *testing.T) (*gin.Engine, *pool.Manager, *testutil.TestEnv, func()) {
	env := testutil.NewTestEnv(t, 2)
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "debug"})
	poolMgr := pool.NewManager(executor)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	handler := NewPoolHandler(poolMgr)
	handler.RegisterRoutes(router.Group("/api/v1"))

	cleanup := func() {
		env.Cleanup()
	}

	return router, poolMgr, env, cleanup
}

// createTestPool creates a pool with the given devices and returns cleanup function
func createTestPool(t *testing.T, poolMgr *pool.Manager, devices []string) (string, func()) {
	poolName := testutil.GeneratePoolName()

	createReq := pool.CreateConfig{
		Name: poolName,
		VDevSpec: []pool.VDevSpec{{
			Type:    "mirror",
			Devices: devices,
		}},
		Properties: map[string]string{
			"ashift": "12",
		},
	}

	if err := poolMgr.Create(context.Background(), createReq); err != nil {
		t.Fatalf("failed to create pool %s: %v", poolName, err)
	}

	cleanup := func() {
		if err := poolMgr.Destroy(context.Background(), poolName, true); err != nil {
			t.Logf("cleanup: failed to destroy pool %s: %v", poolName, err)
		}
	}

	return poolName, cleanup
}

func TestPoolAPI(t *testing.T) {
	router, poolMgr, env, cleanup := setupPoolTestRouter(t)
	defer cleanup()

	poolsURI := "/api/v1/pools"
	poolName := testutil.GeneratePoolName()

	// Track pool state for cleanup
	var poolCreated bool
	var poolDestroyed bool
	defer func() {
		if poolCreated && !poolDestroyed {
			if err := poolMgr.Destroy(context.Background(), poolName, true); err != nil {
				t.Logf("cleanup: failed to destroy pool %s: %v", poolName, err)
			}
		}
	}()

	t.Run("CreatePool", func(t *testing.T) {
		createReq := pool.CreateConfig{
			Name: poolName,
			VDevSpec: []pool.VDevSpec{{
				Type:    "mirror",
				Devices: env.GetLoopDevices(),
			}},
			Properties: map[string]string{
				"ashift": "12",
			},
		}

		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest("POST", poolsURI, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("create pool returned wrong status: got %v want %v. Error: %v",
				w.Code, http.StatusCreated, w.Body.String())
			t.Errorf("error: %v", w.Body.String())
		}

		// Mark pool as created for cleanup
		poolCreated = true
	})

	t.Run("GetPoolStatus", func(t *testing.T) {
		req := httptest.NewRequest("GET", poolsURI+"/"+poolName+"/status", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("get pool status returned wrong status: got %v want %v",
				w.Code, http.StatusOK)
			t.Errorf("error: %v", w.Body.String())
		}

		var status pool.PoolStatus
		if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if _, ok := status.Pools[poolName]; !ok {
			t.Error("pool not found in status")
		}
	})

	t.Run("PropertyOperations", func(t *testing.T) {
		// Set property
		setReq := setPropertyRequest{
			Value: "test pool",
		}
		body, _ := json.Marshal(setReq)
		req := httptest.NewRequest("PUT", poolsURI+"/"+poolName+"/properties/test:comment",
			bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("set property returned wrong status: got %v want %v",
				w.Code, http.StatusOK)
			t.Errorf("error: %v", w.Body.String())
		}

		// Get property
		req = httptest.NewRequest("GET", poolsURI+"/"+poolName+"/properties/test:comment", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("get property returned wrong status: got %v want %v",
				w.Code, http.StatusOK)
			t.Errorf("error: %v", w.Body.String())
		}

		var prop pool.Property
		if err := json.Unmarshal(w.Body.Bytes(), &prop); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		// Strip single quotes from the value before comparing
		gotValue := strings.Trim(prop.Value.(string), "'")
		if gotValue != "test pool" {
			t.Errorf("property value = %v, want 'test pool'", prop.Value)
		}
	})

	t.Run("DeviceOperations", func(t *testing.T) {
		// Create new environment for device operations
		envDevices := testutil.NewTestEnv(t, 2)
		defer envDevices.Cleanup()

		poolName, cleanupPool := createTestPool(t, poolMgr, envDevices.GetLoopDevices()[:2])
		defer cleanupPool()

		devices := envDevices.GetLoopDevices()
		if len(devices) < 2 {
			t.Skip("not enough devices for device operations test")
		}

		envAdd := testutil.NewTestEnv(t, 2)
		defer envAdd.Cleanup()

		time.Sleep(2 * time.Second)

		// Attach device
		t.Run("Attach", func(t *testing.T) {
			attachReq := attachDeviceRequest{
				Device:    devices[0],
				NewDevice: envAdd.GetLoopDevices()[0],
			}
			body, _ := json.Marshal(attachReq)
			req := httptest.NewRequest("POST",
				poolsURI+"/"+poolName+"/devices/attach",
				bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("error: %v", w.Body.String())
				t.Fatalf("attach device returned wrong status: got %v want %v",
					w.Code, http.StatusOK)
			}

			time.Sleep(2 * time.Second)
		})

		// Replace device
		t.Run("Replace", func(t *testing.T) {
			replaceReq := replaceDeviceRequest{
				OldDevice: devices[0],
				NewDevice: envAdd.GetLoopDevices()[1],
			}
			body, _ := json.Marshal(replaceReq)
			req := httptest.NewRequest("POST",
				poolsURI+"/"+poolName+"/devices/replace",
				bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("error: %v", w.Body.String())
				t.Fatalf("replace device returned wrong status: got %v want %v",
					w.Code, http.StatusOK)
			}
			time.Sleep(2 * time.Second)
		})

		// Detach device
		t.Run("Detach", func(t *testing.T) {
			detachReq := detachDeviceRequest{
				Device: devices[1],
			}
			body, _ := json.Marshal(detachReq)
			req := httptest.NewRequest("POST",
				poolsURI+"/"+poolName+"/devices/detach",
				bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("detach device returned wrong status: got %v want %v\nerror: %v",
					w.Code, http.StatusOK, w.Body.String())
			}
			// Let the detach operation complete
			time.Sleep(2 * time.Second)
		})
	})

	t.Run("MaintenanceOperations", func(t *testing.T) {
		envMaint := testutil.NewTestEnv(t, 2)
		defer envMaint.Cleanup()

		poolName, cleanupPool := createTestPool(t, poolMgr, envMaint.GetLoopDevices())
		defer cleanupPool()

		t.Run("Scrub start/stop", func(t *testing.T) {
			// Start scrub
			req := httptest.NewRequest("POST", poolsURI+"/"+poolName+"/scrub", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("error: %v", w.Body.String())
				t.Fatalf("start scrub returned wrong status: got %v want %v",
					w.Code, http.StatusOK)
			}

			time.Sleep(2 * time.Second)

			// Stop scrub
			req = httptest.NewRequest("POST", poolsURI+"/"+poolName+"/scrub?stop=true", nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				if !strings.Contains(w.Body.String(), "there is no active scrub") {
					t.Errorf("error: %v", w.Body.String())
					t.Fatalf("stop scrub returned wrong status: got %v want %v",
						w.Code, http.StatusOK)
				}
			}
			time.Sleep(2 * time.Second)
		})

		t.Run("Resilver", func(t *testing.T) {
			// Resilver
			req := httptest.NewRequest("POST", poolsURI+"/"+poolName+"/resilver", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("error: %v", w.Body.String())
				t.Fatalf("resilver returned wrong status: got %v want %v",
					w.Code, http.StatusOK)
			}
			time.Sleep(2 * time.Second)
		})
	})

	t.Run("ExportImport", func(t *testing.T) {
		envExp := testutil.NewTestEnv(t, 2)
		defer envExp.Cleanup()

		poolName, cleanupPool := createTestPool(t, poolMgr, envExp.GetLoopDevices())
		defer cleanupPool()

		// Export pool
		req := httptest.NewRequest("POST", poolsURI+"/"+poolName+"/export", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("error: %v", w.Body.String())
			t.Fatalf("export pool returned wrong status: got %v want %v",
				w.Code, http.StatusOK)
		}

		time.Sleep(2 * time.Second)

		// Import pool
		importReq := pool.ImportConfig{
			Name:  poolName,
			Force: true,
		}
		body, _ := json.Marshal(importReq)
		req = httptest.NewRequest("POST", poolsURI+"/import", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("error: %v", w.Body.String())
			t.Fatalf("import pool returned wrong status: got %v want %v",
				w.Code, http.StatusOK)
		}
		time.Sleep(2 * time.Second)
	})

	t.Run("DestroyPool", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", poolsURI+"/"+poolName+"?force=true", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("destroy pool returned wrong status: got %v want %v",
				w.Code, http.StatusNoContent)
			t.Errorf("error: %v", w.Body.String())
		}
		poolDestroyed = true
	})
}
