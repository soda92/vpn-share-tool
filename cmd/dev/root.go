package main

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dev",
	Short: "Development CLI for VPN Share Tool",
	Long:  `A unified CLI tool for building, running, and deploying the VPN Share Tool and its components.`,
}

func execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
