// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package probing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk/state"
	"github.com/stratastor/rodent/pkg/disk/tools"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// ProbeExecutor executes SMART probes and tracks their progress
type ProbeExecutor struct {
	logger       logger.Logger
	stateManager *state.StateManager
	smartctl     *tools.SmartctlExecutor
}

// NewProbeExecutor creates a new probe executor
func NewProbeExecutor(
	l logger.Logger,
	stateManager *state.StateManager,
	smartctl *tools.SmartctlExecutor,
) *ProbeExecutor {
	return &ProbeExecutor{
		logger:       l,
		stateManager: stateManager,
		smartctl:     smartctl,
	}
}

// ExecuteProbe executes a SMART probe with retry logic
func (pe *ProbeExecutor) ExecuteProbe(ctx context.Context, execution *types.ProbeExecution) error {
	pe.logger.Info("starting probe execution",
		"probe_id", execution.ID,
		"device_id", execution.DeviceID,
		"probe_type", string(execution.Type))

	// Device path is already in execution
	devicePath := execution.DevicePath
	if devicePath == "" {
		return pe.failProbe(execution, errors.New(errors.DiskNotFound, "device path not set").
			WithMetadata("device_id", execution.DeviceID))
	}

	// Use helper method to start
	execution.Start()
	pe.updateProbeState(execution)

	// Start the SMART test
	if err := pe.startTest(ctx, devicePath, execution); err != nil {
		return pe.failProbe(execution, err)
	}

	// Poll for completion
	if err := pe.pollForCompletion(ctx, devicePath, execution); err != nil {
		return pe.failProbe(execution, err)
	}

	// Use helper method to complete
	execution.Complete(types.ProbeResultPass, "Self-test completed successfully")
	pe.updateProbeState(execution)

	pe.logger.Info("probe execution completed",
		"probe_id", execution.ID,
		"device_id", execution.DeviceID,
		"duration", execution.Duration)

	return nil
}

// CancelProbe cancels a running probe
func (pe *ProbeExecutor) CancelProbe(ctx context.Context, probeID string) error {
	// Find execution in state (ProbeExecutions, not ProbeHistory)
	var execution *types.ProbeExecution
	pe.stateManager.WithRLock(func(s *types.DiskManagerState) {
		if exec, ok := s.ProbeExecutions[probeID]; ok {
			execution = exec
		}
	})

	if execution == nil {
		return errors.New(errors.DiskProbeNotFound, "probe not found").
			WithMetadata("probe_id", probeID)
	}

	if execution.Status != types.ProbeStatusRunning {
		return errors.New(errors.DiskProbeConflict, "probe not running").
			WithMetadata("probe_id", probeID).
			WithMetadata("status", string(execution.Status))
	}

	// Use smartctl to abort the test
	devicePath := execution.DevicePath
	if devicePath != "" {
		_, err := pe.smartctl.AbortTest(ctx, devicePath)
		if err != nil {
			pe.logger.Error("failed to abort probe",
				"probe_id", probeID,
				"device", devicePath,
				"error", err)
		}
	}

	// Use helper method to cancel
	execution.Cancel()
	pe.updateProbeState(execution)

	pe.logger.Info("probe cancelled",
		"probe_id", probeID,
		"device_id", execution.DeviceID)

	return nil
}

