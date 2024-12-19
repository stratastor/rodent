package status

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Rodent server status",
		Run: func(cmd *cobra.Command, args []string) {
			pidFile := "/var/run/rodent.pid"
			if _, err := os.Stat(pidFile); err == nil {
				fmt.Println("Rodent server is running")
			} else {
				fmt.Println("Rodent server is not running")
			}
		},
	}
}
