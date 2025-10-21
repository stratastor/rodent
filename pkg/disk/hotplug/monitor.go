// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package hotplug

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/pilebones/go-udev/netlink"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
)

// Monitor monitors udev events for hotplug disk detection.
//
// It connects directly to the kernel's netlink socket (NETLINK_KOBJECT_UEVENT)
// to receive udev events in real-time with no buffering delays. Events are
// deduplicated using a correlation map with 2-second TTL to handle duplicate
// kernel notifications.
//
// The monitor runs in its own goroutine and sends parsed events to a buffered
// channel. It handles context cancellation gracefully and tracks statistics.
//
// Thread-safety: Safe for concurrent use. Statistics and correlation map use
// read-write mutexes for efficient concurrent access.
type Monitor struct {
	logger logger.Logger
	ctx    context.Context
	cancel context.CancelFunc

	// Event channels
	events chan *UdevEvent
	errors chan error

	// Netlink connection
	conn *netlink.UEventConn

	// Correlation tracking
	correlationMap map[EventCorrelationKey]*CorrelatedEvent
	correlationMu  sync.RWMutex
	correlationTTL time.Duration

	// Statistics
	stats      MonitorStats
	statsMu    sync.RWMutex

	// Configuration
	subsystems []string // Subsystems to monitor (default: block)
	bufferSize int      // Event buffer size
}

// MonitorStats tracks monitoring statistics
type MonitorStats struct {
	EventsReceived  uint64
	EventsProcessed uint64
	EventsDropped   uint64
	EventsFiltered  uint64
	Duplicates      uint64
	Errors          uint64
	LastEvent       time.Time
	StartTime       time.Time
}

// NewMonitor creates a new udev monitor
func NewMonitor(l logger.Logger, subsystems []string, bufferSize int) *Monitor {
	if len(subsystems) == 0 {
		subsystems = []string{"block"}
	}

	if bufferSize <= 0 {
		bufferSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Monitor{
		logger:         l,
		ctx:            ctx,
		cancel:         cancel,
		events:         make(chan *UdevEvent, bufferSize),
		errors:         make(chan error, 10),
		correlationMap: make(map[EventCorrelationKey]*CorrelatedEvent),
		correlationTTL: 2 * time.Second, // Deduplicate events within 2 seconds
		subsystems:     subsystems,
		bufferSize:     bufferSize,
		stats: MonitorStats{
			StartTime: time.Now(),
		},
	}
}

// Start begins monitoring udev events
// This connects directly to the kernel's netlink socket (NETLINK_KOBJECT_UEVENT)
func (m *Monitor) Start(udevadmPath string) error {
	m.logger.Info("starting udev monitor via netlink", "subsystems", m.subsystems)

	// Create netlink connection
	m.conn = new(netlink.UEventConn)
	if err := m.conn.Connect(netlink.UdevEvent); err != nil {
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "netlink_connect")
	}

	// Start the correlation cleanup goroutine
	go m.correlationCleanup()

	// Start netlink monitor in a goroutine
	go m.runMonitor()

	return nil
}

// Stop stops the udev monitor
func (m *Monitor) Stop() error {
	m.logger.Info("stopping udev monitor")
	m.cancel()

	// Close netlink connection
	if m.conn != nil {
		m.conn.Close()
	}

	close(m.events)
	close(m.errors)
	return nil
}

// Events returns the event channel
func (m *Monitor) Events() <-chan *UdevEvent {
	return m.events
}

// Errors returns the error channel
func (m *Monitor) Errors() <-chan error {
	return m.errors
}

// GetStats returns current monitoring statistics
func (m *Monitor) GetStats() MonitorStats {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()
	return m.stats
}

// runMonitor monitors netlink udev events in real-time
func (m *Monitor) runMonitor() {
	m.logger.Info("netlink monitor started successfully")

	// Create channels for netlink events
	queue := make(chan netlink.UEvent)
	netlinkErrors := make(chan error)

	// Build matcher for subsystem filtering
	var matcher *netlink.RuleDefinitions
	if len(m.subsystems) > 0 {
		rules := make([]netlink.RuleDefinition, 0, len(m.subsystems))
		for _, subsystem := range m.subsystems {
			rules = append(rules, netlink.RuleDefinition{
				Env: map[string]string{"SUBSYSTEM": subsystem},
			})
		}
		matcher = &netlink.RuleDefinitions{Rules: rules}
	}

	// Start monitoring
	m.conn.Monitor(queue, netlinkErrors, matcher)

	// Process events
	for {
		select {
		case <-m.ctx.Done():
			m.logger.Info("netlink monitor stopped")
			return

		case uevent := <-queue:
			m.processNetlinkEvent(uevent)

		case err := <-netlinkErrors:
			m.statsMu.Lock()
			m.stats.Errors++
			m.statsMu.Unlock()
			m.errors <- errors.Wrap(err, errors.OperationFailed).
				WithMetadata("operation", "netlink_monitor")
		}
	}
}

