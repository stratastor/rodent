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

// TransferAction represents the intended action for the transfer
type TransferAction string

const (
	TransferActionNone   TransferAction = ""
	TransferActionPause  TransferAction = "pause"
	TransferActionStop   TransferAction = "stop"
	TransferActionResume TransferAction = "resume"
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
	// Internal state for action flow tracking
	pendingAction TransferAction `json:"-"                       yaml:"-"`
}

// TransferProgress tracks the progress of a transfer operation
type TransferProgress struct {
	BytesTransferred int64     `json:"bytes_transferred"       yaml:"bytes_transferred"`
	TotalBytes       int64     `json:"total_bytes,omitempty"   yaml:"total_bytes,omitempty"`
	TransferRate     int64     `json:"transfer_rate"           yaml:"transfer_rate"`
	ElapsedTime      int64     `json:"elapsed_time"            yaml:"elapsed_time"`
	LastUpdate       time.Time `json:"last_update"             yaml:"last_update"`
	EstimatedETA     int64     `json:"estimated_eta,omitempty" yaml:"estimated_eta,omitempty"`
	Phase            string    `json:"phase,omitempty"         yaml:"phase,omitempty"`
	PhaseDescription string    `json:"phase_description,omitempty" yaml:"phase_description,omitempty"`
}

// TransferType represents different types of transfer queries
type TransferType string

const (
	TransferTypeAll       TransferType = "all"
	TransferTypeActive    TransferType = "active"
	TransferTypeCompleted TransferType = "completed"
	TransferTypeFailed    TransferType = "failed"
)


// TransferManager manages enterprise-grade ZFS transfer operations
type TransferManager struct {
	mu              sync.RWMutex
	activeTransfers map[string]*TransferInfo
	transfersDir    string
	logger          logger.Logger
}

