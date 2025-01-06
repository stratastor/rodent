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

package logs

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/stratastor/rodent/config"
)

func NewLogsCmd() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View Rodent server logs",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := config.GetConfig()
			if cfg == nil {
				fmt.Println("No configuration loaded")
				return
			}

			if cfg.Logs.Output == "stdout" {
				fmt.Println("Logs are being written to stdout.")
				return
			}

			logFile := cfg.Logs.Path
			if _, err := os.Stat(logFile); os.IsNotExist(err) {
				fmt.Println("Log file does not exist:", logFile)
				return
			}

			tailCmd := "tail"
			tailArgs := []string{"-n", "100"}
			if follow {
				tailArgs = append(tailArgs, "-f")
			}
			tailArgs = append(tailArgs, logFile)

			execCmd := exec.Command(tailCmd, tailArgs...)
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			if err := execCmd.Run(); err != nil {
				fmt.Printf("Failed to read logs: %v\n", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	return cmd
}
