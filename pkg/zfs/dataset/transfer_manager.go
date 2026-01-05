// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2024 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kballard/go-shellquote"
	"gopkg.in/yaml.v3"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/internal/events"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
	eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
)

// TransferStatus represents the current state of a transfer
type TransferStatus string

const (
	TransferStatusStarting  TransferStatus = "starting"
	TransferStatusRunning   TransferStatus = "running"
	TransferStatusPaused    TransferStatus = "paused"
	TransferStatusCompleted TransferStatus = "completed"
	TransferStatusFailed    TransferStatus = "failed"
	TransferStatusCancelled TransferStatus = "cancelled"
	TransferStatusSkipped   TransferStatus = "skipped" // Target already in sync, nothing to transfer
	TransferStatusUnknown   TransferStatus = "unknown"
)

// TransferAction represents the intended action for the transfer
type TransferAction string

const (
	TransferActionNone   TransferAction = ""
	TransferActionPause  TransferAction = "pause"
	TransferActionStop   TransferAction = "stop"
	TransferActionResume TransferAction = "resume"
)

const cooloffPeriod = 3 * time.Minute

// TransferInfo holds comprehensive information about a ZFS transfer
type TransferInfo struct {
	ID           string            `json:"id"                       yaml:"id"`
	PolicyID     string            `json:"policy_id,omitempty"      yaml:"policy_id,omitempty"` // ID of the transfer policy that created this transfer (if any)
	Status       TransferStatus    `json:"status"                   yaml:"status"`
	Config       TransferConfig    `json:"config"                   yaml:"config"`
	Progress     TransferProgress  `json:"progress"                 yaml:"progress"`
	CreatedAt    time.Time         `json:"created_at"               yaml:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"     yaml:"started_at,omitempty"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"   yaml:"completed_at,omitempty"`
	LastPausedAt *time.Time        `json:"last_paused_at,omitempty" yaml:"last_paused_at,omitempty"`
	PID          int               `json:"pid,omitempty"            yaml:"pid,omitempty"`
	LogFile      string            `json:"log_file"                 yaml:"log_file"`
	PIDFile      string            `json:"pid_file"                 yaml:"pid_file"`
	ConfigFile   string            `json:"config_file"              yaml:"config_file"`
	ProgressFile string            `json:"progress_file"            yaml:"progress_file"`
	ErrorMessage string            `json:"error_message,omitempty"  yaml:"error_message,omitempty"`
	SizeInfo     *TransferSizeInfo `json:"size_info,omitempty"      yaml:"size_info,omitempty"` // Transfer size calculated via dry-run
	// Internal state for action flow tracking
	pendingAction TransferAction `json:"-"                        yaml:"-"`
}

// TransferProgress tracks the progress of a transfer operation
type TransferProgress struct {
	BytesTransferred int64     `json:"bytes_transferred"           yaml:"bytes_transferred"`
	TotalBytes       int64     `json:"total_bytes,omitempty"       yaml:"total_bytes,omitempty"`
	TransferRate     int64     `json:"transfer_rate"               yaml:"transfer_rate"`
	ElapsedTime      int64     `json:"elapsed_time"                yaml:"elapsed_time"`
	LastUpdate       time.Time `json:"last_update"                 yaml:"last_update"`
	EstimatedETA     int64     `json:"estimated_eta,omitempty"     yaml:"estimated_eta,omitempty"`
	Phase            string    `json:"phase,omitempty"             yaml:"phase,omitempty"`
	PhaseDescription string    `json:"phase_description,omitempty" yaml:"phase_description,omitempty"`
}

// TransferSizeInfo represents size calculation details for transfer metrics.
// Calculated via `zfs send -nPv` dry-run for accurate stream size.
type TransferSizeInfo struct {
	// CalculatedTransferSize is the exact stream size from dry-run in bytes
	CalculatedTransferSize int64 `json:"calculated_transfer_size" yaml:"calculated_transfer_size"`
	// ActualTransferType indicates the type: "full", "incremental", or "intermediary"
	ActualTransferType string `json:"actual_transfer_type"     yaml:"actual_transfer_type"`
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
	return tm.startTransferInternal(ctx, cfg, "")
}

