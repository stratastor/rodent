// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2024 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kballard/go-shellquote"
	"gopkg.in/yaml.v3"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
)

// TransferStatus represents the current state of a transfer
type TransferStatus string

const (
	TransferStatusPending   TransferStatus = "pending"
	TransferStatusRunning   TransferStatus = "running"
	TransferStatusPaused    TransferStatus = "paused"
	TransferStatusCompleted TransferStatus = "completed"
	TransferStatusFailed    TransferStatus = "failed"
	TransferStatusCancelled TransferStatus = "cancelled"
)

// TransferOperation represents different types of transfer operations
type TransferOperation string

const (
	TransferOperationSend        TransferOperation = "send"
	TransferOperationReceive     TransferOperation = "receive"
	TransferOperationSendReceive TransferOperation = "send_receive"
)

// TransferInfo holds comprehensive information about a ZFS transfer
type TransferInfo struct {
	ID           string            `json:"id"                      yaml:"id"`
	Operation    TransferOperation `json:"operation"               yaml:"operation"`
	Status       TransferStatus    `json:"status"                  yaml:"status"`
	Config       TransferConfig    `json:"config"                  yaml:"config"`
	Progress     TransferProgress  `json:"progress"                yaml:"progress"`
	CreatedAt    time.Time         `json:"created_at"              yaml:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"    yaml:"started_at,omitempty"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"  yaml:"completed_at,omitempty"`
	PID          int               `json:"pid,omitempty"           yaml:"pid,omitempty"`
	LogFile      string            `json:"log_file"                yaml:"log_file"`
	PIDFile      string            `json:"pid_file"                yaml:"pid_file"`
	ConfigFile   string            `json:"config_file"             yaml:"config_file"`
	ProgressFile string            `json:"progress_file"           yaml:"progress_file"`
	ErrorMessage string            `json:"error_message,omitempty" yaml:"error_message,omitempty"`
}

// TransferProgress tracks the progress of a transfer operation
type TransferProgress struct {
	BytesTransferred int64     `json:"bytes_transferred"       yaml:"bytes_transferred"`
	TotalBytes       int64     `json:"total_bytes,omitempty"   yaml:"total_bytes,omitempty"`
	TransferRate     int64     `json:"transfer_rate"           yaml:"transfer_rate"`
	ElapsedTime      int64     `json:"elapsed_time"            yaml:"elapsed_time"`
	LastUpdate       time.Time `json:"last_update"             yaml:"last_update"`
	EstimatedETA     int64     `json:"estimated_eta,omitempty" yaml:"estimated_eta,omitempty"`
}

// TransferManager manages enterprise-grade ZFS transfer operations
type TransferManager struct {
	mu              sync.RWMutex
	activeTransfers map[string]*TransferInfo
	transfersDir    string
	logger          logger.Logger
	datasetManager  *Manager
}

// NewTransferManager creates a new transfer manager instance
func (m *Manager) NewTransferManager() (*TransferManager, error) {
	l, err := logger.NewTag(logger.Config{LogLevel: "info"}, "zfs-transfer-manager")
	if err != nil {
		return nil, errors.Wrap(err, errors.RodentMisc)
	}

	tm := &TransferManager{
		activeTransfers: make(map[string]*TransferInfo),
		transfersDir:    config.GetTransfersDir(),
		logger:          l,
		datasetManager:  m,
	}

	// Load existing transfers from disk
	if err := tm.loadExistingTransfers(); err != nil {
		l.Warn("Failed to load existing transfers", "error", err)
	}

	return tm, nil
}

