package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run main application (desktop)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRunDesktop()
	},
}

var runTestCmd = &cobra.Command{
	Use:   "test",
	Aliases: []string{"test-project"},
	Short: "Run test project",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRunTestProject()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.AddCommand(runTestCmd)
}

func runRunDesktop() error {
	fmt.Println("Running application...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	// Build frontend
	if err := buildFrontendIn(filepath.Join(rootDir, "core", "frontend")); err != nil {
		return err
	}

	// Run Go app
	if err := execCmd(rootDir, nil, "go", "run", "main.go"); err != nil {
		return fmt.Errorf("go run failed: %w", err)
	}
	return nil
}

func runRunTestProject() error {
	fmt.Println("Running Test Project...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}
	testProjectDir := filepath.Join(rootDir, "test_project")
	frontendDir := filepath.Join(testProjectDir, "frontend")

	// Build frontend
	if err := buildFrontendIn(frontendDir); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}

	// Run Go app
	if err := execCmd(testProjectDir, nil, "go", "run", "main.go"); err != nil {
		return fmt.Errorf("go run failed: %w", err)
	}
	return nil
}