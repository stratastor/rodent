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
			// Load config using precedence rules
			_ = config.LoadConfig(configPath)
			loadedPath := config.GetLoadedConfigPath()

			if loadedPath == "" {
				// If no config was found, show where it was saved
				if err := config.SaveConfig(""); err != nil {
					return fmt.Errorf("failed to save default configuration: %v", err)
				}
				loadedPath = config.GetLoadedConfigPath()
				fmt.Printf("No configuration found. Default configuration saved to: %s\n", loadedPath)
			} else {
				fmt.Printf("Configuration loaded from: %s\n", loadedPath)
			}

			// Show load precedence attempted
			// fmt.Println("\nConfig search paths (in order):")
			// fmt.Printf("1. Command line argument: %s\n", configPath)
			// fmt.Printf("2. RODENT_CONFIG env: %s\n", os.Getenv("RODENT_CONFIG"))
			// if os.Geteuid() != 0 {
			// 	home, _ := os.UserHomeDir()
			// 	fmt.Printf("3. User config: %s\n", filepath.Join(home, ".rodent", constants.ConfigFileName))
			// }
			// fmt.Printf("4. System config: %s\n", filepath.Join(constants.SystemConfigDir, constants.ConfigFileName))

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

			loadedPath := config.GetLoadedConfigPath()
			if loadedPath != "" {
				fmt.Printf("# Configuration loaded from: %s\n", loadedPath)
			} else {
				fmt.Println("# Using default configuration")
			}
			fmt.Printf("---\n%s", string(ymlData))
			return nil
		},
	}

	return cmd
}
