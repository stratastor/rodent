/*
 * Copyright 2024-2025 Raamsri Kumar <raam@tinkershack.in> 
 * Copyright 2024-2025 The StrataSTOR Authors and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

	"github.com/stratastor/logger"
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

	logger logger.Logger
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
	Flags   CommandFlags  // Command flags to apply
	Timeout time.Duration // Command-specific timeout

	// TODO: Implement these Capture* options? Not actively used in the code; everything is captured.
	CaptureOutput bool // Whether to capture command output
	CaptureStderr bool // Capture stderr even on success
}

func NewCommandExecutor(useSudo bool, logConfig logger.Config) *CommandExecutor {
	l, err := logger.NewTag(logConfig, "zfs-cmd")
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}

	return &CommandExecutor{
		features: make(map[string]bool),
		useSudo:  useSudo,
		logger:   l,
	}
}

func (e *CommandExecutor) Execute(ctx context.Context, opts CommandOptions, cmd string, args ...string) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Split command to get base command (zfs/zpool)
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil, errors.New(errors.CommandNotFound, "empty command")
	}

	// Validate command and arguments
	if err := e.validateCommand(parts[0], args); err != nil {
		return nil, err
	}

	// Build command with security checks
	cmdArgs := e.buildCommandArgs(cmd, opts, args...)

	// Additional security checks for built command
	if err := e.validateBuiltCommand(cmdArgs); err != nil {
		return nil, err
	}

	// Set timeout
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Debug logging
	e.logger.Debug("Executing command", "cmd", strings.Join(cmdArgs, " "))

	// Create command
	execCmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)

	// Prevent shell expansion
	execCmd.Env = []string{}

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
	// TODO: Review -p flag as it means different things for different commands. Example: zfs create and zfs get.
	// 1. zfs create: -p is used to create parent datasets
	// 2. zfs get: -p is used to display property values in parsable format
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

// validateBuiltCommand performs additional security checks on the final command
func (e *CommandExecutor) validateBuiltCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(errors.CommandInvalidInput, "empty command")
	}

	// Ensure first argument is an absolute path to zfs/zpool
	switch args[0] {
	case "sudo":
		if len(args) < 2 {
			return errors.New(errors.CommandInvalidInput, "invalid sudo command")
		}
		if args[1] != BinZFS && args[1] != BinZpool {
			return errors.New(errors.CommandNotFound, "invalid command binary")
		}
	case BinZFS, BinZpool:
		// Direct command is okay
	default:
		return errors.New(errors.CommandNotFound, "invalid command binary")
	}

	// Validate argument count
	if len(args) > maxCommandArgs {
		return errors.New(errors.CommandInvalidInput, "too many arguments")
	}

	// Check for path traversal attempts
	for _, arg := range args {
		if strings.Contains(arg, "..") {
			return errors.New(errors.CommandInvalidInput, "path traversal not allowed")
		}
	}

	return nil
}
