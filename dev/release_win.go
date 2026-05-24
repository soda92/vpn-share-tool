package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
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
	Use:   "release-windows",
	Short: "Release the Windows binary to the share folder",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRelease()
	},
}

var releaseUpdater bool

func init() {
	releaseCmd.Flags().BoolVar(&releaseUpdater, "updater", false, "Release the updater instead of the main application")
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
				WindowsPath: `\\192.168.1.81\文件共享\VPN共享工具`,
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

// IncrementSuffix handles 'a' -> 'b', 'z' -> 'aa', 'aa' -> 'ab'
func IncrementSuffix(s string) string {
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
		return IncrementSuffix(string(runes[:last])) + "a"
	}

	// If it was just "z", return "aa"
	return "aa"
}

// BumpVersion calculates the next version, updates the config file, and returns the new version string and config.
func BumpVersion() (string, *ReleaseConfig, error) {
	config, err := loadReleaseConfig()
	if err != nil {
		return "", nil, err
	}

	today := time.Now().Format("2006-01-02")

	if config.Version.CurrentDate != today {
		// New day, increment counter, reset suffix
		config.Version.Counter++
		config.Version.Suffix = "a"
		config.Version.CurrentDate = today
	} else {
		// Same day, increment suffix
		config.Version.Suffix = IncrementSuffix(config.Version.Suffix)
	}

	if err := saveReleaseConfig(config); err != nil {
		return "", nil, fmt.Errorf("failed to save bumped version: %w", err)
	}

	versionStr := fmt.Sprintf("v%d%s", config.Version.Counter, config.Version.Suffix)
	return versionStr, config, nil
}

// GetCurrentVersion returns the current version string and config without modification.
func GetCurrentVersion() (string, *ReleaseConfig, error) {
	config, err := loadReleaseConfig()
	if err != nil {
		return "", nil, err
	}
	versionStr := fmt.Sprintf("v%d%s", config.Version.Counter, config.Version.Suffix)
	return versionStr, config, nil
}

func incrementVersion(config *ReleaseConfig) string {
	// Deprecated in favor of BumpVersion logic, but keeping for compatibility if needed internally
	v, _, err := BumpVersion()
	if err != nil {
		// Or handle more gracefully depending on expected usage.
		panic(fmt.Sprintf("failed to bump version in incrementVersion: %v", err))
	}
	return v
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

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

func zipFile(src, dst string) error {
	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(dst)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	archive, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	fileToZip, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fileToZip.Close()

	info, err := fileToZip.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = "vpn-share-tool.exe"
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	if _, err = io.Copy(writer, fileToZip); err != nil {
		return err
	}

	// Close zipWriter first to write central directory structure to archive
	if err := zipWriter.Close(); err != nil {
		return err
	}

	// Sync file system buffers for archive file
	return archive.Sync()
}

func runRelease() error {
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	var filePrefix string
	var srcPath string

	if releaseUpdater {
		filePrefix = "vpn-share-tool"
		srcPath = filepath.Join(rootDir, "dist", "updater.exe")
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			srcPath = filepath.Join(rootDir, "fyne-cross", "bin", "windows-amd64", "updater.exe")
			if _, err := os.Stat(srcPath); os.IsNotExist(err) {
				return fmt.Errorf("updater source file not found. Please run 'go run dev.go build updater' first")
			}
		}
	} else {
		filePrefix = "vpn-share-tool-app"
		srcPath = filepath.Join(rootDir, "dist", "vpn-share-tool.exe")
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			srcPath = filepath.Join(rootDir, "fyne-cross", "bin", "windows-amd64", "vpn-share-tool.exe")
			if _, err := os.Stat(srcPath); os.IsNotExist(err) {
				return fmt.Errorf("application source file not found. Please run 'go run dev.go build windows --local' first")
			}
		}
	}

	// Get CURRENT version (bumped during build)
	versionStr, config, err := GetCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

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

	// 1. Publish raw EXE (ONLY for the updater binary, which is small, so old clients can download it)
	if releaseUpdater {
		exeFilename := fmt.Sprintf("%s_%s.exe", filePrefix, versionStr)
		destExePath := filepath.Join(sharePath, exeFilename)

		fmt.Printf("Publishing EXE release %s...\n", versionStr)
		fmt.Printf("Source: %s\n", srcPath)
		fmt.Printf("Dest:   %s\n", destExePath)

		if err := copyFile(srcPath, destExePath); err != nil {
			return fmt.Errorf("failed to copy exe file: %w", err)
		}
	}

	// 2. Publish ZIP package (for both updater and app)
	zipFilename := fmt.Sprintf("%s_%s.zip", filePrefix, versionStr)
	destZipPath := filepath.Join(sharePath, zipFilename)

	fmt.Printf("Publishing ZIP release %s...\n", versionStr)
	fmt.Printf("Source: %s\n", srcPath)
	fmt.Printf("Dest:   %s\n", destZipPath)

	if err := zipFile(srcPath, destZipPath); err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}

	// Only compute and upload hash for the main application (not the updater)
	if !releaseUpdater {
		zipSha256Path := destZipPath + ".sha256"

		// Delete old .sha256 file first if it exists
		if err := os.Remove(zipSha256Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove old zip sha256 file: %w", err)
		}

		zipHash, err := computeSHA256(destZipPath)
		if err != nil {
			return fmt.Errorf("failed to compute zip sha256: %w", err)
		}

		if err := os.WriteFile(zipSha256Path, []byte(zipHash), 0644); err != nil {
			return fmt.Errorf("failed to write zip sha256 file: %w", err)
		}
		fmt.Printf("ZIP SHA256: %s -> %s\n", zipHash, zipSha256Path)
	}

	fmt.Println("✅ Published successfully.")
	return nil
}

func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
