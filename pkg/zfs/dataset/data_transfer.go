// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2024 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"
	stderrors "errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kballard/go-shellquote"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
)

var (
	// Validate snapshot names (ZFS allows alphanumeric, _, -, :, and . in names)
	snapshotNameRegex = regexp.MustCompile(
		`^[a-zA-Z0-9][a-zA-Z0-9_.:-]*(/[a-zA-Z0-9][a-zA-Z0-9_.:-]*)*@[a-zA-Z0-9][a-zA-Z0-9_.:-]*$`,
	)
	// Validate dataset names (ZFS allows alphanumeric, _, -, :, and . in names)
	datasetNameRegex = regexp.MustCompile(
		`^[a-zA-Z0-9][a-zA-Z0-9_.:-]*(/[a-zA-Z0-9][a-zA-Z0-9_.:-]*)*$`,
	)
	propertyValueRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:/@+-]*$`)

	maxRetries    = 3
	retryInterval = 5 * time.Second
)

// TransferLogConfig holds configuration for transfer-specific log management
type TransferLogConfig struct {
	MaxSizeBytes     int64 `json:"max_size_bytes,omitempty"     yaml:"max_size_bytes"`     // Default: 10KB
	TruncateOnFinish bool  `json:"truncate_on_finish,omitempty" yaml:"truncate_on_finish"` // Default: true
	RetainOnFailure  bool  `json:"retain_on_failure,omitempty"  yaml:"retain_on_failure"`  // Default: true
	HeaderLines      int   `json:"header_lines,omitempty"       yaml:"header_lines"`       // Default: 20
	FooterLines      int   `json:"footer_lines,omitempty"       yaml:"footer_lines"`       // Default: 20
}

type TransferConfig struct {
	SendConfig    SendConfig         `json:"send"                 binding:"required"`
	ReceiveConfig ReceiveConfig      `json:"receive"              binding:"required"`
	LogConfig     *TransferLogConfig `json:"log_config,omitempty"                    yaml:"log_config,omitempty"`
}

type SendConfig struct {
	// Required parameters
	Snapshot     string `json:"snapshot"      binding:"required"`
	FromSnapshot string `json:"from_snapshot"`

	// Send options
	Replicate    bool `json:"replicate"`     // -R: Replicate
	SkipMissing  bool `json:"skip_missing"`  // -s: Skip missing snapshots, used with -R
	Properties   bool `json:"properties"`    // -p: Include properties
	Raw          bool `json:"raw"`           // -w: Raw encrypted stream
	LargeBlocks  bool `json:"large_blocks"`  // -L: Allow larger blocks
	EmbedData    bool `json:"embed_data"`    // -e: Embed data
	Holds        bool `json:"holds"`         // -h: Include user holds
	BackupStream bool `json:"backup_stream"` // -b: Backup stream

	// Incremental options (mutually exclusive)
	Intermediary bool `json:"intermediary"` // -I: Include intermediary snapshots
	Incremental  bool `json:"incremental"`  // -i: Simple incremental

	// Performance
	Compressed bool `json:"compressed"` // -c: Stream compression
	DryRun     bool `json:"dry_run"`    // -n: Dry run
	Verbose    bool `json:"verbose"`    // -v: Verbose output

	// Resume options
	ResumeToken string `json:"resume_token"` // Token for resuming send
	Parsable    bool   `json:"parsable"`     // -P: Print machine-parsable verbose information about the stream package generated

	// Transfer control
	// TODO: Implement timeout
	Timeout time.Duration `json:"timeout"`

	// Logging
	LogLevel string `json:"log_level"` // Log level for send operation, not related to zfs verbose output
}

type ReceiveConfig struct {
	Target       string            `json:"target"                binding:"required"`
	Force        bool              `json:"force"`         // -F: Force rollback
	Unmounted    bool              `json:"unmounted"`     // -u: Do not mount
	Resumable    bool              `json:"resumable"`     // -s: Allow resume
	Properties   map[string]string `json:"properties"`    // -o: Properties to set
	Origin       string            `json:"origin"`        // -o origin=snapshot
	ExcludeProps []string          `json:"exclude_props"` // -x: Properties to exclude
	UseParent    bool              `json:"use_parent"`    // -d: Use parent filesystem
	DryRun       bool              `json:"dry_run"`       // -n: Dry run
	Verbose      bool              `json:"verbose"`       // -v: Print verbose info
	RemoteConfig RemoteConfig      `json:"remote_host,omitempty"`
}

// RemoteConfig defines SSH connection parameters
type RemoteConfig struct {
	Host             string `json:"host"`                          // Remote hostname/IP
	Port             int    `json:"port"`                          // SSH port (default: 22)
	User             string `json:"user"`                          // SSH user
	PrivateKey       string `json:"private_key,omitempty"`         // Path to private key
	SSHOptions       string `json:"options,omitempty"`             // Additional SSH options
	SkipHostKeyCheck bool   `json:"skip_host_key_check,omitempty"` // Skip SSH host key check
}

// Allowed SSH options to prevent abuse
var allowedSSHOptions = map[string]bool{
	"AddressFamily":            true,
	"Compression":              true,
	"ConnectionAttempts":       true,
	"ConnectTimeout":           true,
	"TCPKeepAlive":             true,
	"ServerAliveInterval":      true,
	"ServerAliveCountMax":      true,
	"Ciphers":                  true,
	"MACs":                     true,
	"KexAlgorithms":            true,
	"PreferredAuthentications": true,
}

// validateSSHOption validates a single SSH option
func validateSSHOption(option string) error {
	parts := strings.SplitN(option, "=", 2)
	if len(parts) != 2 {
		return errors.New(errors.CommandInvalidInput, "Invalid SSH option format")
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Check if option is allowed
	if !allowedSSHOptions[key] {
		return errors.New(errors.CommandInvalidInput, "SSH option not allowed").
			WithMetadata("option", key)
	}

	// Validate option value
	if strings.ContainsAny(value, "&|;<>()$`\\\"'") {
		return errors.New(errors.CommandInvalidInput, "Invalid SSH option value").
			WithMetadata("option", key)
	}

	return nil
}