// NewTransferManager creates a new transfer manager instance
func NewTransferManager(logCfg logger.Config) (*TransferManager, error) {
	l, err := logger.NewTag(logCfg, "zfs-transfer-manager")
	if err != nil {
		return nil, errors.Wrap(err, errors.RodentMisc)
	}

	tm := &TransferManager{
		activeTransfers: make(map[string]*TransferInfo),
		transfersDir:    config.GetTransfersDir(),
		logger:          l,
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

	// Pre-transfer validation: Check for initial snapshot requirement
	sendCfg := info.Config.SendConfig
	recvCfg := info.Config.ReceiveConfig
	
	if sendCfg.FromSnapshot != "" && sendCfg.ResumeToken == "" {
		tm.logger.Info("Validating incremental transfer requirements", "id", info.ID, "from_snapshot", sendCfg.FromSnapshot)
		
		exists, err := tm.snapshotExistsOnTarget(sendCfg.FromSnapshot, recvCfg)
		if err != nil {
			tm.logger.Warn("Could not verify initial snapshot on target, proceeding anyway", "id", info.ID, "error", err)
		} else if !exists {
			tm.logger.Info("Initial snapshot missing on target, performing automatic initial send", "id", info.ID, "snapshot", sendCfg.FromSnapshot)
			
			// Update progress to show initial send phase
			info.Progress.Phase = "initial_send"
			info.Progress.PhaseDescription = fmt.Sprintf("Sending initial snapshot: %s", sendCfg.FromSnapshot)
			info.Progress.LastUpdate = time.Now()
			tm.saveProgress(info)
			
			if err := tm.performInitialSend(ctx, info, sendCfg.FromSnapshot); err != nil {
				tm.updateTransferStatusLocked(
					info,
					TransferStatusFailed,
					fmt.Sprintf("Failed to send initial snapshot: %v", err),
				)
				return
			}
			
			// Update progress to show incremental phase
			info.Progress.Phase = "incremental_send"
			info.Progress.PhaseDescription = fmt.Sprintf("Sending incremental changes from %s to %s", sendCfg.FromSnapshot, sendCfg.Snapshot)
			info.Progress.LastUpdate = time.Now()
			tm.saveProgress(info)
			
			tm.logger.Info("Initial snapshot sent successfully, proceeding with incremental transfer", "id", info.ID)
		} else {
			tm.logger.Debug("Initial snapshot exists on target, proceeding with incremental transfer", "id", info.ID)
			
			// Update progress to show incremental phase
			info.Progress.Phase = "incremental_send"
			info.Progress.PhaseDescription = fmt.Sprintf("Sending incremental changes from %s to %s", sendCfg.FromSnapshot, sendCfg.Snapshot)
			info.Progress.LastUpdate = time.Now()
			tm.saveProgress(info)
		}
	} else {
		// Not an incremental transfer - set phase for full send
		info.Progress.Phase = "full_send"
		if sendCfg.ResumeToken != "" {
			info.Progress.PhaseDescription = "Resuming transfer from saved state"
		} else {
			info.Progress.PhaseDescription = fmt.Sprintf("Sending full snapshot: %s", sendCfg.Snapshot)
		}
		info.Progress.LastUpdate = time.Now()
		tm.saveProgress(info)
	}

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

	// Set up process group for proper signal handling
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
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

	// Save PID (this is the process group leader)
	info.PID = cmd.Process.Pid
	if err := tm.savePID(info); err != nil {
		tm.logger.Warn("Failed to save PID", "error", err)
	}
	tm.logger.Debug("Transfer PID saved", "id", info.ID, "pid", info.PID)

	// Monitor progress in background
	go tm.monitorTransferProgress(info, logFile)

	// Setup signal handling for verbose output and system interrupts
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1)

	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGUSR1:
				// Request verbose output (SIGUSR1 works on both Linux and macOS)
				if cmd.Process != nil {
					cmd.Process.Signal(sig)
				}
			case syscall.SIGTERM, syscall.SIGINT:
				// This is a system interrupt to the Go program (like Ctrl+C)
				tm.logger.Info(
					"Transfer interrupted by system signal",
					"id",
					info.ID,
					"signal",
					sig,
				)
				tm.StopTransfer(info.ID)
				return
			}
		}
	}()

	// Wait for command completion
	err = cmd.Wait()

	// Check final status to decide what to do
	tm.mu.Lock()
	finalStatus := info.Status
	finalAction := info.pendingAction
	tm.mu.Unlock()

	tm.logger.Debug("Transfer command completed",
		"id", info.ID, "cmd_error", err, "final_status", finalStatus, "pending_action", finalAction)

	// Only update status if transfer hasn't been explicitly paused, stopped, or cancelled
	switch finalStatus {
	case TransferStatusPaused, TransferStatusCancelled:
		// These statuses were set by explicit actions, don't overwrite
		tm.logger.Debug(
			"Transfer status already set by action",
			"id",
			info.ID,
			"status",
			finalStatus,
		)
		return
	case TransferStatusCompleted, TransferStatusFailed:
		// These might be stale, check if we should update
		if finalAction != TransferActionNone {
			tm.logger.Debug("Transfer has pending action, not updating status",
				"id", info.ID, "action", finalAction)
			return
		}
	}

	// Update status based on command completion result
	if err != nil {
		if ctx.Err() != nil {
			tm.updateTransferStatusLocked(info, TransferStatusCancelled, "Transfer cancelled")
			tm.logger.Info("Status Update: Transfer cancelled", "id", info.ID)
		} else {
			tm.updateTransferStatusLocked(info, TransferStatusFailed, fmt.Sprintf("Transfer failed: %v", err))
			tm.logger.Error("Status Update: Transfer failed", "id", info.ID, "error", err)
		}
	} else {
		tm.updateTransferStatusLocked(info, TransferStatusCompleted, "")
		tm.logger.Info("Status Update: Transfer completed", "id", info.ID)
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
		if sendCfg.Parsable {
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
		if sendCfg.Parsable {
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
	recvPart := []string{"zfs", "receive"}
	if recvCfg.Force {
		if sendCfg.ResumeToken == "" {
			recvPart = append(recvPart, "-F")
		}
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
	tm.logger.Debug("Built transfer command", "command", cmdStr)
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

	// Set pending action to pause to prevent executeTransfer from updating status
	info.pendingAction = TransferActionPause

	tm.logger.Debug("Process group before pause", "id", info.ID, "pgid", info.PID)
	// Terminate the entire process group gracefully
	if info.PID > 0 {
		// Send SIGTERM to the entire process group (negative PID)
		if err := syscall.Kill(-info.PID, syscall.SIGTERM); err != nil {
			tm.logger.Warn(
				"Failed to terminate transfer process group gracefully",
				"id",
				info.ID,
				"error",
				err,
			)
			// Try force kill on process group
			if err := syscall.Kill(-info.PID, syscall.SIGKILL); err != nil {
				tm.logger.Error(
					"Failed to force kill transfer process group",
					"id",
					info.ID,
					"error",
					err,
				)
				info.pendingAction = TransferActionNone // Reset on error
				return errors.Wrap(err, errors.TransferPauseFailed)
			}
		}

		// Update status to paused but keep pending action until executeTransfer completes
		tm.updateTransferStatusUnlocked(info, TransferStatusPaused, "")
		// sleep for a couple of seconds to allow graceful termination
		tm.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		tm.mu.Lock()

		// Force kill process group if still running
		if tm.isProcessRunning(info.PID) {
			tm.logger.Warn(
				"Transfer process group did not terminate gracefully, forcing kill",
				"id",
				info.ID,
			)
			if err := syscall.Kill(-info.PID, syscall.SIGKILL); err != nil {
				tm.logger.Error(
					"Failed to force kill transfer process group",
					"id",
					info.ID,
					"error",
					err,
				)
				info.pendingAction = TransferActionNone // Reset on error
				return errors.Wrap(err, errors.TransferPauseFailed)
			}
		}
	}

	tm.logger.Info("Transfer paused", "id", transferID)
	// Note: pendingAction is intentionally left as TransferActionPause until executeTransfer() sees it
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

	// Clear any pending action and update status to running
	info.pendingAction = TransferActionNone
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

	// Set pending action to stop to prevent executeTransfer from updating status
	info.pendingAction = TransferActionStop

	// Terminate the entire process group
	if info.PID > 0 {
		// Send SIGTERM to the entire process group (negative PID)
		if err := syscall.Kill(-info.PID, syscall.SIGTERM); err != nil {
			tm.logger.Warn(
				"Failed to terminate transfer process group gracefully",
				"id",
				info.ID,
				"error",
				err,
			)
			// Try force kill on process group
			if err := syscall.Kill(-info.PID, syscall.SIGKILL); err != nil {
				tm.logger.Error(
					"Failed to force kill transfer process group",
					"id",
					info.ID,
					"error",
					err,
				)
				info.pendingAction = TransferActionNone // Reset on error
				return errors.Wrap(err, errors.TransferStopFailed)
			}
		}
	}

	// If transfer was paused, we need to abort the partial receive
	if info.Status == TransferStatusPaused {
		if err := tm.abortPartialReceive(info.Config.ReceiveConfig.Target, info.Config.ReceiveConfig.RemoteConfig); err != nil {
			tm.logger.Warn("Failed to abort partial receive", "error", err)
		}
	}

	// Update status to cancelled but keep pending action until executeTransfer completes
	tm.updateTransferStatusUnlocked(info, TransferStatusCancelled, "Transfer stopped by user")
	// Note: pendingAction is intentionally left as TransferActionStop until executeTransfer() sees it

	tm.logger.Info("Transfer stopped", "id", transferID)
	return nil
}

// DeleteTransfer removes a transfer and its associated files (active or historical)
func (tm *TransferManager) DeleteTransfer(transferID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if it's an active transfer (running/paused only)
	if info, exists := tm.activeTransfers[transferID]; exists {
		// Can only delete finished transfers
		if info.Status == TransferStatusRunning || info.Status == TransferStatusPaused {
			return errors.New(errors.TransferInvalidState, "Cannot delete active transfer")
		}

		// This shouldn't happen with the new logic, but handle gracefully
		// Remove files
		files := []string{info.LogFile, info.PIDFile, info.ConfigFile, info.ProgressFile}
		for _, file := range files {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				tm.logger.Warn("Failed to remove transfer file", "file", file, "error", err)
			}
		}

		// Remove from active transfers
		delete(tm.activeTransfers, transferID)
	} else {
		// Check if it's a historical transfer (completed/failed/cancelled transfers)
		configFile := filepath.Join(tm.transfersDir, fmt.Sprintf("%s.yaml", transferID))
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			return errors.New(errors.TransferNotFound, "Transfer not found")
		}

		// Remove historical transfer files
		files := []string{
			filepath.Join(tm.transfersDir, fmt.Sprintf("%s.yaml", transferID)),
			filepath.Join(tm.transfersDir, fmt.Sprintf("%s.log", transferID)),
			filepath.Join(tm.transfersDir, fmt.Sprintf("%s.pid", transferID)),
			filepath.Join(tm.transfersDir, fmt.Sprintf("%s.progress", transferID)),
		}
		for _, file := range files {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				tm.logger.Warn("Failed to remove transfer file", "file", file, "error", err)
			}
		}
	}

	tm.logger.Info("Transfer deleted", "id", transferID)
	return nil
}

