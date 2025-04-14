// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package smb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/facl"
	"github.com/stratastor/rodent/pkg/shares"
)

const (
	DefaultSMBConfigPath = "/etc/samba/smb.conf"
	SharesConfigDir      = "/etc/samba/shares.d"
	TemplateDir          = "/etc/rodent/templates/smb"
	DefaultTemplate      = "share.tmpl"
	GlobalTemplate       = "global.tmpl"
	ConfigFileExt        = ".json"
	SmbConfigFileExt     = ".smb.conf"
)

var (
	// Ensure safe share names
	shareNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9_.]{0,62}$`)
	pathRegex      = regexp.MustCompile(`^/[a-zA-Z0-9/._-]+$`)
)

// TODO: Add main smb conf path variable to facilitate testing
// Manager implements SMB share management
type Manager struct {
	logger     logger.Logger
	executor   *command.CommandExecutor
	configDir  string
	templates  map[string]*template.Template
	mutex      sync.RWMutex
	aclManager *facl.ACLManager
}

// NewManager creates a new SMB shares manager
func NewManager(
	logger logger.Logger,
	executor *command.CommandExecutor,
	aclManager *facl.ACLManager,
) (*Manager, error) {
	// Create shares config directory if it doesn't exist
	if err := os.MkdirAll(SharesConfigDir, 0755); err != nil {
		return nil, errors.Wrap(err, errors.RodentMisc).
			WithMetadata("path", SharesConfigDir)
	}

	// Define template function map
	funcMap := template.FuncMap{
		"join": strings.Join,
	}

	// Load templates
	templates := make(map[string]*template.Template)

	// Load default template from embedded files or fallback to default content
	defaultTemplate, err := template.New(DefaultTemplate).
		Funcs(funcMap).
		Parse(DefaultTemplateContent())
	if err != nil {
		return nil, errors.Wrap(err, errors.RodentMisc).
			WithMetadata("template", DefaultTemplate)
	}
	templates[DefaultTemplate] = defaultTemplate

	// Load global template from embedded files or fallback to default content
	globalTemplate, err := template.New(GlobalTemplate).
		Funcs(funcMap).
		Parse(GlobalTemplateContent())
	if err != nil {
		return nil, errors.Wrap(err, errors.RodentMisc).
			WithMetadata("template", GlobalTemplate)
	}
	templates[GlobalTemplate] = globalTemplate

	return &Manager{
		logger:     logger,
		executor:   executor,
		configDir:  SharesConfigDir,
		templates:  templates,
		aclManager: aclManager,
	}, nil
}

func (m *Manager) validateShareConfig(config *SMBShareConfig) error {
	// Validate share name
	if config.Name == "" {
		return errors.New(errors.SharesInvalidInput, "Share name cannot be empty")
	}

	if !shareNameRegex.MatchString(config.Name) {
		return errors.New(errors.SharesInvalidInput, "Invalid share name format").
			WithMetadata("name", config.Name)
	}

	// Validate path
	if config.Path == "" {
		return errors.New(errors.SharesInvalidInput, "Share path cannot be empty")
	}

	if !pathRegex.MatchString(config.Path) {
		return errors.New(errors.SharesInvalidInput, "Invalid path format").
			WithMetadata("path", config.Path)
	}

	// Check if path exists
	if _, err := os.Stat(config.Path); os.IsNotExist(err) {
		return errors.New(errors.SharesInvalidInput, "Path does not exist").
			WithMetadata("path", config.Path)
	}

	// Initialize maps if nil to prevent null pointer dereferences
	if config.Tags == nil {
		config.Tags = make(map[string]string)
	}

	if config.CustomParameters == nil {
		config.CustomParameters = make(map[string]string)
	}

	return nil
}

// ListShares returns a list of all configured SMB shares
func (m *Manager) ListShares(ctx context.Context) ([]shares.ShareConfig, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get all share config files
	files, err := filepath.Glob(filepath.Join(m.configDir, "*"+ConfigFileExt))
	if err != nil {
		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "list")
	}

	var result []shares.ShareConfig

	// Read each share config file
	for _, file := range files {
		// Skip smb.conf
		if filepath.Base(file) == "smb.conf" {
			continue
		}

		// Read share config
		data, err := os.ReadFile(file)
		if err != nil {
			m.logger.Warn("Failed to read share config file", "file", file, "error", err)
			continue
		}

		var smbConfig SMBShareConfig
		if err := json.Unmarshal(data, &smbConfig); err != nil {
			m.logger.Warn("Failed to parse share config file", "file", file, "error", err)
			continue
		}

		// Create ShareConfig from SMBShareConfig
		shareConfig := shares.ShareConfig{
			Name:        smbConfig.Name,
			Description: smbConfig.Description,
			Path:        smbConfig.Path,
			Type:        shares.ShareTypeSMB,
			Enabled:     smbConfig.Enabled,
			Tags:        smbConfig.Tags,
			Created:     getFileCreationTime(file),
			Modified:    getFileModificationTime(file),
		}

		// Get share status
		status, err := m.getShareStatus(ctx, smbConfig.Name)
		if err != nil {
			m.logger.Warn("Failed to get share status", "share", smbConfig.Name, "error", err)
			shareConfig.Status = shares.ShareStatusInactive
		} else if status {
			shareConfig.Status = shares.ShareStatusActive
		} else {
			shareConfig.Status = shares.ShareStatusInactive
		}

		result = append(result, shareConfig)
	}

	return result, nil
}

// ListSharesByType returns a list of SMB shares
func (m *Manager) ListSharesByType(
	ctx context.Context,
	shareType shares.ShareType,
) ([]shares.ShareConfig, error) {
	if shareType != shares.ShareTypeSMB {
		return nil, errors.New(errors.SharesInvalidInput, "Unsupported share type").
			WithMetadata("type", string(shareType))
	}

	return m.ListShares(ctx)
}

// GetShare returns the configuration for a specific SMB share
func (m *Manager) GetShare(ctx context.Context, name string) (*shares.ShareConfig, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Validate share name
	if !shareNameRegex.MatchString(name) {
		return nil, errors.New(errors.SharesInvalidInput, "Invalid share name format").
			WithMetadata("name", name)
	}

	// Read share config file
	filePath := filepath.Join(m.configDir, name+ConfigFileExt)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.SharesNotFound, "Share not found").
				WithMetadata("name", name)
		}
		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "get").
			WithMetadata("name", name)
	}

	var smbConfig SMBShareConfig
	if err := json.Unmarshal(data, &smbConfig); err != nil {
		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "parse").
			WithMetadata("name", name)
	}

	// Create ShareConfig from SMBShareConfig
	shareConfig := &shares.ShareConfig{
		Name:        smbConfig.Name,
		Description: smbConfig.Description,
		Path:        smbConfig.Path,
		Type:        shares.ShareTypeSMB,
		Enabled:     smbConfig.Enabled,
		Tags:        smbConfig.Tags,
		Created:     getFileCreationTime(filePath),
		Modified:    getFileModificationTime(filePath),
	}

	// Get share status
	status, err := m.getShareStatus(ctx, name)
	if err != nil {
		m.logger.Warn("Failed to get share status", "share", name, "error", err)
		shareConfig.Status = shares.ShareStatusInactive
	} else if status {
		shareConfig.Status = shares.ShareStatusActive
	} else {
		shareConfig.Status = shares.ShareStatusInactive
	}

	return shareConfig, nil
}

// GetSMBShare returns the SMB specific configuration for a share
func (m *Manager) GetSMBShare(ctx context.Context, name string) (*SMBShareConfig, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Validate share name
	if !shareNameRegex.MatchString(name) {
		return nil, errors.New(errors.SharesInvalidInput, "Invalid share name format").
			WithMetadata("name", name)
	}

	// Read share config file
	filePath := filepath.Join(m.configDir, name+ConfigFileExt)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.SharesNotFound, "Share not found").
				WithMetadata("name", name)
		}
		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "get").
			WithMetadata("name", name)
	}

	var smbConfig SMBShareConfig
	if err := json.Unmarshal(data, &smbConfig); err != nil {
		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "parse").
			WithMetadata("name", name)
	}

	return &smbConfig, nil
}

// CreateShare creates a new SMB share
func (m *Manager) CreateShare(ctx context.Context, config interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Convert interface to SMBShareConfig
	smbConfig, ok := config.(*SMBShareConfig)
	if !ok {
		return errors.New(errors.SharesInvalidInput, "Invalid share configuration type")
	}

	// Validate share configuration
	if err := m.validateShareConfig(smbConfig); err != nil {
		return err
	}

	// Check if share already exists
	filePath := filepath.Join(m.configDir, smbConfig.Name+ConfigFileExt)
	if _, err := os.Stat(filePath); err == nil {
		return errors.New(errors.SharesAlreadyExists, "Share already exists").
			WithMetadata("name", smbConfig.Name)
	}

	// Save share configuration
	data, err := json.MarshalIndent(smbConfig, "", "  ")
	if err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "marshal").
			WithMetadata("name", smbConfig.Name)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "save").
			WithMetadata("name", smbConfig.Name)
	}

	// Generate SMB configuration
	if err := m.generateShareConfig(smbConfig); err != nil {
		return err
	}

	// Reload SMB configuration
	if err := m.ReloadConfig(ctx); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "reload").
			WithMetadata("name", smbConfig.Name)
	}

	return nil
}

// UpdateShare updates an existing SMB share
func (m *Manager) UpdateShare(ctx context.Context, name string, config interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Convert interface to SMBShareConfig
	smbConfig, ok := config.(*SMBShareConfig)
	if !ok {
		return errors.New(errors.SharesInvalidInput, "Invalid share configuration type")
	}

	// Validate share configuration
	if err := m.validateShareConfig(smbConfig); err != nil {
		return err
	}

	// Ensure name consistency
	if name != smbConfig.Name {
		return errors.New(errors.SharesInvalidInput, "Share name mismatch").
			WithMetadata("name", name).
			WithMetadata("config_name", smbConfig.Name)
	}

	// Check if share exists
	filePath := filepath.Join(m.configDir, name+ConfigFileExt)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return errors.New(errors.SharesNotFound, "Share not found").
			WithMetadata("name", name)
	}

	// Save share configuration
	data, err := json.MarshalIndent(smbConfig, "", "  ")
	if err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "marshal").
			WithMetadata("name", name)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "save").
			WithMetadata("name", name)
	}

	// Generate SMB configuration
	if err := m.generateShareConfig(smbConfig); err != nil {
		return err
	}

	// Reload SMB configuration
	if err := m.ReloadConfig(ctx); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "reload").
			WithMetadata("name", name)
	}

	return nil
}

// DeleteShare deletes an SMB share
func (m *Manager) DeleteShare(ctx context.Context, name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Validate share name
	if !shareNameRegex.MatchString(name) {
		return errors.New(errors.SharesInvalidInput, "Invalid share name format").
			WithMetadata("name", name)
	}

	// Check if share exists
	filePath := filepath.Join(m.configDir, name+ConfigFileExt)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return errors.New(errors.SharesNotFound, "Share not found").
			WithMetadata("name", name)
	}

	// Remove share configuration file
	if err := os.Remove(filePath); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "delete").
			WithMetadata("name", name)
	}

	// Remove generated SMB configuration
	smbConfPath := filepath.Join(SharesConfigDir, name+SmbConfigFileExt)
	if err := os.Remove(smbConfPath); err != nil && !os.IsNotExist(err) {
		m.logger.Warn("Failed to remove SMB configuration file",
			"file", smbConfPath,
			"error", err)
	}

	// Reload SMB configuration
	if err := m.ReloadConfig(ctx); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "reload").
			WithMetadata("name", name)
	}

	return nil
}

// GetShareStats returns statistics for an SMB share
func (m *Manager) GetShareStats(ctx context.Context, name string) (*shares.ShareStats, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Validate share name
	if !shareNameRegex.MatchString(name) {
		return nil, errors.New(errors.SharesInvalidInput, "Invalid share name format").
			WithMetadata("name", name)
	}

	// Get SMB statistics
	smbStats, err := m.GetSMBShareStats(ctx, name)
	if err != nil {
		return nil, err
	}

	// Create ShareStats from SMBShareStats
	stats := &shares.ShareStats{
		ActiveConnections: smbStats.ActiveSessions,
		OpenFiles:         smbStats.OpenFiles,
		Status:            smbStats.Status,
		ConfModified:      smbStats.ConfModified,
	}

	// Set last accessed time if there are open files
	if len(smbStats.Files) > 0 {
		// Find the most recent access
		var latestTime time.Time
		for _, file := range smbStats.Files {
			if file.OpenedAt.After(latestTime) {
				latestTime = file.OpenedAt
			}
		}
		stats.LastAccessed = latestTime
	}

	return stats, nil
}

// GetSMBShareStats returns detailed SMB statistics for a share
func (m *Manager) GetSMBShareStats(ctx context.Context, name string) (*SMBShareStats, error) {
	// Validate share name
	if !shareNameRegex.MatchString(name) {
		return nil, errors.New(errors.SharesInvalidInput, "Invalid share name format").
			WithMetadata("name", name)
	}

	filePath := filepath.Join(m.configDir, name+ConfigFileExt)

	// Run smbstatus to get detailed information
	out, err := exec.CommandContext(ctx, "sudo", "smbstatus", "-j").Output()
	if err != nil {
		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "stats").
			WithMetadata("name", name)
	}

	// Parse JSON output
	var smbStatus struct {
		Sessions map[string]struct {
			SessionID     string `json:"session_id"`
			Username      string `json:"username"`
			GroupName     string `json:"groupname"`
			RemoteMachine string `json:"remote_machine"`
			Encryption    struct {
				Cipher string `json:"cipher"`
				Degree string `json:"degree"`
			} `json:"encryption"`
			Signing struct {
				Cipher string `json:"cipher"`
				Degree string `json:"degree"`
			} `json:"signing"`
		} `json:"sessions"`
		Tcons map[string]struct {
			Service     string `json:"service"`
			SessionID   string `json:"session_id"`
			Machine     string `json:"machine"`
			ConnectedAt string `json:"connected_at"`
		} `json:"tcons"`
		OpenFiles map[string]struct {
			ServicePath string `json:"service_path"`
			Filename    string `json:"filename"`
			Opens       map[string]struct {
				UID       int    `json:"uid"`
				OpenedAt  string `json:"opened_at"`
				ShareMode struct {
					Read   bool   `json:"READ"`
					Write  bool   `json:"WRITE"`
					Delete bool   `json:"DELETE"`
					Text   string `json:"text"`
				} `json:"sharemode"`
				AccessMask struct {
					ReadData   bool   `json:"READ_DATA"`
					WriteData  bool   `json:"WRITE_DATA"`
					AppendData bool   `json:"APPEND_DATA"`
					Text       string `json:"text"`
				} `json:"access_mask"`
			} `json:"opens"`
		} `json:"open_files"`
	}

	if err := json.Unmarshal(out, &smbStatus); err != nil {
		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "parse_stats").
			WithMetadata("name", name)
	}

	// Create SMBShareStats
	stats := &SMBShareStats{
		Sessions: make([]SMBSession, 0),
		Files:    make([]SMBOpenFile, 0),
		Status:   shares.ShareStatusInactive,
	}

	// Track session IDs for this share
	shareSessions := make(map[string]bool)

	// Map to store connection times for each session
	sessionConnectedTimes := make(map[string]time.Time)

	// Gather connection info from tcons and record connection times
	for _, tcon := range smbStatus.Tcons {
		if tcon.Service == name {
			shareSessions[tcon.SessionID] = true
			stats.Status = shares.ShareStatusActive

			// Parse and store connection time for this session
			connectedAt, err := time.Parse(time.RFC3339, tcon.ConnectedAt)
			if err == nil {
				sessionConnectedTimes[tcon.SessionID] = connectedAt
			}
		}
	}

	// Collect sessions for this share
	for sessionID, session := range smbStatus.Sessions {
		if shareSessions[sessionID] {
			// Get the connection time from our map
			connectedAt := sessionConnectedTimes[sessionID]

			smbSession := SMBSession{
				SessionID:     sessionID,
				Username:      session.Username,
				GroupName:     session.GroupName,
				RemoteMachine: session.RemoteMachine,
				ConnectedAt:   connectedAt,
				Encryption:    session.Encryption.Degree,
				Signing:       session.Signing.Degree,
			}

			stats.Sessions = append(stats.Sessions, smbSession)
		}
	}

	// Collect open files for this share
	for path, fileInfo := range smbStatus.OpenFiles {
		// There are multiple ways a file might be associated with a share:
		// 1. Direct match on ServicePath to the share name
		// 2. ServicePath is a subdirectory of the share
		// 3. The file could be in a share that's mounted at a different path

		// Here we check if the service path (without leading slash) matches the share name
		// OR if the service path contains the tcon service name from any active connections
		belongsToShare := strings.TrimPrefix(fileInfo.ServicePath, "/") == name

		// If not a direct match, check if the service path is used by this share in any tcon
		if !belongsToShare {
			for _, tcon := range smbStatus.Tcons {
				if tcon.Service == name {
					// This is our share - if the path contains our share's path, include it
					if strings.Contains(path, fileInfo.ServicePath) {
						belongsToShare = true
						break
					}
				}
			}
		}

		if belongsToShare {
			for openID, openInfo := range fileInfo.Opens {
				openedAt, _ := time.Parse(time.RFC3339, openInfo.OpenedAt)

				// Get username from session if possible
				var username string
				var sessionID string

				// Find the session ID for this open file
				for _, tcon := range smbStatus.Tcons {
					if tcon.Service == name {
						sessionID = tcon.SessionID
						break
					}
				}

				// Get username from the session
				if session, ok := smbStatus.Sessions[sessionID]; ok {
					username = session.Username
				}

				smbFile := SMBOpenFile{
					Path:         path,
					ShareName:    name,
					Username:     username,
					OpenedAt:     openedAt,
					AccessMode:   openInfo.ShareMode.Text,
					AccessRights: openInfo.AccessMask.Text,
					OpenID:       openID,
				}

				stats.Files = append(stats.Files, smbFile)
			}
		}
	}

	// Update counters
	stats.ActiveSessions = len(stats.Sessions)
	stats.OpenFiles = len(stats.Files)
	stats.ConfModified = getFileModificationTime(filePath)

	return stats, nil
}

// Exists checks if an SMB share exists
func (m *Manager) Exists(ctx context.Context, name string) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Validate share name
	if !shareNameRegex.MatchString(name) {
		return false, errors.New(errors.SharesInvalidInput, "Invalid share name format").
			WithMetadata("name", name)
	}

	// Check if share configuration file exists
	filePath := filepath.Join(m.configDir, name+ConfigFileExt)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "check_exists").
			WithMetadata("name", name)
	}

	return true, nil
}

// ReloadConfig reloads the SMB configuration
func (m *Manager) ReloadConfig(ctx context.Context) error {
	// Update main SMB configuration file
	if err := m.updateMainConfig(); err != nil {
		return err
	}

	// Reload SMB service configuration
	cmd := exec.CommandContext(ctx, "sudo", "smbcontrol", "smbd", "reload-config")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "reload_config")
	}

	return nil
}

// UpdateGlobalConfig updates the global SMB configuration
func (m *Manager) UpdateGlobalConfig(ctx context.Context, config *SMBGlobalConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Validate global configuration
	if config.WorkGroup == "" {
		return errors.New(errors.SharesInvalidInput, "Workgroup cannot be empty")
	}

	if config.SecurityMode == "" {
		return errors.New(errors.SharesInvalidInput, "Security mode cannot be empty")
	}

	// Save global configuration
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "marshal_global")
	}

	filePath := filepath.Join(m.configDir, "global.conf")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "save_global")
	}

	// Generate global configuration section
	if err := m.generateGlobalConfig(config); err != nil {
		return err
	}

	// Update main SMB configuration
	if err := m.updateMainConfig(); err != nil {
		return err
	}

	// Reload configuration
	return m.ReloadConfig(ctx)
}

// Modify the GetGlobalConfig method in manager.go
func (m *Manager) GetGlobalConfig(ctx context.Context) (*SMBGlobalConfig, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Read global config file
	filePath := filepath.Join(m.configDir, "global.conf")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := config.GetConfig()
			// Return default configuration if file doesn't exist
			return NewSMBGlobalConfigWithAD(cfg.Shares.SMB.Realm, cfg.Shares.SMB.Workgroup), nil
		}

		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "get_global")
	}

	var globalConfig SMBGlobalConfig
	if err := json.Unmarshal(data, &globalConfig); err != nil {
		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "parse_global")
	}

	return &globalConfig, nil
}

// GetSMBServiceStatus returns the status of the SMB service
func (m *Manager) GetSMBServiceStatus(ctx context.Context) (*SMBServiceStatus, error) {
	// Check if SMB service is running
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", "smbd")
	out, err := cmd.Output()

	status := &SMBServiceStatus{}

	if err != nil {
		status.Running = false
		return status, nil
	}

	// Service is running
	if strings.TrimSpace(string(out)) == "active" {
		status.Running = true

		// Get SMB version
		versionCmd := exec.CommandContext(ctx, "smbd", "--version")
		versionOut, err := versionCmd.Output()
		if err == nil {
			status.Version = strings.TrimSpace(string(versionOut))
		}

		// Get PID
		pidCmd := exec.CommandContext(ctx, "pidof", "smbd")
		pidOut, err := pidCmd.Output()
		if err == nil {
			pidStr := strings.Split(strings.TrimSpace(string(pidOut)), " ")[0]
			pid, err := strconv.Atoi(pidStr)
			if err == nil {
				status.PID = pid
			}
		}

		// Get config file
		status.ConfigFile = DefaultSMBConfigPath

		// Get start time
		if status.PID > 0 {
			procFile := fmt.Sprintf("/proc/%d/stat", status.PID)
			procData, err := os.ReadFile(procFile)
			if err == nil {
				procFields := strings.Fields(string(procData))
				if len(procFields) > 22 {
					startTime, err := strconv.ParseInt(procFields[21], 10, 64)
					if err == nil {
						// Convert clock ticks to seconds
						var clockTicks int64 = 100 // Usually 100 Hz on Linux
						uptime, err := os.ReadFile("/proc/uptime")
						if err == nil {
							uptimeFields := strings.Fields(string(uptime))
							if len(uptimeFields) > 0 {
								upSec, err := strconv.ParseFloat(uptimeFields[0], 64)
								if err == nil {
									bootTime := time.Now().Add(-time.Duration(upSec) * time.Second)
									status.StartTime = bootTime.Add(
										time.Duration(startTime/clockTicks) * time.Second,
									)
								}
							}
						}
					}
				}
			}
		}

		// Get active shares and sessions
		smbStatus, err := exec.CommandContext(ctx, "sudo", "smbstatus", "-f", "-j").Output()
		if err == nil {
			var parsedStatus struct {
				Sessions map[string]interface{} `json:"sessions"`
				Tcons    map[string]interface{} `json:"tcons"`
			}

			if err := json.Unmarshal(smbStatus, &parsedStatus); err == nil {
				status.ActiveSessions = len(parsedStatus.Sessions)
				status.ActiveShares = len(parsedStatus.Tcons)
			}
		}
	}

	return status, nil
}

// Helper functions

// updateMainConfig updates the main SMB configuration file
func (m *Manager) updateMainConfig() error {
	// Start with global configuration
	var content strings.Builder

	// Read global configuration
	globalPath := filepath.Join(SharesConfigDir, "global.smb.conf")
	globalData, err := os.ReadFile(globalPath)
	if err == nil {
		content.WriteString(string(globalData))
		content.WriteString("\n\n")
	} else if !os.IsNotExist(err) {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "read_global_config")
	}

	// Find all individual share config files
	shareConfigs, err := filepath.Glob(filepath.Join(SharesConfigDir, "*"+SmbConfigFileExt))
	if err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "find_share_configs")
	}

	// Append each share configuration
	content.WriteString(
		"# Do not manually edit share definitions - managed by StrataSTOR Rodent service\n",
	)
	for _, shareConfig := range shareConfigs {
		// Skip global config that was already included
		if filepath.Base(shareConfig) == "global.smb.conf" {
			continue
		}

		shareData, err := os.ReadFile(shareConfig)
		if err != nil {
			m.logger.Warn("Failed to read share config", "file", shareConfig, "error", err)
			continue
		}

		content.WriteString(string(shareData))
		content.WriteString("\n\n")
	}

	// Write updated config
	if err := os.WriteFile(DefaultSMBConfigPath, []byte(content.String()), 0644); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "write_config")
	}

	return nil
}

// generateShareConfig generates SMB configuration for a share
func (m *Manager) generateShareConfig(config *SMBShareConfig) error {
	// Get the template
	tmplName := DefaultTemplate
	tmpl, ok := m.templates[tmplName]
	if !ok {
		return errors.New(errors.SharesInternalError, "Share template not found")
	}

	// Create a new template with the function map
	funcMap := template.FuncMap{
		"join": strings.Join,
	}

	// Clone the template with the function map
	tmpl, err := tmpl.Clone()
	if err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "clone_template").
			WithMetadata("name", config.Name)
	}

	tmpl = tmpl.Funcs(funcMap)

	// Render the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "render_template").
			WithMetadata("name", config.Name)
	}

	// Write the configuration file
	filePath := filepath.Join(SharesConfigDir, config.Name+SmbConfigFileExt)
	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "write_config").
			WithMetadata("name", config.Name)
	}

	return nil
}

// generateGlobalConfig generates the global SMB configuration
func (m *Manager) generateGlobalConfig(config *SMBGlobalConfig) error {
	// Get the template
	tmpl, ok := m.templates[GlobalTemplate]
	if !ok {
		return errors.New(errors.SharesInternalError, "Global template not found")
	}

	// Render the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "render_global_template")
	}

	// Write the configuration file
	filePath := filepath.Join(SharesConfigDir, "global.smb.conf")
	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "write_global_config")
	}

	return nil
}

// getShareStatus checks if a share is active
func (m *Manager) getShareStatus(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "sudo", "smbstatus", "-f", "-j")
	out, err := cmd.Output()
	if err != nil {
		return false, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "check_status").
			WithMetadata("name", name)
	}

	var smbStatus struct {
		Tcons map[string]struct {
			Service string `json:"service"`
		} `json:"tcons"`
	}

	if err := json.Unmarshal(out, &smbStatus); err != nil {
		return false, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "parse_status").
			WithMetadata("name", name)
	}

	// Check if the share is active
	for _, tcon := range smbStatus.Tcons {
		if tcon.Service == name {
			return true, nil
		}
	}

	return false, nil
}

// BulkUpdateShares updates multiple SMB shares with the same parameters
func (m *Manager) BulkUpdateShares(
	ctx context.Context,
	config SMBBulkUpdateConfig,
) ([]SMBBulkUpdateResult, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Validate input
	if len(config.Parameters) == 0 {
		return nil, errors.New(errors.SharesInvalidInput, "No parameters specified for bulk update")
	}

	if !config.All && len(config.ShareNames) == 0 && len(config.Tags) == 0 {
		return nil, errors.New(errors.SharesInvalidInput, "No shares specified for bulk update")
	}

	// Get all shares
	allShares, err := m.getAllShareConfigs()
	if err != nil {
		return nil, err
	}

	// Filter shares to update
	var sharesToUpdate []*SMBShareConfig
	if config.All {
		sharesToUpdate = allShares
	} else if len(config.ShareNames) > 0 {
		// Filter by name
		for _, share := range allShares {
			for _, name := range config.ShareNames {
				if share.Name == name {
					sharesToUpdate = append(sharesToUpdate, share)
					break
				}
			}
		}
	} else if len(config.Tags) > 0 {
		// Filter by tags
		for _, share := range allShares {
			match := true
			for key, value := range config.Tags {
				if shareValue, ok := share.Tags[key]; !ok || shareValue != value {
					match = false
					break
				}
			}
			if match {
				sharesToUpdate = append(sharesToUpdate, share)
			}
		}
	}

	if len(sharesToUpdate) == 0 {
		return nil, errors.New(errors.SharesNotFound, "No matching shares found for update")
	}

	// Update shares
	results := make([]SMBBulkUpdateResult, 0, len(sharesToUpdate))
	for _, share := range sharesToUpdate {
		result := SMBBulkUpdateResult{
			ShareName: share.Name,
			Success:   true,
		}

		// Update custom parameters
		if share.CustomParameters == nil {
			share.CustomParameters = make(map[string]string)
		}

		// Apply bulk updates
		for key, value := range config.Parameters {
			share.CustomParameters[key] = value
		}

		// Save updated configuration
		err := m.saveShareConfig(share)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			// Generate SMB configuration
			err = m.generateShareConfig(share)
			if err != nil {
				result.Success = false
				result.Error = err.Error()
			}
		}

		results = append(results, result)
	}

	// Reload SMB configuration if at least one share was updated successfully
	anySuccess := false
	for _, result := range results {
		if result.Success {
			anySuccess = true
			break
		}
	}

	if anySuccess {
		if err := m.ReloadConfig(ctx); err != nil {
			return results, errors.Wrap(err, errors.SharesOperationFailed).
				WithMetadata("operation", "reload_after_bulk_update")
		}
	}

	return results, nil
}

// getAllShareConfigs returns all SMB share configurations
func (m *Manager) getAllShareConfigs() ([]*SMBShareConfig, error) {
	// Get all share config files
	files, err := filepath.Glob(filepath.Join(m.configDir, "*"+ConfigFileExt))
	if err != nil {
		return nil, errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "list_share_configs")
	}

	var result []*SMBShareConfig

	// Read each share config file
	for _, file := range files {
		// Skip global config
		if filepath.Base(file) == "global.conf" {
			continue
		}

		// Read share config
		data, err := os.ReadFile(file)
		if err != nil {
			m.logger.Warn("Failed to read share config file", "file", file, "error", err)
			continue
		}

		var smbConfig SMBShareConfig
		if err := json.Unmarshal(data, &smbConfig); err != nil {
			m.logger.Warn("Failed to parse share config file", "file", file, "error", err)
			continue
		}

		result = append(result, &smbConfig)
	}

	return result, nil
}

// saveShareConfig saves the share configuration to disk
func (m *Manager) saveShareConfig(config *SMBShareConfig) error {
	// Marshal config to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "marshal_config").
			WithMetadata("name", config.Name)
	}

	// Write to file
	filePath := filepath.Join(m.configDir, config.Name+ConfigFileExt)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, errors.SharesOperationFailed).
			WithMetadata("operation", "save_config").
			WithMetadata("name", config.Name)
	}

	return nil
}

// EnsureShareDefaults ensures that a share configuration has all required defaults
// This is useful for migration of old configs or handling partial user input
func (m *Manager) EnsureShareDefaults(config *SMBShareConfig) {
	// Get a new config with defaults
	defaultConfig := NewSMBShareConfig(config.Name, config.Path)

	// Make sure Tags is initialized
	if config.Tags == nil {
		config.Tags = make(map[string]string)
	}

	// Make sure CustomParameters is initialized
	if config.CustomParameters == nil {
		config.CustomParameters = defaultConfig.CustomParameters
	} else {
		// Add default parameters that aren't already specified
		for k, v := range defaultConfig.CustomParameters {
			if _, exists := config.CustomParameters[k]; !exists {
				config.CustomParameters[k] = v
			}
		}
	}

	// Set description if empty
	if config.Description == "" {
		config.Description = defaultConfig.Description
	}

	// Ensure we have a create/directory mask
	if config.CreateMask == "" {
		config.CreateMask = defaultConfig.CreateMask
	}

	if config.DirectoryMask == "" {
		config.DirectoryMask = defaultConfig.DirectoryMask
	}
}

// Expose the share name regex for validation
func GetShareNameRegex() *regexp.Regexp {
	return shareNameRegex
}

// getFileModificationTime returns the modification time of a file
func getFileModificationTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}

	return info.ModTime()
}