// parseSSHOptions safely parses custom SSH options
func parseSSHOptions(options string) ([]string, error) {
	if options == "" {
		return nil, nil
	}

	var result []string
	// Split options respecting quotes
	opts := strings.Fields(options)

	for _, opt := range opts {
		if !strings.HasPrefix(opt, "-o") {
			// All custom options must be in -o format
			return nil, errors.New(errors.CommandInvalidInput,
				"SSH options must use -o format")
		}

		// Remove -o prefix if present
		opt = strings.TrimPrefix(opt, "-o")
		opt = strings.TrimSpace(opt)

		if err := validateSSHOption(opt); err != nil {
			return nil, err
		}
		// Add validated option
		result = append(result, "-o", opt)
	}

	return result, nil
}

// GetResumeToken gets the resume token from a partially received dataset
func (m *Manager) GetResumeToken(ctx context.Context, cfg NameConfig) (string, error) {
	args := []string{"get", "-H", "-o", "value", "receive_resume_token", cfg.Name}

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs get", args...)
	if err != nil {
		return "", errors.Wrap(err, errors.ZFSPropertyError)
	}

	token := strings.TrimSpace(string(out))
	if token == "-" {
		return "", errors.New(
			errors.ZFSDatasetNoReceiveToken,
			"No resume token available",
		) // No resume token available
	}

	return token, nil
}