// StartTransfer initiates a new managed ZFS transfer operation
func (tm *TransferManager) StartTransfer(ctx context.Context, cfg TransferConfig) (string, error) {
	transferID := common.UUID7()

	transferInfo := &TransferInfo{
		ID:           transferID,
		Operation:    TransferOperationSendReceive,
		Status:       TransferStatusPending,
		Config:       cfg,
		Progress:     TransferProgress{LastUpdate: time.Now()},
		CreatedAt:    time.Now(),
		LogFile:      filepath.Join(tm.transfersDir, fmt.Sprintf("%s.log", transferID)),
		PIDFile:      filepath.Join(tm.transfersDir, fmt.Sprintf("%s.pid", transferID)),
		ConfigFile:   filepath.Join(tm.transfersDir, fmt.Sprintf("%s.yaml", transferID)),
		ProgressFile: filepath.Join(tm.transfersDir, fmt.Sprintf("%s.progress", transferID)),
	}

	// Validate configuration
	if err := validateSendConfig(cfg.SendConfig); err != nil {
		return "", err
	}
	if err := validateReceiveConfig(cfg.ReceiveConfig); err != nil {
		return "", err
	}
	if cfg.ReceiveConfig.RemoteConfig.Host != "" {
		if err := validateSSHConfig(cfg.ReceiveConfig.RemoteConfig); err != nil {
			return "", err
		}
	}

	// Ensure receive config has resumable flag for pause/resume functionality
	if !cfg.ReceiveConfig.Resumable {
		tm.logger.Warn(
			"Receive config does not have resumable flag set, pause/resume will not work properly",
		)
	}

	// Save transfer configuration
	if err := tm.saveTransferConfig(transferInfo); err != nil {
		return "", err
	}

	// Register transfer
	tm.mu.Lock()
	tm.activeTransfers[transferID] = transferInfo
	tm.mu.Unlock()

	// Start transfer in background
	go tm.executeTransfer(ctx, transferInfo)

	tm.logger.Info("Transfer initiated", "id", transferID, "operation", transferInfo.Operation)
	return transferID, nil
}

// executeTransfer runs the actual ZFS transfer operation
func (tm *TransferManager) executeTransfer(ctx context.Context, info *TransferInfo) {
	defer tm.handleTransferCompletion(info)

	// Update status to running (caller doesn't hold lock)
	tm.updateTransferStatusLocked(info, TransferStatusRunning, "")
	startTime := time.Now()
	info.StartedAt = &startTime

	// Create log file
	logFile, err := os.Create(info.LogFile)
	if err != nil {
		tm.updateTransferStatusLocked(
			info,
			TransferStatusFailed,
			fmt.Sprintf("Failed to create log file: %v", err),
		)
		return
	}
	defer logFile.Close()

	// Build and execute command
	cmd, err := tm.buildTransferCommand(info)
	if err != nil {
		tm.updateTransferStatusLocked(
			info,
			TransferStatusFailed,
			fmt.Sprintf("Failed to build command: %v", err),
		)
		return
	}

	// Setup output redirection based on dry run mode
	if info.Config.SendConfig.DryRun {
		// For dry run with -v flag, verbose output goes to stdout
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		// For normal operation, data goes to stdout (piped to receive), verbose to stderr
		cmd.Stderr = logFile // Verbose output goes to log file
	}

	// Start command
	if err := cmd.Start(); err != nil {
		tm.updateTransferStatusLocked(
			info,
			TransferStatusFailed,
			fmt.Sprintf("Failed to start command: %v", err),
		)
		return
	}

	// Save PID
	info.PID = cmd.Process.Pid
	if err := tm.savePID(info); err != nil {
		tm.logger.Warn("Failed to save PID", "error", err)
	}

	// Monitor progress in background
	go tm.monitorTransferProgress(info, logFile)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1, syscall.SIGINFO)

	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGUSR1, syscall.SIGINFO:
				// Request verbose output
				if cmd.Process != nil {
					cmd.Process.Signal(sig)
				}
			case syscall.SIGTERM, syscall.SIGINT:
				tm.StopTransfer(info.ID)
				return
			}
		}
	}()

	// Wait for command completion
	err = cmd.Wait()
	if err != nil {
		if ctx.Err() != nil {
			tm.updateTransferStatusLocked(info, TransferStatusCancelled, "Transfer cancelled")
		} else {
			tm.updateTransferStatusLocked(info, TransferStatusFailed, fmt.Sprintf("Transfer failed: %v", err))
		}
	} else {
		tm.updateTransferStatusLocked(info, TransferStatusCompleted, "")
	}
}