// GetTransfer returns information about a specific transfer (active or historical)
func (tm *TransferManager) GetTransfer(transferID string) (*TransferInfo, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// First check active transfers
	if info, exists := tm.activeTransfers[transferID]; exists {
		infoCopy := *info
		return &infoCopy, nil
	}

	// Check historical transfers
	configFile := filepath.Join(tm.transfersDir, fmt.Sprintf("%s.yaml", transferID))
	if transfer := tm.loadTransferFromFile(configFile); transfer != nil {
		return transfer, nil
	}

	return nil, errors.New(errors.TransferNotFound, "Transfer not found")
}

// ListTransfers returns a list of all transfers
func (tm *TransferManager) ListTransfers() []*TransferInfo {
	return tm.ListTransfersByType(TransferTypeActive)
}

// ListTransfersByType returns transfers filtered by type
func (tm *TransferManager) ListTransfersByType(transferType TransferType) []*TransferInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	switch transferType {
	case TransferTypeActive:
		return tm.getActiveTransfers()
	case TransferTypeCompleted:
		return tm.getHistoricalTransfersByStatus(TransferStatusCompleted)
	case TransferTypeFailed:
		return tm.getHistoricalTransfersByStatus(TransferStatusFailed)
	case TransferTypeAll:
		active := tm.getActiveTransfers()
		historical := tm.getAllHistoricalTransfers()
		return append(active, historical...)
	default:
		return tm.getActiveTransfers()
	}
}

