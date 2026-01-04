package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	androidHome    string
	androidNdkHome string
)

func init() {
	userHome, err := os.UserHomeDir()
	if err != nil {
		// Fallback if UserHomeDir fails
		if h := os.Getenv("HOME"); h != "" {
			userHome = h
		} else {
			fmt.Println("⚠️ Warning: Could not determine user home directory.")
		}
	}

	// 1. Android Home
	// Priority: Env var -> Default path (~/Android/Sdk)
	if v := os.Getenv("ANDROID_HOME"); v != "" {
		androidHome = v
	} else {
		androidHome = filepath.Join(userHome, "Android", "Sdk")
	}

	// 2. Android NDK Home
	// Priority: Env var -> Default path (~/Android/Sdk/ndk/27.0.12077973)
	if v := os.Getenv("ANDROID_NDK_HOME"); v != "" {
		androidNdkHome = v
	} else {
		// Using the specific version from previous configuration as default fallback
		androidNdkHome = filepath.Join(androidHome, "ndk", "27.0.12077973")
	}
}

func execCmd(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	} else {
		cmd.Env = os.Environ()
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func execCmdSilent(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	} else {
		cmd.Env = os.Environ()
	}
	cmd.Stdout = io.Discard // Suppress stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func buildFrontendIn(frontendDir string) error {
	distDir := filepath.Join(frontendDir, "dist")

	// Clean dist
	if err := os.RemoveAll(distDir); err != nil {
		return fmt.Errorf("failed to clean dist dir: %w", err)
	}

	// Build frontend (Silent)
	if err := execCmdSilent(frontendDir, nil, "npm", "run", "build"); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}
	// Check for companion code directory (e.g. core_web -> core)
	cleanPath := filepath.Clean(frontendDir)
	parent, folder := filepath.Split(cleanPath)

	if strings.HasSuffix(folder, "_web") {
		codeFolder := strings.TrimSuffix(folder, "_web")
		targetDistDir := filepath.Join(parent, codeFolder, "dist")

		// Clean target directory
		if err := os.RemoveAll(targetDistDir); err != nil {
			return fmt.Errorf("failed to clean target dist dir: %w", err)
		}

		// Copy built artifacts to target dist
		if err := os.CopyFS(targetDistDir, os.DirFS(distDir)); err != nil {
			return fmt.Errorf("failed to copy dist to code dir: %w", err)
		}
	}
	return nil
}