// buildTransferCommand constructs the ZFS send/receive command with comprehensive flag support
func (tm *TransferManager) buildTransferCommand(info *TransferInfo) (*exec.Cmd, error) {
	sendCfg := info.Config.SendConfig
	recvCfg := info.Config.ReceiveConfig

	// Build send command based on the four documented command variants:
	// 1. zfs send [-DLPVbcehnpsvw] [-R [-X dataset[,dataset]…]] [[-I|-i] snapshot] snapshot
	// 2. zfs send [-DLPVcensvw] [-i snapshot|bookmark] filesystem|volume|snapshot
	// 3. zfs send [-PVenv] -t receive_resume_token
	// 4. zfs send [-PVnv] -S filesystem

	sendPart := []string{command.BinZFS, "send"}

	if sendCfg.ResumeToken != "" {
		// Variant 3: Resume token send - only -PVenv flags are applicable
		if sendCfg.Progress {
			sendPart = append(sendPart, "-P")
		}
		if sendCfg.Verbose {
			sendPart = append(sendPart, "-v")
		}
		if sendCfg.EmbedData {
			sendPart = append(sendPart, "-e")
		}
		if sendCfg.DryRun {
			sendPart = append(sendPart, "-n")
		}

		// Process title for monitoring
		sendPart = append(sendPart, "-V")

		// Add resume token
		sendPart = append(sendPart, "-t", sendCfg.ResumeToken)

	} else {
		// Variants 1 & 2: Regular send commands

		// Common flags for all variants
		if sendCfg.Progress {
			sendPart = append(sendPart, "-P") // Parsable verbose info
		}
		if sendCfg.Verbose {
			sendPart = append(sendPart, "-v")
		}
		if sendCfg.DryRun {
			sendPart = append(sendPart, "-n")
		}

		// Process title for monitoring
		sendPart = append(sendPart, "-V")

		// Additional flags for non-resume sends
		if sendCfg.Compressed {
			sendPart = append(sendPart, "-c")
		}
		if sendCfg.EmbedData {
			sendPart = append(sendPart, "-e")
		}
		if sendCfg.Properties {
			sendPart = append(sendPart, "-p")
		}
		if sendCfg.Raw {
			sendPart = append(sendPart, "-w")
		}

		// Flags specific to variant 1 (with snapshots)
		if sendCfg.LargeBlocks {
			sendPart = append(sendPart, "-L")
		}
		if sendCfg.Holds {
			sendPart = append(sendPart, "-h")
		}
		if sendCfg.BackupStream {
			sendPart = append(sendPart, "-b")
		}

		// Replication flag and options
		if sendCfg.Replicate {
			sendPart = append(sendPart, "-R")
			if sendCfg.SkipMissing {
				sendPart = append(sendPart, "-s") // Only valid with -R
			}
		}

		// Incremental options (mutually exclusive)
		if sendCfg.FromSnapshot != "" {
			if sendCfg.Intermediary {
				sendPart = append(sendPart, "-I", sendCfg.FromSnapshot)
			} else {
				sendPart = append(sendPart, "-i", sendCfg.FromSnapshot)
			}
		}

		// Add target snapshot/filesystem/volume
		sendPart = append(sendPart, sendCfg.Snapshot)
	}

	// Build receive command
	recvPart := []string{command.BinZFS, "receive"}
	if recvCfg.Force {
		recvPart = append(recvPart, "-F")
	}
	if recvCfg.Unmounted {
		recvPart = append(recvPart, "-u")
	}
	if recvCfg.Resumable {
		recvPart = append(recvPart, "-s") // Essential for pause/resume functionality
	}
	if recvCfg.UseParent {
		recvPart = append(recvPart, "-d")
	}
	if recvCfg.DryRun {
		recvPart = append(recvPart, "-n")
	}
	if recvCfg.Verbose {
		recvPart = append(recvPart, "-v")
	}

	// Add properties
	if recvCfg.Origin != "" {
		recvPart = append(recvPart, "-o", fmt.Sprintf("origin=%s", recvCfg.Origin))
	}
	for k, v := range recvCfg.Properties {
		recvPart = append(recvPart, "-o", fmt.Sprintf("%s=%s", k, v))
	}
	for _, prop := range recvCfg.ExcludeProps {
		recvPart = append(recvPart, "-x", prop)
	}

	recvPart = append(recvPart, recvCfg.Target)

	// Sanitize command parts
	sendPart = sanitizeCommandArgs(sendPart)
	recvPart = sanitizeCommandArgs(recvPart)

	// Build full command
	var cmdStr string
	if recvCfg.RemoteConfig.Host != "" {
		sshPart, err := buildSSHCommand(recvCfg.RemoteConfig)
		if err != nil {
			return nil, err
		}
		cmdStr = fmt.Sprintf("sudo %s | %s sudo %s",
			shellquote.Join(sendPart...),
			shellquote.Join(sshPart...),
			shellquote.Join(recvPart...))
	} else {
		cmdStr = fmt.Sprintf("sudo %s | sudo %s",
			shellquote.Join(sendPart...),
			shellquote.Join(recvPart...))
	}

	return exec.Command("bash", "-c", cmdStr), nil
}

