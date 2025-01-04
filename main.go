package main

import (
	"fmt"

	"github.com/stratastor/rodent/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
