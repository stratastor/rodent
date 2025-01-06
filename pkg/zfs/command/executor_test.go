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
	"context"
	"testing"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/errors"
)

func TestCommandSecurity(t *testing.T) {
	executor := NewCommandExecutor(true, logger.Config{LogLevel: "debug"})

	tests := []struct {
		name    string
		cmd     string
		args    []string
		wantErr *errors.RodentError
	}{
		{
			name: "command_injection_semicolon",
			cmd:  "zfs; rm -rf /",
			wantErr: &errors.RodentError{
				Code:   errors.CommandNotFound,
				Domain: errors.DomainCommand,
			},
		},
		{
			name: "path_traversal",
			cmd:  "zfs",
			args: []string{"create", "../../../etc/passwd"},
			wantErr: &errors.RodentError{
				Code:   errors.CommandInvalidInput, // Changed from CommandExecution
				Domain: errors.DomainCommand,
			},
		},
		{
			name: "sudo_injection",
			cmd:  "sudo",
			args: []string{"-i", "bash"},
			wantErr: &errors.RodentError{
				Code:   errors.CommandNotFound,
				Domain: errors.DomainCommand,
			},
		},
		{
			name: "environment_injection",
			cmd:  "zfs",
			args: []string{"LD_PRELOAD=/tmp/evil.so"},
			wantErr: &errors.RodentError{
				Code:   errors.CommandExecution,
				Domain: errors.DomainCommand,
			},
		},
		{
			name: "too_many_args",
			cmd:  "zfs",
			args: make([]string, 100),
			wantErr: &errors.RodentError{
				Code:   errors.CommandInvalidInput,
				Domain: errors.DomainCommand,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executor.Execute(context.Background(), CommandOptions{}, tt.cmd, tt.args...)

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			re, ok := err.(*errors.RodentError)
			if !ok {
				t.Fatalf("expected RodentError, got %T", err)
			}

			// Only check code and domain
			if re.Code != tt.wantErr.Code || re.Domain != tt.wantErr.Domain {
				t.Errorf("Execute() error = [%s-%d], want [%s-%d]",
					re.Domain, re.Code, tt.wantErr.Domain, tt.wantErr.Code)
			}
		})
	}
}

func TestDeviceSecurity(t *testing.T) {
	executor := NewCommandExecutor(true, logger.Config{LogLevel: "debug"})

	tests := []struct {
		name    string
		cmd     string
		args    []string
		wantErr *errors.RodentError
	}{
		{
			name: "device_path_traversal",
			cmd:  "zpool",
			args: []string{"create", "pool", "../../../dev/sda"},
			wantErr: &errors.RodentError{
				Code:   errors.CommandInvalidInput, // Changed from CommandExecution
				Domain: errors.DomainCommand,
			},
		},
		{
			name: "restricted_device",
			cmd:  "zpool",
			args: []string{"create", "pool", "/dev/sda"},
			wantErr: &errors.RodentError{
				Code:   errors.CommandExecution,
				Domain: errors.DomainCommand,
			},
		},
		{
			name: "symbolic_link",
			cmd:  "zpool",
			args: []string{"create", "pool", "/dev/disk/by-id/evil"},
			wantErr: &errors.RodentError{
				Code:   errors.CommandExecution,
				Domain: errors.DomainCommand,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executor.Execute(context.Background(), CommandOptions{}, tt.cmd, tt.args...)

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			re, ok := err.(*errors.RodentError)
			if !ok {
				t.Fatalf("expected RodentError, got %T", err)
			}

			// Only check code and domain
			if re.Code != tt.wantErr.Code || re.Domain != tt.wantErr.Domain {
				t.Errorf("Execute() error = [%s-%d], want [%s-%d]",
					re.Domain, re.Code, tt.wantErr.Domain, tt.wantErr.Code)
			}
		})
	}
}
