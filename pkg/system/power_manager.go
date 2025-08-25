// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/stratastor/logger"
	generalCmd "github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/pkg/errors"
)

// ScheduledShutdownInfo represents information about a scheduled shutdown
type ScheduledShutdownInfo struct {
	Scheduled     bool      `json:"scheduled"`
	Mode          string    `json:"mode,omitempty"`           // "poweroff", "reboot", "halt", etc.
	Time          time.Time `json:"time,omitempty"`           // When the shutdown is scheduled
	Message       string    `json:"message,omitempty"`        // Shutdown message
	WallMessage   string    `json:"wall_message,omitempty"`   // Message shown to users
	TimeRemaining string    `json:"time_remaining,omitempty"` // Human readable time remaining
}

// PowerManager manages system power operations
type PowerManager struct {
	executor CommandExecutor
	logger   logger.Logger
}

// NewPowerManager creates a new power manager
func NewPowerManager(logger logger.Logger) *PowerManager {
	return &PowerManager{
		executor: &commandExecutorWrapper{
			executor: generalCmd.NewCommandExecutor(true), // Use sudo for power operations
		},
		logger: logger,
	}
}

// GetScheduledShutdown checks if there is a scheduled shutdown
func (pm *PowerManager) GetScheduledShutdown(ctx context.Context) (*ScheduledShutdownInfo, error) {
	info := &ScheduledShutdownInfo{
		Scheduled: false,
	}

	// Check systemd shutdown schedule file
	shutdownFile := "/run/systemd/shutdown/scheduled"
	if _, err := os.Stat(shutdownFile); os.IsNotExist(err) {
		// No scheduled shutdown
		return info, nil
	}

	// Parse the scheduled file
	if err := pm.parseScheduledShutdown(shutdownFile, info); err != nil {
		pm.logger.Warn("Failed to parse scheduled shutdown file", "error", err)
		return info, nil // Return unscheduled info if we can't parse
	}

	info.Scheduled = true
	
	// Calculate time remaining
	if !info.Time.IsZero() {
		remaining := time.Until(info.Time)
		if remaining > 0 {
			info.TimeRemaining = pm.formatDuration(remaining)
		} else {
			info.TimeRemaining = "Shutdown is overdue"
		}
	}

	return info, nil
}

// parseScheduledShutdown parses the systemd scheduled shutdown file
func (pm *PowerManager) parseScheduledShutdown(filename string, info *ScheduledShutdownInfo) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse key=value pairs
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "USEC":
				// Microseconds since epoch
				if usec, err := strconv.ParseInt(value, 10, 64); err == nil {
					info.Time = time.Unix(0, usec*1000) // Convert to nanoseconds
				}
			case "MODE":
				info.Mode = value
			case "WALL_MESSAGE":
				info.WallMessage = value
			}
		}
	}

	return scanner.Err()
}

// formatDuration formats a duration in a human-readable way
func (pm *PowerManager) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0f minutes", d.Minutes())
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%d hours", hours)
		}
		return fmt.Sprintf("%d hours %d minutes", hours, minutes)
	}
}

// Shutdown shuts down the system
func (pm *PowerManager) Shutdown(ctx context.Context, request PowerOperationRequest) error {
	pm.logger.Warn("System shutdown requested", "force", request.Force, "message", request.Message)

	// Build shutdown command
	args := []string{}

	if request.Force {
		args = append(args, "--force")
	}

	// Add delay (shutdown now)
	args = append(args, "now")

	// Add message if provided
	if request.Message != "" {
		args = append(args, request.Message)
	} else {
		args = append(args, "System shutdown requested via Rodent API")
	}

	// Log the shutdown
	pm.logger.Error("SYSTEM SHUTDOWN: Executing shutdown command", 
		"force", request.Force, 
		"message", request.Message,
		"timestamp", time.Now().Format(time.RFC3339))

	// Execute shutdown command
	result, err := pm.executor.ExecuteCommand(ctx, "shutdown", args...)
	if err != nil {
		pm.logger.Error("Failed to execute shutdown", "error", err)
		return errors.New(errors.ServerInternalError, fmt.Sprintf("Failed to shutdown system: %s", err.Error())).
			WithMetadata("operation", "shutdown").
			WithMetadata("output", result.Stdout)
	}

	pm.logger.Error("System shutdown initiated successfully")
	return nil
}

