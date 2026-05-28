package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build main application (desktop)",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return generateSecretsGo()
	},
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
	Aliases: []string{"test-project", "demo_site"},
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

var buildLinuxSoCmd = &cobra.Command{
	Use:   "linux-so",
	Short: "Build Linux shared library for Flutter FFI",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildLinuxSo()
	},
}

var buildUpdaterCmd = &cobra.Command{
	Use:   "updater",
	Short: "Build Windows updater application",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildUpdater()
	},
}

var buildMSICmd = &cobra.Command{
	Use:   "msi",
	Short: "Build Windows MSI installer package (requires wixl on Linux or candle/light on Windows)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildMSI()
	},
}

var noFrontend bool
var buildWindowsLocal bool

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.AddCommand(buildPylibCmd)
	buildCmd.AddCommand(buildAndroidCmd)
	buildCmd.AddCommand(buildAARCmd)
	buildCmd.AddCommand(buildWindowsCmd)
	buildCmd.AddCommand(buildTestCmd)
	buildCmd.AddCommand(buildServerCmd)
	buildCmd.AddCommand(buildLinuxSoCmd)
	buildCmd.AddCommand(buildUpdaterCmd)
	buildCmd.AddCommand(buildMSICmd)

	buildCmd.PersistentFlags().BoolVar(&noFrontend, "no-frontend", false, "Skip frontend build")
	buildWindowsCmd.Flags().BoolVar(&buildWindowsLocal, "local", false, "Build locally using mingw-w64 toolchain instead of fyne-cross")
	buildUpdaterCmd.Flags().BoolVar(&buildWindowsLocal, "local", false, "Build locally using mingw-w64 toolchain instead of fyne-cross")
}