// getActiveTransfers returns currently active transfers
func (tm *TransferManager) getActiveTransfers() []*TransferInfo {
	transfers := make([]*TransferInfo, 0, len(tm.activeTransfers))
	for _, info := range tm.activeTransfers {
		infoCopy := *info
		transfers = append(transfers, &infoCopy)
	}
	return transfers
}

// getHistoricalTransfersByStatus loads transfers from disk by status
func (tm *TransferManager) getHistoricalTransfersByStatus(status TransferStatus) []*TransferInfo {
	allHistorical := tm.getAllHistoricalTransfers()
	filtered := make([]*TransferInfo, 0)
	
	for _, transfer := range allHistorical {
		if transfer.Status == status {
			filtered = append(filtered, transfer)
		}
	}
	
	return filtered
}

// getAllHistoricalTransfers loads all transfers from disk
func (tm *TransferManager) getAllHistoricalTransfers() []*TransferInfo {
	files, err := filepath.Glob(filepath.Join(tm.transfersDir, "*.yaml"))
	if err != nil {
		tm.logger.Warn("Failed to list transfer files", "error", err)
		return []*TransferInfo{}
	}

	transfers := make([]*TransferInfo, 0)
	for _, file := range files {
		if transfer := tm.loadTransferFromFile(file); transfer != nil {
			// Skip if it's already in active transfers
			if _, exists := tm.activeTransfers[transfer.ID]; !exists {
				transfers = append(transfers, transfer)
			}
		}
	}

	return transfers
}

// loadTransferFromFile loads a transfer from a YAML file
func (tm *TransferManager) loadTransferFromFile(filePath string) *TransferInfo {
	data, err := os.ReadFile(filePath)
	if err != nil {
		tm.logger.Debug("Failed to read transfer file", "file", filePath, "error", err)
		return nil
	}

	var transfer TransferInfo
	if err := yaml.Unmarshal(data, &transfer); err != nil {
		tm.logger.Debug("Failed to unmarshal transfer file", "file", filePath, "error", err)
		return nil
	}

	return &transfer
}

