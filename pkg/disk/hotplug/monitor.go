// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package hotplug

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
)

// Monitor monitors udev events for hotplug disk detection.
//
// It runs 'udevadm monitor --subsystem-match=block --property' as a long-running
// process and parses the event stream in real-time. Events are deduplicated using
// a correlation map with 2-second TTL to handle duplicate kernel notifications.
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
// This uses udevadm monitor under the hood, which connects to the kernel via netlink
func (m *Monitor) Start(udevadmPath string) error {
	m.logger.Info("starting udev monitor", "subsystems", m.subsystems)

	// Start the correlation cleanup goroutine
	go m.correlationCleanup()

	// Start udevadm monitor in a goroutine
	go m.runMonitor(udevadmPath)

	return nil
}

// Stop stops the udev monitor
func (m *Monitor) Stop() error {
	m.logger.Info("stopping udev monitor")
	m.cancel()
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

// runMonitor runs the udevadm monitor command and processes output
func (m *Monitor) runMonitor(udevadmPath string) {
	// Build udevadm command arguments
	args := []string{"monitor", "--property"}
	for _, subsystem := range m.subsystems {
		args = append(args, "--subsystem-match="+subsystem)
	}

	// Start udevadm monitor process
	// Note: We'll need to use exec.CommandContext for this
	cmd := m.buildMonitorCommand(m.ctx, udevadmPath, args)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.errors <- errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "udev_monitor_pipe")
		return
	}

	if err := cmd.Start(); err != nil {
		m.errors <- errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "udev_monitor_start")
		return
	}

	m.logger.Info("udev monitor started successfully")

	// Process udevadm output
	m.processMonitorOutput(stdout)

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		if m.ctx.Err() == nil {
			// Only log error if context wasn't cancelled
			m.errors <- errors.Wrap(err, errors.OperationFailed).
				WithMetadata("operation", "udev_monitor_wait")
		}
	}

	m.logger.Info("udev monitor stopped")
}

// processMonitorOutput processes the output from udevadm monitor
func (m *Monitor) processMonitorOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	var currentEvent *UdevEvent
	var inEvent bool

	for scanner.Scan() {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		// Event separator line
		if strings.HasPrefix(line, "KERNEL") || strings.HasPrefix(line, "UDEV") {
			// Save previous event if any
			if currentEvent != nil && m.isRelevantEvent(currentEvent) {
				m.emitEvent(currentEvent)
			}

			// Start new event
			currentEvent = &UdevEvent{
				Properties: make(map[string]string),
				Timestamp:  time.Now(),
			}
			inEvent = true

			// Parse the header line: "KERNEL[12345.678] add /devices/..."
			m.parseHeaderLine(line, currentEvent)
			continue
		}

		// Property lines
		if inEvent && currentEvent != nil {
			m.parsePropertyLine(line, currentEvent)
		}
	}

	// Save last event
	if currentEvent != nil && m.isRelevantEvent(currentEvent) {
		m.emitEvent(currentEvent)
	}

	if err := scanner.Err(); err != nil {
		m.statsMu.Lock()
		m.stats.Errors++
		m.statsMu.Unlock()

		m.errors <- errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "udev_monitor_scan")
	}
}

// parseHeaderLine parses the udev event header line
func (m *Monitor) parseHeaderLine(line string, event *UdevEvent) {
	// Line format: "KERNEL[12345.678] add /devices/pci0000:00/..."
	// or: "UDEV[12345.678] add /devices/pci0000:00/..."

	parts := strings.SplitN(line, "]", 2)
	if len(parts) < 2 {
		return
	}

	// Extract sequence number from [12345.678]
	seqPart := strings.TrimPrefix(parts[0], "KERNEL[")
	seqPart = strings.TrimPrefix(seqPart, "UDEV[")
	if seqNum, err := strconv.ParseFloat(seqPart, 64); err == nil {
		event.SeqNum = uint64(seqNum * 1000) // Convert to milliseconds
	}

	// Extract action and path
	remainder := strings.TrimSpace(parts[1])
	actionPath := strings.SplitN(remainder, " ", 2)
	if len(actionPath) >= 1 {
		event.Action = UdevAction(actionPath[0])
	}
	if len(actionPath) >= 2 {
		event.SysPath = actionPath[1]
	}
}

// parsePropertyLine parses a udev property line
func (m *Monitor) parsePropertyLine(line string, event *UdevEvent) {
	// Property lines are in KEY=VALUE format
	if !strings.Contains(line, "=") {
		return
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Store all properties
	event.Properties[key] = value

	// Extract important properties into event fields
	switch key {
	case "DEVNAME":
		event.DevPath = value
		// Extract device name from path (e.g., /dev/sda -> sda)
		if idx := strings.LastIndex(value, "/"); idx >= 0 {
			event.DevName = value[idx+1:]
		}
	case "DEVTYPE":
		event.DevType = value
	case "SUBSYSTEM":
		event.Subsystem = value
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

// buildMonitorCommand creates the exec.Cmd for udevadm monitor
func (m *Monitor) buildMonitorCommand(ctx context.Context, path string, args []string) *exec.Cmd {
	return exec.CommandContext(ctx, path, args...)
}
