// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package privilege

import (
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/command"
)

// OperationsFactory creates FileOperations instances
type OperationsFactory struct {
	logger   logger.Logger
	executor *command.CommandExecutor
	config   *Config
}

// NewOperationsFactory creates a new OperationsFactory
func NewOperationsFactory(
	logger logger.Logger,
	executor *command.CommandExecutor,
	config *Config,
) *OperationsFactory {
	return &OperationsFactory{
		logger:   logger, 
		executor: executor,
		config:   config,
	}
}

// Create returns a FileOperations instance
func (f *OperationsFactory) Create() FileOperations {
	return NewSudoFileOperations(f.logger, f.executor, f.config.AllowedPaths)
}