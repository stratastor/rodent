// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hotplug

import (
	"strings"
	"time"

	"github.com/pilebones/go-udev/netlink"
	"github.com/stratastor/rodent/pkg/errors"
)

// Start begins monitoring udev events
// Linux implementation: connects directly to kernel netlink socket
func (m *Monitor) Start(udevadmPath string) error {
	m.logger.Info("starting udev monitor via netlink", "subsystems", m.subsystems)

	// Create netlink connection
	conn := new(netlink.UEventConn)
	if err := conn.Connect(netlink.UdevEvent); err != nil {
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "netlink_connect")
	}
	m.conn = conn

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
		if conn, ok := m.conn.(*netlink.UEventConn); ok {
			conn.Close()
		}
	}

	close(m.events)
	close(m.errors)
	return nil
}

// runMonitor monitors netlink udev events in real-time
func (m *Monitor) runMonitor() {
	m.logger.Info("netlink monitor started successfully")

	conn, ok := m.conn.(*netlink.UEventConn)
	if !ok {
		m.logger.Error("invalid netlink connection type")
		return
	}

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
	conn.Monitor(queue, netlinkErrors, matcher)

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
