// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
)

// DiskManagerConfig represents the complete disk manager configuration
type DiskManagerConfig struct {
	// Discovery configuration
	Discovery DiscoveryConfig `yaml:"discovery" json:"discovery"`

	// SMART monitoring configuration
	Monitoring MonitoringConfig `yaml:"monitoring" json:"monitoring"`

	// Probe scheduling configuration
	Probing ProbingConfig `yaml:"probing" json:"probing"`

	// Naming strategy configuration
	Naming NamingConfig `yaml:"naming" json:"naming"`

	// Topology discovery configuration
	Topology TopologyConfig `yaml:"topology" json:"topology"`

	// Tool paths configuration
	Tools ToolsConfig `yaml:"tools" json:"tools"`

	// Performance tuning
	Performance PerformanceConfig `yaml:"performance" json:"performance"`

	// Event configuration
	Events EventConfig `yaml:"events" json:"events"`
}

// DiscoveryConfig configures disk discovery behavior
type DiscoveryConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	Interval          time.Duration `yaml:"interval" json:"interval"`                     // Discovery interval
	Timeout           time.Duration `yaml:"timeout" json:"timeout"`                       // Discovery timeout
	ReconcileInterval time.Duration `yaml:"reconcile_interval" json:"reconcile_interval"` // Reconciliation interval
	UdevMonitor       bool          `yaml:"udev_monitor" json:"udev_monitor"`             // Enable udev monitoring
	AutoValidate      bool          `yaml:"auto_validate" json:"auto_validate"`           // Auto-validate new disks
}

// MonitoringConfig configures SMART monitoring
type MonitoringConfig struct {
	Enabled          bool               `yaml:"enabled" json:"enabled"`
	Interval         time.Duration      `yaml:"interval" json:"interval"`                   // Health check interval
	Thresholds       *SMARTThresholds   `yaml:"thresholds" json:"thresholds"`               // SMART thresholds
	MetricRetention  time.Duration      `yaml:"metric_retention" json:"metric_retention"`   // Metric retention period
	AlertOnWarning   bool               `yaml:"alert_on_warning" json:"alert_on_warning"`   // Send alerts on warnings
	AlertOnCritical  bool               `yaml:"alert_on_critical" json:"alert_on_critical"` // Send alerts on critical
}

// ProbingConfig configures SMART probe scheduling
type ProbingConfig struct {
	Enabled                bool          `yaml:"enabled" json:"enabled"`
	QuickProbeSchedule     string        `yaml:"quick_probe_schedule" json:"quick_probe_schedule"`         // Cron expression
	ExtensiveProbeSchedule string        `yaml:"extensive_probe_schedule" json:"extensive_probe_schedule"` // Cron expression
	MaxConcurrent          int           `yaml:"max_concurrent" json:"max_concurrent"`                     // Max concurrent probes
	Timeout                time.Duration `yaml:"timeout" json:"timeout"`                                   // Probe timeout
	RetentionPeriod        time.Duration `yaml:"retention_period" json:"retention_period"`                 // History retention
	ConflictCheck          bool          `yaml:"conflict_check" json:"conflict_check"`                     // Enable conflict detection
	RetryOnConflict        bool          `yaml:"retry_on_conflict" json:"retry_on_conflict"`               // Retry on conflict
	RetryDelay             time.Duration `yaml:"retry_delay" json:"retry_delay"`                           // Retry delay
	MaxRetries             int           `yaml:"max_retries" json:"max_retries"`                           // Max retry attempts
}

// NamingConfig configures device naming strategy
type NamingConfig struct {
	Strategy         NamingStrategy `yaml:"strategy" json:"strategy"`                     // Naming strategy
	AutoSelect       bool           `yaml:"auto_select" json:"auto_select"`               // Auto-select based on disk count
	VdevConfPath     string         `yaml:"vdev_conf_path" json:"vdev_conf_path"`         // Path to vdev_id.conf
	VdevConfTemplate string         `yaml:"vdev_conf_template" json:"vdev_conf_template"` // Template for vdev_id.conf
	SlotMapping      map[string]string `yaml:"slot_mapping" json:"slot_mapping"`          // Manual slot mappings
}

// TopologyConfig configures topology discovery
type TopologyConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	DiscoverSES     bool          `yaml:"discover_ses" json:"discover_ses"`         // Discover SES elements
	DiscoverNVMe    bool          `yaml:"discover_nvme" json:"discover_nvme"`       // Discover NVMe topology
	RefreshInterval time.Duration `yaml:"refresh_interval" json:"refresh_interval"` // Topology refresh interval
	PowerDomains    map[string][]string `yaml:"power_domains" json:"power_domains"` // User-defined power domains
}