// getDefaultLogConfig returns default log configuration values
func getDefaultLogConfig() TransferLogConfig {
	return TransferLogConfig{
		MaxSizeBytes:     10 * 1024, // 10KB default
		TruncateOnFinish: true,
		RetainOnFailure:  true,
		HeaderLines:      20,
		FooterLines:      20,
	}
}

// getEffectiveLogConfig returns the effective log config for a transfer
func (tm *TransferManager) getEffectiveLogConfig(info *TransferInfo) TransferLogConfig {
	if info.Config.LogConfig != nil {
		// Use transfer-specific config with defaults for zero values
		config := *info.Config.LogConfig
		defaults := getDefaultLogConfig()
		
		if config.MaxSizeBytes == 0 {
			config.MaxSizeBytes = defaults.MaxSizeBytes
		}
		if config.HeaderLines == 0 {
			config.HeaderLines = defaults.HeaderLines
		}
		if config.FooterLines == 0 {
			config.FooterLines = defaults.FooterLines
		}
		
		return config
	}
	return getDefaultLogConfig()
}

// Log Management Methods

// GetTransferLog returns the full log content for a transfer
func (tm *TransferManager) GetTransferLog(transferID string) (string, error) {
	logFile := filepath.Join(tm.transfersDir, fmt.Sprintf("%s.log", transferID))
	
	// Check if file exists and get size
	fileInfo, err := os.Stat(logFile)
	if os.IsNotExist(err) {
		return "", errors.New(errors.TransferNotFound, "Transfer log not found")
	}
	if err != nil {
		return "", errors.Wrap(err, errors.RodentMisc)
	}
	
	// Cap at 90MB to prevent memory issues
	const maxLogSize = 90 * 1024 * 1024 // 90MB
	if fileInfo.Size() > maxLogSize {
		return "", errors.New(errors.RodentMisc, fmt.Sprintf("Log file too large (%d bytes). Maximum allowed: %d bytes. Use GetTransferLogGist() for large files.", fileInfo.Size(), maxLogSize))
	}
	
	content, err := os.ReadFile(logFile)
	if err != nil {
		return "", errors.Wrap(err, errors.RodentMisc)
	}
	
	return string(content), nil
}

// GetTransferLogGist returns a truncated version of the log (header + footer) using efficient utilities
func (tm *TransferManager) GetTransferLogGist(transferID string) (string, error) {
	logFile := filepath.Join(tm.transfersDir, fmt.Sprintf("%s.log", transferID))
	
	// Check if file exists and get size
	stat, err := os.Stat(logFile)
	if os.IsNotExist(err) {
		return "", errors.New(errors.TransferNotFound, "Transfer log not found")
	}
	if err != nil {
		return "", errors.Wrap(err, errors.RodentMisc)
	}
	
	// Get transfer info to use its log configuration
	transfer, err := tm.GetTransfer(transferID)
	if err != nil {
		// If we can't get transfer info, use default config
		return tm.truncateLogContentEfficient(logFile, stat.Size(), getDefaultLogConfig())
	}
	
	logConfig := tm.getEffectiveLogConfig(transfer)
	return tm.truncateLogContentEfficient(logFile, stat.Size(), logConfig)
}

// truncateLogContentEfficient uses file size and system utilities for memory-efficient log truncation
func (tm *TransferManager) truncateLogContentEfficient(logFile string, fileSize int64, logConfig TransferLogConfig) (string, error) {
	const sizeLimitBytes = 100 * 1024 // 100KB
	
	// If file is small enough, return full content
	if fileSize <= sizeLimitBytes {
		content, err := os.ReadFile(logFile)
		if err != nil {
			return "", fmt.Errorf("failed to read small log file: %w", err)
		}
		return string(content), nil
	}
	
	// For large files, use head and tail
	headerLines := logConfig.HeaderLines
	footerLines := logConfig.FooterLines
	
	// Get header lines using head
	headCmd := exec.Command("head", "-n", fmt.Sprintf("%d", headerLines), logFile)
	headerOutput, err := headCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get header lines: %w", err)
	}
	
	// Get footer lines using tail
	tailCmd := exec.Command("tail", "-n", fmt.Sprintf("%d", footerLines), logFile)
	footerOutput, err := tailCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get footer lines: %w", err)
	}
	
	// Combine with separator indicating truncation
	result := string(headerOutput) + "\n" +
		fmt.Sprintf("... [File truncated - original size: %d bytes] ...\n\n", fileSize) +
		string(footerOutput)
	
	return result, nil
}