// DEPRECATED: SendReceive is deprecated, use TransferManager.StartTransfer() instead
// SendReceive handles data transfer on the same machine
func (m *Manager) SendReceive(
	ctx context.Context,
	sendCfg SendConfig,
	recvCfg ReceiveConfig,
) error {
	// Validate configurations
	if err := validateSendConfig(sendCfg); err != nil {
		return err
	}
	if err := validateReceiveConfig(recvCfg); err != nil {
		return err
	}
	if recvCfg.RemoteConfig.Host != "" {
		if err := validateSSHConfig(recvCfg.RemoteConfig); err != nil {
			return err
		}
	}

	// Use context with timeout
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 24*time.Hour)
		defer cancel()
	}

	// Build send command
	sendPart := []string{command.BinZFS, "send"}

	// TODO: Enforce flag rules and combinations

	if sendCfg.ResumeToken != "" {
		sendPart = append(sendPart, "-t", sendCfg.ResumeToken)
	}
	if sendCfg.Parsable {
		sendPart = append(sendPart, "-P")
	}
	if sendCfg.Compressed {
		sendPart = append(sendPart, "-c")
	}
	if sendCfg.EmbedData {
		sendPart = append(sendPart, "-e")
	}
	if sendCfg.LargeBlocks {
		sendPart = append(sendPart, "-L")
	}
	if sendCfg.Holds {
		sendPart = append(sendPart, "-h")
	}
	if sendCfg.BackupStream {
		sendPart = append(sendPart, "-b")
	}
	// TODO: handle verbose output
	// NOte: check the std stream as stdout stream is used for the data transfer

	if sendCfg.Replicate {
		sendPart = append(sendPart, "-R")
	}
	if sendCfg.Properties {
		sendPart = append(sendPart, "-p")
	}
	if sendCfg.Raw {
		sendPart = append(sendPart, "-w")
	}
	if sendCfg.Verbose {
		sendPart = append(sendPart, "-v")
	}
	if sendCfg.DryRun {
		sendPart = append(sendPart, "-n")
	}

	// Set the process title to a per-second report of how much data has been sent
	sendPart = append(sendPart, "-V")

	// Incremental options (mutually exclusive)
	if sendCfg.FromSnapshot != "" && sendCfg.Intermediary {
		sendPart = append(sendPart, "-I", sendCfg.FromSnapshot)
	} else if sendCfg.FromSnapshot != "" {
		sendPart = append(sendPart, "-i", sendCfg.FromSnapshot)
	}

	sendPart = append(sendPart, sendCfg.Snapshot)

	// Build receive command
	recvPart := []string{command.BinZFS, "receive"}
	if recvCfg.Force {
		recvPart = append(recvPart, "-F")
	}
	if recvCfg.Unmounted {
		recvPart = append(recvPart, "-u")
	}
	if recvCfg.Resumable {
		recvPart = append(recvPart, "-s")
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

	// Build full command with SSH if remote
	var fullCmd string
	if recvCfg.RemoteConfig.Host != "" {
		sshPart, err := buildSSHCommand(recvCfg.RemoteConfig)
		if err != nil {
			return errors.Wrap(err, errors.CommandInvalidInput)
		}
		fullCmd = fmt.Sprintf("sudo %s | %s sudo %s 2>&1",
			shellquote.Join(sendPart...),
			shellquote.Join(sshPart...),
			shellquote.Join(recvPart...))
	} else {
		fullCmd = fmt.Sprintf("sudo %s | sudo %s 2>&1",
			shellquote.Join(sendPart...),
			shellquote.Join(recvPart...))
	}

	l, err := logger.NewTag(logger.Config{LogLevel: sendCfg.LogLevel}, "zfs-data-transfer")
	if err != nil {
		return errors.Wrap(err, errors.RodentMisc)
	}
	l.Debug("Executing command",
		"cmd", fullCmd)

	// Execute with retries
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, errors.CommandContext)
		}

		cmd := exec.CommandContext(ctx, "bash", "-c", fullCmd)
		var output strings.Builder
		cmd.Stdout = &output
		cmd.Stderr = &output

		err = cmd.Run()
		outputStr := output.String()

		if outputStr != "" {
			l.Debug("Command output", "output", outputStr, "attempt", attempt)
		}

		if err == nil {
			return nil
		}

		lastErr = err
		var exitErr *exec.ExitError
		if stderrors.As(err, &exitErr) {
			// Don't retry on certain exit codes
			if exitErr.ExitCode() == 1 || exitErr.ExitCode() == 2 {
				return errors.Wrap(err, errors.ZFSDatasetReceive).
					WithMetadata("exit_code", fmt.Sprintf("%d", exitErr.ExitCode())).
					WithMetadata("output", outputStr).
					WithMetadata("command", fullCmd)
			}
		}

		if attempt < maxRetries {
			l.Debug("Retrying command",
				"attempt", attempt,
				"max_attempts", maxRetries,
				"retry_interval", retryInterval)
			time.Sleep(retryInterval)
		}
	}

	return errors.Wrap(lastErr, errors.ZFSDatasetReceive).
		WithMetadata("output", "Max retries exceeded").
		WithMetadata("command", fullCmd)
}

