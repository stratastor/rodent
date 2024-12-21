package main

import (
	"log"

	"github.com/stratastor/rodent/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		log.Printf("Command execution failed: %v", err)
	}
}
