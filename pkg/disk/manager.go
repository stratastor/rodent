// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"context"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/events"
	"github.com/stratastor/rodent/pkg/disk/config"
	"github.com/stratastor/rodent/pkg/disk/discovery"
	diskevents "github.com/stratastor/rodent/pkg/disk/events"
	"github.com/stratastor/rodent/pkg/disk/health"
	"github.com/stratastor/rodent/pkg/disk/hotplug"
	"github.com/stratastor/rodent/pkg/disk/probing"
	"github.com/stratastor/rodent/pkg/disk/state"
	"github.com/stratastor/rodent/pkg/disk/tools"
	"github.com/stratastor/rodent/pkg/disk/topology"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// Manager is the main disk management service
type Manager struct {
	logger logger.Logger
	ctx    context.Context
	cancel context.CancelFunc

	// Core components
	configManager  *config.ConfigManager
	stateManager   *state.StateManager
	toolChecker    *tools.ToolChecker
	discoverer     *discovery.Discoverer
	topoMapper     *topology.Mapper
	healthMonitor  *health.Monitor
	probeScheduler *probing.ProbeScheduler
	eventEmitter   *diskevents.Emitter
	hotplugHandler *hotplug.EventHandler

	// Background tasks
	scheduler gocron.Scheduler
	wg        sync.WaitGroup

	// Device cache
	deviceCache map[string]*types.PhysicalDisk
	cacheMu     sync.RWMutex
}

// NewManager creates a new disk manager
func NewManager(
	l logger.Logger,
	executor *command.CommandExecutor,
	eventBus *events.EventBus,
	poolManager probing.ZFSPoolManager,
) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize config manager
	configMgr := config.NewConfigManager(l)
	if err := configMgr.Load(); err != nil {
		cancel()
		return nil, errors.Wrap(err, errors.DiskConfigLoadFailed).
			WithMetadata("operation", "manager_init")
	}

	cfg := configMgr.Get()

	// Initialize state manager
	stateMgr := state.NewStateManager(l, "")
	if err := stateMgr.Load(); err != nil {
		cancel()
		return nil, errors.Wrap(err, errors.DiskStateLoadFailed).
			WithMetadata("operation", "manager_init")
	}

	// Initialize tool checker
	toolChecker := tools.NewToolChecker(l, &cfg.Tools)
	toolChecker.CheckAll()

	// Validate required tools
	requiredTools := cfg.Tools.RequiredTools
	if len(requiredTools) == 0 {
		requiredTools = []string{"smartctl", "lsblk", "udevadm"}
	}
	if err := toolChecker.ValidateRequired(requiredTools); err != nil {
		cancel()
		return nil, errors.Wrap(err, errors.DiskToolNotFound).
			WithMetadata("operation", "manager_init")
	}

	// Initialize tool executors (useSudo=true for disk operations)
	smartctl := tools.NewSmartctlExecutor(l, cfg.Tools.SmartctlPath, true)
	lsblk := tools.NewLsblkExecutor(l, cfg.Tools.LsblkPath, true)
	udevadm := tools.NewUdevadmExecutor(l, cfg.Tools.UdevadmPath, true)

	// Initialize optional tools (may be nil)
	var lsscsi *tools.LsscsiExecutor
	if toolChecker.IsAvailable("lsscsi") {
		lsscsi = tools.NewLsscsiExecutor(l, cfg.Tools.LsscsiPath, true)
	}

	var sgses *tools.SgSesExecutor
	if toolChecker.IsAvailable("sg_ses") {
		sgses = tools.NewSgSesExecutor(l, cfg.Tools.SgSesPath, true)
	}

	// Initialize discoverer
	discoverer := discovery.NewDiscoverer(l, lsblk, smartctl, udevadm, toolChecker)

	// Initialize topology mapper
	topoMapper := topology.NewMapper(l, lsscsi, sgses, toolChecker)

	// Initialize health monitor
	healthMonitor := health.NewMonitor(l, smartctl, cfg.Monitoring.Thresholds)

	// Initialize conflict checker
	conflictChecker := probing.NewZFSConflictChecker(l, stateMgr, poolManager)

	// Initialize probe scheduler
	probeScheduler, err := probing.NewProbeScheduler(
		l,
		stateMgr,
		smartctl,
		conflictChecker,
		cfg.Probing.MaxConcurrent,
	)
	if err != nil {
		cancel()
		return nil, errors.Wrap(err, errors.DiskProbeScheduleFailed).
			WithMetadata("operation", "manager_init")
	}

	// Initialize event emitter
	eventEmitter := diskevents.NewEmitter(l, eventBus)

	// Create scheduler for periodic tasks
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		cancel()
		return nil, errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "create_task_scheduler")
	}

	m := &Manager{
		logger:         l,
		ctx:            ctx,
		cancel:         cancel,
		configManager:  configMgr,
		stateManager:   stateMgr,
		toolChecker:    toolChecker,
		discoverer:     discoverer,
		topoMapper:     topoMapper,
		healthMonitor:  healthMonitor,
		probeScheduler: probeScheduler,
		eventEmitter:   eventEmitter,
		scheduler:      scheduler,
		deviceCache:    make(map[string]*types.PhysicalDisk),
	}

	// Initialize hotplug handler (only if udev monitoring is enabled)
	if cfg.Discovery.UdevMonitor {
		hotplugCfg := &hotplug.HandlerConfig{
			UdevadmPath:       cfg.Tools.UdevadmPath,
			MonitorSubsystems: []string{"block"},
			MonitorBufferSize: 100,
			ReconcileInterval: cfg.Discovery.ReconcileInterval,
			DiscoveryFunc:     m.discoverDevices,
			CacheFunc:         m.getDeviceCache,
			OnDeviceAdded:     m.handleDeviceAdded,
			OnDeviceRemoved:   m.handleDeviceRemoved,
			OnDeviceChanged:   m.handleDeviceChanged,
		}

		m.hotplugHandler = hotplug.NewEventHandler(l, hotplugCfg)
	}

	// Set device resolver now that Manager is created
	probeScheduler.SetDeviceResolver(m)

	return m, nil
}