// Reboot reboots the system
func (pm *PowerManager) Reboot(ctx context.Context, request PowerOperationRequest) error {
	pm.logger.Warn("System reboot requested", "force", request.Force, "message", request.Message)

	// Build reboot command
	args := []string{}

	if request.Force {
		args = append(args, "--force")
	}

	// Add delay (reboot now)
	args = append(args, "now")

	// Add message if provided
	if request.Message != "" {
		args = append(args, request.Message)
	} else {
		args = append(args, "System reboot requested via Rodent API")
	}

	// Log the reboot
	pm.logger.Error("SYSTEM REBOOT: Executing reboot command", 
		"force", request.Force, 
		"message", request.Message,
		"timestamp", time.Now().Format(time.RFC3339))

	// Execute reboot command (using shutdown -r for consistency)
	rebootArgs := []string{"-r"}
	rebootArgs = append(rebootArgs, args...)
	
	result, err := pm.executor.ExecuteCommand(ctx, "shutdown", rebootArgs...)
	if err != nil {
		pm.logger.Error("Failed to execute reboot", "error", err)
		return errors.New(errors.ServerInternalError, fmt.Sprintf("Failed to reboot system: %s", err.Error())).
			WithMetadata("operation", "reboot").
			WithMetadata("output", result.Stdout)
	}

	pm.logger.Error("System reboot initiated successfully")
	return nil
}

// GetPowerStatus gets current power management status
func (pm *PowerManager) GetPowerStatus(ctx context.Context) (map[string]interface{}, error) {
	status := make(map[string]interface{})

	// Check if systemctl is available and get power-related status
	result, err := pm.executor.ExecuteCommand(ctx, "systemctl", "is-system-running")
	if err == nil {
		status["system_state"] = strings.TrimSpace(result.Stdout)
	}

	// Get uptime information
	result, err = pm.executor.ExecuteCommand(ctx, "uptime", "-p")
	if err == nil {
		status["uptime"] = strings.TrimSpace(result.Stdout)
	}

	// Check for pending restart (if reboot-required file exists)
	if _, err := os.Stat("/var/run/reboot-required"); err == nil {
		status["reboot_required"] = true
		
		// Get reason if available
		if data, err := os.ReadFile("/var/run/reboot-required.pkgs"); err == nil {
			status["reboot_reason"] = strings.TrimSpace(string(data))
		}
	} else {
		status["reboot_required"] = false
	}

	// Get load average
	result, err = pm.executor.ExecuteCommand(ctx, "uptime")
	if err == nil {
		status["load_info"] = strings.TrimSpace(result.Stdout)
	}

	// Get scheduled shutdown info
	scheduledInfo, err := pm.GetScheduledShutdown(ctx)
	if err == nil {
		status["scheduled_shutdown"] = scheduledInfo
	}

	return status, nil
}

// ScheduleShutdown schedules a shutdown at a specific time
func (pm *PowerManager) ScheduleShutdown(ctx context.Context, delay time.Duration, message string) error {
	if delay < time.Minute {
		return errors.New(errors.ServerRequestValidation, "Minimum shutdown delay is 1 minute")
	}

	// Convert delay to minutes for shutdown command
	minutes := int(delay.Minutes())
	
	pm.logger.Warn("Scheduled shutdown requested", "delay_minutes", minutes, "message", message)

	args := []string{fmt.Sprintf("+%d", minutes)}
	
	if message != "" {
		args = append(args, message)
	} else {
		args = append(args, "Scheduled system shutdown via Rodent API")
	}

	result, err := pm.executor.ExecuteCommand(ctx, "shutdown", args...)
	if err != nil {
		pm.logger.Error("Failed to schedule shutdown", "error", err)
		return errors.New(errors.ServerInternalError, fmt.Sprintf("Failed to schedule shutdown: %s", err.Error())).
			WithMetadata("operation", "schedule_shutdown").
			WithMetadata("delay_minutes", fmt.Sprintf("%d", minutes)).
			WithMetadata("output", result.Stdout)
	}

	pm.logger.Warn("System shutdown scheduled successfully", "delay_minutes", minutes)
	return nil
}

// CancelScheduledShutdown cancels a scheduled shutdown
func (pm *PowerManager) CancelScheduledShutdown(ctx context.Context) error {
	pm.logger.Info("Cancelling scheduled shutdown")

	result, err := pm.executor.ExecuteCommand(ctx, "shutdown", "-c")
	if err != nil {
		pm.logger.Error("Failed to cancel scheduled shutdown", "error", err)
		return errors.New(errors.ServerInternalError, fmt.Sprintf("Failed to cancel shutdown: %s", err.Error())).
			WithMetadata("operation", "cancel_shutdown").
			WithMetadata("output", result.Stdout)
	}

	pm.logger.Info("Scheduled shutdown cancelled successfully")
	return nil
}

// ValidatePowerOperation validates power operation requests
func (pm *PowerManager) ValidatePowerOperation(operation string, request PowerOperationRequest) error {
	// Validate operation type
	validOperations := []string{"shutdown", "reboot"}
	valid := false
	for _, op := range validOperations {
		if operation == op {
			valid = true
			break
		}
	}
	
	if !valid {
		return errors.New(errors.ServerRequestValidation, fmt.Sprintf("Invalid power operation: %s", operation))
	}

	// Validate message length if provided
	if len(request.Message) > 200 {
		return errors.New(errors.ServerRequestValidation, "Power operation message too long (max 200 characters)")
	}

	return nil
}