// processLogOnCompletion handles log processing when a transfer completes
func (tm *TransferManager) processLogOnCompletion(info *TransferInfo) {
	logConfig := tm.getEffectiveLogConfig(info)
	
	if !logConfig.TruncateOnFinish {
		return
	}
	
	// Don't truncate failed transfers if configured to retain them
	if info.Status == TransferStatusFailed && logConfig.RetainOnFailure {
		tm.logger.Debug("Retaining full log for failed transfer", "id", info.ID)
		return
	}
	
	// Check log file size efficiently using stat
	logStat, err := os.Stat(info.LogFile)
	if err != nil {
		tm.logger.Warn("Failed to stat log file", "id", info.ID, "error", err)
		return
	}
	
	if logStat.Size() <= logConfig.MaxSizeBytes {
		tm.logger.Debug("Log file under size limit, not truncating", 
			"id", info.ID, "size", logStat.Size())
		return
	}
	
	// Get truncated content efficiently
	truncatedContent, err := tm.truncateLogContentEfficient(info.LogFile, logStat.Size(), logConfig)
	if err != nil {
		tm.logger.Warn("Failed to truncate log efficiently", "id", info.ID, "error", err)
		return
	}
	
	// Write truncated content directly
	err = os.WriteFile(info.LogFile, []byte(truncatedContent), 0644)
	if err != nil {
		tm.logger.Warn("Failed to write truncated log", "id", info.ID, "error", err)
		return
	}
	
	tm.logger.Info("Log truncated for completed transfer", 
		"id", info.ID, 
		"original_size", logStat.Size(), 
		"new_size", len(truncatedContent))
}

// Helper methods

// snapshotExistsOnTarget checks if a snapshot exists on the target filesystem
func (tm *TransferManager) snapshotExistsOnTarget(snapshot string, recvCfg ReceiveConfig) (bool, error) {
	// Extract dataset name from snapshot (format: dataset@snapshot)
	parts := strings.Split(snapshot, "@")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid snapshot format: %s", snapshot)
	}
	
	// Build target snapshot name using receive target
	targetSnapshot := fmt.Sprintf("%s@%s", recvCfg.Target, parts[1])
	
	var cmd *exec.Cmd
	
	if recvCfg.RemoteConfig.Host != "" {
		// Remote target - use SSH
		sshPart, err := buildSSHCommand(recvCfg.RemoteConfig)
		if err != nil {
			return false, fmt.Errorf("failed to build SSH command: %w", err)
		}
		
		cmdStr := fmt.Sprintf("%s sudo zfs list -H -t snapshot %s",
			shellquote.Join(sshPart...), shellquote.Join(targetSnapshot))
		tm.logger.Debug("Checking remote snapshot existence", "command", cmdStr)
		cmd = exec.Command("bash", "-c", cmdStr)
	} else {
		// Local target
		tm.logger.Debug("Checking local snapshot existence", "snapshot", targetSnapshot)
		cmd = exec.Command("sudo", "zfs", "list", "-H", "-t", "snapshot", targetSnapshot)
	}
	
	err := cmd.Run()
	if err != nil {
		// Exit code 1 typically means snapshot doesn't exist
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		// Other errors (network, permissions, etc.)
		return false, fmt.Errorf("failed to check snapshot existence: %w", err)
	}
	
	return true, nil
}

