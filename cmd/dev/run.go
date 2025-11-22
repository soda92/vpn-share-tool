package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func runRunDesktop() error {
	fmt.Println("Running application...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	// Build frontend
	if err := buildFrontend(rootDir); err != nil {
		return err
	}

	// Run Go app
	if err := runCmd(rootDir, nil, "go", "run", "main.go"); err != nil {
		return fmt.Errorf("go run failed: %w", err)
	}
	return nil
}

func runFlutter(args []string) error {
	fmt.Println("Running Flutter command...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}
	flutterDir := filepath.Join(rootDir, "flutter_gui")

	env := append(os.Environ(),
		"ANDROID_HOME="+androidHome,
		"ANDROID_NDK_HOME="+androidNdkHome,
	)

	if err := runCmd(flutterDir, env, "flutter", args...); err != nil {
		return fmt.Errorf("flutter command failed: %w", err)
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
	if err := runCmd(frontendDir, nil, "npm", "run", "build"); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}

	// Run Go app
	// go run main.go
	if err := runCmd(testProjectDir, nil, "go", "run", "main.go"); err != nil {
		return fmt.Errorf("go run failed: %w", err)
	}
	return nil
}
