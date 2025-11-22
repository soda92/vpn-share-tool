package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"fmt"
)

const (
	androidHome    = "/home/soda/Android/Sdk"
	androidNdkHome = "/home/soda/Android/Sdk/ndk/27.0.12077973/"
)

func runCmd(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	} else {
		cmd.Env = os.Environ()
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildFrontend(rootDir string) error {
	frontendDir := filepath.Join(rootDir, "core", "frontend")
	distDir := filepath.Join(frontendDir, "dist")

	// Clean dist
	if err := os.RemoveAll(distDir); err != nil {
		return fmt.Errorf("failed to clean dist dir: %w", err)
	}

	// Build frontend
	if err := runCmd(frontendDir, nil, "npm", "run", "build"); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}
	return nil
}
