package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build main application (desktop)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildDesktop()
	},
}

var buildAndroidCmd = &cobra.Command{
	Use:   "android",
	Short: "Build Fyne Android application",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildAndroidFyne()
	},
}

var buildAARCmd = &cobra.Command{
	Use:   "aar",
	Short: "Build Android AAR for Flutter",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildAAR()
	},
}

var buildLinuxCmd = &cobra.Command{
	Use:   "linux",
	Short: "Build Linux C-shared library",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildLinux()
	},
}

var buildWindowsCmd = &cobra.Command{
	Use:   "windows",
	Short: "Build Windows application (fyne-cross)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildWindows()
	},
}

var buildTestCmd = &cobra.Command{
	Use:     "test",
	Aliases: []string{"test-project"},
	Short:   "Build test project",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildTestProject()
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.AddCommand(buildAndroidCmd)
	buildCmd.AddCommand(buildAARCmd)
	buildCmd.AddCommand(buildLinuxCmd)
	buildCmd.AddCommand(buildWindowsCmd)
	buildCmd.AddCommand(buildTestCmd)
}

func runBuildDesktop() error {
	fmt.Println("Building main application (Desktop)...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	// Build frontend
	if err := buildFrontendIn(filepath.Join(rootDir, "core", "frontend")); err != nil {
		return err
	}

	// Build Go binary
	if err := execCmd(rootDir, nil, "go", "build"); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Println("✅ Build successful.")
	return nil
}

func runBuildAndroidFyne() error {
	fmt.Println("Building Fyne Android application...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	env := append(os.Environ(),
		"ANDROID_HOME="+androidHome,
		"ANDROID_NDK_HOME="+androidNdkHome,
	)

	if err := execCmd(rootDir, env, "fyne", "package", "-os", "android", "-app-id", "com.example.vpnsharetool", "-icon", "Icon.png"); err != nil {
		return fmt.Errorf("fyne package failed: %w", err)
	}

	fmt.Println("✅ Android Fyne build successful.")
	return nil
}

func runBuildAAR() error {
	fmt.Println("Building Android AAR for Flutter...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}
	// Build frontend
	if err := buildFrontendIn(filepath.Join(rootDir, "core", "frontend")); err != nil {
		return fmt.Errorf("failed to build frontend for AAR: %w", err)
	}

	env := append(os.Environ(),
		"ANDROID_NDK_HOME="+androidNdkHome,
		"GOFLAGS=-mod=mod",
	)

	if err := execCmd(rootDir, env, "gomobile", "bind", "-target=android", "-androidapi", "21", "-o", "flutter_gui/android/libs/core.aar", "github.com/soda92/vpn-share-tool/mobile"); err != nil {
		return fmt.Errorf("gomobile bind failed: %w", err)
	}

	fmt.Println("✅ AAR build successful.")
	return nil
}

func runBuildLinux() error {
	fmt.Println("Building Linux C-shared library for Flutter...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}
	if err := execCmd(rootDir, nil, "go", "build", "-buildmode=c-shared", "-o", "flutter_gui/linux/libcore.so", "./linux_bridge"); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}
	fmt.Println("✅ Linux build successful.")
	return nil
}

func runBuildWindows() error {
	fmt.Println("Building Windows application (cross-compile)...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	// Build frontend
	if err := buildFrontendIn(filepath.Join(rootDir, "core", "frontend")); err != nil {
		return err
	}

	if err := execCmd(rootDir, nil, "fyne-cross", "windows", "-arch", "amd64", "--app-id", "vpn.share.tool"); err != nil {
		return fmt.Errorf("fyne-cross failed: %w", err)
	}
	fmt.Println("✅ Windows build successful.")
	return nil
}

func runBuildTestProject() error {
	fmt.Println("Building Test Project...")
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

	// Build Go binary
	if err := execCmd(testProjectDir, nil, "go", "build", "main.go"); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Println("✅ Test project build successful.")
	return nil
}