// Start starts the disk manager
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("starting disk manager")

	// Get configuration
	cfg := m.configManager.Get()

	// Initial discovery
	if err := m.runDiscovery(ctx); err != nil {
		m.logger.Error("initial discovery failed", "error", err)
		return err
	}

	// Schedule periodic discovery
	if cfg.Discovery.Enabled && cfg.Discovery.Interval > 0 {
		_, err := m.scheduler.NewJob(
			gocron.DurationJob(cfg.Discovery.Interval),
			gocron.NewTask(func() {
				if err := m.runDiscovery(m.ctx); err != nil {
					m.logger.Error("periodic discovery failed", "error", err)
				}
			}),
			gocron.WithName("periodic_discovery"),
		)
		if err != nil {
			return errors.Wrap(err, errors.DiskDiscoveryFailed).
				WithMetadata("operation", "schedule_discovery")
		}
	}

	// Schedule periodic health monitoring
	if cfg.Monitoring.Enabled && cfg.Monitoring.Interval > 0 {
		_, err := m.scheduler.NewJob(
			gocron.DurationJob(cfg.Monitoring.Interval),
			gocron.NewTask(func() {
				if err := m.runHealthCheck(m.ctx); err != nil {
					m.logger.Error("periodic health check failed", "error", err)
				}
			}),
			gocron.WithName("periodic_health_check"),
		)
		if err != nil {
			return errors.Wrap(err, errors.DiskHealthCheckFailed).
				WithMetadata("operation", "schedule_health_check")
		}
	}

	// Start probe scheduler
	if cfg.Probing.Enabled {
		if err := m.probeScheduler.Start(ctx); err != nil {
			return err
		}
	}

	// Start hotplug handler
	if m.hotplugHandler != nil {
		if err := m.hotplugHandler.Start(cfg.Tools.UdevadmPath); err != nil {
			m.logger.Warn("failed to start hotplug handler, continuing without it",
				"error", err)
		} else {
			m.logger.Info("hotplug monitoring enabled")
		}
	}

	// Start background scheduler
	m.scheduler.Start()

	m.logger.Info("disk manager started",
		"discovery_interval", cfg.Discovery.Interval,
		"health_check_interval", cfg.Monitoring.Interval)

	return nil
}