// PauseTransfer pauses a running transfer gracefully without fetching resume token
func (tm *TransferManager) PauseTransfer(transferID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	info, exists := tm.activeTransfers[transferID]
	if !exists {
		return errors.New(errors.TransferNotFound, "Transfer not found")
	}

	if info.Status != TransferStatusRunning {
		return errors.New(errors.TransferInvalidState, "Transfer is not running")
	}

	if !info.Config.ReceiveConfig.Resumable {
		return errors.New(
			errors.TransferInvalidState,
			"Transfer was not started with resumable option (-s)",
		)
	}

	// Terminate the process gracefully
	if info.PID > 0 {
		if err := syscall.Kill(info.PID, syscall.SIGTERM); err != nil {
			return errors.Wrap(err, errors.TransferPauseFailed)
		}

		// Wait a bit for graceful shutdown
		time.Sleep(2 * time.Second)

		// Force kill if still running
		if tm.isProcessRunning(info.PID) {
			syscall.Kill(info.PID, syscall.SIGKILL)
		}
	}

	// Don't fetch resume token here - do it during resume to handle network failures robustly
	tm.updateTransferStatusUnlocked(info, TransferStatusPaused, "")
	tm.logger.Info("Transfer paused", "id", transferID)
	return nil
}

// ResumeTransfer resumes a paused transfer by fetching resume token at resume time
func (tm *TransferManager) ResumeTransfer(ctx context.Context, transferID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	info, exists := tm.activeTransfers[transferID]
	if !exists {
		return errors.New(errors.TransferNotFound, "Transfer not found")
	}

	if info.Status != TransferStatusPaused {
		return errors.New(errors.TransferInvalidState, "Transfer is not paused")
	}

	// Fetch resume token NOW (when network connectivity is more likely to be restored)
	token, err := tm.getReceiveResumeTokenWithRetry(
		info.Config.ReceiveConfig.Target,
		info.Config.ReceiveConfig.RemoteConfig,
	)
	if err != nil {
		tm.updateTransferStatusUnlocked(info, TransferStatusFailed,
			fmt.Sprintf("Failed to get resume token for resuming transfer: %v", err))
		return errors.Wrap(err, errors.TransferResumeFailed)
	}

	if token == "" {
		tm.updateTransferStatusUnlocked(info, TransferStatusFailed,
			"No resume token available - partial receive may have been cleaned up")
		return errors.New(
			errors.TransferInvalidState,
			"No resume token available for resuming transfer",
		)
	}

	// Update send config to use the fetched resume token
	info.Config.SendConfig.ResumeToken = token

	tm.updateTransferStatusUnlocked(info, TransferStatusRunning, "")

	// Start transfer in background
	go tm.executeTransfer(ctx, info)

	tm.logger.Info(
		"Transfer resumed",
		"id",
		transferID,
		"resume_token",
		token[:min(32, len(token))]+"...",
	)
	return nil
}

