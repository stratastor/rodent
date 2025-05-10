// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package privilege

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
	"github.com/stratastor/rodent/pkg/errors"
)

// SudoFileOperations implements FileOperations using sudo
type SudoFileOperations struct {
	logger        logger.Logger
	executor      *command.CommandExecutor
	allowedPaths  []string // Paths that are allowed to be accessed
	allowedRegexp []*regexp.Regexp // Regexp patterns for allowed paths
}

// NewSudoFileOperations creates a new SudoFileOperations instance
func NewSudoFileOperations(
	logger logger.Logger,
	executor *command.CommandExecutor,
	allowedPaths []string,
) *SudoFileOperations {
	// Compile regexp patterns for path validation
	allowedRegexp := make([]*regexp.Regexp, 0, len(allowedPaths))
	for _, path := range allowedPaths {
		re := regexp.MustCompile("^" + regexp.QuoteMeta(path) + "($|/.*)")
		allowedRegexp = append(allowedRegexp, re)
	}
	
	return &SudoFileOperations{
		logger:        logger,
		executor:      executor,
		allowedPaths:  allowedPaths,
		allowedRegexp: allowedRegexp,
	}
}

// isPathAllowed checks if a path is allowed to be accessed with sudo
func (s *SudoFileOperations) isPathAllowed(path string) bool {
	// Always check with absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	
	for _, re := range s.allowedRegexp {
		if re.MatchString(absPath) {
			return true
		}
	}
	return false
}

// ReadFile implements FileOperations.ReadFile
func (s *SudoFileOperations) ReadFile(ctx context.Context, path string) ([]byte, error) {
	// Validate path
	if !s.isPathAllowed(path) {
		return nil, errors.New(errors.PermissionDenied, "Path not allowed for privileged access").
			WithMetadata("path", path)
	}
	
	// Use cat with sudo to read the file
	cmd := exec.CommandContext(ctx, "sudo", "cat", path)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, errors.Wrap(err, errors.OperationFailed).
				WithMetadata("operation", "read_file").
				WithMetadata("path", path).
				WithMetadata("stderr", string(exitErr.Stderr))
		}
		return nil, errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "read_file").
			WithMetadata("path", path)
	}
	
	return output, nil
}

// WriteFile implements FileOperations.WriteFile
func (s *SudoFileOperations) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	// Validate path
	if !s.isPathAllowed(path) {
		return errors.New(errors.PermissionDenied, "Path not allowed for privileged access").
			WithMetadata("path", path)
	}
	
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "rodent-sudo-*")
	if err != nil {
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "create_temp_file")
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	
	// Write data to temporary file
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "write_temp_file")
	}
	tmpFile.Close()
	
	// Use sudo with cp to write to the destination
	cmd := exec.CommandContext(ctx, "sudo", "cp", tmpPath, path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "write_file").
			WithMetadata("path", path).
			WithMetadata("output", string(output))
	}
	
	// Set permissions if specified
	if perm != 0 {
		permStr := fmt.Sprintf("%o", perm)
		chmodCmd := exec.CommandContext(ctx, "sudo", "chmod", permStr, path)
		if output, err := chmodCmd.CombinedOutput(); err != nil {
			return errors.Wrap(err, errors.OperationFailed).
				WithMetadata("operation", "chmod").
				WithMetadata("path", path).
				WithMetadata("permissions", permStr).
				WithMetadata("output", string(output))
		}
	}
	
	return nil
}

// AppendFile implements FileOperations.AppendFile
func (s *SudoFileOperations) AppendFile(ctx context.Context, path string, data []byte) error {
	// Validate path
	if !s.isPathAllowed(path) {
		return errors.New(errors.PermissionDenied, "Path not allowed for privileged access").
			WithMetadata("path", path)
	}
	
	// Create temporary file with the data to append
	tmpFile, err := os.CreateTemp("", "rodent-sudo-append-*")
	if err != nil {
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "create_temp_file")
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	
	// Write append data to temporary file
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "write_temp_file")
	}
	tmpFile.Close()
	
	// Use tee with append flag to append to the destination via sudo
	cmd := exec.CommandContext(ctx, "sudo", "tee", "-a", path)
	cmd.Stdin, err = os.Open(tmpPath)
	if err != nil {
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "open_temp_file").
			WithMetadata("path", tmpPath)
	}
	
	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "append_file").
			WithMetadata("path", path).
			WithMetadata("output", string(output))
	}
	
	return nil
}

// DeleteFile implements FileOperations.DeleteFile
func (s *SudoFileOperations) DeleteFile(ctx context.Context, path string) error {
	// Validate path
	if !s.isPathAllowed(path) {
		return errors.New(errors.PermissionDenied, "Path not allowed for privileged access").
			WithMetadata("path", path)
	}
	
	// Use sudo with rm to delete the file
	cmd := exec.CommandContext(ctx, "sudo", "rm", "-f", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "delete_file").
			WithMetadata("path", path).
			WithMetadata("output", string(output))
	}
	
	return nil
}

// CopyFile implements FileOperations.CopyFile
func (s *SudoFileOperations) CopyFile(ctx context.Context, src, dst string) error {
	// Validate destination path (source could be a regular file)
	if !s.isPathAllowed(dst) {
		return errors.New(errors.PermissionDenied, "Destination path not allowed for privileged access").
			WithMetadata("path", dst)
	}
	
	// Use sudo with cp to copy the file
	cmd := exec.CommandContext(ctx, "sudo", "cp", src, dst)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "copy_file").
			WithMetadata("src", src).
			WithMetadata("dst", dst).
			WithMetadata("output", string(output))
	}
	
	return nil
}

// Exists implements FileOperations.Exists
func (s *SudoFileOperations) Exists(ctx context.Context, path string) (bool, error) {
	// Validate path
	if !s.isPathAllowed(path) {
		return false, errors.New(errors.PermissionDenied, "Path not allowed for privileged access").
			WithMetadata("path", path)
	}
	
	// Use sudo with test to check if file exists
	cmd := exec.CommandContext(ctx, "sudo", "test", "-e", path)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// File does not exist (exit code 1)
			return false, nil
		}
		// Some other error occurred
		return false, errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "check_exists").
			WithMetadata("path", path)
	}
	
	return true, nil
}

// ExecuteCommand implements FileOperations.ExecuteCommand
func (s *SudoFileOperations) ExecuteCommand(ctx context.Context, command string, args ...string) ([]byte, error) {
	// Prepend sudo to the command
	sudoArgs := append([]string{command}, args...)
	cmd := exec.CommandContext(ctx, "sudo", sudoArgs...)
	
	// Execute the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, errors.Wrap(err, errors.OperationFailed).
			WithMetadata("operation", "execute_command").
			WithMetadata("command", command).
			WithMetadata("args", strings.Join(args, " ")).
			WithMetadata("output", string(output))
	}
	
	return output, nil
}