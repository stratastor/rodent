package serve

import (
	"context"
	"os"

	"github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/constants"
	"github.com/stratastor/rodent/pkg/lifecycle"
	"github.com/stratastor/rodent/pkg/server"
)

var detached bool

func NewServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Rodent server",
		Run:   runServe,
	}

	cmd.Flags().BoolVarP(&detached, "detach", "d", false, "Run as a daemon")
	return cmd
}

func runServe(cmd *cobra.Command, args []string) {
	lcfg := config.NewLoggerConfig(config.GetConfig())
	log, err := logger.NewTag(lcfg, "serve")
	if err != nil {
		panic(err)
	}

	rc := config.GetConfig()
	pidFile := constants.RodentPIDFilePath
	// Check for existing instance before proceeding
	if err := lifecycle.EnsureSingleInstance(pidFile); err != nil {
		log.Error("Failed to start: %v", err)
		os.Exit(1)
	}

	if detached {
		ctx := &daemon.Context{
			PidFileName: pidFile,
			PidFilePerm: 0644,
			LogFileName: rc.Logs.Path,
			LogFilePerm: 0640,
			WorkDir:     "/",
			Umask:       027,
			Args:        []string{"rodent", "serve"},
		}

		d, err := ctx.Reborn()
		if err != nil {
			log.Error("Failed to start daemon: %v", err)
			os.Exit(1)
		}

		if d != nil {
			log.Info("Rodent is running as a daemon")
			return
		}
		defer ctx.Release()
	}

	startServer()
}

func startServer() {
	lcfg := config.NewLoggerConfig(config.GetConfig())
	log, err := logger.NewTag(lcfg, "serve")
	if err != nil {
		panic(err)
	}
	cfg := config.GetConfig()

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register the context canceller
	lifecycle.RegisterContextCanceller(cancel)

	// Register shutdown hook for server cleanup
	lifecycle.RegisterShutdownHook(func() {
		log.Info("Shutting down server...")
		if err := server.Shutdown(ctx); err != nil {
			log.Error("Error during server shutdown: %v", err)
		}
	})

	// Start handling lifecycle signals (e.g., SIGTERM, SIGHUP)
	go lifecycle.HandleSignals(ctx)

	// Start the server
	log.Info("Starting Rodent server on port %d...", cfg.Server.Port)
	if err := server.Start(ctx, cfg.Server.Port); err != nil {
		log.Error("Failed to start server: %v", err)
	}
}