// performInitialSend executes a full send of the initial snapshot to the target
func (tm *TransferManager) performInitialSend(_ context.Context, info *TransferInfo, fromSnapshot string) error {
	tm.logger.Info("Performing initial snapshot send", "id", info.ID, "snapshot", fromSnapshot)
	
	// Create a temporary config for the initial send (full send, not incremental)
	initialConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot:    fromSnapshot,
			// Copy relevant flags from original config but remove incremental settings
			Replicate:    info.Config.SendConfig.Replicate,
			SkipMissing:  info.Config.SendConfig.SkipMissing,
			Properties:   info.Config.SendConfig.Properties,
			Raw:          info.Config.SendConfig.Raw,
			LargeBlocks:  info.Config.SendConfig.LargeBlocks,
			EmbedData:    info.Config.SendConfig.EmbedData,
			Holds:        info.Config.SendConfig.Holds,
			BackupStream: info.Config.SendConfig.BackupStream,
			Compressed:   info.Config.SendConfig.Compressed,
			Verbose:      info.Config.SendConfig.Verbose,
			Parsable:     info.Config.SendConfig.Parsable,
			Timeout:      info.Config.SendConfig.Timeout,
			LogLevel:     info.Config.SendConfig.LogLevel,
			// Explicitly clear incremental settings
			FromSnapshot: "",
			Intermediary: false,
			Incremental:  false,
		},
		ReceiveConfig: info.Config.ReceiveConfig, // Use same receive config
	}
	
	// Create temporary transfer info for initial send
	initialInfo := &TransferInfo{
		ID:        info.ID + "-initial",
		Operation: TransferOperationSendReceive,
		Status:    TransferStatusRunning,
		Config:    initialConfig,
		Progress:  TransferProgress{LastUpdate: time.Now()},
		CreatedAt: time.Now(),
		LogFile:   info.LogFile, // Use same log file
	}
	
	// Build and execute initial send command
	cmd, err := tm.buildTransferCommand(initialInfo)
	if err != nil {
		return fmt.Errorf("failed to build initial send command: %w", err)
	}
	
	// Setup output redirection to log file (create if doesn't exist)
	logFile, err := os.OpenFile(info.LogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()
	
	// Log the initial send operation
	fmt.Fprintf(logFile, "\n=== Initial Snapshot Send: %s ===\n", fromSnapshot)
	
	if initialConfig.SendConfig.DryRun {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		cmd.Stderr = logFile // Verbose output goes to log file
	}
	
	// Set up process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	
	tm.logger.Info("Starting initial snapshot send", "id", info.ID, "snapshot", fromSnapshot)
	
	// Execute initial send
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start initial send command: %w", err)
	}
	
	// Wait for completion
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("initial send failed: %w", err)
	}
	
	fmt.Fprint(logFile, "=== Initial Snapshot Send Completed ===\n\n")
	tm.logger.Info("Initial snapshot send completed", "id", info.ID, "snapshot", fromSnapshot)
	
	return nil
}

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
		tm.logger.Debug("Executing remote command to get resume token", "command", cmdStr)
		cmd = exec.Command("bash", "-c", cmdStr)
	} else {
		tm.logger.Debug("Executing local command to get resume token", "target", target)
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
		tm.logger.Debug("Executing remote command to abort partial receive", "command", cmdStr)
		cmd = exec.Command("bash", "-c", cmdStr)
	} else {
		tm.logger.Debug("Executing local command to abort partial receive", "target", target)
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
	pidData := fmt.Appendf(nil, "%d", info.PID)
	return os.WriteFile(info.PIDFile, pidData, 0644)
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
	// Clear any pending action now that executeTransfer has completed
	tm.mu.Lock()
	info.pendingAction = TransferActionNone
	
	// Remove completed/failed transfers from active transfers so they become historical
	if info.Status == TransferStatusCompleted || info.Status == TransferStatusFailed || info.Status == TransferStatusCancelled {
		delete(tm.activeTransfers, info.ID)
	}
	tm.mu.Unlock()

	// Process log truncation if transfer is completed or failed
	if info.Status == TransferStatusCompleted || info.Status == TransferStatusFailed {
		tm.processLogOnCompletion(info)
	}

	// Clean up PID file
	if info.PIDFile != "" {
		os.Remove(info.PIDFile)
	}

	tm.logger.Info("Transfer completed", "id", info.ID, "status", info.Status)
}