// Stop stops the disk manager gracefully
func (m *Manager) Stop(ctx context.Context) error {
	m.logger.Info("stopping disk manager")

	// Stop scheduler
	if err := m.scheduler.Shutdown(); err != nil {
		m.logger.Error("error stopping scheduler", "error", err)
	}

	// Stop probe scheduler
	if err := m.probeScheduler.Stop(ctx); err != nil {
		m.logger.Error("error stopping probe scheduler", "error", err)
	}

	// Stop hotplug handler
	if m.hotplugHandler != nil {
		if err := m.hotplugHandler.Stop(); err != nil {
			m.logger.Error("error stopping hotplug handler", "error", err)
		}
	}

	// Cancel context
	m.cancel()

	// Wait for background tasks with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("all background tasks stopped")
	case <-time.After(30 * time.Second):
		m.logger.Warn("timeout waiting for background tasks")
	}

	// Final state save
	if err := m.stateManager.Save(); err != nil {
		m.logger.Error("failed to save final state", "error", err)
	}

	m.logger.Info("disk manager stopped")
	return nil
}

// runDiscovery performs full disk discovery
func (m *Manager) runDiscovery(ctx context.Context) error {
	m.logger.Debug("running disk discovery")

	// Discover all physical disks
	disks, err := m.discoverer.DiscoverAll(ctx)
	if err != nil {
		return errors.Wrap(err, errors.DiskDiscoveryFailed)
	}

	m.logger.Info("discovered disks", "count", len(disks))

	// Update device cache
	m.cacheMu.Lock()
	m.deviceCache = make(map[string]*types.PhysicalDisk)
	for _, disk := range disks {
		m.deviceCache[disk.DeviceID] = disk
	}
	m.cacheMu.Unlock()

	// Update state with device info
	m.stateManager.WithLock(func(s *types.DiskManagerState) {
		if s.Devices == nil {
			s.Devices = make(map[string]*types.DeviceState)
		}

		// Update or add devices
		for _, disk := range disks {
			if existing, ok := s.Devices[disk.DeviceID]; ok {
				// Update existing device
				existing.Health = disk.Health
				existing.LastSeenAt = time.Now()
			} else {
				// New device discovered
				s.Devices[disk.DeviceID] = &types.DeviceState{
					DeviceID:    disk.DeviceID,
					State:       types.DiskStateDiscovered,
					Health:      disk.Health,
					FirstSeenAt: time.Now(),
					LastSeenAt:  time.Now(),
				}
				// Emit discovery event
				m.eventEmitter.EmitDiskDiscovered(disk)
			}
		}

		// Update statistics
		s.Statistics.TotalDiscoveries++
		s.Statistics.LastDiscoveryAt = time.Now()
		s.Statistics.CurrentDeviceCount = len(disks)
	})

	m.stateManager.SaveDebounced()

	return nil
}

// runHealthCheck performs health check on all disks
func (m *Manager) runHealthCheck(ctx context.Context) error {
	m.logger.Debug("running health check")

	// Get all devices from cache
	m.cacheMu.RLock()
	disks := make([]*types.PhysicalDisk, 0, len(m.deviceCache))
	for _, disk := range m.deviceCache {
		disks = append(disks, disk)
	}
	m.cacheMu.RUnlock()

	if len(disks) == 0 {
		m.logger.Debug("no devices to check")
		return nil
	}

	// Check health
	healthStatuses, err := m.healthMonitor.CheckAllHealth(ctx, disks)
	if err != nil {
		return errors.Wrap(err, errors.DiskHealthCheckFailed)
	}

	// Update state and cache
	for _, status := range healthStatuses {
		// Update cache
		m.cacheMu.Lock()
		if disk, ok := m.deviceCache[status.DeviceID]; ok {
			oldHealth := disk.Health
			disk.Health = status.Health
			disk.SMARTInfo = status.SMARTInfo

			// Emit health change event if changed
			if oldHealth != status.Health {
				m.eventEmitter.EmitDiskHealthChanged(disk, oldHealth, status.Health)
			}
		}
		m.cacheMu.Unlock()

		// Update state
		m.stateManager.WithLock(func(s *types.DiskManagerState) {
			if deviceState, ok := s.Devices[status.DeviceID]; ok {
				if deviceState.Health != status.Health {
					deviceState.Health = status.Health
					deviceState.HealthChanges++
				}
			}
		})
	}

	m.stateManager.SaveDebounced()

	return nil
}

// GetInventory returns the current disk inventory
func (m *Manager) GetInventory(filter *types.DiskFilter) []*types.PhysicalDisk {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	disks := make([]*types.PhysicalDisk, 0, len(m.deviceCache))
	for _, disk := range m.deviceCache {
		if filter == nil || disk.MatchesFilter(filter) {
			disks = append(disks, disk)
		}
	}

	return disks
}