// StopTransfer stops a running or paused transfer
func (tm *TransferManager) StopTransfer(transferID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	info, exists := tm.activeTransfers[transferID]
	if !exists {
		return errors.New(errors.TransferNotFound, "Transfer not found")
	}

	if info.Status == TransferStatusCompleted || info.Status == TransferStatusFailed ||
		info.Status == TransferStatusCancelled {
		return errors.New(errors.TransferInvalidState, "Transfer is already finished")
	}

	// Terminate the process
	if info.PID > 0 {
		if err := syscall.Kill(info.PID, syscall.SIGTERM); err != nil {
			// If SIGTERM fails, try SIGKILL
			syscall.Kill(info.PID, syscall.SIGKILL)
		}
	}

	// If transfer was paused, we need to abort the partial receive
	if info.Status == TransferStatusPaused {
		if err := tm.abortPartialReceive(info.Config.ReceiveConfig.Target, info.Config.ReceiveConfig.RemoteConfig); err != nil {
			tm.logger.Warn("Failed to abort partial receive", "error", err)
		}
	}

	tm.updateTransferStatusUnlocked(info, TransferStatusCancelled, "Transfer stopped by user")
	tm.logger.Info("Transfer stopped", "id", transferID)
	return nil
}

// DeleteTransfer removes a transfer and its associated files
func (tm *TransferManager) DeleteTransfer(transferID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	info, exists := tm.activeTransfers[transferID]
	if !exists {
		return errors.New(errors.TransferNotFound, "Transfer not found")
	}

	// Can only delete finished transfers
	if info.Status == TransferStatusRunning || info.Status == TransferStatusPaused {
		return errors.New(errors.TransferInvalidState, "Cannot delete active transfer")
	}

	// Remove files
	files := []string{info.LogFile, info.PIDFile, info.ConfigFile, info.ProgressFile}
	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			tm.logger.Warn("Failed to remove transfer file", "file", file, "error", err)
		}
	}

	// Remove from active transfers
	delete(tm.activeTransfers, transferID)

	tm.logger.Info("Transfer deleted", "id", transferID)
	return nil
}

// GetTransfer returns information about a specific transfer
func (tm *TransferManager) GetTransfer(transferID string) (*TransferInfo, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	info, exists := tm.activeTransfers[transferID]
	if !exists {
		return nil, errors.New(errors.TransferNotFound, "Transfer not found")
	}

	// Return a copy
	infoCopy := *info
	return &infoCopy, nil
}

// ListTransfers returns a list of all transfers
func (tm *TransferManager) ListTransfers() []*TransferInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	transfers := make([]*TransferInfo, 0, len(tm.activeTransfers))
	for _, info := range tm.activeTransfers {
		infoCopy := *info
		transfers = append(transfers, &infoCopy)
	}

	return transfers
}

// Helper methods

// updateTransferStatusLocked updates transfer status when caller doesn't hold lock
func (tm *TransferManager) updateTransferStatusLocked(
	info *TransferInfo,
	status TransferStatus,
	errorMsg string,
) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.updateTransferStatusUnlocked(info, status, errorMsg)
}

// updateTransferStatusUnlocked updates transfer status when caller already holds lock
func (tm *TransferManager) updateTransferStatusUnlocked(
	info *TransferInfo,
	status TransferStatus,
	errorMsg string,
) {
	info.Status = status
	info.ErrorMessage = errorMsg
	if status == TransferStatusCompleted || status == TransferStatusFailed ||
		status == TransferStatusCancelled {
		completedTime := time.Now()
		info.CompletedAt = &completedTime
	}

	// Save updated config
	tm.saveTransferConfig(info)
}

// getReceiveResumeTokenWithRetry gets the resume token with retry logic for network resilience
func (tm *TransferManager) getReceiveResumeTokenWithRetry(
	target string,
	remoteConfig RemoteConfig,
) (string, error) {
	const maxRetries = 3
	const retryDelay = 5 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		token, err := tm.getReceiveResumeToken(target, remoteConfig)
		if err == nil {
			return token, nil
		}

		lastErr = err
		tm.logger.Warn("Failed to get resume token", "attempt", attempt, "error", err)

		if attempt < maxRetries {
			time.Sleep(retryDelay)
		}
	}

	return "", fmt.Errorf("failed to get resume token after %d attempts: %w", maxRetries, lastErr)
}

