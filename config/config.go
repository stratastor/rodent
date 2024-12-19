package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/viper"
	"github.com/stratastor/logger"
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
		log, err := logger.NewTag(logConfig, "config")
		if err != nil {
			fmt.Printf("Failed to create logger: %v\n", err)
			os.Exit(1)
		}

		viper.SetConfigType("yaml")

		// Check environment variable for config path
		envConfigPath := os.Getenv("RODENT_CONFIG")
		if configFilePath != "" {
			configPath = configFilePath
		} else if envConfigPath != "" {
			configPath = envConfigPath
		} else {
			viper.SetConfigName("rodent.yml")
			viper.AddConfigPath(".")             // Current directory
			viper.AddConfigPath("/etc/rodent/")  // System-wide path
			viper.AddConfigPath("$HOME/.rodent") // User directory
		}

		// If config path is explicitly provided
		if configPath != "" {
			absPath, err := filepath.Abs(configPath)
			if err != nil {
				log.Error("Invalid config path: %v", err)
			}
			viper.SetConfigFile(absPath)
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

		// Bind environment variables
		viper.AutomaticEnv()

		// Load configuration file
		if err := viper.ReadInConfig(); err != nil {
			log.Warn("Config file not found or unreadable, using defaults: %v", err)
		} else {
			configPath = viper.ConfigFileUsed() // Store the path of the loaded config
		}

		// Unmarshal into Config struct
		var cfg Config
		if err := viper.Unmarshal(&cfg); err != nil {
			log.Error("Failed to parse configuration: %v", err)
		}

		instance = &cfg
	})

	return instance
}

// TODO: Use this function to save the configuration to a file
// SaveConfig persists the current configuration to a specified path.
func SaveConfig(path string) error {
	configJSON, err := json.MarshalIndent(instance, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	if err := os.WriteFile(path, configJSON, 0644); err != nil {
		return fmt.Errorf("failed to write configuration to file: %w", err)
	}

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
