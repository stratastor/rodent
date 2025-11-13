// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
	generalCmd "github.com/stratastor/rodent/internal/command"
)

// Manager is the main system management interface
type Manager struct {
	infoCollector   *InfoCollector
	hostnameManager *HostnameManager
	userManager     *UserManager
	powerManager    *PowerManager
	logger          logger.Logger
}

// NewManager creates a new system manager
func NewManager(logger logger.Logger) *Manager {
	return &Manager{
		infoCollector:   NewInfoCollector(logger),
		hostnameManager: NewHostnameManager(logger),
		userManager:     NewUserManager(logger),
		powerManager:    NewPowerManager(logger),
		logger:          logger,
	}
}

// System Information Methods

// GetSystemInfo gets comprehensive system information
func (m *Manager) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	return m.infoCollector.GetSystemInfo(ctx)
}

// GetOSInfo gets operating system information
func (m *Manager) GetOSInfo(ctx context.Context) (*OSInfo, error) {
	info, err := m.infoCollector.GetSystemInfo(ctx)
	if err != nil {
		return nil, err
	}
	return &info.OS, nil
}

// GetHardwareInfo gets hardware information
func (m *Manager) GetHardwareInfo(ctx context.Context) (*HardwareInfo, error) {
	info, err := m.infoCollector.GetSystemInfo(ctx)
	if err != nil {
		return nil, err
	}
	return &info.Hardware, nil
}

// GetPerformanceInfo gets performance metrics
func (m *Manager) GetPerformanceInfo(ctx context.Context) (*PerformanceInfo, error) {
	info, err := m.infoCollector.GetSystemInfo(ctx)
	if err != nil {
		return nil, err
	}
	return &info.Performance, nil
}

// Hostname Management Methods

// GetHostname gets the current system hostname
func (m *Manager) GetHostname(ctx context.Context) (string, error) {
	return m.hostnameManager.GetHostname(ctx)
}

// GetHostnameInfo gets comprehensive hostname information
func (m *Manager) GetHostnameInfo(ctx context.Context) (*HostnameInfo, error) {
	return m.hostnameManager.GetHostnameInfo(ctx)
}

// SetHostname sets the system hostname
func (m *Manager) SetHostname(ctx context.Context, request SetHostnameRequest) error {
	if err := m.hostnameManager.SetHostname(ctx, request); err != nil {
		return err
	}
	
	m.logger.Info("System hostname changed successfully", "new_hostname", request.Hostname)
	return nil
}

// User Management Methods

// GetUsers lists all system users
func (m *Manager) GetUsers(ctx context.Context) ([]User, error) {
	return m.userManager.GetUsers(ctx)
}

// GetUser gets a specific user by username
func (m *Manager) GetUser(ctx context.Context, username string) (*User, error) {
	return m.userManager.GetUser(ctx, username)
}

// CreateUser creates a new system user
func (m *Manager) CreateUser(ctx context.Context, request CreateUserRequest) error {
	if err := m.userManager.CreateUser(ctx, request); err != nil {
		return err
	}
	
	m.logger.Info("System user created successfully", "username", request.Username)
	return nil
}

// DeleteUser deletes a system user
func (m *Manager) DeleteUser(ctx context.Context, username string) error {
	if err := m.userManager.DeleteUser(ctx, username); err != nil {
		return err
	}

	m.logger.Info("System user deleted successfully", "username", username)
	return nil
}

// UpdateUser updates an existing system user
func (m *Manager) UpdateUser(ctx context.Context, request UpdateUserRequest) error {
	if err := m.userManager.UpdateUser(ctx, request); err != nil {
		return err
	}

	m.logger.Info("System user updated successfully", "username", request.Username)
	return nil
}

// SetPassword sets or changes a user's password
func (m *Manager) SetPassword(ctx context.Context, username, password string) error {
	if err := m.userManager.SetPassword(ctx, username, password); err != nil {
		return err
	}

	m.logger.Info("User password set successfully", "username", username)
	return nil
}