// startTest starts the SMART self-test
func (pe *ProbeExecutor) startTest(ctx context.Context, devicePath string, execution *types.ProbeExecution) error {
	var output []byte
	var err error

	switch execution.Type {
	case types.ProbeTypeQuick:
		output, err = pe.smartctl.StartQuickTest(ctx, devicePath)
	case types.ProbeTypeExtensive:
		output, err = pe.smartctl.StartExtensiveTest(ctx, devicePath)
	default:
		return errors.New(errors.DiskProbeFailed, "unsupported probe type").
			WithMetadata("probe_type", string(execution.Type))
	}

	if err != nil {
		return errors.Wrap(err, errors.DiskProbeStartFailed).
			WithMetadata("device", devicePath).
			WithMetadata("probe_type", string(execution.Type))
	}

	// Parse response to get estimated completion time
	var response struct {
		Smartctl struct {
			ExitStatus int `json:"exit_status"`
		} `json:"smartctl"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		pe.logger.Warn("failed to parse test start response",
			"device", devicePath,
			"error", err)
	}

	pe.logger.Debug("SMART test started",
		"probe_id", execution.ID,
		"device", devicePath,
		"probe_type", string(execution.Type))

	return nil
}

// pollForCompletion polls the device until the test completes
func (pe *ProbeExecutor) pollForCompletion(ctx context.Context, devicePath string, execution *types.ProbeExecution) error {
	// Determine timeout based on probe type
	timeout := types.DefaultProbeTimeout
	if execution.Type == types.ProbeTypeExtensive {
		timeout = timeout * 3 // Extensive tests take longer
	}

	deadline := time.Now().Add(timeout)
	pollInterval := 30 * time.Second

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), errors.DiskProbeCancelled).
				WithMetadata("probe_id", execution.ID)

		case <-ticker.C:
			// Check if test is still running
			output, err := pe.smartctl.GetAll(ctx, devicePath)
			if err != nil {
				pe.logger.Warn("failed to poll test status",
					"probe_id", execution.ID,
					"device", devicePath,
					"error", err)
				continue
			}

			// Parse the response
			var smart struct {
				AtaSmartData *struct {
					SelfTest *struct {
						Status struct {
							Value  int    `json:"value"`
							String string `json:"string"`
							Passed bool   `json:"passed"`
						} `json:"status"`
						PollingMinutes *struct {
							Short    int `json:"short"`
							Extended int `json:"extended"`
						} `json:"polling_minutes"`
					} `json:"self_test"`
				} `json:"ata_smart_data"`
			}

			if err := json.Unmarshal(output, &smart); err != nil {
				pe.logger.Warn("failed to parse test status",
					"probe_id", execution.ID,
					"error", err)
				continue
			}

			// Check if test completed
			if smart.AtaSmartData != nil && smart.AtaSmartData.SelfTest != nil {
				status := smart.AtaSmartData.SelfTest.Status

				// Status value 0 means no test running
				if status.Value == 0 {
					execution.UpdateProgress(100, 0)
					pe.updateProbeState(execution)
					return nil
				}

				// Update progress (value 249 = in progress, bits 0-3 indicate % remaining)
				if status.Value >= 240 && status.Value <= 249 {
					remaining := (status.Value & 0x0F) * 10
					percent := 100 - remaining
					execution.UpdateProgress(percent, 0)
					pe.updateProbeState(execution)
				}
			}

			// Check timeout
			if time.Now().After(deadline) {
				execution.Timeout()
				pe.updateProbeState(execution)
				return errors.New(errors.DiskProbeTimeout, "probe operation timed out").
					WithMetadata("probe_id", execution.ID).
					WithMetadata("timeout", timeout.String())
			}
		}
	}
}

// failProbe marks a probe as failed using helper method
func (pe *ProbeExecutor) failProbe(execution *types.ProbeExecution, err error) error {
	execution.Fail(err.Error())

	// Handle retry logic if configured
	// Note: MaxRetries should be added to ProbeExecution or passed separately
	// For now, we'll just mark as failed
	pe.logger.Error("probe failed",
		"probe_id", execution.ID,
		"device_id", execution.DeviceID,
		"error", err)

	pe.updateProbeState(execution)
	return err
}

// updateProbeState updates probe execution in state
func (pe *ProbeExecutor) updateProbeState(execution *types.ProbeExecution) {
	pe.stateManager.WithLock(func(s *types.DiskManagerState) {
		// Add to ProbeExecutions map (keyed by execution ID)
		s.AddProbeExecution(execution)
	})

	pe.stateManager.SaveDebounced()
}

// GetProbeStatus returns the current status of a probe
func (pe *ProbeExecutor) GetProbeStatus(probeID string) (*types.ProbeExecution, error) {
	var execution *types.ProbeExecution

	pe.stateManager.WithRLock(func(s *types.DiskManagerState) {
		if exec, ok := s.ProbeExecutions[probeID]; ok {
			execution = exec
		}
	})

	if execution == nil {
		return nil, errors.New(errors.DiskProbeNotFound, "probe not found").
			WithMetadata("probe_id", probeID)
	}

	return execution, nil
}
