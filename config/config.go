// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/internal/constants"
	"gopkg.in/yaml.v3"
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
		Realm         string `mapstructure:"realm"`
		BaseDN        string `mapstructure:"baseDN"`
		AdminDN       string `mapstructure:"adminDN"`
		UserOU        string `mapstructure:"userOU"`     // OU for users, relative to BaseDN
		GroupOU       string `mapstructure:"groupOU"`    // OU for groups, relative to BaseDN
		ComputerOU    string `mapstructure:"computerOU"` // OU for computers, relative to BaseDN
		DC            struct {
			Enabled       bool   `mapstructure:"enabled"`
			ContainerName string `mapstructure:"containerName"`
			Hostname      string `mapstructure:"hostname"`
			Realm         string `mapstructure:"realm"`
			Domain        string `mapstructure:"domain"`
			DnsForwarder  string `mapstructure:"dnsForwarder"`
			EtcVolume     string `mapstructure:"etcVolume"`
			PrivateVolume string `mapstructure:"privateVolume"`
			VarVolume     string `mapstructure:"varVolume"`
		} `mapstructure:"dc"`
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
		BaseURL string `mapstructure:"baseURL"` // Base URL for Toggle REST API service
		RPCAddr string `mapstructure:"rpcAddr"` // Address for Toggle gRPC service
	}

	StrataSecure bool `mapstructure:"strataSecure"`

	Shares struct {
		SMB struct {
			Realm     string `mapstructure:"realm"`
			Workgroup string `mapstructure:"workgroup"`
		} `mapstructure:"smb"`
	} `mapstructure:"shares"`

	Keys struct {
		SSH struct {
			Username           string `mapstructure:"username"`
			DirPath            string `mapstructure:"dirPath"`
			Algorithm          string `mapstructure:"algorithm"`
			KnownHostsFile     string `mapstructure:"knownHostsFile"`
			AuthorizedKeysFile string `mapstructure:"authorizedKeysFile"`
		} `mapstructure:"ssh"`
	} `mapstructure:"keys"`

	Development struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"development"`

	Environment string `mapstructure:"environment"`
}

// LoadConfig loads the configuration with precedence rules.
func LoadConfig(configFilePath string) *Config {
	once.Do(func() {
		// Setup basic logger for initialization
		logConfig := logger.Config{
			LogLevel:     "info",
			EnableSentry: false,
			SentryDSN:    "",
		}
		l, err := logger.NewTag(logConfig, "config")
		if err != nil {
			fmt.Printf("Failed to create logger: %v\n", err)
			os.Exit(1)
		}

		// Reset viper to avoid any potential carryover
		viper.Reset()
		viper.SetConfigType("yaml")

		// Determine which config file to use with clear priorities
		systemConfigPath := filepath.Join(GetConfigDir(), constants.ConfigFileName)

		if configFilePath != "" {
			// 1. Priority: Explicit path from command line
			configPath = configFilePath
		} else if envPath := os.Getenv("RODENT_CONFIG"); envPath != "" {
			// 2. Priority: Environment variable
			configPath = envPath
		} else {
			// 3. Priority: Always default to system-wide config
			configPath = systemConfigPath
		}

		l.Info("Using config file", "path", configPath)

		// Convert to absolute path if possible for consistency
		absPath, err := filepath.Abs(configPath)
		if err == nil {
			configPath = absPath
		}

		// Set config file path for viper
		viper.SetConfigFile(configPath)

		// Set defaults
		viper.SetDefault("environment", "dev")
		viper.SetDefault("server.port", 8042)
		viper.SetDefault("server.logLevel", "debug")
		viper.SetDefault("server.daemonize", false)
		viper.SetDefault("health.interval", "30s")
		viper.SetDefault("health.endpoint", "/health")
		viper.SetDefault("logs.path", "/var/log/rodent/rodent.log")
		viper.SetDefault("logs.retention", "7d")
		viper.SetDefault("logs.output", "stdout")
		viper.SetDefault("logger.logLevel", "debug")
		viper.SetDefault("logger.enableSentry", false)
		viper.SetDefault("logger.sentryDSN", "")

		// Set defaults for AD configuration - use lowercase consistently
		viper.SetDefault("ad.adminPassword", "")
		viper.SetDefault("ad.ldapURL", "")
		viper.SetDefault("ad.realm", "")
		viper.SetDefault("ad.baseDN", "")
		viper.SetDefault("ad.adminDN", "")
		viper.SetDefault("ad.userOU", "OU=StrataUsers")         // Will be appended to BaseDN
		viper.SetDefault("ad.groupOU", "OU=StrataGroups")       // Will be appended to BaseDN
		viper.SetDefault("ad.computerOU", "OU=StrataComputers") // Will be appended to BaseDN

		// Set defaults for AD DC configuration
		viper.SetDefault("ad.dc.enabled", false)
		viper.SetDefault("ad.dc.containerName", "dc1")
		viper.SetDefault("ad.dc.hostname", "DC1")
		viper.SetDefault("ad.dc.realm", "AD.STRATA.INTERNAL")
		viper.SetDefault("ad.dc.domain", "AD")
		viper.SetDefault("ad.dc.dnsForwarder", "8.8.8.8")
		viper.SetDefault("ad.dc.etcVolume", "dc1_etc")
		viper.SetDefault("ad.dc.privateVolume", "dc1_private")
		viper.SetDefault("ad.dc.varVolume", "dc1_var")

		// Set defaults for Toggle configuration
		viper.SetDefault("toggle.jwt", "")
		viper.SetDefault("toggle.baseURL", "")

		// Set defaults for Shares configuration
		viper.SetDefault("shares.smb.realm", "AD.STRATA.INTERNAL")
		viper.SetDefault("shares.smb.workgroup", "AD")

		// Set defaults for SSH keys
		viper.SetDefault("keys.ssh.username", "ubuntu")
		viper.SetDefault("keys.ssh.dirPath", "~/.rodent/ssh")
		viper.SetDefault("keys.ssh.algorithm", "ed25519")
		viper.SetDefault("keys.ssh.knownHostsFile", "~/.rodent/ssh/known_hosts")
		viper.SetDefault("keys.ssh.authorizedKeysFile", "~/.ssh/authorized_keys")

		// Set defaults for StrataSecure
		viper.SetDefault("strataSecure", true)
		viper.SetDefault("development.enabled", false)

		// Bind environment variables
		viper.AutomaticEnv()
		viper.SetEnvPrefix("RODENT")
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

		// Try to read the config file
		err = viper.ReadInConfig()

		// Handle missing or invalid config
		if err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				// File doesn't exist, create a default one
				l.Info(
					"Config file not found, creating default at system path",
					"path",
					systemConfigPath,
				)

				// Ensure parent directory exists
				if err := os.MkdirAll(GetConfigDir(), 0755); err != nil {
					l.Error("Failed to create config directory", "err", err)
				}

				// Use defaults for now
				var cfg Config
				if err := viper.Unmarshal(&cfg); err != nil {
					l.Error("Failed to unmarshal default configuration", "err", err)
				}

				instance = &cfg
				configPath = systemConfigPath

				// Save default config to the system path
				if err := SaveConfig(systemConfigPath); err != nil {
					l.Error("Failed to save default configuration", "err", err)
				}
			} else {
				// Some other error (parse error, etc.)
				l.Error("Error reading config file", "err", err)

				// Still use defaults
				var cfg Config
				if err := viper.Unmarshal(&cfg); err != nil {
					l.Error("Failed to unmarshal default configuration", "err", err)
				}

				instance = &cfg
			}
		} else {
			// Successfully loaded config
			l.Info("Config file loaded successfully", "path", viper.ConfigFileUsed())
			configPath = viper.ConfigFileUsed()

			var cfg Config
			if err := viper.Unmarshal(&cfg); err != nil {
				l.Error("Failed to parse configuration", "err", err)
			} else {
				instance = &cfg
			}
		}

		// Verify AD password was loaded
		if instance.AD.AdminPassword == "" {
			l.Warn("AD admin password is empty, AD operations may fail")
		}

		// Log config values for debugging (redact sensitive data)
		debugCfg := *instance
		debugCfg.AD.AdminPassword = "[REDACTED]"
		l.Debug("Loaded configuration", "config", fmt.Sprintf("%+v", debugCfg))
	})

	return instance
}

// SaveConfig persists the current configuration to a specified path.
func SaveConfig(path string) error {
	if path == "" {
		// Determine default save location based on user privileges
		if os.Geteuid() == 0 {
			if err := os.MkdirAll(GetConfigDir(), 0755); err != nil {
				return fmt.Errorf("failed to create system config directory: %w", err)
			}
			path = filepath.Join(GetConfigDir(), constants.ConfigFileName)
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