// StartTransferWithPolicy starts a new transfer that was initiated by a transfer policy
func (tm *TransferManager) StartTransferWithPolicy(
	ctx context.Context,
	cfg TransferConfig,
	policyID string,
) (string, error) {
	return tm.startTransferInternal(ctx, cfg, policyID)
}

// startTransferInternal is the internal implementation for starting transfers
func (tm *TransferManager) startTransferInternal(
	ctx context.Context,
	cfg TransferConfig,
	policyID string,
) (string, error) {
	transferID := common.UUID7()

	transferInfo := &TransferInfo{
		ID:           transferID,
		PolicyID:     policyID, // Set policy ID if transfer was initiated by a policy
		Status:       TransferStatusStarting,
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

	// Calculate transfer size via dry-run (non-blocking, optional)
	// This provides accurate size metrics for business reporting
	if sizeInfo, err := tm.calculateTransferSize(cfg); err == nil && sizeInfo != nil {
		transferInfo.SizeInfo = sizeInfo
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

	tm.logger.Info("Transfer initiated", "id", transferID)

	// Emit transfer started event with complete transfer information
	tm.emitTransferEvent(
		transferInfo,
		eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_STARTED,
	)

	return transferID, nil
}

// CreateSkippedTransfer creates a transfer record with "skipped" status
// This is used when the target is already in sync with the source
func (tm *TransferManager) CreateSkippedTransfer(
	cfg TransferConfig,
	policyID string,
	skipReason string,
) (string, error) {
	transferID := common.UUID7()
	now := time.Now()

	transferInfo := &TransferInfo{
		ID:           transferID,
		PolicyID:     policyID,
		Status:       TransferStatusSkipped,
		Config:       cfg,
		Progress:     TransferProgress{LastUpdate: now},
		CreatedAt:    now,
		ErrorMessage: skipReason, // Use ErrorMessage to store skip reason for visibility
		LogFile:      filepath.Join(tm.transfersDir, fmt.Sprintf("%s.log", transferID)),
		PIDFile:      filepath.Join(tm.transfersDir, fmt.Sprintf("%s.pid", transferID)),
		ConfigFile:   filepath.Join(tm.transfersDir, fmt.Sprintf("%s.yaml", transferID)),
		ProgressFile: filepath.Join(tm.transfersDir, fmt.Sprintf("%s.progress", transferID)),
	}

	// Set completion time since this is a terminal state
	transferInfo.CompletedAt = &now

	// Save transfer configuration
	if err := tm.saveTransferConfig(transferInfo); err != nil {
		return "", err
	}

	// Register transfer (for listing)
	tm.mu.Lock()
	tm.activeTransfers[transferID] = transferInfo
	tm.mu.Unlock()

	tm.logger.Info("Transfer skipped - target already in sync",
		"id", transferID,
		"policy_id", policyID,
		"reason", skipReason)

	// Emit completed event (skipped is a form of completion)
	tm.emitTransferEvent(
		transferInfo,
		eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_COMPLETED,
	)

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
		tm.logger.Info(
			"Validating incremental transfer requirements",
			"id",
			info.ID,
			"from_snapshot",
			sendCfg.FromSnapshot,
		)

		exists, err := tm.snapshotExistsOnTarget(sendCfg.FromSnapshot, recvCfg)
		if err != nil {
			tm.logger.Warn(
				"Could not verify initial snapshot on target, proceeding anyway",
				"id",
				info.ID,
				"error",
				err,
			)
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

			// Check if transfer was paused/stopped during initial send
			tm.mu.Lock()
			wasPausedOrStopped := info.Status == TransferStatusPaused || info.Status == TransferStatusCancelled
			tm.mu.Unlock()

			if wasPausedOrStopped {
				tm.logger.Info("Transfer paused/stopped during initial send, not proceeding to main transfer", "id", info.ID)
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
		// Kill the process immediately if we can't save PID
		// Without PID, the transfer cannot be paused/stopped later
		tm.logger.Error(
			"Failed to save PID, killing process",
			"error",
			err,
			"id",
			info.ID,
			"pid",
			info.PID,
		)
		if killErr := syscall.Kill(-info.PID, syscall.SIGKILL); killErr != nil {
			tm.logger.Error("Failed to kill process after PID save failure", "error", killErr)
		}
		cmd.Wait() // Clean up zombie process
		tm.updateTransferStatusLocked(
			info,
			TransferStatusFailed,
			fmt.Sprintf("Failed to save transfer PID: %v", err),
		)
		return
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
			// Check if error is due to stale/incompatible token or ZFS busy cleaning up
			logContent := tm.readLastLinesFromLogFile(info.LogFile, 20)
			isPartiallyComplete := strings.Contains(logContent, "partially-complete state")
			isStaleToken := strings.Contains(logContent, "kernel modules must be upgraded")

			if isPartiallyComplete || isStaleToken {
				// Dataset still busy or token is stale - keep transfer paused for retry
				var reason string
				if isPartiallyComplete {
					reason = "ZFS still cleaning up from previous pause"
				} else {
					reason = "Resume token became stale"
				}

				tm.logger.Warn("Resume failed - "+reason,
					"id", info.ID,
					"hint", "User should retry resume to get fresh token")

				tm.mu.Lock()
				info.Status = TransferStatusPaused
				info.ErrorMessage = "Resume token is stale or ZFS is still processing. Please retry resume after a minute."
				tm.saveTransferConfig(info)
				tm.mu.Unlock()

				tm.logger.Info("Transfer kept in paused state for retry", "id", info.ID, "reason", reason)
				return
			}

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
		sshPart, err := BuildSSHCommand(recvCfg.RemoteConfig)
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

// calculateTransferSize runs a dry-run send to calculate the exact stream size.
// This uses `zfs send -nPv` which outputs machine-parsable size information.
// The function parses the output to extract the total stream size in bytes.
func (tm *TransferManager) calculateTransferSize(cfg TransferConfig) (*TransferSizeInfo, error) {
	sendCfg := cfg.SendConfig

	// Build dry-run send command with parsable output
	sendPart := []string{command.BinZFS, "send", "-n", "-P", "-v"}

	// Add flags that affect stream size calculation
	if sendCfg.Compressed {
		sendPart = append(sendPart, "-c")
	}
	if sendCfg.Properties {
		sendPart = append(sendPart, "-p")
	}
	if sendCfg.Raw {
		sendPart = append(sendPart, "-w")
	}
	if sendCfg.LargeBlocks {
		sendPart = append(sendPart, "-L")
	}
	if sendCfg.Replicate {
		sendPart = append(sendPart, "-R")
		if sendCfg.SkipMissing {
			sendPart = append(sendPart, "-s")
		}
	}

	// Determine transfer type and add incremental options
	transferType := "full"

	if sendCfg.FromSnapshot != "" {
		if sendCfg.Intermediary {
			sendPart = append(sendPart, "-I", sendCfg.FromSnapshot)
			transferType = "intermediary"
		} else {
			sendPart = append(sendPart, "-i", sendCfg.FromSnapshot)
			transferType = "incremental"
		}
	}

	// Add target snapshot
	sendPart = append(sendPart, sendCfg.Snapshot)

	// Sanitize and build command
	sendPart = sanitizeCommandArgs(sendPart)
	cmdStr := fmt.Sprintf("sudo %s", shellquote.Join(sendPart...))

	tm.logger.Debug("Calculating transfer size via dry-run", "command", cmdStr)

	// Execute dry-run
	cmd := exec.Command("bash", "-c", cmdStr)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		tm.logger.Warn("Failed to calculate transfer size",
			"error", err,
			"stderr", stderr.String(),
			"snapshot", sendCfg.Snapshot)
		// Return nil without error - size calculation is optional
		return nil, nil
	}

	// Parse the parsable output to find the total size
	// Format: "size\t<bytes>" on the last line
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var streamSize int64
	for _, line := range lines {
		if strings.HasPrefix(line, "size\t") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 2 {
				size, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				if err != nil {
					tm.logger.Warn("Failed to parse stream size",
						"line", line,
						"error", err)
					return nil, nil
				}
				streamSize = size
				break
			}
		}
	}

	if streamSize == 0 {
		tm.logger.Debug("No size line found in dry-run output",
			"output", output,
			"snapshot", sendCfg.Snapshot)
		return nil, nil
	}

	sizeInfo := &TransferSizeInfo{
		CalculatedTransferSize: streamSize,
		ActualTransferType:     transferType,
	}

	tm.logger.Info("Calculated transfer size",
		"calculated_transfer_size", streamSize,
		"actual_transfer_type", transferType,
		"snapshot", sendCfg.Snapshot)

	return sizeInfo, nil
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

	// Check if we have a valid PID to pause (defensive - should not happen with fail-fast on PID save)
	if info.PID == 0 {
		info.pendingAction = TransferActionNone // Reset pending action
		tm.logger.Error(
			"Unexpected: transfer has no PID during pause",
			"id",
			info.ID,
			"status",
			info.Status,
		)
		return errors.New(
			errors.TransferPauseFailed,
			"transfer PID not available (unexpected state - please check logs)",
		)
	}

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

		// Record pause time for cooloff period enforcement
		now := time.Now()
		info.LastPausedAt = &now
		tm.saveTransferConfig(info)

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

	// Enforce cooloff period to allow ZFS to clean up after pause
	if info.LastPausedAt != nil {
		elapsed := time.Since(*info.LastPausedAt)
		if elapsed < cooloffPeriod {
			remaining := cooloffPeriod - elapsed
			remainingSeconds := int(remaining.Seconds())
			return errors.New(
				errors.TransferResumeFailed,
				fmt.Sprintf(
					"Please wait %d more seconds before resuming to allow ZFS to clean up from pause",
					remainingSeconds,
				),
			)
		}
	}

	// Fetch resume token (works for both initial_send and main send phases)
	tm.logger.Info(
		"Fetching resume token for paused transfer",
		"id",
		transferID,
		"phase",
		info.Progress.Phase,
	)
	token, err := tm.getReceiveResumeTokenWithRetry(
		info.Config.ReceiveConfig.Target,
		info.Config.ReceiveConfig.RemoteConfig,
	)
	if err != nil {
		// Check if error is due to dataset being busy (transient - don't fail transfer)
		errStr := err.Error()
		isBusy := strings.Contains(errStr, "dataset is busy") ||
			strings.Contains(errStr, "resource busy")

		if isBusy {
			// Dataset still cleaning up from pause - don't fail, let user retry
			tm.logger.Warn("Dataset busy, resume can be retried",
				"id", transferID,
				"error", err,
				"hint", "ZFS is still cleaning up from pause, please retry in a few seconds")
			return errors.New(
				errors.TransferResumeFailed,
				"Dataset busy - ZFS is still processing the paused receive. Please retry resume in a few seconds",
			)
		}

		// Non-transient error - fail the transfer
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
	info.Status = TransferStatusRunning
	info.ErrorMessage = ""

	// Emit transfer resumed event before starting execution
	tm.emitTransferEvent(info, eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_RESUMED)

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

	// Check if we have a valid PID to stop (defensive - should not happen with fail-fast on PID save)
	if info.PID == 0 {
		info.pendingAction = TransferActionNone // Reset pending action
		tm.logger.Error(
			"Unexpected: transfer has no PID during stop",
			"id",
			info.ID,
			"status",
			info.Status,
		)
		return errors.New(
			errors.TransferStopFailed,
			"transfer PID not available (unexpected state - please check logs)",
		)
	}

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
				// If the process is paused already, the current PID will not be active anymore.
				// Do not return error if the process is already paused.
				if info.Status != TransferStatusPaused {
					info.pendingAction = TransferActionNone // Reset on error
					return errors.Wrap(err, errors.TransferStopFailed)
				}
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

	var transferInfo *TransferInfo

	// Check if it's an active transfer (running/paused only)
	if info, exists := tm.activeTransfers[transferID]; exists {
		// Can only delete finished transfers
		if info.Status == TransferStatusRunning || info.Status == TransferStatusPaused {
			return errors.New(errors.TransferInvalidState, "Cannot delete active transfer")
		}

		transferInfo = info

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

		// Load transfer info before deleting for event emission
		transferInfo = tm.loadTransferFromFile(configFile)

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

	// Emit transfer deleted event so Toggle can sync its records
	if transferInfo != nil {
		tm.emitTransferEvent(
			transferInfo,
			eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_DELETED,
		)
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
		return "", errors.New(
			errors.RodentMisc,
			fmt.Sprintf(
				"Log file too large (%d bytes). Maximum allowed: %d bytes. Use GetTransferLogGist() for large files.",
				fileInfo.Size(),
				maxLogSize,
			),
		)
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
func (tm *TransferManager) truncateLogContentEfficient(
	logFile string,
	fileSize int64,
	logConfig TransferLogConfig,
) (string, error) {
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
func (tm *TransferManager) snapshotExistsOnTarget(
	snapshot string,
	recvCfg ReceiveConfig,
) (bool, error) {
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
		sshPart, err := BuildSSHCommand(recvCfg.RemoteConfig)
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
func (tm *TransferManager) performInitialSend(
	_ context.Context,
	info *TransferInfo,
	fromSnapshot string,
) error {
	tm.logger.Info("Performing initial snapshot send", "id", info.ID, "snapshot", fromSnapshot)

	// Create a temporary config for the initial send (full send, not incremental)
	initialConfig := TransferConfig{
		SendConfig: SendConfig{
			Snapshot: fromSnapshot,
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

	// Save PID so the initial send can be paused/stopped
	info.PID = cmd.Process.Pid
	if err := tm.savePID(info); err != nil {
		// Kill the process immediately if we can't save PID
		// Without PID, the transfer cannot be paused/stopped later
		tm.logger.Error(
			"Failed to save PID for initial send, killing process",
			"error",
			err,
			"id",
			info.ID,
			"pid",
			info.PID,
		)
		if killErr := syscall.Kill(-info.PID, syscall.SIGKILL); killErr != nil {
			tm.logger.Error(
				"Failed to kill initial send process after PID save failure",
				"error",
				killErr,
			)
		}
		cmd.Wait() // Clean up zombie process
		return fmt.Errorf("failed to save transfer PID: %w", err)
	}
	tm.logger.Debug("Initial send PID saved", "id", info.ID, "pid", info.PID)

	// Wait for completion
	waitErr := cmd.Wait()

	// Check if transfer was paused/stopped before treating Wait error as failure
	tm.mu.Lock()
	wasPaused := info.Status == TransferStatusPaused
	wasCancelled := info.Status == TransferStatusCancelled
	tm.mu.Unlock()

	if waitErr != nil && !wasPaused && !wasCancelled {
		return fmt.Errorf("initial send failed: %w", waitErr)
	}

	// If transfer was paused/cancelled, return without error (status already set)
	if wasPaused || wasCancelled {
		tm.logger.Info(
			"Initial send terminated due to pause/stop",
			"id",
			info.ID,
			"status",
			info.Status,
		)
		return nil
	}

	// Clear PID after initial send completes (main transfer will set it again)
	info.PID = 0

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

	// Map transfer status to event operation
	var operation eventspb.DataTransferTransferPayload_DataTransferOperation
	shouldEmit := true

	switch status {
	case TransferStatusCompleted:
		completedTime := time.Now()
		info.CompletedAt = &completedTime
		operation = eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_COMPLETED
	case TransferStatusFailed:
		completedTime := time.Now()
		info.CompletedAt = &completedTime
		operation = eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_FAILED
	case TransferStatusCancelled:
		completedTime := time.Now()
		info.CompletedAt = &completedTime
		operation = eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_CANCELLED
	case TransferStatusPaused:
		operation = eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_PAUSED
	default:
		// Don't emit events for other status changes (running, starting, etc.)
		shouldEmit = false
	}

	// Emit event for terminal states and paused state
	if shouldEmit {
		tm.emitTransferEvent(info, operation)
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
		sshPart, err := BuildSSHCommand(remoteConfig)
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

	// Capture both stdout and stderr for better error reporting
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			tm.logger.Debug("Get resume token stderr", "stderr", stderrStr, "target", target)
			return "", fmt.Errorf("%w: %s", err, stderrStr)
		}
		return "", err
	}

	token := strings.TrimSpace(stdout.String())
	if token == "-" {
		return "", errors.New(errors.ZFSDatasetNoReceiveToken, "No resume token available")
	}

	return token, nil
}

// abortPartialReceive aborts a partial receive operation with retry logic
func (tm *TransferManager) abortPartialReceive(target string, remoteConfig RemoteConfig) error {
	const maxRetries = 5
	const retryDelay = 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := tm.abortPartialReceiveOnce(target, remoteConfig)
		if err == nil {
			if attempt > 1 {
				tm.logger.Info("Abort succeeded after retry", "target", target, "attempt", attempt)
			}
			return nil
		}

		lastErr = err
		errStr := err.Error()

		// Check if error is due to dataset being busy (needs retry)
		isBusy := strings.Contains(errStr, "dataset is busy") ||
			strings.Contains(errStr, "resource busy")

		if !isBusy {
			// Non-busy error - return immediately
			return err
		}

		// Dataset is busy - retry with delay
		if attempt < maxRetries {
			tm.logger.Debug("Dataset busy during abort, will retry",
				"target", target,
				"attempt", attempt,
				"retry_in_seconds", retryDelay.Seconds())
			time.Sleep(retryDelay)
		}
	}

	return fmt.Errorf("failed to abort after %d attempts (dataset busy): %w", maxRetries, lastErr)
}

// abortPartialReceiveOnce attempts to abort a partial receive once
func (tm *TransferManager) abortPartialReceiveOnce(target string, remoteConfig RemoteConfig) error {
	var cmd *exec.Cmd

	if remoteConfig.Host != "" {
		// Remote dataset
		sshPart, err := BuildSSHCommand(remoteConfig)
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

	// Capture both stdout and stderr for better error reporting
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Include stderr in error message for debugging
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			tm.logger.Debug("Abort command stderr", "stderr", stderrStr, "target", target)
			return fmt.Errorf("%w: %s", err, stderrStr)
		}
		return err
	}

	return nil
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
	// Save PID to .pid file
	pidData := fmt.Appendf(nil, "%d", info.PID)
	if err := os.WriteFile(info.PIDFile, pidData, 0644); err != nil {
		return err
	}

	// Also save to YAML config so loadExistingTransfers can detect running processes
	return tm.saveTransferConfig(info)
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

		// Track original status to detect changes
		originalStatus := info.Status

		// Only load transfers that should be in activeTransfers
		// Completed/failed/cancelled/unknown transfers are handled as historical transfers
		if info.Status == TransferStatusCompleted || info.Status == TransferStatusFailed ||
			info.Status == TransferStatusCancelled || info.Status == TransferStatusUnknown {
			continue // Skip - these are historical transfers
		}

		// Check if process is still running for active transfers
		if info.PID > 0 && tm.isProcessRunning(info.PID) {
			info.Status = TransferStatusRunning
		} else if info.Status == TransferStatusRunning {
			// Process not running, but transfer was running
			if info.Config.ReceiveConfig.Resumable {
				// If resumable, mark as paused for potential resume
				info.Status = TransferStatusPaused
				info.ErrorMessage = ""
			} else {
				// If not resumable, we can't determine if it completed successfully or failed
				// Mark as unknown since we don't have enough information
				info.Status = TransferStatusUnknown
				info.ErrorMessage = "Process no longer running (status uncertain)"
			}
		} else if info.Status == TransferStatusPaused {
			// Transfer was paused, keep it paused regardless of process state
			// (paused transfers don't have running processes)
			info.ErrorMessage = ""
		}

		// Save the corrected status back to disk if it changed
		// This ensures GetTransfer returns consistent status with ListTransfers
		if originalStatus != info.Status {
			if err := tm.saveTransferConfig(&info); err != nil {
				tm.logger.Warn("Failed to save corrected transfer status",
					"transfer_id", info.ID,
					"old_status", originalStatus,
					"new_status", info.Status,
					"error", err)
			}
		}

		// Only add truly active transfers (running/paused) to activeTransfers
		if info.Status == TransferStatusRunning || info.Status == TransferStatusPaused {
			tm.activeTransfers[info.ID] = &info
		}
	}

	return nil
}

// buildDataTransferPayload creates a DataTransferTransferPayload from TransferInfo
// The payload contains essential routing fields plus the complete TransferInfo as JSON
// This ensures Toggle receives all transfer details for proper sync without proto schema coupling
func (tm *TransferManager) buildDataTransferPayload(
	info *TransferInfo,
	operation eventspb.DataTransferTransferPayload_DataTransferOperation,
) *eventspb.DataTransferTransferPayload {
	// Serialize TransferInfo to JSON for complete transfer details
	infoJSON, err := json.Marshal(info)
	if err != nil {
		tm.logger.Error(
			"Failed to marshal TransferInfo to JSON",
			"error",
			err,
			"transfer_id",
			info.ID,
		)
		infoJSON = []byte("{}")
	}

	return &eventspb.DataTransferTransferPayload{
		TransferId:       info.ID,
		Operation:        operation,
		TransferInfoJson: string(infoJSON),
	}
}

// emitTransferEvent emits DataTransfer events with complete transfer information
func (tm *TransferManager) emitTransferEvent(
	info *TransferInfo,
	operation eventspb.DataTransferTransferPayload_DataTransferOperation,
) {
	var level eventspb.EventLevel
	var action string

	switch operation {
	case eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_STARTED:
		level = eventspb.EventLevel_EVENT_LEVEL_INFO
		action = "started"
	case eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_PAUSED:
		level = eventspb.EventLevel_EVENT_LEVEL_INFO
		action = "paused"
	case eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_RESUMED:
		level = eventspb.EventLevel_EVENT_LEVEL_INFO
		action = "resumed"
	case eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_COMPLETED:
		level = eventspb.EventLevel_EVENT_LEVEL_INFO
		action = "completed"
	case eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_FAILED:
		level = eventspb.EventLevel_EVENT_LEVEL_ERROR
		action = "failed"
	case eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_CANCELLED:
		level = eventspb.EventLevel_EVENT_LEVEL_WARN
		action = "cancelled"
	case eventspb.DataTransferTransferPayload_DATA_TRANSFER_OPERATION_DELETED:
		level = eventspb.EventLevel_EVENT_LEVEL_INFO
		action = "deleted"
	default:
		return // Don't emit events for unspecified operations
	}

	// Build complete payload with all TransferInfo details
	payload := tm.buildDataTransferPayload(info, operation)

	// Metadata for additional context
	transferMeta := map[string]string{
		"component":   "zfs-transfer",
		"action":      action,
		"transfer_id": info.ID,
		"status":      string(info.Status),
	}

	// Add duration to metadata if available
	if info.CompletedAt != nil && info.StartedAt != nil {
		transferMeta["duration_seconds"] = fmt.Sprintf(
			"%.0f",
			info.CompletedAt.Sub(*info.StartedAt).Seconds(),
		)
	}

	events.EmitDataTransfer(level, payload, transferMeta)
}

func (tm *TransferManager) handleTransferCompletion(info *TransferInfo) {
	// Clear any pending action now that executeTransfer has completed
	tm.mu.Lock()
	info.pendingAction = TransferActionNone

	// Remove completed/failed transfers from active transfers so they become historical
	if info.Status == TransferStatusCompleted || info.Status == TransferStatusFailed ||
		info.Status == TransferStatusCancelled {
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

// readLastLinesFromLogFile reads the last N lines from a log file
func (tm *TransferManager) readLastLinesFromLogFile(logFilePath string, numLines int) string {
	cmd := exec.Command("tail", "-n", fmt.Sprintf("%d", numLines), logFilePath)
	output, err := cmd.Output()
	if err != nil {
		tm.logger.Debug("Failed to read log file", "path", logFilePath, "error", err)
		return ""
	}
	return string(output)
}

// Shutdown gracefully terminates all active transfers
// Sends SIGTERM to all process groups concurrently, waits for graceful exit,
// then sends SIGKILL to any stragglers
func (tm *TransferManager) Shutdown(timeout time.Duration) error {
	tm.mu.Lock()

	// Get snapshot of active transfers
	activeTransfers := make([]*TransferInfo, 0, len(tm.activeTransfers))
	for _, info := range tm.activeTransfers {
		if info.Status == TransferStatusRunning && info.PID > 0 {
			activeTransfers = append(activeTransfers, info)
		}
	}
	tm.mu.Unlock()

	if len(activeTransfers) == 0 {
		tm.logger.Info("No active transfers to shutdown")
		return nil
	}

	tm.logger.Info("Shutting down active transfers", "count", len(activeTransfers))

	// Send SIGTERM to all process groups concurrently
	for _, info := range activeTransfers {
		go func(info *TransferInfo) {
			tm.logger.Debug("Sending SIGTERM to transfer process group",
				"id", info.ID, "pid", info.PID)
			// Negative PID sends signal to entire process group
			if err := syscall.Kill(-info.PID, syscall.SIGTERM); err != nil {
				tm.logger.Debug("Failed to send SIGTERM",
					"id", info.ID, "pid", info.PID, "error", err)
			}
		}(info)
	}

	// Wait for processes to exit gracefully
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	remaining := make(map[string]*TransferInfo)
	for _, info := range activeTransfers {
		remaining[info.ID] = info
	}

	for time.Now().Before(deadline) && len(remaining) > 0 {
		<-ticker.C

		for id, info := range remaining {
			if !tm.isProcessRunning(info.PID) {
				tm.logger.Debug("Transfer process exited gracefully",
					"id", info.ID, "pid", info.PID)
				delete(remaining, id)
			}
		}
	}

	// Kill any remaining processes with SIGKILL
	if len(remaining) > 0 {
		tm.logger.Warn("Forcefully killing remaining transfer processes",
			"count", len(remaining))

		for _, info := range remaining {
			tm.logger.Debug("Sending SIGKILL to transfer process group",
				"id", info.ID, "pid", info.PID)
			// Ignore errors - process might have exited between checks
			_ = syscall.Kill(-info.PID, syscall.SIGKILL)
		}

		// Give SIGKILL a moment to take effect
		time.Sleep(500 * time.Millisecond)
	}

	tm.logger.Info("Transfer shutdown complete")
	return nil
}
