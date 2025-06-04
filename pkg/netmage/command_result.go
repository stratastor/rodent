// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package netmage

import (
	"context"
	"strconv"
	"strings"

	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/internal/common"
	"github.com/stratastor/rodent/pkg/errors"
)

// CommandResult represents the result of a command execution
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandExecutor wraps the internal command executor for network operations
type CommandExecutor struct {
	executor *command.CommandExecutor
}

// NewCommandExecutor creates a new command executor wrapper
func NewCommandExecutor(useSudo bool) *CommandExecutor {
	return &CommandExecutor{
		executor: command.NewCommandExecutor(useSudo),
	}
}

// ExecuteCommand executes a command and returns structured result
func (ce *CommandExecutor) ExecuteCommand(
	ctx context.Context,
	cmd string,
	args ...string,
) (*CommandResult, error) {
	common.Log.Debug("Executing command", "cmd", cmd, "args", args)
	output, err := ce.executor.ExecuteWithCombinedOutput(ctx, cmd, args...)

	result := &CommandResult{
		Stdout:   string(output),
		Stderr:   "",
		ExitCode: 0,
	}

	if err != nil {
		// Extract exit code if available from RodentError metadata
		if rodentErr, ok := err.(*errors.RodentError); ok && rodentErr.Metadata != nil {
			if exitCodeStr, exists := rodentErr.Metadata["exit_code"]; exists {
				if exitCode, parseErr := strconv.Atoi(exitCodeStr); parseErr == nil {
					result.ExitCode = exitCode
				} else {
					result.ExitCode = 1
				}
			} else {
				result.ExitCode = 1
			}
			if stderr, exists := rodentErr.Metadata["stderr"]; exists {
				result.Stderr = stderr
			} else {
				result.Stderr = err.Error()
			}
			result.Stdout = "" // Combined output is in stderr for errors
		} else {
			result.ExitCode = 1
			result.Stderr = err.Error()
			result.Stdout = string(output)
		}
		return result, err
	}

	// For successful commands, output goes to stdout
	result.Stdout = strings.TrimSpace(string(output))
	return result, nil
}
