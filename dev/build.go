package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build main application (desktop)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildDesktop()
	},
}

var buildPylibCmd = &cobra.Command{
	Use:   "pylib",
	Short: "Build Python library (inject CA cert)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildPylib()
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

var buildServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Build discovery server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildServer()
	},
}

var noFrontend bool

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.AddCommand(buildPylibCmd)
	buildCmd.AddCommand(buildAndroidCmd)
	buildCmd.AddCommand(buildAARCmd)
	buildCmd.AddCommand(buildWindowsCmd)
	buildCmd.AddCommand(buildTestCmd)
	buildCmd.AddCommand(buildServerCmd)

	buildCmd.PersistentFlags().BoolVar(&noFrontend, "no-frontend", false, "Skip frontend build")
}

func copyServerCerts() error {
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	files := []string{"server.crt", "server.key"}
	for _, file := range files {
		src := filepath.Join(rootDir, "certs", file)
		dst := filepath.Join(rootDir, "discovery", "resources", file)

		data, err := os.ReadFile(src)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("⚠️ Server certs not found. Running 'dev certs' to generate...")
				if err := runGenCerts(); err != nil {
					return err
				}
				data, err = os.ReadFile(src)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		if err := os.WriteFile(dst, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func runBuildServer() error {
	fmt.Println("Building Discovery Server...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	if err := copyServerCerts(); err != nil {
		return fmt.Errorf("failed to copy server certs: %w", err)
	}

	// Build server frontend
	if !noFrontend {
		fmt.Println("Building server frontend...")
		if err := buildFrontendIn(filepath.Join(rootDir, "discovery_web")); err != nil {
			return fmt.Errorf("failed to build server frontend: %w", err)
		}
		// Move dist to api/dist because frontend_embed.go is in api package
		srcDist := filepath.Join(rootDir, "discovery", "dist")
		dstDist := filepath.Join(rootDir, "discovery", "api", "dist")
		os.RemoveAll(dstDist) // Clean
		if err := os.Rename(srcDist, dstDist); err != nil {
			return fmt.Errorf("failed to move dist to api/dist: %w", err)
		}
	} else {
		fmt.Println("Skipping server frontend build.")
	}

	// Build Server Binary
	fmt.Println("Building server binary...")
	output := filepath.Join(rootDir, "dist", "discovery")
	if runtime.GOOS == "windows" {
		output += ".exe"
	}

	// Ensure dist dir exists
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}

	if err := execCmd(rootDir, nil, "go", "build", "-o", output, "./cmd/discovery"); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Printf("✅ Server build successful: %s\n", output)
	return nil
}

func copyCertsToCore() error {
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}
	src := filepath.Join(rootDir, "certs", "ca.crt")
	dst := filepath.Join(rootDir, "core", "resources", "ca.crt")

	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("⚠️ CA cert not found. Running 'dev certs' to generate...")
			if err := runGenCerts(); err != nil {
				return err
			}
			data, err = os.ReadFile(src)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return os.WriteFile(dst, data, 0644)
}

func runBuildDesktop() error {
	fmt.Println("Building main application (Desktop)...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	if err := copyCertsToCore(); err != nil {
		return fmt.Errorf("failed to copy certs: %w", err)
	}

	// Build frontend
	if !noFrontend {
		if err := buildFrontendIn(filepath.Join(rootDir, "core", "debug_web")); err != nil {
			return err
		}
	} else {
		fmt.Println("Skipping frontend build.")
	}

	toolCmdDir := filepath.Join(rootDir, "cmd", "vpn-share-tool")

	// Build Go binary
	if err := execCmdFiltered(toolCmdDir, nil, "go", "build", "-o", "vpn-share-tool"); err != nil {
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

	if err := execCmd(filepath.Join(rootDir, "cmd", "vpn-share-tool"), env, "fyne", "package", "-os", "android", "-app-id", "com.example.vpnsharetool", "-icon", "Icon.png"); err != nil {
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
	if err := buildFrontendIn(filepath.Join(rootDir, "core", "debug_web")); err != nil {
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

func runBuildWindows() error {
	fmt.Println("Building Windows application (cross-compile)...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	if err := copyCertsToCore(); err != nil {
		return fmt.Errorf("failed to copy certs: %w", err)
	}

	// Bump version before building
	version, _, err := BumpVersion()
	if err != nil {
		return fmt.Errorf("failed to bump version: %w", err)
	}
	fmt.Printf("Build Version: %s\n", version)

	// Write version to gui/version.txt
	versionFile := filepath.Join(rootDir, "gui", "version.txt")
	if err := os.WriteFile(versionFile, []byte(version), 0644); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}

	// Build frontend
	if !noFrontend {
		if err := buildFrontendIn(filepath.Join(rootDir, "core", "debug_web")); err != nil {
			return err
		}
	} else {
		fmt.Println("Skipping frontend build.")
	}

	if err := execCmd(rootDir, nil, "fyne-cross", "windows", "-arch", "amd64", "--app-id", "vpn.share.tool", "./cmd/vpn-share-tool"); err != nil {
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
	testProjectDir := filepath.Join(rootDir, "demo_site")
	frontendDir := filepath.Join(rootDir, "demo_site_web")

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

func runBuildPylib() error {
	fmt.Println("Building Python library...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	srcFile := filepath.Join(rootDir, "pylib", "libproxy.py")
	certFile := filepath.Join(rootDir, "certs", "ca.crt")
	dstDir := filepath.Join(rootDir, "dist")
	dstFile := filepath.Join(dstDir, "libproxy.py")

	// Read source file
	srcContent, err := os.ReadFile(srcFile)
	if err != nil {
		return fmt.Errorf("failed to read libproxy.py: %w", err)
	}

	// Read cert file
	// Ensure certs exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		fmt.Println("⚠️ CA cert not found. Running 'dev certs' to generate...")
		if err := runGenCerts(); err != nil {
			return err
		}
	}
	certContent, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read ca.crt: %w", err)
	}

	// Replace placeholder
	contentStr := string(srcContent)
	certStr := string(certContent)
	
	// Escape backslashes and double quotes if necessary, but usually PEM cert is safe in python triple quotes blocks if it doesn't contain triple quotes.
	// However, we should check if the cert ends with a newline.
	
	newContent := strings.Replace(contentStr, "__CA_CERT_PLACEHOLDER__", certStr, 1)

	// Ensure dist dir exists
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create dist dir: %w", err)
	}

	// Write to destination
	if err := os.WriteFile(dstFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write libproxy.py to dist: %w", err)
	}

	fmt.Printf("✅ Pylib build successful: %s\n", dstFile)
	return nil
}