func runBuildLinuxSo() error {
	fmt.Println("Building Linux Shared Library (libcore.so)...")
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Ensure certs are synced before building the library
	if err := copyCertsToCore(); err != nil {
		fmt.Printf("⚠️  Warning during cert copy: %v\n", err)
	}

	output := filepath.Join(rootDir, "flutter", "linux", "libcore.so")
	header := filepath.Join(rootDir, "flutter", "linux", "libcore.h")

	// Ensure target directory exists
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}

	if err := execCmd(rootDir, nil, "go", "build", "-buildmode=c-shared", "-o", output, "./mobile/ffi"); err != nil {
		return fmt.Errorf("failed to build libcore.so: %w", err)
	}

	fmt.Printf("✅ Build successful: %s\n", output)
	fmt.Printf("✅ Header generated: %s\n", header)
	return nil
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

	if err := execCmd(rootDir, nil, "go", "build", "-tags", "secrets_gen", "-o", output, "./cmd/discovery"); err != nil {
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

	if err := buildViewers(); err != nil {
		return fmt.Errorf("failed to build archive viewers: %w", err)
	}

	toolCmdDir := filepath.Join(rootDir, "cmd", "vpn-share-tool")

	// Build Go binary
	if err := execCmdFiltered(toolCmdDir, nil, "go", "build", "-tags", "secrets_gen", "-o", "vpn-share-tool"); err != nil {
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

	// Ensure the target directory for the AAR exists
	libsDir := filepath.Join(rootDir, "flutter", "android", "libs")
	if err := os.MkdirAll(libsDir, 0755); err != nil {
		return fmt.Errorf("failed to create libs directory: %w", err)
	}

	env := append(os.Environ(),
		"ANDROID_NDK_HOME="+androidNdkHome,
		"GOFLAGS=-mod=mod",
	)
	if err := execCmd(rootDir, env, "gomobile", "bind", "-target=android", "-androidapi", "21", "-o", "flutter/android/libs/core.aar", "github.com/soda92/vpn-share-tool/mobile"); err != nil {
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

	if err := buildViewers(); err != nil {
		return fmt.Errorf("failed to build archive viewers: %w", err)
	}

	// Build frontend
	if !noFrontend {
		if err := buildFrontendIn(filepath.Join(rootDir, "core", "debug_web")); err != nil {
			return err
		}
	} else {
		fmt.Println("Skipping frontend build.")
	}

	if buildWindowsLocal {
		fmt.Println("Building locally using mingw-w64 toolchain...")
		output := filepath.Join(rootDir, "dist", "vpn-share-tool.exe")
		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			return err
		}

		env := append(os.Environ(),
			"CGO_ENABLED=1",
			"GOOS=windows",
			"GOARCH=amd64",
			"CC=x86_64-w64-mingw32-gcc",
			"CXX=x86_64-w64-mingw32-g++",
		)

		if err := execCmd(rootDir, env, "fyne", "build", "-os", "windows", "--tags", "secrets_gen", "-o", output, "--src", "./cmd/vpn-share-tool"); err != nil {
			return fmt.Errorf("local mingw64 build failed: %w", err)
		}
		fmt.Printf("✅ Windows build successful: %s\n", output)
		return nil
	}

	if err := execCmd(rootDir, nil, "fyne-cross", "windows", "-image", "fyne-cross-windows:go1.26", "-arch", "amd64", "-tags", "secrets_gen", "--app-id", "vpn.share.tool", "./cmd/vpn-share-tool"); err != nil {
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

func runBuildUpdater() error {
	fmt.Println("Building Updater application (pure Go, console)...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	output := filepath.Join(rootDir, "dist", "updater.exe")
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}

	env := append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=windows",
		"GOARCH=amd64",
	)

	version, _, err := GetCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}
	fmt.Printf("Updater Build Version: %s\n", version)

	ldflags := fmt.Sprintf("-s -w -X main.UpdaterVersion=%s", version)

	if err := execCmd(rootDir, env, "go", "build", "-trimpath", "-ldflags="+ldflags, "-o", output, "./cmd/updater"); err != nil {
		return fmt.Errorf("updater build failed: %w", err)
	}

	fmt.Printf("✅ Updater build successful: %s\n", output)
	return nil
}

func buildViewers() error {
	fmt.Println("Building standalone archive viewers...")
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	embedDir := filepath.Join(rootDir, "core", "archive", "embed")
	if err := os.MkdirAll(embedDir, 0755); err != nil {
		return err
	}

	// 1. Build Linux viewer
	linuxOut := filepath.Join(embedDir, "viewer_linux")
	envLinux := append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	if err := execCmd(rootDir, envLinux, "go", "build", "-trimpath", "-ldflags=-s -w", "-o", linuxOut, "./cmd/viewer"); err != nil {
		return fmt.Errorf("failed to build linux viewer: %w", err)
	}
	fmt.Printf("✅ Linux viewer built: %s\n", linuxOut)

	// 2. Build Windows viewer
	windowsOut := filepath.Join(embedDir, "viewer_windows.exe")
	envWin := append(os.Environ(), "CGO_ENABLED=0", "GOOS=windows", "GOARCH=amd64")
	if err := execCmd(rootDir, envWin, "go", "build", "-trimpath", "-ldflags=-s -w", "-o", windowsOut, "./cmd/viewer"); err != nil {
		return fmt.Errorf("failed to build windows viewer: %w", err)
	}
	fmt.Printf("✅ Windows viewer built: %s\n", windowsOut)

	return nil
}

// generateSecretsGo reads .env and/or system environment variables,
// and writes common/secrets_gen.go so Go builds can embed telemetry secrets.
func generateSecretsGo() error {
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	envFile := filepath.Join(rootDir, ".env")
	envMap := make(map[string]string)

	// Try reading .env
	if data, err := os.ReadFile(envFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Strip quotes
			if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
				(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
				if len(val) >= 2 {
					val = val[1 : len(val)-1]
				}
			}
			envMap[key] = val
		}
	}

	getSecret := func(key string) string {
		if val, ok := envMap[key]; ok && val != "" {
			return val
		}
		return os.Getenv(key)
	}

	sentryDSN := getSecret("VITE_SENTRY_DSN")
	posthogKey := getSecret("VITE_POSTHOG_KEY")
	posthogHost := getSecret("VITE_POSTHOG_HOST")
	if posthogHost == "" {
		posthogHost = "https://us.i.posthog.com"
	}

	secretsFile := filepath.Join(rootDir, "common", "secrets_gen.go")
	content := fmt.Sprintf(`//go:build secrets_gen

package common

var (
	SentryDSN   = "%s"
	PosthogKey  = "%s"
	PosthogHost = "%s"
)
`, sentryDSN, posthogKey, posthogHost)

	// Ensure common directory exists
	if err := os.MkdirAll(filepath.Dir(secretsFile), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(secretsFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write secrets_gen.go: %w", err)
	}

	fmt.Println("✅ Generated common/secrets_gen.go from environment.")
	return nil
}

func runBuildMSI() error {
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	config, err := loadReleaseConfig()
	if err != nil {
		return fmt.Errorf("failed to load release config: %w", err)
	}
	minorVal := parseSuffixToInt(config.Version.Suffix)
	versionStr := fmt.Sprintf("%d.%d.0", config.Version.Counter, minorVal)
	fmt.Printf("Building MSI for version: %s\n", versionStr)

	// Locate source exe
	var exePath string
	candidates := []string{
		filepath.Join(rootDir, "dist", "vpn-share-tool.exe"),
		filepath.Join(rootDir, "fyne-cross", "bin", "windows-amd64", "vpn-share-tool.exe"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			exePath = c
			break
		}
	}

	if exePath == "" {
		return fmt.Errorf("could not find vpn-share-tool.exe. Please run 'go run ./dev build windows' first")
	}
	fmt.Printf("Source EXE: %s\n", exePath)

	wxsPath := filepath.Join(rootDir, "packaging", "wix", "vpn-share-tool.wxs")
	msiOutPath := filepath.Join(rootDir, "packaging", "wix", fmt.Sprintf("vpn-share-tool-%s.msi", versionStr))

	if runtime.GOOS == "windows" {
		candle := "candle.exe"
		light := "light.exe"

		_, errCandle := exec.LookPath(candle)
		_, errLight := exec.LookPath(light)

		wixBinDir := filepath.Join(rootDir, "packaging", "wix", "wix_bin")
		if errCandle != nil || errLight != nil {
			candleLocal := filepath.Join(wixBinDir, "candle.exe")
			lightLocal := filepath.Join(wixBinDir, "light.exe")
			if _, errC := os.Stat(candleLocal); errC == nil {
				if _, errL := os.Stat(lightLocal); errL == nil {
					candle = candleLocal
					light = lightLocal
					errCandle = nil
					errLight = nil
				}
			}
		}

		if errCandle != nil || errLight != nil {
			fmt.Println("WiX Toolset not found in PATH or wix_bin directory.")
			if err := downloadAndExtractWiX(wixBinDir); err != nil {
				return fmt.Errorf("failed to download WiX Toolset: %w", err)
			}
			candle = filepath.Join(wixBinDir, "candle.exe")
			light = filepath.Join(wixBinDir, "light.exe")
		}

		wixobjPath := filepath.Join(rootDir, "packaging", "wix", "vpn-share-tool.wixobj")
		defer os.Remove(wixobjPath)

		err = execCmd(rootDir, nil, candle,
			"-dVersion="+versionStr,
			"-dSourceExePath="+exePath,
			"-out", wixobjPath,
			wxsPath,
		)
		if err != nil {
			return fmt.Errorf("candle compilation failed: %w", err)
		}

		err = execCmd(rootDir, nil, light,
			"-ext", "WixUIExtension",
			"-out", msiOutPath,
			wixobjPath,
		)
		if err != nil {
			return fmt.Errorf("light linking failed: %w", err)
		}

	} else {
		wixl, err := exec.LookPath("wixl")
		if err != nil {
			return fmt.Errorf("wixl not found in PATH. Please install 'msitools' (pacman -S msitools / yay -S msitools) to build MSI on Linux")
		}

		err = execCmd(rootDir, nil, wixl,
			"-o", msiOutPath,
			"-D", "Version="+versionStr,
			"-D", "SourceExePath="+exePath,
			wxsPath,
		)
		if err != nil {
			return fmt.Errorf("wixl build failed: %w", err)
		}
	}

	fmt.Printf("✅ MSI built successfully: %s\n", msiOutPath)
	return nil
}

func downloadAndExtractWiX(destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	zipPath := filepath.Join(destDir, "wix.zip")
	defer os.Remove(zipPath)

	fmt.Println("Downloading WiX Toolset binaries from GitHub...")
	url := "https://github.com/wixtoolset/wix3/releases/download/wix3112rtm/wix311-binaries.zip"

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	out.Close()

	fmt.Println("Extracting WiX Toolset binaries...")
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func parseSuffixToInt(suffix string) int {
	if suffix == "" {
		return 0
	}
	val := 0
	for i := 0; i < len(suffix); i++ {
		char := suffix[i]
		if char >= 'a' && char <= 'z' {
			val = val*26 + int(char-'a'+1)
		} else if char >= 'A' && char <= 'Z' {
			val = val*26 + int(char-'A'+1)
		}
	}
	return val
}
