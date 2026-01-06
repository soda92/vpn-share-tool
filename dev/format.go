package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var formatCmd = &cobra.Command{
	Use:   "format",
	Short: "Format all Go files",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Formatting Go files...")
		runGoFmt()
	},
}

func init() {
	rootCmd.AddCommand(formatCmd)
}

func runGoFmt() {
	// go fmt ./... formats all packages in the module
	c := exec.Command("go", "fmt", "./...")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		fmt.Printf("Error running go fmt: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Done.")
}