// getReceiveResumeToken gets the resume token from the receiving dataset
func (tm *TransferManager) getReceiveResumeToken(
	target string,
	remoteConfig RemoteConfig,
) (string, error) {
	var cmd *exec.Cmd

	if remoteConfig.Host != "" {
		// Remote dataset
		sshPart, err := buildSSHCommand(remoteConfig)
		if err != nil {
			return "", err
		}

		cmdStr := fmt.Sprintf("%s sudo zfs get -H -o value receive_resume_token %s",
			shellquote.Join(sshPart...), shellquote.Join(target))
		cmd = exec.Command("bash", "-c", cmdStr)
	} else {
		// Local dataset
		cmd = exec.Command("sudo", "zfs", "get", "-H", "-o", "value", "receive_resume_token", target)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	token := strings.TrimSpace(string(output))
	if token == "-" {
		return "", errors.New(errors.ZFSDatasetNoReceiveToken, "No resume token available")
	}

	return token, nil
}

// abortPartialReceive aborts a partial receive operation
func (tm *TransferManager) abortPartialReceive(target string, remoteConfig RemoteConfig) error {
	var cmd *exec.Cmd

	if remoteConfig.Host != "" {
		// Remote dataset
		sshPart, err := buildSSHCommand(remoteConfig)
		if err != nil {
			return err
		}

		cmdStr := fmt.Sprintf("%s sudo zfs receive -A %s",
			shellquote.Join(sshPart...), shellquote.Join(target))
		cmd = exec.Command("bash", "-c", cmdStr)
	} else {
		// Local dataset
		cmd = exec.Command("sudo", "zfs", "receive", "-A", target)
	}

	return cmd.Run()
}

// monitorTransferProgress monitors and updates transfer progress
func (tm *TransferManager) monitorTransferProgress(info *TransferInfo, logFile *os.File) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if info.Status != TransferStatusRunning {
				return
			}

			// TODO: Parse verbose output for actual progress data
			// This requires parsing the verbose output format from ZFS send
			tm.updateProgressFromLog(info, logFile)

			// Save progress to file
			tm.saveProgress(info)

		case <-time.After(1 * time.Minute):
			// Check if process is still running
			if info.PID > 0 {
				if !tm.isProcessRunning(info.PID) {
					tm.updateTransferStatusLocked(
						info,
						TransferStatusFailed,
						"Process unexpectedly terminated",
					)
					return
				}
			}
		}
	}
}

// updateProgressFromLog parses log file for progress information
func (tm *TransferManager) updateProgressFromLog(info *TransferInfo, logFile *os.File) {
	// TODO: Implement actual progress parsing from verbose output
	// The -P flag provides parsable progress information that can be parsed
	// For now, just update timestamp and elapsed time
	_ = logFile
	info.Progress.LastUpdate = time.Now()
	if info.StartedAt != nil {
		info.Progress.ElapsedTime = int64(time.Since(*info.StartedAt).Seconds())
	}
}

func (tm *TransferManager) saveTransferConfig(info *TransferInfo) error {
	data, err := yaml.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(info.ConfigFile, data, 0644)
}

func (tm *TransferManager) savePID(info *TransferInfo) error {
	return os.WriteFile(info.PIDFile, []byte(fmt.Sprintf("%d", info.PID)), 0644)
}

func (tm *TransferManager) saveProgress(info *TransferInfo) error {
	data, err := json.Marshal(info.Progress)
	if err != nil {
		return err
	}
	return os.WriteFile(info.ProgressFile, data, 0644)
}

func (tm *TransferManager) isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func (tm *TransferManager) loadExistingTransfers() error {
	files, err := filepath.Glob(filepath.Join(tm.transfersDir, "*.yaml"))
	if err != nil {
		return err
	}

	for _, file := range files {
		var info TransferInfo
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		if err := yaml.Unmarshal(data, &info); err != nil {
			continue
		}

		// Check if process is still running
		if info.PID > 0 && tm.isProcessRunning(info.PID) {
			info.Status = TransferStatusRunning
		} else if info.Status == TransferStatusRunning || info.Status == TransferStatusPaused {
			info.Status = TransferStatusFailed
			info.ErrorMessage = "Process terminated unexpectedly"
		}

		tm.activeTransfers[info.ID] = &info
	}

	return nil
}

func (tm *TransferManager) handleTransferCompletion(info *TransferInfo) {
	// Clean up PID file
	if info.PIDFile != "" {
		os.Remove(info.PIDFile)
	}

	tm.logger.Info("Transfer completed", "id", info.ID, "status", info.Status)
}