// GetDisk returns a specific disk by ID
func (m *Manager) GetDisk(deviceID string) (*types.PhysicalDisk, error) {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	if disk, ok := m.deviceCache[deviceID]; ok {
		return disk, nil
	}

	return nil, errors.New(errors.DiskNotFound, "disk not found").
		WithMetadata("device_id", deviceID)
}

// TriggerDiscovery manually triggers a discovery scan
func (m *Manager) TriggerDiscovery(ctx context.Context) error {
	m.logger.Info("manual discovery triggered")
	return m.runDiscovery(ctx)
}

// TriggerHealthCheck manually triggers a health check
func (m *Manager) TriggerHealthCheck(ctx context.Context) error {
	m.logger.Info("manual health check triggered")
	return m.runHealthCheck(ctx)
}

// ResolveDevices resolves a device filter to deviceID -> devicePath map
// Implements probing.DeviceResolver interface
func (m *Manager) ResolveDevices(filter *types.DiskFilter) (map[string]string, error) {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	result := make(map[string]string)
	for _, disk := range m.deviceCache {
		if filter == nil || disk.MatchesFilter(filter) {
			result[disk.DeviceID] = disk.DevicePath
		}
	}

	return result, nil
}

// ============================================================================
// Hotplug Event Handlers
// ============================================================================

// discoverDevices is a wrapper for discovery used by the reconciler
func (m *Manager) discoverDevices(ctx context.Context) ([]*types.PhysicalDisk, error) {
	return m.discoverer.DiscoverAll(ctx)
}

// getDeviceCache returns a copy of the current device cache
func (m *Manager) getDeviceCache() map[string]*types.PhysicalDisk {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	// Return a shallow copy to prevent modification
	cache := make(map[string]*types.PhysicalDisk, len(m.deviceCache))
	for k, v := range m.deviceCache {
		cache[k] = v
	}

	return cache
}

// handleDeviceAdded handles a new device being added to the system
func (m *Manager) handleDeviceAdded(ctx context.Context, deviceID string) error {
	m.logger.Info("processing device addition", "device_id", deviceID)

	// Trigger discovery to pick up the new device
	if err := m.runDiscovery(ctx); err != nil {
		return errors.Wrap(err, errors.DiskDiscoveryFailed).
			WithMetadata("device_id", deviceID).
			WithMetadata("event", "device_added")
	}

	// Emit event for the discovered disk
	m.cacheMu.RLock()
	disk, exists := m.deviceCache[deviceID]
	m.cacheMu.RUnlock()

	if exists {
		m.eventEmitter.EmitDiskDiscovered(disk)
	}

	return nil
}

// handleDeviceRemoved handles a device being removed from the system
func (m *Manager) handleDeviceRemoved(deviceID string) error {
	m.logger.Info("processing device removal", "device_id", deviceID)

	// Remove from cache
	m.cacheMu.Lock()
	disk, exists := m.deviceCache[deviceID]
	if exists {
		delete(m.deviceCache, deviceID)
	}
	m.cacheMu.Unlock()

	if !exists {
		m.logger.Debug("device not in cache", "device_id", deviceID)
		return nil
	}

	// Update state
	m.stateManager.UpdateDeviceState(deviceID, types.DiskStateOffline, types.HealthUnknown)

	// Emit event
	m.eventEmitter.EmitDiskRemoved(disk)

	return nil
}

// handleDeviceChanged handles a device change event
func (m *Manager) handleDeviceChanged(ctx context.Context, deviceID string) error {
	m.logger.Info("processing device change", "device_id", deviceID)

	// Re-discover to update device information
	if err := m.runDiscovery(ctx); err != nil {
		return errors.Wrap(err, errors.DiskDiscoveryFailed).
			WithMetadata("device_id", deviceID).
			WithMetadata("event", "device_changed")
	}

	// Trigger health check for this device
	m.cacheMu.RLock()
	disk, exists := m.deviceCache[deviceID]
	m.cacheMu.RUnlock()

	if exists {
		m.logger.Debug("triggering health check for changed device",
			"device_id", deviceID)

		// Queue health check (async)
		go func() {
			checkCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if _, err := m.healthMonitor.CheckHealth(checkCtx, disk); err != nil {
				m.logger.Warn("health check failed for changed device",
					"device_id", deviceID,
					"error", err)
			}
		}()
	}

	return nil
}
