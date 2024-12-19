package main

import (
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/cmd"
	"github.com/stratastor/rodent/config"
)

func main() {
	log, err := logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "main")
	if err != nil {
		panic(err)
	}
	rootCmd := cmd.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		log.Error("Command execution failed: %v", err)
	}
}
