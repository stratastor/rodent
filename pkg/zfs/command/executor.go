package command

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/stratastor/rodent/pkg/errors"
)

// CommandExecutor provides safe execution of ZFS commands
type CommandExecutor struct {
	mu           sync.RWMutex
	zfsVersion   string
	zpoolVersion string
	features     map[string]bool // Supported ZFS features

	useSudo bool          // Whether to use sudo for privileged commands
	timeout time.Duration // Default command timeout
}

// CommandFlags represents supported command flags
type CommandFlags uint8

const (
	FlagJSON      CommandFlags = 1 << iota // -j for JSON output
	FlagParsable                           // -p for parsable output
	FlagRecursive                          // -r for recursive operations
	FlagForce                              // -f to force operation
	FlagNoHeaders                          // -H to disable output headers
)

// CommandOptions configures command execution
type CommandOptions struct {
	Flags         CommandFlags  // Command flags to apply
	Timeout       time.Duration // Command-specific timeout
	CaptureOutput bool          // Whether to capture command output
}

func NewCommandExecutor(useSudo bool) *CommandExecutor {
	return &CommandExecutor{
		features: make(map[string]bool),
		useSudo:  useSudo,
	}
}

func (e *CommandExecutor) Execute(ctx context.Context, opts CommandOptions, cmd string, args ...string) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}

	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Build command with appropriate prefixes and flags
	cmdArgs := e.buildCommandArgs(cmd, opts, args...)

	// Debug logging
	fmt.Printf("Executing command: %s\n", strings.Join(cmdArgs, " "))

	// Create command
	execCmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)

	// Set up pipes for output
	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, errors.CommandPipe)
	}
	stderr, err := execCmd.StderrPipe()
	if err != nil {
		return nil, errors.Wrap(err, errors.CommandPipe)
	}

	// Start command execution
	if err := execCmd.Start(); err != nil {
		return nil, errors.NewCommandError(
			strings.Join(cmdArgs, " "),
			-1,
			fmt.Sprintf("failed to start command: %v", err),
		)
	}

	// Read output in goroutine
	var outData []byte
	var outErr error
	var stderrBuf bytes.Buffer
	done := make(chan struct{})

	go func() {
		defer close(done)
		data, err := io.ReadAll(stdout)
		if err != nil {
			outErr = errors.Wrap(err, errors.CommandOutputParse)
			return
		}
		outData = data
		// Capture stderr for error reporting
		stderrData, _ := io.ReadAll(stderr)
		stderrBuf.Write(stderrData)
	}()

	// Wait for either:
	// 1. Command completion
	// 2. Context cancellation
	// 3. Timeout
	select {
	case <-ctx.Done():
		// Kill process on timeout/cancellation
		if err := execCmd.Process.Kill(); err != nil {
			return nil, errors.Wrap(err, errors.CommandTimeout)
		}
		return nil, errors.New(errors.CommandTimeout, "command execution timed out")

	case <-done:
		if outErr != nil {
			return nil, outErr
		}

		// Wait for command completion and check exit status
		if err := execCmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return nil, errors.NewCommandError(
					strings.Join(cmdArgs, " "),
					exitErr.ExitCode(),
					stderrBuf.String(),
				)
			}
			return nil, errors.Wrap(err, errors.CommandExecution).
				WithMetadata("command", strings.Join(cmdArgs, " ")).
				WithMetadata("stderr", stderrBuf.String())
		}

		return outData, nil
	}
}

func (e *CommandExecutor) buildCommandArgs(cmd string, opts CommandOptions, args ...string) []string {
	var cmdArgs []string

	// Add sudo if required
	if e.useSudo && SudoRequiredCommands[cmd] {
		cmdArgs = append(cmdArgs, "sudo")
	}

	// Split command into base command and subcommand
	parts := strings.Fields(cmd) // This splits on whitespace better than SplitN

	// Add base command (zfs or zpool)
	switch {
	case strings.HasPrefix(parts[0], "zfs"):
		cmdArgs = append(cmdArgs, BinZFS)
	case strings.HasPrefix(parts[0], "zpool"):
		cmdArgs = append(cmdArgs, BinZpool)
	}

	// Add subcommand if present
	if len(parts) > 1 {
		cmdArgs = append(cmdArgs, parts[1])
	}

	// Add command flags based on options
	if opts.Flags&FlagJSON != 0 && JSONSupportedCommands[cmd] {
		cmdArgs = append(cmdArgs, "-j")
	}
	if opts.Flags&FlagParsable != 0 {
		cmdArgs = append(cmdArgs, "-p")
	}
	if opts.Flags&FlagRecursive != 0 {
		cmdArgs = append(cmdArgs, "-r")
	}
	if opts.Flags&FlagForce != 0 {
		cmdArgs = append(cmdArgs, "-f")
	}
	if opts.Flags&FlagNoHeaders != 0 {
		cmdArgs = append(cmdArgs, "-H")
	}

	// Add remaining arguments, but skip the operation if it's duplicated
	for _, arg := range args {
		if len(parts) > 1 && arg == parts[1] {
			continue // Skip if argument matches the operation
		}
		cmdArgs = append(cmdArgs, arg)
	}

	return cmdArgs
}

// validateCommand checks command and args for security
func (e *CommandExecutor) validateCommand(name string, args []string) error {
	// Only allow zfs/zpool commands
	if name != "zfs" && name != "zpool" {
		return errors.New(errors.CommandNotFound,
			"only zfs and zpool commands are allowed")
	}

	// Validate args don't contain dangerous characters
	for _, arg := range args {
		if strings.ContainsAny(arg, ";&|><$`\\") {
			return errors.New(errors.CommandInvalidInput,
				"argument contains invalid characters")
		}
	}

	return nil
}
