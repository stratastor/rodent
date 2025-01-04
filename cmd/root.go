package cmd

import (
	"github.com/spf13/cobra"
	"github.com/stratastor/rodent/cmd/config"
	"github.com/stratastor/rodent/cmd/health"
	"github.com/stratastor/rodent/cmd/logs"
	"github.com/stratastor/rodent/cmd/serve"
	"github.com/stratastor/rodent/cmd/status"
	"github.com/stratastor/rodent/cmd/version"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "rodent",
		Short: "Rodent: StrataSTOR Node Agent",
	}

	rootCmd.AddCommand(serve.NewServeCmd())
	rootCmd.AddCommand(version.NewVersionCmd())
	rootCmd.AddCommand(health.NewHealthCmd())
	rootCmd.AddCommand(status.NewStatusCmd())
	rootCmd.AddCommand(logs.NewLogsCmd())
	rootCmd.AddCommand(config.NewConfigCmd())

	return rootCmd
}
