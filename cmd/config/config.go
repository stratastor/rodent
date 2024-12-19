package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stratastor/rodent/config"
	"gopkg.in/yaml.v2"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Rodent configuration",
	}

	cmd.AddCommand(NewLoadConfigCmd())
	cmd.AddCommand(NewPrintConfigCmd())
	return cmd
}

func NewLoadConfigCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "load",
		Short: "Load the configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = config.LoadConfig(configPath)
			fmt.Printf("Configuration loaded from: %s\n", config.GetLoadedConfigPath())
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")
	return cmd
}

func NewPrintConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print",
		Short: "Print the currently loaded configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()
			if cfg == nil {
				return fmt.Errorf("no configuration loaded")
			}

			// Convert the config to YAML format
			ymlData, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal config to YAML: %v", err)
			}

			fmt.Printf("Current Configuration:\n%s\n", string(ymlData))
			return nil
		},
	}

	return cmd
}