// ToolsConfig configures tool paths and versions
type ToolsConfig struct {
	SmartctlPath  string            `yaml:"smartctl_path" json:"smartctl_path"`
	LsblkPath     string            `yaml:"lsblk_path" json:"lsblk_path"`
	LsscsiPath    string            `yaml:"lsscsi_path" json:"lsscsi_path"`
	UdevadmPath   string            `yaml:"udevadm_path" json:"udevadm_path"`
	SgSesPath     string            `yaml:"sg_ses_path" json:"sg_ses_path"`
	ZpoolPath     string            `yaml:"zpool_path" json:"zpool_path"`
	CheckVersions bool              `yaml:"check_versions" json:"check_versions"` // Check tool versions on startup
	RequiredTools []string          `yaml:"required_tools" json:"required_tools"` // Required tools (fail if missing)
	OptionalTools []string          `yaml:"optional_tools" json:"optional_tools"` // Optional tools (warn if missing)
	Metadata      map[string]string `yaml:"metadata" json:"metadata"`             // Tool metadata
}

// PerformanceConfig configures performance tuning
type PerformanceConfig struct {
	CacheSize    int           `yaml:"cache_size" json:"cache_size"`       // Disk inventory cache size
	CacheTTL     time.Duration `yaml:"cache_ttl" json:"cache_ttl"`         // Cache entry TTL
	WorkerCount  int           `yaml:"worker_count" json:"worker_count"`   // Worker goroutines
	BatchSize    int           `yaml:"batch_size" json:"batch_size"`       // Batch operation size
	RateLimitRPS int           `yaml:"rate_limit_rps" json:"rate_limit_rps"` // Rate limit (ops/sec)
}

// EventConfig configures event emission
type EventConfig struct {
	EmitDiscovery    bool `yaml:"emit_discovery" json:"emit_discovery"`       // Emit discovery events
	EmitHealthChange bool `yaml:"emit_health_change" json:"emit_health_change"` // Emit health change events
	EmitProbeStart   bool `yaml:"emit_probe_start" json:"emit_probe_start"`   // Emit probe start events
	EmitProbeComplete bool `yaml:"emit_probe_complete" json:"emit_probe_complete"` // Emit probe completion events
	EmitTopology     bool `yaml:"emit_topology" json:"emit_topology"`         // Emit topology events
	EmitErrors       bool `yaml:"emit_errors" json:"emit_errors"`             // Emit error events
}

// DefaultDiskManagerConfig returns the default configuration
func DefaultDiskManagerConfig() *DiskManagerConfig {
	return &DiskManagerConfig{
		Discovery: DiscoveryConfig{
			Enabled:           true,
			Interval:          DefaultDiscoveryInterval,
			Timeout:           DefaultDiscoveryTimeout,
			ReconcileInterval: DefaultReconcileInterval,
			UdevMonitor:       DefaultUdevMonitorEnabled,
			AutoValidate:      true,
		},
		Monitoring: MonitoringConfig{
			Enabled:         true,
			Interval:        DefaultHealthCheckInterval,
			Thresholds:      DefaultSMARTThresholds(),
			MetricRetention: DefaultMetricRetention,
			AlertOnWarning:  true,
			AlertOnCritical: true,
		},
		Probing: ProbingConfig{
			Enabled:                true,
			QuickProbeSchedule:     "0 2 * * 0",      // Weekly on Sunday at 2 AM
			ExtensiveProbeSchedule: "0 3 1 */3 *",    // Quarterly on 1st at 3 AM
			MaxConcurrent:          DefaultMaxConcurrentProbes,
			Timeout:                DefaultProbeTimeout,
			RetentionPeriod:        DefaultProbeRetention,
			ConflictCheck:          true,
			RetryOnConflict:        true,
			RetryDelay:             DefaultConflictRetryDelay,
			MaxRetries:             DefaultMaxConflictRetries,
		},
		Naming: NamingConfig{
			Strategy:     NamingByID,
			AutoSelect:   true,
			VdevConfPath: "/etc/zfs/vdev_id.conf",
		},
		Topology: TopologyConfig{
			Enabled:         true,
			DiscoverSES:     true,
			DiscoverNVMe:    true,
			RefreshInterval: 15 * time.Minute,
			PowerDomains:    make(map[string][]string),
		},
		Tools: ToolsConfig{
			SmartctlPath:  DefaultSmartctlPath,
			LsblkPath:     DefaultLsblkPath,
			LsscsiPath:    DefaultLsscsiPath,
			UdevadmPath:   DefaultUdevadmPath,
			SgSesPath:     DefaultSgSesPath,
			ZpoolPath:     DefaultZpoolPath,
			CheckVersions: true,
			RequiredTools: []string{"smartctl", "lsblk"},
			OptionalTools: []string{"lsscsi", "sg_ses", "zpool"},
			Metadata:      make(map[string]string),
		},
		Performance: PerformanceConfig{
			CacheSize:    DefaultCacheSize,
			CacheTTL:     DefaultCacheTTL,
			WorkerCount:  DefaultWorkerCount,
			BatchSize:    100,
			RateLimitRPS: 100,
		},
		Events: EventConfig{
			EmitDiscovery:     true,
			EmitHealthChange:  true,
			EmitProbeStart:    true,
			EmitProbeComplete: true,
			EmitTopology:      true,
			EmitErrors:        true,
		},
	}
}