// processNetlinkEvent converts a netlink UEvent to our UdevEvent and emits it
func (m *Monitor) processNetlinkEvent(uevent netlink.UEvent) {
	// Create our event structure
	event := &UdevEvent{
		Action:     UdevAction(uevent.Action),
		SysPath:    uevent.KObj,
		Properties: uevent.Env,
		Timestamp:  time.Now(),
	}

	// Extract important properties from the environment map
	if devname, ok := uevent.Env["DEVNAME"]; ok {
		event.DevPath = devname
		// Extract device name from path (e.g., /dev/sda -> sda)
		if idx := strings.LastIndex(devname, "/"); idx >= 0 {
			event.DevName = devname[idx+1:]
		}
	}

	if devtype, ok := uevent.Env["DEVTYPE"]; ok {
		event.DevType = devtype
	}

	if subsystem, ok := uevent.Env["SUBSYSTEM"]; ok {
		event.Subsystem = subsystem
	}

	// Check if relevant and emit
	if m.isRelevantEvent(event) {
		m.emitEvent(event)
	}
}

// isRelevantEvent checks if an event should be processed
func (m *Monitor) isRelevantEvent(event *UdevEvent) bool {
	// Must have a device path
	if event.DevPath == "" {
		m.statsMu.Lock()
		m.stats.EventsFiltered++
		m.statsMu.Unlock()
		return false
	}

	// Must be a disk device (not a partition)
	if event.DevType != "disk" {
		m.statsMu.Lock()
		m.stats.EventsFiltered++
		m.statsMu.Unlock()
		return false
	}

	// Must have a relevant action
	switch event.Action {
	case UdevActionAdd, UdevActionRemove, UdevActionChange:
		return true
	default:
		m.statsMu.Lock()
		m.stats.EventsFiltered++
		m.statsMu.Unlock()
		return false
	}
}

// emitEvent sends an event to the channel if it's not a duplicate
func (m *Monitor) emitEvent(event *UdevEvent) {
	m.statsMu.Lock()
	m.stats.EventsReceived++
	m.stats.LastEvent = event.Timestamp
	m.statsMu.Unlock()

	// Check for duplicates using correlation
	if m.isDuplicate(event) {
		m.statsMu.Lock()
		m.stats.Duplicates++
		m.statsMu.Unlock()
		m.logger.Debug("duplicate event filtered",
			"device", event.DevPath,
			"action", event.Action)
		return
	}

	// Try to send event (non-blocking)
	select {
	case m.events <- event:
		m.statsMu.Lock()
		m.stats.EventsProcessed++
		m.statsMu.Unlock()
		m.logger.Debug("udev event",
			"action", event.Action,
			"device", event.DevPath,
			"type", event.DevType)
	default:
		// Buffer full, drop event
		m.statsMu.Lock()
		m.stats.EventsDropped++
		m.statsMu.Unlock()
		m.logger.Warn("event buffer full, dropping event",
			"device", event.DevPath,
			"action", event.Action)
	}
}

// isDuplicate checks if an event is a duplicate within the correlation window
func (m *Monitor) isDuplicate(event *UdevEvent) bool {
	key := EventCorrelationKey{
		DevPath: event.DevPath,
		Action:  event.Action,
	}

	m.correlationMu.Lock()
	defer m.correlationMu.Unlock()

	if correlated, exists := m.correlationMap[key]; exists {
		// Check if within TTL window
		if time.Since(correlated.Timestamp) < m.correlationTTL {
			correlated.Count++
			correlated.Timestamp = time.Now()
			return true
		}
	}

	// Not a duplicate, add to correlation map
	m.correlationMap[key] = &CorrelatedEvent{
		Event:     event,
		Timestamp: time.Now(),
		Count:     1,
	}

	return false
}

// correlationCleanup periodically cleans up old correlation entries
func (m *Monitor) correlationCleanup() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupCorrelation()
		}
	}
}

// cleanupCorrelation removes old correlation entries
func (m *Monitor) cleanupCorrelation() {
	m.correlationMu.Lock()
	defer m.correlationMu.Unlock()

	now := time.Now()
	for key, correlated := range m.correlationMap {
		if now.Sub(correlated.Timestamp) > m.correlationTTL*2 {
			delete(m.correlationMap, key)
		}
	}
}
