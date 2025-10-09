// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package types

import "time"

// ProbeExecution represents a SMART probe execution (self-test)
type ProbeExecution struct {
	ID         string      `json:"id"`          // Unique execution ID
	DeviceID   string      `json:"device_id"`   // Target device ID
	DevicePath string      `json:"device_path"` // Device path at execution time
	Type       ProbeType   `json:"type"`        // Quick or Extensive
	Status     ProbeStatus `json:"status"`      // Execution status
	Result     ProbeResult `json:"result"`      // Test result (if completed)

	// Scheduling
	ScheduledAt  time.Time  `json:"scheduled_at"`            // When scheduled
	StartedAt    *time.Time `json:"started_at,omitempty"`    // When started
	CompletedAt  *time.Time `json:"completed_at,omitempty"`  // When completed
	Duration     int64      `json:"duration"`                // Duration in seconds (if completed)

	// Progress
	PercentComplete int    `json:"percent_complete"` // Progress percentage
	EstimatedTime   int    `json:"estimated_time"`   // Estimated time remaining (seconds)

	// Results
	ErrorMessage  string            `json:"error_message,omitempty"`   // Error message if failed
	OutputSummary string            `json:"output_summary,omitempty"`  // Summary of test output
	Metadata      map[string]string `json:"metadata,omitempty"`        // Additional metadata

	// Conflict handling
	ConflictDetected bool   `json:"conflict_detected"`           // Whether conflict was detected
	ConflictReason   string `json:"conflict_reason,omitempty"`   // Reason for conflict
	RetryCount       int    `json:"retry_count"`                 // Number of retry attempts
	NextRetryAt      *time.Time `json:"next_retry_at,omitempty"` // When to retry (if scheduled)
}

// ProbeSchedule represents a scheduled probe configuration
type ProbeSchedule struct {
	ID              string    `json:"id"`               // Schedule ID
	Enabled         bool      `json:"enabled"`          // Whether schedule is enabled
	Type            ProbeType `json:"type"`             // Quick or Extensive
	CronExpression  string    `json:"cron_expression"`  // Cron schedule expression
	DeviceFilter    *DiskFilter `json:"device_filter,omitempty"` // Filter for target devices
	MaxConcurrent   int       `json:"max_concurrent"`   // Max concurrent probes
	Timeout         int       `json:"timeout"`          // Timeout in seconds
	RetryOnConflict bool      `json:"retry_on_conflict"` // Whether to retry on conflict
	RetryDelay      int       `json:"retry_delay"`      // Retry delay in seconds
	MaxRetries      int       `json:"max_retries"`      // Max retry attempts
	CreatedAt       time.Time `json:"created_at"`       // When schedule was created
	UpdatedAt       time.Time `json:"updated_at"`       // Last update
	Metadata        map[string]string `json:"metadata,omitempty"` // Additional metadata
}

// ProbeHistory represents historical probe execution data
type ProbeHistory struct {
	DeviceID   string            `json:"device_id"`   // Device ID
	Executions []*ProbeExecution `json:"executions"`  // Historical executions
	UpdatedAt  time.Time         `json:"updated_at"`  // Last update
}

// ConflictCheck represents a conflict check result
type ConflictCheck struct {
	HasConflict bool     `json:"has_conflict"` // Whether conflict exists
	Reasons     []string `json:"reasons"`      // Conflict reasons
	CheckedAt   time.Time `json:"checked_at"`  // When checked
}

// ConflictType represents types of conflicts that prevent probe execution
type ConflictType string

const (
	ConflictZPoolScrub   ConflictType = "zpool_scrub"   // ZFS scrub in progress
	ConflictZPoolResilver ConflictType = "zpool_resilver" // ZFS resilver in progress
	ConflictProbeRunning ConflictType = "probe_running" // Another probe already running
	ConflictHighIO       ConflictType = "high_io"       // High I/O activity
	ConflictDiskBusy     ConflictType = "disk_busy"     // Disk busy with other operations
)

// NewProbeExecution creates a new probe execution
func NewProbeExecution(deviceID, devicePath string, probeType ProbeType) *ProbeExecution {
	return &ProbeExecution{
		ID:              GenerateProbeID(),
		DeviceID:        deviceID,
		DevicePath:      devicePath,
		Type:            probeType,
		Status:          ProbeStatusScheduled,
		ScheduledAt:     time.Now(),
		PercentComplete: 0,
		Metadata:        make(map[string]string),
	}
}

// NewProbeSchedule creates a new probe schedule
func NewProbeSchedule(probeType ProbeType, cronExpr string) *ProbeSchedule {
	now := time.Now()
	return &ProbeSchedule{
		ID:              GenerateScheduleID(),
		Enabled:         true,
		Type:            probeType,
		CronExpression:  cronExpr,
		MaxConcurrent:   DefaultMaxConcurrentProbes,
		Timeout:         int(DefaultProbeTimeout.Seconds()),
		RetryOnConflict: true,
		RetryDelay:      int(DefaultConflictRetryDelay.Seconds()),
		MaxRetries:      DefaultMaxConflictRetries,
		CreatedAt:       now,
		UpdatedAt:       now,
		Metadata:        make(map[string]string),
	}
}