// LockUser locks a user account
func (m *Manager) LockUser(ctx context.Context, username string) error {
	if err := m.userManager.LockUser(ctx, username); err != nil {
		return err
	}

	m.logger.Info("User account locked successfully", "username", username)
	return nil
}

// UnlockUser unlocks a user account
func (m *Manager) UnlockUser(ctx context.Context, username string) error {
	if err := m.userManager.UnlockUser(ctx, username); err != nil {
		return err
	}

	m.logger.Info("User account unlocked successfully", "username", username)
	return nil
}

// AddUserToGroup adds a user to a group
func (m *Manager) AddUserToGroup(ctx context.Context, username, groupName string) error {
	if err := m.userManager.AddUserToGroup(ctx, username, groupName); err != nil {
		return err
	}

	m.logger.Info("User added to group successfully", "username", username, "group", groupName)
	return nil
}

// RemoveUserFromGroup removes a user from a group
func (m *Manager) RemoveUserFromGroup(ctx context.Context, username, groupName string) error {
	if err := m.userManager.RemoveUserFromGroup(ctx, username, groupName); err != nil {
		return err
	}

	m.logger.Info("User removed from group successfully", "username", username, "group", groupName)
	return nil
}

// SetPrimaryGroup sets a user's primary group
func (m *Manager) SetPrimaryGroup(ctx context.Context, username, groupName string) error {
	if err := m.userManager.SetPrimaryGroup(ctx, username, groupName); err != nil {
		return err
	}

	m.logger.Info("User primary group set successfully", "username", username, "group", groupName)
	return nil
}

// GetUserGroups gets all groups a user belongs to
func (m *Manager) GetUserGroups(ctx context.Context, username string) ([]string, error) {
	return m.userManager.GetUserGroups(ctx, username)
}

// Group Management Methods

// GetGroups lists all system groups
func (m *Manager) GetGroups(ctx context.Context) ([]Group, error) {
	return m.userManager.GetGroups(ctx)
}

// GetGroup gets a specific group by name
func (m *Manager) GetGroup(ctx context.Context, groupName string) (*Group, error) {
	return m.userManager.GetGroup(ctx, groupName)
}

// CreateGroup creates a new system group
func (m *Manager) CreateGroup(ctx context.Context, request CreateGroupRequest) error {
	if err := m.userManager.CreateGroup(ctx, request); err != nil {
		return err
	}
	
	m.logger.Info("System group created successfully", "name", request.Name)
	return nil
}

// DeleteGroup deletes a system group
func (m *Manager) DeleteGroup(ctx context.Context, groupName string) error {
	if err := m.userManager.DeleteGroup(ctx, groupName); err != nil {
		return err
	}
	
	m.logger.Info("System group deleted successfully", "name", groupName)
	return nil
}

// Power Management Methods

// GetPowerStatus gets current power management status
func (m *Manager) GetPowerStatus(ctx context.Context) (map[string]interface{}, error) {
	return m.powerManager.GetPowerStatus(ctx)
}

// GetScheduledShutdown checks for scheduled shutdowns
func (m *Manager) GetScheduledShutdown(ctx context.Context) (*ScheduledShutdownInfo, error) {
	return m.powerManager.GetScheduledShutdown(ctx)
}

// Shutdown shuts down the system immediately
func (m *Manager) Shutdown(ctx context.Context, request PowerOperationRequest) error {
	if err := m.powerManager.ValidatePowerOperation("shutdown", request); err != nil {
		return err
	}
	
	return m.powerManager.Shutdown(ctx, request)
}

// Reboot reboots the system immediately
func (m *Manager) Reboot(ctx context.Context, request PowerOperationRequest) error {
	if err := m.powerManager.ValidatePowerOperation("reboot", request); err != nil {
		return err
	}
	
	return m.powerManager.Reboot(ctx, request)
}

// ScheduleShutdown schedules a shutdown after a delay
func (m *Manager) ScheduleShutdown(ctx context.Context, delay time.Duration, message string) error {
	return m.powerManager.ScheduleShutdown(ctx, delay, message)
}

