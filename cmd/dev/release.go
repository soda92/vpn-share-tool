package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

const releaseConfigName = "Release.toml"

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release the Windows binary to the share folder",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRelease()
	},
}

func init() {
	rootCmd.AddCommand(releaseCmd)
}

type ShareConfig struct {
	WindowsPath string
	LinuxPath   string
}

type VersionConfig struct {
	CurrentDate string
	Counter     int
	Suffix      string
}

type ReleaseConfig struct {
	Share   ShareConfig
	Version VersionConfig
}

func getReleaseConfigPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, releaseConfigName), nil
}

func loadReleaseConfig() (*ReleaseConfig, error) {
	path, err := getReleaseConfigPath()
	if err != nil {
		return nil, err
	}

	var config ReleaseConfig
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create default
		config = ReleaseConfig{
			Share: ShareConfig{
				WindowsPath: `Z:\VPN共享工具`,
				LinuxPath:   `/mnt/samba_share/VPN共享工具`,
			},
			Version: VersionConfig{
				CurrentDate: time.Now().Format("2006-01-02"),
				Counter:     22,
				Suffix:      "b",
			},
		}
		if err := saveReleaseConfig(&config); err != nil {
			return nil, err
		}
	} else {
		if _, err := toml.DecodeFile(path, &config); err != nil {
			return nil, err
		}
	}
	return &config, nil
}

func saveReleaseConfig(config *ReleaseConfig) error {
	path, err := getReleaseConfigPath()
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(config)
}

// incrementSuffix handles 'a' -> 'b', 'z' -> 'aa', 'aa' -> 'ab'
func incrementSuffix(s string) string {
	if s == "" {
		return "a"
	}
	runes := []rune(s)
	last := len(runes) - 1
	
	if runes[last] < 'z' {
		runes[last]++
		return string(runes)
	}
	
	// If last char is 'z', reset it to 'a' and increment previous
	runes[last] = 'a'
	if last > 0 {
		return incrementSuffix(string(runes[:last])) + "a"
	}
	
	// If it was just "z", return "aa"
	return "aa"
}

func incrementVersion(config *ReleaseConfig) string {
	today := time.Now().Format("2006-01-02")

	if config.Version.CurrentDate != today {
		// New day, increment counter, reset suffix
		config.Version.Counter++
		config.Version.Suffix = "a"
		config.Version.CurrentDate = today
	} else {
		// Same day, increment suffix
		config.Version.Suffix = incrementSuffix(config.Version.Suffix)
	}
	return fmt.Sprintf("v%d%s", config.Version.Counter, config.Version.Suffix)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(dst)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func runRelease() error {
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	// Source file (built by `build windows`)
	srcPath := filepath.Join(rootDir, "fyne-cross", "bin", "windows-amd64", "vpn-share-tool.exe")
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("source file not found: %s. Please run 'go run ./cmd/dev build windows' first", srcPath)
	}

	config, err := loadReleaseConfig()
	if err != nil {
		return fmt.Errorf("failed to load release config: %w", err)
	}

	// Calculate new version
	versionStr := incrementVersion(config)
	
	// Determine share path
	var sharePath string
	if runtime.GOOS == "windows" {
		sharePath = config.Share.WindowsPath
	} else {
		sharePath = config.Share.LinuxPath
	}

	if sharePath == "" {
		return fmt.Errorf("share path not configured for OS: %s", runtime.GOOS)
	}

	// Check if share path is reachable
	if _, err := os.Stat(sharePath); err != nil {
		return fmt.Errorf("share path is unreachable: %s (%w)", sharePath, err)
	}

	// Construct destination
	filename := fmt.Sprintf("vpn-share-tool_%s.exe", versionStr)
	destPath := filepath.Join(sharePath, filename)

	fmt.Printf("Publishing release %s...\n", versionStr)
	fmt.Printf("Source: %s\n", srcPath)
	fmt.Printf("Dest:   %s\n", destPath)

	if err := copyFile(srcPath, destPath); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Save updated config ONLY after successful copy
	if err := saveReleaseConfig(config); err != nil {
		return fmt.Errorf("failed to save updated release config: %w", err)
	}

	fmt.Println("✅ Published successfully.")
	return nil
}