// Start marks the probe as started
func (p *ProbeExecution) Start() {
	now := time.Now()
	p.StartedAt = &now
	p.Status = ProbeStatusRunning
}

// Complete marks the probe as completed
func (p *ProbeExecution) Complete(result ProbeResult, summary string) {
	now := time.Now()
	p.CompletedAt = &now
	p.Status = ProbeStatusCompleted
	p.Result = result
	p.OutputSummary = summary
	p.PercentComplete = 100
	if p.StartedAt != nil {
		p.Duration = int64(now.Sub(*p.StartedAt).Seconds())
	}
}

// Fail marks the probe as failed
func (p *ProbeExecution) Fail(errorMsg string) {
	now := time.Now()
	p.CompletedAt = &now
	p.Status = ProbeStatusFailed
	p.ErrorMessage = errorMsg
	if p.StartedAt != nil {
		p.Duration = int64(now.Sub(*p.StartedAt).Seconds())
	}
}

// Cancel marks the probe as cancelled
func (p *ProbeExecution) Cancel() {
	now := time.Now()
	p.CompletedAt = &now
	p.Status = ProbeStatusCancelled
	if p.StartedAt != nil {
		p.Duration = int64(now.Sub(*p.StartedAt).Seconds())
	}
}

// Timeout marks the probe as timed out
func (p *ProbeExecution) Timeout() {
	now := time.Now()
	p.CompletedAt = &now
	p.Status = ProbeStatusTimeout
	p.ErrorMessage = "Probe execution timed out"
	if p.StartedAt != nil {
		p.Duration = int64(now.Sub(*p.StartedAt).Seconds())
	}
}

// MarkConflict marks the probe as conflicted
func (p *ProbeExecution) MarkConflict(reason string) {
	now := time.Now()
	p.Status = ProbeStatusConflicted
	p.ConflictDetected = true
	p.ConflictReason = reason
	p.CompletedAt = &now
}

// ScheduleRetry schedules a retry attempt
func (p *ProbeExecution) ScheduleRetry(retryDelay time.Duration) {
	p.RetryCount++
	nextRetry := time.Now().Add(retryDelay)
	p.NextRetryAt = &nextRetry
	p.Status = ProbeStatusScheduled
}

// UpdateProgress updates probe progress
func (p *ProbeExecution) UpdateProgress(percent int, estimatedSeconds int) {
	p.PercentComplete = percent
	p.EstimatedTime = estimatedSeconds
}

// IsRunning returns true if probe is currently running
func (p *ProbeExecution) IsRunning() bool {
	return p.Status == ProbeStatusRunning
}

// IsCompleted returns true if probe has completed (success or failure)
func (p *ProbeExecution) IsCompleted() bool {
	return p.Status == ProbeStatusCompleted || p.Status == ProbeStatusFailed ||
		p.Status == ProbeStatusCancelled || p.Status == ProbeStatusTimeout
}

// CanRetry returns true if probe can be retried
func (p *ProbeExecution) CanRetry(maxRetries int) bool {
	return p.Status == ProbeStatusConflicted && p.RetryCount < maxRetries
}

// GetEstimatedDuration returns estimated duration based on probe type and device type
func (p *ProbeExecution) GetEstimatedDuration(deviceType DeviceType) time.Duration {
	switch p.Type {
	case ProbeTypeQuick:
		// Quick tests typically take 1-2 minutes
		return 2 * time.Minute
	case ProbeTypeExtensive:
		// Extensive tests vary by device type and size
		switch deviceType {
		case DeviceTypeSSD, DeviceTypeNVMe:
			// SSDs: 10-30 minutes
			return 30 * time.Minute
		case DeviceTypeHDD:
			// HDDs: 1-2 hours per TB (assume 4TB average)
			return 4 * time.Hour
		default:
			// Default to 1 hour
			return 1 * time.Hour
		}
	}
	return DefaultProbeTimeout
}

// ProbeStatistics represents aggregate probe statistics
type ProbeStatistics struct {
	DeviceID         string    `json:"device_id,omitempty"` // Device ID (empty for global stats)
	TotalExecutions  int       `json:"total_executions"`
	CompletedCount   int       `json:"completed_count"`
	FailedCount      int       `json:"failed_count"`
	CancelledCount   int       `json:"cancelled_count"`
	ConflictedCount  int       `json:"conflicted_count"`
	TimeoutCount     int       `json:"timeout_count"`
	LastProbeAt      *time.Time `json:"last_probe_at,omitempty"`
	LastProbeStatus  ProbeStatus `json:"last_probe_status,omitempty"`
	LastProbeResult  ProbeResult `json:"last_probe_result,omitempty"`
	AverageDuration  float64   `json:"average_duration"` // Average duration in seconds
	QuickProbeCount  int       `json:"quick_probe_count"`
	ExtensiveProbeCount int    `json:"extensive_probe_count"`
	CalculatedAt     time.Time `json:"calculated_at"`
}

// Helper functions for ID generation (simplified - real implementation should use UUID)
func GenerateProbeID() string {
	return "probe-" + time.Now().Format("20060102-150405")
}

func GenerateScheduleID() string {
	return "schedule-" + time.Now().Format("20060102-150405")
}