// CancelScheduledShutdown cancels a scheduled shutdown
func (m *Manager) CancelScheduledShutdown(ctx context.Context) error {
	return m.powerManager.CancelScheduledShutdown(ctx)
}

// System Configuration Methods

// GetTimezone gets the current system timezone
func (m *Manager) GetTimezone(ctx context.Context) (string, error) {
	info, err := m.infoCollector.GetSystemInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.Timezone, nil
}

// SetTimezone sets the system timezone
func (m *Manager) SetTimezone(ctx context.Context, request SetTimezoneRequest) error {
	m.logger.Info("Setting system timezone", "timezone", request.Timezone)
	
	// Use timedatectl to set timezone with sudo
	wrapper := &commandExecutorWrapper{
		executor: generalCmd.NewCommandExecutor(true),
	}
	
	result, err := wrapper.ExecuteCommand(ctx, "timedatectl", "set-timezone", request.Timezone)
	if err != nil {
		m.logger.Error("Failed to set timezone", "timezone", request.Timezone, "error", err)
		return errors.Wrap(err, errors.SystemTimezoneSetFailed).
			WithMetadata("timezone", request.Timezone).
			WithMetadata("output", result.Stdout)
	}
	
	m.logger.Info("System timezone changed successfully", "timezone", request.Timezone)
	return nil
}

// GetLocale gets the current system locale
func (m *Manager) GetLocale(ctx context.Context) (string, error) {
	info, err := m.infoCollector.GetSystemInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.Locale, nil
}

// SetLocale sets the system locale
func (m *Manager) SetLocale(ctx context.Context, request SetLocaleRequest) error {
	m.logger.Info("Setting system locale", "locale", request.Locale)
	
	// Use localectl to set locale with sudo
	wrapper := &commandExecutorWrapper{
		executor: generalCmd.NewCommandExecutor(true),
	}
	
	result, err := wrapper.ExecuteCommand(ctx, "localectl", "set-locale", "LANG="+request.Locale)
	if err != nil {
		m.logger.Error("Failed to set locale", "locale", request.Locale, "error", err)
		return errors.Wrap(err, errors.SystemLocaleSetFailed).
			WithMetadata("locale", request.Locale).
			WithMetadata("output", result.Stdout)
	}
	
	m.logger.Info("System locale changed successfully", "locale", request.Locale)
	return nil
}

// Validation and Utility Methods

// ValidateSystemOperation validates system operations
func (m *Manager) ValidateSystemOperation(operation string, params map[string]interface{}) error {
	switch operation {
	case "shutdown", "reboot":
		// Power operations are handled by power manager
		return nil
	case "set_hostname":
		hostname, ok := params["hostname"].(string)
		if !ok || hostname == "" {
			return errors.New(errors.SystemHostnameInvalid, "Invalid hostname parameter")
		}
		return m.hostnameManager.validateHostname(hostname)
	case "create_user":
		// User creation validation is handled by user manager
		return nil
	case "create_group":
		// Group creation validation is handled by user manager
		return nil
	default:
		return errors.New(errors.SystemOperationNotSupported, "Unknown system operation: "+operation)
	}
}

// GetSystemHealth provides a consolidated system health status
func (m *Manager) GetSystemHealth(ctx context.Context) (map[string]interface{}, error) {
	health := make(map[string]interface{})
	
	// Get basic system info
	sysInfo, err := m.GetSystemInfo(ctx)
	if err != nil {
		health["system_info_error"] = err.Error()
	} else {
		health["uptime"] = sysInfo.Uptime.String()
		health["hostname"] = sysInfo.Hostname
		health["os"] = sysInfo.OS.PrettyName
		health["kernel"] = sysInfo.OS.KernelRelease
	}
	
	// Get power status including scheduled shutdowns
	powerStatus, err := m.GetPowerStatus(ctx)
	if err != nil {
		health["power_status_error"] = err.Error()
	} else {
		health["power_status"] = powerStatus
	}
	
	// Add timestamp
	health["timestamp"] = time.Now().Format(time.RFC3339)
	health["status"] = "healthy"
	
	return health, nil
}