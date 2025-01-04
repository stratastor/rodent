package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/zfs/command"
	"github.com/stratastor/rodent/pkg/zfs/dataset"
	"github.com/stratastor/rodent/pkg/zfs/pool"
	"github.com/stratastor/rodent/pkg/zfs/testutil"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *pool.Manager, *dataset.Manager, string, func()) {
	// Setup test environment
	env := testutil.NewTestEnv(t, 3)
	executor := command.NewCommandExecutor(true, logger.Config{LogLevel: "debug"})
	poolMgr := pool.NewManager(executor)
	datasetMgr := dataset.NewManager(executor)

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

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Create handler and register routes
	handler := NewDatasetHandler(datasetMgr)
	handler.RegisterRoutes(router.Group("/api/v1"))

	cleanup := func() {
		if err := poolMgr.Destroy(context.Background(), poolName, true); err != nil {
			t.Logf("cleanup: failed to destroy pool: %v", err)
		}
		env.Cleanup()
	}

	return router, poolMgr, datasetMgr, poolName, cleanup
}

func TestDatasetAPI(t *testing.T) {
	router, _, _, poolName, cleanup := setupTestRouter(t)
	defer cleanup()

	dsURI := "/api/v1/dataset"

	// Create base filesystem for snapshots first
	baseFS := poolName + "/fs1"
	createReq := dataset.FilesystemConfig{
		NameConfig: dataset.NameConfig{
			Name: baseFS,
		},
		Properties: map[string]string{
			"compression": "on",
		},
	}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", dsURI+"/filesystem", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("create filesystem returned wrong status: got %v want %v",
			w.Code, http.StatusCreated)
		t.Errorf("error: %v", w.Body.String())
	}

	t.Run("FilesystemOperations", func(t *testing.T) {
		// Create filesystem
		fsName := poolName + "/fsTEST"
		createReq := dataset.FilesystemConfig{
			NameConfig: dataset.NameConfig{
				Name: fsName,
			},
			Properties: map[string]string{
				"compression": "on",
				"quota":       "10M",
			},
			Parents: true,
		}

		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest("POST", dsURI+"/filesystem", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("create filesystem returned wrong status: got %v want %v",
				w.Code, http.StatusCreated)
			t.Errorf("error: %v", w.Body.String())
		}

		// List filesystems
		listReq := dataset.ListConfig{
			Type:      "filesystem",
			Recursive: false,
		}
		body, _ = json.Marshal(listReq)
		req = httptest.NewRequest("GET", dsURI+"/filesystems", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("list filesystems returned wrong status: got %v want %v",
				w.Code, http.StatusOK)
		}

		var result struct {
			Result dataset.ListResult `json:"result"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if _, ok := result.Result.Datasets[fsName]; !ok {
			t.Error("created filesystem not found in list")
		}
	})

	t.Run("PropertyOperations", func(t *testing.T) {
		fsName := poolName + "/fs1"

		// Set property
		setReq := dataset.SetPropertyConfig{
			PropertyConfig: dataset.PropertyConfig{
				NameConfig: dataset.NameConfig{
					Name: fsName,
				},
				Property: "compression",
			},
			Value: "lz4",
		}

		body, _ := json.Marshal(setReq)
		req := httptest.NewRequest("PUT", dsURI+"/property", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("set property returned wrong status: got %v want %v",
				w.Code, http.StatusCreated)
		}

		// Get property
		getReq := dataset.PropertyConfig{
			NameConfig: dataset.NameConfig{
				Name: fsName,
			},
			Property: "compression",
		}

		body, _ = json.Marshal(getReq)
		req = httptest.NewRequest("GET", dsURI+"/property", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("get property returned wrong status: got %v want %v",
				w.Code, http.StatusOK)
		}

		var result struct {
			Result dataset.ListResult `json:"result"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if result.Result.Datasets[fsName].Properties["compression"].Value != "lz4" {
			t.Error("property value not set correctly")
		}
	})

	t.Run("VolumeOperations", func(t *testing.T) {
		volName := poolName + "/vol1"
		createReq := dataset.VolumeConfig{
			NameConfig: dataset.NameConfig{
				Name: volName,
			},
			Size: "10M",
			Properties: map[string]string{
				"compression":  "on",
				"volblocksize": "128K",
			},
		}

		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest("POST", dsURI+"/volume", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("create volume returned wrong status: got %v want %v",
				w.Code, http.StatusCreated)
			t.Errorf("error: %v", w.Body.String())
		}

		// List volumes
		listReq := dataset.ListConfig{
			Type: "volume",
		}
		body, _ = json.Marshal(listReq)
		req = httptest.NewRequest("GET", dsURI+"/volumes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("list volumes returned wrong status: got %v want %v",
				w.Code, http.StatusOK)
			t.Errorf("error: %v", w.Body.String())
		}
	})

	t.Run("SnapshotOperations", func(t *testing.T) {
		// Create snapshot
		snapReq := dataset.SnapshotConfig{
			NameConfig: dataset.NameConfig{
				Name: baseFS,
			},
			SnapName:  "snap1",
			Recursive: false,
			Properties: map[string]string{
				"comment:test": "test snapshot",
			},
		}
		body, _ = json.Marshal(snapReq)
		req = httptest.NewRequest("POST", dsURI+"/snapshot", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("create snapshot returned wrong status: got %v want %v. Error: %v",
				w.Code, http.StatusCreated, w.Body.String())
			t.Errorf("error: %v", w.Body.String())
		}

		// List snapshots
		listReq := dataset.ListConfig{
			Type: "snapshot",
			Name: baseFS,
		}
		body, _ = json.Marshal(listReq)
		req = httptest.NewRequest("GET", dsURI+"/snapshots", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("list snapshots returned wrong status: got %v want %v",
				w.Code, http.StatusOK)
			t.Errorf("error: %v", w.Body.String())
		}
	})

	t.Run("CloneOperations", func(t *testing.T) {
		// Create clone from snapshot
		cloneReq := dataset.CloneConfig{

			NameConfig: dataset.NameConfig{
				Name: baseFS + "@snap1",
			},
			CloneName: poolName + "/clone1",
			Properties: map[string]string{
				"mountpoint": "/mnt/clone1",
			},
		}
		body, _ := json.Marshal(cloneReq)
		req := httptest.NewRequest("POST", dsURI+"/clone", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("create clone returned wrong status: got %v want %v",
				w.Code, http.StatusCreated)
			t.Errorf("error: %v", w.Body.String())
		}

		// Promote clone
		promoteReq := dataset.NameConfig{
			Name: poolName + "/clone1",
		}
		body, _ = json.Marshal(promoteReq)
		req = httptest.NewRequest("POST", dsURI+"/clone/promote", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("promote clone returned wrong status: got %v want %v",
				w.Code, http.StatusOK)
			t.Errorf("error: %v", w.Body.String())
		}
	})

	t.Run("DestroyOperations", func(t *testing.T) {
		// Destroy dataset
		destroyReq := dataset.DestroyConfig{
			NameConfig: dataset.NameConfig{
				Name: poolName + "/clone1",
			},
			RecursiveDestroyDependents: true,
			Force:                      true,
		}
		body, _ := json.Marshal(destroyReq)
		req := httptest.NewRequest(http.MethodDelete, dsURI, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("destroy dataset returned wrong status: got %v want %v",
				w.Code, http.StatusNoContent)
			t.Errorf("error: %v", w.Body.String())
		}
	})

	t.Run("ErrorCases", func(t *testing.T) {
		testCases := []struct {
			name     string
			method   string
			path     string
			body     interface{}
			wantCode int
		}{
			{
				name:   "invalid filesystem name",
				method: http.MethodPost,
				path:   dsURI + "/filesystem",
				body: dataset.FilesystemConfig{
					NameConfig: dataset.NameConfig{Name: "invalid/name@with/special#chars"},
				},
				wantCode: http.StatusBadRequest,
			},
			{
				name:   "missing volume size",
				method: http.MethodPost,
				path:   dsURI + "/volume",
				body: dataset.VolumeConfig{
					NameConfig: dataset.NameConfig{Name: poolName + "/vol2"},
				},
				wantCode: http.StatusBadRequest,
			},
			{
				name:   "invalid property value",
				method: http.MethodPut,
				path:   dsURI + "/property",
				body: dataset.SetPropertyConfig{
					PropertyConfig: dataset.PropertyConfig{
						NameConfig: dataset.NameConfig{Name: poolName + "/fs1"},
						Property:   "compression",
					},
					Value: "invalid_value",
				},
				wantCode: http.StatusBadRequest,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				body, _ := json.Marshal(tc.body)
				req := httptest.NewRequest(tc.method, tc.path, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				if w.Code != tc.wantCode {
					t.Errorf("%s returned wrong status: got %v want %v",
						tc.name, w.Code, tc.wantCode)
					t.Errorf("error: %v", w.Body.String())
				}
			})
		}
	})
}
