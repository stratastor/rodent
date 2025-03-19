// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/viper"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/constants"
	"gopkg.in/yaml.v2"
)

var (
	instance   *Config
	once       sync.Once
	configPath string // Tracks where the config was loaded from
)

type Config struct {
	Server struct {
		Port      int    `mapstructure:"port"`
		LogLevel  string `mapstructure:"logLevel"`
		Daemonize bool   `mapstructure:"daemonize"`
	} `mapstructure:"server"`

	Health struct {
		Interval string `mapstructure:"interval"`
		Endpoint string `mapstructure:"endpoint"`
	} `mapstructure:"health"`

	AD struct {
		AdminPassword string `mapstructure:"adminPassword"`
		LDAPURL       string `mapstructure:"ldapURL"`
		BaseDN        string `mapstructure:"baseDN"`
		AdminDN       string `mapstructure:"adminDN"`
	} `mapstructure:"ad"`

	Logs struct {
		Path      string `mapstructure:"path"`
		Retention string `mapstructure:"retention"`
		Output    string `mapstructure:"output"` // stdout or file
	} `mapstructure:"logs"`

	Logger struct {
		LogLevel     string `mapstructure:"logLevel"`
		EnableSentry bool   `mapstructure:"enableSentry"`
		SentryDSN    string `mapstructure:"sentryDSN"`
	} `mapstructure:"logger"`

	Toggle struct {
		JWT     string `mapstructure:"jwt"`     // JWT for Toggle service authentication
		BaseURL string `mapstructure:"baseURL"` // Optional base URL for Toggle service
	}

	StrataSecure bool `mapstructure:"strataSecure"`

	Environment string `mapstructure:"environment"`
}

// LoadConfig loads the configuration with precedence rules.
func LoadConfig(configFilePath string) *Config {
	once.Do(func() {
		logConfig := logger.Config{
			LogLevel:     "info",
			EnableSentry: false,
			SentryDSN:    "",
		} // Adjust the logger.Config initialization manually
		l, err := logger.NewTag(logConfig, "config")
		if err != nil {
			fmt.Printf("Failed to create logger: %v\n", err)
			// TODO: Why exit here? Return error/nil instead?
			os.Exit(1)
		}

		viper.SetConfigType("yaml")

		// Determine config path with precedence
		configPath = determineConfigPath(configFilePath)

		if configPath != "" {
			absPath, err := filepath.Abs(configPath)
			if err != nil {
				l.Error("Invalid config path", "err", err)
			} else {
				viper.SetConfigFile(absPath)
			}
		}

		// Set defaults
		viper.SetDefault("environment", "dev")
		viper.SetDefault("server.port", 8042)
		viper.SetDefault("server.logLevel", "info")
		viper.SetDefault("server.daemonize", false)
		viper.SetDefault("health.interval", "30s")
		viper.SetDefault("health.endpoint", "/health")
		viper.SetDefault("logs.path", "/var/log/rodent/rodent.log")
		viper.SetDefault("logs.retention", "7d")
		viper.SetDefault("logs.output", "stdout")
		viper.SetDefault("logger.logLevel", "info")
		viper.SetDefault("logger.enableSentry", false)
		viper.SetDefault("logger.sentryDSN", "")

		// Set defaults for AD configuration
		viper.SetDefault("ad.adminPassword", "")
		viper.SetDefault("ad.ldapURL", "ldaps://localhost:636")
		viper.SetDefault("ad.baseDN", "CN=Users,DC=ad,DC=strata,DC=internal")
		viper.SetDefault("ad.adminDN", "CN=Administrator,CN=Users,DC=ad,DC=strata,DC=internal")

		// Set defaults for Toggle configuration
		viper.SetDefault("toggle.jwt", "")
		viper.SetDefault("toggle.baseURL", "")

		// Set defaults for StrataSecure
		viper.SetDefault("strataSecure", true)

		// Bind environment variables
		viper.AutomaticEnv()

		// Load configuration file
		if err := viper.ReadInConfig(); err != nil {
			l.Debug("Config file not found or unreadable, using defaults", "err", err)
		} else {
			configPath = viper.ConfigFileUsed()
		}

		// Unmarshal into Config struct
		var cfg Config
		if err := viper.Unmarshal(&cfg); err != nil {
			l.Error("Failed to parse configuration", "err", err)
		}

		instance = &cfg

		// Save the configuration if it was loaded from a non-standard location
		if configPath == "" {
			if err := SaveConfig(""); err != nil {
				l.Error("Failed to persist configuration", "err", err)
			}
		}
	})

	return instance
}

// SaveConfig persists the current configuration to a specified path.
func SaveConfig(path string) error {
	if path == "" {
		// Determine default save location based on user privileges
		if os.Geteuid() == 0 {
			if err := os.MkdirAll(constants.SystemConfigDir, 0755); err != nil {
				return fmt.Errorf("failed to create system config directory: %w", err)
			}
			path = filepath.Join(constants.SystemConfigDir, constants.ConfigFileName)
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			userConfigDir := filepath.Join(home, ".rodent")
			if err := os.MkdirAll(userConfigDir, 0755); err != nil {
				return fmt.Errorf("failed to create user config directory: %w", err)
			}
			path = filepath.Join(userConfigDir, constants.ConfigFileName)
		}
	}

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save configuration
	configYAML, err := yaml.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	if err := os.WriteFile(path, configYAML, 0644); err != nil {
		return fmt.Errorf("failed to write configuration to file: %w", err)
	}

	// Update the tracked config path
	configPath = path

	return nil
}

func determineConfigPath(explicitPath string) string {
	// 1. Explicit path from command line
	if explicitPath != "" {
		return explicitPath
	}

	// 2. Environment variable
	if envPath := os.Getenv("RODENT_CONFIG"); envPath != "" {
		return envPath
	}

	// 3. Current working directory
	currentDir, err := os.Getwd()
	if err == nil {
		cwdConfig := filepath.Join(currentDir, constants.ConfigFileName)
		if _, err := os.Stat(cwdConfig); err == nil {
			return cwdConfig
		}
	}

	// 4. User-specific config for non-root users
	if os.Geteuid() != 0 {
		home, err := os.UserHomeDir()
		if err == nil {
			userConfig := filepath.Join(home, ".rodent", constants.ConfigFileName)
			if _, err := os.Stat(userConfig); err == nil {
				return userConfig
			}
		}
	}

	// 5. System-wide config
	systemConfig := filepath.Join(constants.SystemConfigDir, constants.ConfigFileName)
	if _, err := os.Stat(systemConfig); err == nil {
		return systemConfig
	}

	return ""
}

// GetLoadedConfigPath returns the path of the currently loaded configuration file.
func GetLoadedConfigPath() string {
	return configPath
}

// GetConfig returns the current configuration instance.
func GetConfig() *Config {
	if instance == nil {
		// TODO: Review this logic
		// Load default configuration
		return LoadConfig("")
	}
	return instance
}

func NewLoggerConfig(cfg *Config) logger.Config {
	if cfg == nil {
		return logger.Config{
			LogLevel:     "info",
			EnableSentry: false,
			SentryDSN:    "",
		}
	}

	return logger.Config{
		LogLevel:     cfg.Logger.LogLevel,
		EnableSentry: cfg.Logger.EnableSentry,
		SentryDSN:    cfg.Logger.SentryDSN,
	}
}
