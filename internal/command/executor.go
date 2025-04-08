// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/stratastor/logger"
	rterrors "github.com/stratastor/rodent/pkg/errors"
)

// Dangerous characters that could enable command injection
var dangerousChars = "&|><$`\\[];{}"

// Command execution timeout
const defaultCommandTimeout = 30 * time.Second

// ExecCommand executes a system command with proper security checks
func ExecCommand(
	ctx context.Context,
	logger logger.Logger,
	name string,
	args ...string,
) ([]byte, error) {
	// Validate command and arguments
	if err := validateCommand(name, args); err != nil {
		return nil, err
	}

	// Apply timeout if not already set
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, defaultCommandTimeout)
		defer cancel()
	}

	// Log the command being executed
	cmdString := name + " " + strings.Join(args, " ")
	logger.Debug("Executing command", "cmd", cmdString)

	// Create command with context for cancellation support
	cmd := exec.CommandContext(ctx, name, args...)

	// Prevent shell expansion
	cmd.Env = []string{}

	// Execute the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			logger.Error("Command execution failed with exit code",
				"cmd", cmdString,
				"exit_code", exitErr.ExitCode(),
				"output", string(output))

			return output, rterrors.Wrap(err, rterrors.CommandExecution).
				WithMetadata("command", cmdString).
				WithMetadata("exit_code", fmt.Sprintf("%d", exitErr.ExitCode())).
				WithMetadata("output", string(output))
		}

		logger.Error("Command execution failed",
			"cmd", cmdString,
			"err", err,
			"output", string(output))

		return output, fmt.Errorf("command execution failed: %w: %s", err, string(output))
	}

	return output, nil
}

// validateCommand performs security checks on the command and arguments
func validateCommand(name string, args []string) error {
	// Check for empty command
	if name == "" {
		return rterrors.New(rterrors.CommandInvalidInput, "empty command")
	}

	// Check for absolute path or valid command name
	if !strings.HasPrefix(name, "/") && strings.ContainsAny(name, "/\\") {
		return rterrors.New(
			rterrors.CommandInvalidInput,
			"relative paths are not allowed for commands",
		)
	}

	// Check for dangerous characters in command
	if strings.ContainsAny(name, dangerousChars) {
		return rterrors.New(rterrors.CommandInvalidInput, "command contains invalid characters")
	}

	// Validate args don't contain dangerous characters
	for _, arg := range args {
		if strings.ContainsAny(arg, dangerousChars) {
			return rterrors.New(
				rterrors.CommandInvalidInput,
				"argument contains invalid characters",
			)
		}

		// Check for path traversal attempts
		if strings.Contains(arg, "..") {
			return rterrors.New(rterrors.CommandInvalidInput, "path traversal not allowed")
		}
	}

	// Limit arguments count
	if len(args) > 64 {
		return rterrors.New(rterrors.CommandInvalidInput, "too many arguments")
	}

	return nil
}

// CommandExecutor provides a general-purpose command execution service
type CommandExecutor struct {
	UseSudo bool
	Timeout time.Duration
	WorkDir string
	Env     []string
}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor(useSudo bool) *CommandExecutor {
	return &CommandExecutor{
		UseSudo: useSudo,
		Timeout: 30 * time.Second,
	}
}

// Execute runs a command and returns its output
func (e *CommandExecutor) Execute(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	// Apply timeout if not already set in context
	if _, ok := ctx.Deadline(); !ok && e.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.Timeout)
		defer cancel()
	}

	// Prepend sudo if needed
	cmdArgs := make([]string, 0, len(args)+1)
	if e.UseSudo {
		cmdArgs = append(cmdArgs, "sudo", cmd)
	} else {
		cmdArgs = append(cmdArgs, cmd)
	}
	cmdArgs = append(cmdArgs, args...)

	// Create command
	execCmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	execCmd.Env = append(execCmd.Env, e.Env...)
	if e.WorkDir != "" {
		execCmd.Dir = e.WorkDir
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	// Execute command
	err := execCmd.Run()
	if err != nil {
		return stderr.Bytes(), fmt.Errorf("command failed: %w: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// ExecuteWithCombinedOutput runs a command and returns combined stdout/stderr
func (e *CommandExecutor) ExecuteWithCombinedOutput(
	ctx context.Context,
	cmd string,
	args ...string,
) ([]byte, error) {
	// Apply timeout if not already set in context
	if _, ok := ctx.Deadline(); !ok && e.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.Timeout)
		defer cancel()
	}

	// Prepend sudo if needed
	cmdArgs := make([]string, 0, len(args)+1)
	if e.UseSudo {
		cmdArgs = append(cmdArgs, "sudo", cmd)
	} else {
		cmdArgs = append(cmdArgs, cmd)
	}
	cmdArgs = append(cmdArgs, args...)

	// Create command
	execCmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	execCmd.Env = append(execCmd.Env, e.Env...)
	if e.WorkDir != "" {
		execCmd.Dir = e.WorkDir
	}

	// Capture combined output
	var combinedOutput bytes.Buffer
	execCmd.Stdout = &combinedOutput
	execCmd.Stderr = &combinedOutput

	// Execute command
	err := execCmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return combinedOutput.Bytes(), rterrors.NewCommandError(
				cmd+" "+strings.Join(args, " "),
				exitErr.ExitCode(),
				combinedOutput.String(),
			)
		}
		return combinedOutput.Bytes(), fmt.Errorf(
			"command failed: %w: %s",
			err,
			combinedOutput.String(),
		)
	}

	return combinedOutput.Bytes(), nil
}
