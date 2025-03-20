// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/stratastor/logger"
)

// ConfigTemplate represents a configuration template that can be rendered
type ConfigTemplate struct {
	Name         string
	TemplatePath string // File path (optional if Content provided)
	Content      string // Template content (used instead of TemplatePath)
	OutputPath   string
	Permissions  os.FileMode
	BackupPath   string // Optional path for backup
}

// ServiceConfigManager handles configuration updates for services
type ServiceConfigManager struct {
	logger         logger.Logger
	templates      map[string]*ConfigTemplate
	stateCallbacks []StateChangeCallback
}

// StateChangeCallback is called when a configuration change occurs
type StateChangeCallback func(ctx context.Context, serviceName string, state ServiceState) error

// ServiceState represents the state of a service configuration
type ServiceState struct {
	ServiceName string
	ConfigPath  string
	UpdatedAt   time.Time
	Status      string // e.g. "updated", "failed", "unchanged"
}

// NewServiceConfigManager creates a new service configuration manager
func NewServiceConfigManager(logger logger.Logger) *ServiceConfigManager {
	return &ServiceConfigManager{
		logger:    logger,
		templates: make(map[string]*ConfigTemplate),
	}
}

// RegisterTemplate registers a configuration template
func (m *ServiceConfigManager) RegisterTemplate(name string, template *ConfigTemplate) {
	m.templates[name] = template
}

// RegisterStateCallback registers a callback for state changes
func (m *ServiceConfigManager) RegisterStateCallback(callback StateChangeCallback) {
	m.stateCallbacks = append(m.stateCallbacks, callback)
}

// UpdateConfig updates a configuration file based on a template and data
func (m *ServiceConfigManager) UpdateConfig(
	ctx context.Context,
	templateName string,
	data interface{},
) error {
	tmpl, ok := m.templates[templateName]
	if !ok {
		return fmt.Errorf("template not found: %s", templateName)
	}

	// Create backup if path is specified
	if tmpl.BackupPath != "" {
		if err := m.createBackup(tmpl.OutputPath, tmpl.BackupPath); err != nil {
			m.logger.Warn("Failed to create backup", "template", templateName, "err", err)
			// Continue despite backup failure
		}
	}

	var templateContent string
	var err error

	// Use embedded content if available, otherwise read from file
	if tmpl.Content != "" {
		templateContent = tmpl.Content
	} else {
		// Load template content from file
		content, err := os.ReadFile(tmpl.TemplatePath)
		if err != nil {
			return fmt.Errorf("failed to read template file: %w", err)
		}
		templateContent = string(content)
	}

	// Parse template
	parsedTemplate, err := template.New(tmpl.Name).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Render template with data
	var output bytes.Buffer
	if err := parsedTemplate.Execute(&output, data); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(tmpl.OutputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write output to file
	if err := os.WriteFile(tmpl.OutputPath, output.Bytes(), tmpl.Permissions); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	// Notify callbacks
	state := ServiceState{
		ServiceName: tmpl.Name,
		ConfigPath:  tmpl.OutputPath,
		UpdatedAt:   time.Now(),
		Status:      "updated",
	}

	for _, callback := range m.stateCallbacks {
		if err := callback(ctx, tmpl.Name, state); err != nil {
			m.logger.Warn("Failed to notify state change", "template", templateName, "err", err)
		}
	}

	m.logger.Info("Updated configuration", "template", templateName, "path", tmpl.OutputPath)
	return nil
}

// createBackup creates a backup of the specified file
func (m *ServiceConfigManager) createBackup(sourcePath, backupPath string) error {
	// Check if source file exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		// No backup needed if source doesn't exist
		return nil
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Read source file
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Write to backup file
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}