// Validate validates the configuration
func (c *DiskManagerConfig) Validate() error {
	// Validate discovery config
	if c.Discovery.Enabled {
		if c.Discovery.Interval <= 0 {
			return ErrInvalidConfig("discovery interval must be positive")
		}
		if c.Discovery.Timeout <= 0 {
			return ErrInvalidConfig("discovery timeout must be positive")
		}
	}

	// Validate monitoring config
	if c.Monitoring.Enabled {
		if c.Monitoring.Interval <= 0 {
			return ErrInvalidConfig("monitoring interval must be positive")
		}
	}

	// Validate probing config
	if c.Probing.Enabled {
		if c.Probing.MaxConcurrent <= 0 {
			return ErrInvalidConfig("max concurrent probes must be positive")
		}
		if c.Probing.Timeout <= 0 {
			return ErrInvalidConfig("probe timeout must be positive")
		}

		// Validate cron expressions by attempting to create jobs with them
		if c.Probing.QuickProbeSchedule != "" {
			if err := validateCronExpression(c.Probing.QuickProbeSchedule); err != nil {
				return ErrInvalidConfig(fmt.Sprintf("invalid quick probe cron expression %q: %v", c.Probing.QuickProbeSchedule, err))
			}
		}
		if c.Probing.ExtensiveProbeSchedule != "" {
			if err := validateCronExpression(c.Probing.ExtensiveProbeSchedule); err != nil {
				return ErrInvalidConfig(fmt.Sprintf("invalid extensive probe cron expression %q: %v", c.Probing.ExtensiveProbeSchedule, err))
			}
		}
	}

	// Validate performance config
	if c.Performance.WorkerCount <= 0 {
		return ErrInvalidConfig("worker count must be positive")
	}

	return nil
}

// ErrInvalidConfig creates an invalid config error
func ErrInvalidConfig(msg string) error {
	return &ConfigError{Message: msg}
}

// ConfigError represents a configuration error
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return "invalid config: " + e.Message
}

// validateCronExpression validates a cron expression by attempting to parse it
func validateCronExpression(expr string) error {
	// Create a temporary scheduler to validate the cron expression
	s, err := gocron.NewScheduler()
	if err != nil {
		return err
	}
	defer func() { _ = s.Shutdown() }()

	// Try to create a job with the cron expression
	_, err = s.NewJob(
		gocron.CronJob(expr, false), // false = don't start immediately
		gocron.NewTask(func() {}),    // dummy task
	)
	return err
}

// GetNamingStrategy returns the naming strategy to use based on disk count
func (c *NamingConfig) GetNamingStrategy(diskCount int) NamingStrategy {
	if !c.AutoSelect {
		return c.Strategy
	}

	// Auto-select based on disk count
	if diskCount <= DiskCountThresholdByID {
		return NamingByID
	} else if diskCount <= DiskCountThresholdByPath {
		return NamingByPath
	}
	return NamingByVdev
}

// ShouldGenerateVdevConf returns true if vdev_id.conf should be generated
func (c *NamingConfig) ShouldGenerateVdevConf(diskCount int) bool {
	strategy := c.GetNamingStrategy(diskCount)
	return strategy == NamingByVdev && c.VdevConfPath != ""
}
