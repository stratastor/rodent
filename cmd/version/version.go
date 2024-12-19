package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

const Version = "v0.0.1"

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show Rodent version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Rodent Version:", Version)
		},
	}
}
