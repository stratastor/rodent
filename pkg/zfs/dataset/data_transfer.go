package dataset

import (
	"context"
	stderrors "errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
	"github.com/stratastor/rodent/pkg/zfs/command"
)

type SendConfig struct {
	// Required parameters
	Snapshot     string `json:"snapshot" binding:"required"`
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
	Progress    bool   `json:"progress"`     // -P: Print parsable progress statistics

	// Transfer control
	// TODO: Implement timeout
	Timeout time.Duration `json:"timeout"`

	// Logging
	LogLevel string `json:"log_level"` // Log level for send operation, not related to zfs verbose output
}

type ReceiveConfig struct {
	Target       string            `json:"target" binding:"required"`
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
	Host             string `json:"host" binding:"required"`       // Remote hostname/IP
	Port             int    `json:"port"`                          // SSH port (default: 22)
	User             string `json:"user" binding:"required"`       // SSH user
	PrivateKey       string `json:"private_key,omitempty"`         // Path to private key
	Options          string `json:"options,omitempty"`             // Additional SSH options
	SkipHostKeyCheck bool   `json:"skip_host_key_check,omitempty"` // Skip SSH host key check
}

// GetResumeToken gets the resume token from a partially received dataset
func (m *Manager) GetResumeToken(ctx context.Context, dataset string) (string, error) {
	args := []string{"get", "-H", "-o", "value", "receive_resume_token", dataset}

	out, err := m.executor.Execute(ctx, command.CommandOptions{}, "zfs get", args...)
	if err != nil {
		return "", errors.Wrap(err, errors.ZFSPropertyError)
	}

	token := strings.TrimSpace(string(out))
	if token == "-" {
		return "", errors.New(errors.ZFSDatasetNoReceiveToken, "No resume token available") // No resume token available
	}

	return token, nil
}

// SendReceive handles data transfer on the same machine
func (m *Manager) SendReceive(ctx context.Context, sendCfg SendConfig, recvCfg ReceiveConfig) error {
	// Build send command
	sendPart := []string{command.BinZFS, "send"}

	if sendCfg.ResumeToken != "" {
		sendPart = append(sendPart, "-t", sendCfg.ResumeToken)
	}
	if sendCfg.Progress {
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

	sshPart := []string{}
	if recvCfg.RemoteConfig.Host != "" {
		sshPart = append(sshPart, "ssh")
		if recvCfg.RemoteConfig.Port != 0 {
			sshPart = append(sshPart, "-p", fmt.Sprintf("%d", recvCfg.RemoteConfig.Port))
		}
		if recvCfg.RemoteConfig.PrivateKey != "" {
			sshPart = append(sshPart, "-i", recvCfg.RemoteConfig.PrivateKey)
		}
		if recvCfg.RemoteConfig.Options != "" {
			sshPart = append(sshPart, recvCfg.RemoteConfig.Options)
		}
		if recvCfg.RemoteConfig.SkipHostKeyCheck {
			sshPart = append(sshPart, "-o StrictHostKeyChecking=no")
		}
		sshPart = append(sshPart, fmt.Sprintf("%s@%s", recvCfg.RemoteConfig.User, recvCfg.RemoteConfig.Host))
	}

	// Combine into single piped command
	fullCmd := fmt.Sprintf("sudo %s | %s sudo %s 2>&1",
		strings.Join(sendPart, " "),
		strings.Join(sshPart, " "),
		strings.Join(recvPart, " "))

	// Debug logging
	l, err := logger.NewTag(logger.Config{LogLevel: sendCfg.LogLevel}, "zfs-data-transfer")
	if err != nil {
		return errors.Wrap(err, errors.RodentMisc)
	}
	l.Debug("Executing command",
		"cmd", fullCmd)

	// Execute combined command
	cmd := exec.Command("bash", "-c", fullCmd)

	// Create pipes for both stdout and stderr
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output

	// Execute command
	err = cmd.Run()
	outputStr := output.String()

	// Log the output regardless of error
	if outputStr != "" {
		l.Debug("Command output", "output", outputStr)
	}

	// Handle errors with context
	if err != nil {
		var exitErr *exec.ExitError
		if stderrors.As(err, &exitErr) {
			return errors.Wrap(err, errors.ZFSDatasetReceive).
				WithMetadata("exit_code", fmt.Sprintf("%d", exitErr.ExitCode())).
				WithMetadata("output", outputStr).
				WithMetadata("command", fullCmd)
		}
		// Other types of errors
		return errors.Wrap(err, errors.ZFSDatasetReceive).
			WithMetadata("output", outputStr).
			WithMetadata("command", fullCmd)
	}

	return nil
}
