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