func validateSendConfig(cfg SendConfig) error {
	if cfg.ResumeToken != "" {
		// Resume tokens are opaque but should be printable ASCII
		if !utf8.ValidString(cfg.ResumeToken) {
			return errors.New(errors.CommandInvalidInput, "Invalid resume token")
		}
		return nil
	}

	// Validate snapshot name
	if !snapshotNameRegex.MatchString(cfg.Snapshot) {
		return errors.New(errors.CommandInvalidInput, "Invalid snapshot name")
	}

	// Validate from snapshot if specified
	if cfg.FromSnapshot != "" && !snapshotNameRegex.MatchString(cfg.FromSnapshot) {
		return errors.New(errors.CommandInvalidInput, "Invalid from snapshot name")
	}

	return nil
}

func validateReceiveConfig(cfg ReceiveConfig) error {
	// Validate target dataset
	if !datasetNameRegex.MatchString(cfg.Target) {
		return errors.New(errors.CommandInvalidInput, "Invalid target dataset")
	}

	// Validate property names and values
	for k, v := range cfg.Properties {
		if !propertyValueRegex.MatchString(k) || !propertyValueRegex.MatchString(v) {
			return errors.New(errors.CommandInvalidInput, "Invalid property name or value")
		}
	}

	return nil
}

// ValidateSSHConfig validates SSH connection parameters
func validateSSHConfig(cfg RemoteConfig) error {
	if cfg.Host == "" {
		return errors.New(errors.CommandInvalidInput, "SSH host cannot be empty")
	}
	if cfg.User == "" {
		return errors.New(errors.CommandInvalidInput, "SSH user cannot be empty")
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return errors.New(errors.CommandInvalidInput, "Invalid SSH port")
	}
	return nil
}

// buildSSHCommand constructs SSH command with proper options
func buildSSHCommand(cfg RemoteConfig) ([]string, error) {
	sshCmd := []string{"ssh"}

	// Core SSH options
	if cfg.Port != 0 && cfg.Port != 22 {
		sshCmd = append(sshCmd, "-p", fmt.Sprintf("%d", cfg.Port))
	}
	if cfg.PrivateKey != "" {
		// Validate private key path
		if strings.ContainsAny(cfg.PrivateKey, "&|;<>()$`\\\"'") {
			return nil, errors.New(errors.CommandInvalidInput,
				"Invalid private key path")
		}
		sshCmd = append(sshCmd, "-i", cfg.PrivateKey)

		// Use Rodent-managed known_hosts file (respects config overrides)
		sshCmd = append(sshCmd, "-o", fmt.Sprintf("UserKnownHostsFile=%s", config.GetKnownHostsFilePath()))
	}

	// Security options
	if cfg.SkipHostKeyCheck {
		sshCmd = append(sshCmd, "-o", "StrictHostKeyChecking=no")
	}

	// Connection options
	sshCmd = append(sshCmd,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=10",
		"-o", "ServerAliveCountMax=3",
	)

	// Parse and add custom options
	if cfg.SSHOptions != "" {
		customOpts, err := parseSSHOptions(cfg.SSHOptions)
		if err != nil {
			return nil, err
		}
		sshCmd = append(sshCmd, customOpts...)
	}

	// Validate and add destination
	if strings.ContainsAny(cfg.User, "&|;<>()$`\\\"'") {
		return nil, errors.New(errors.CommandInvalidInput, "Invalid SSH username")
	}
	if strings.ContainsAny(cfg.Host, "&|;<>()$`\\\"'") {
		return nil, errors.New(errors.CommandInvalidInput, "Invalid SSH host")
	}
	sshCmd = append(sshCmd, fmt.Sprintf("%s@%s", cfg.User, cfg.Host))

	return sshCmd, nil
}

// sanitizeCommandArgs sanitizes command arguments
func sanitizeCommandArgs(args []string) []string {
	sanitized := make([]string, 0, len(args))
	for _, arg := range args {
		// Remove any shell metacharacters
		if strings.ContainsAny(arg, "&|><$`\\[]{}") {
			continue
		}
		// Remove any path traversal attempts
		if strings.Contains(arg, "..") {
			continue
		}
		// Ensure absolute paths for binaries
		if strings.HasPrefix(arg, command.BinZFS) || strings.HasPrefix(arg, command.BinZpool) {
			sanitized = append(sanitized, arg)
			continue
		}
		// Regular arguments
		sanitized = append(sanitized, arg)
	}
	return sanitized
}
