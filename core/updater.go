package core

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var RestartArgsProvider func() []string

func SetRestartArgsProvider(f func() []string) {
	RestartArgsProvider = f
}

// TriggerUpdate checks for updates and performs a silent update if available.
// It returns true if an update was performed (and the app should likely exit/restart).
func TriggerUpdate() (bool, error) {
	info, err := CheckForUpdates()
	if err != nil {
		return false, err
	}

	if !IsNewerVersion(Version, info.Version) {
		return false, nil // No update
	}

	log.Printf("Update available: %s -> %s. Applying update...", Version, info.Version)

	if err := ApplyUpdate(info); err != nil {
		return false, err
	}

	return true, nil
}

// ApplyUpdate downloads and applies the update, then restarts the application.
// This function should terminate the process on success.
func ApplyUpdate(info *UpdateInfo) error {
	currentExe, err := os.Executable()
	if err != nil {
		return err
	}

	exeDir := filepath.Dir(currentExe)

	// Download new release (app version) directly as vpn-share-tool.exe.new or similar
	newExe := filepath.Join(exeDir, filepath.Base(currentExe)+".new")
	isZip := strings.HasSuffix(info.URL, ".zip")
	downloadDest := newExe
	if isZip {
		downloadDest = newExe + ".zip"
	}

	log.Printf("Downloading %s to %s...", info.URL, downloadDest)
	client := GetHTTPClient()
	resp, err := client.Get(DiscoveryServerURL + info.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: server returned status %d", resp.StatusCode)
	}

	out, err := os.Create(downloadDest)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(downloadDest)
		return err
	}
	out.Close()

	if resp.ContentLength > 0 {
		fi, err := os.Stat(downloadDest)
		if err != nil {
			os.Remove(downloadDest)
			return fmt.Errorf("failed to stat downloaded file: %w", err)
		}
		if fi.Size() != resp.ContentLength {
			os.Remove(downloadDest)
			return fmt.Errorf("download incomplete: expected %d bytes, got %d", resp.ContentLength, fi.Size())
		}
	}

	if err := verifySHA256(downloadDest, info.Sha256); err != nil {
		os.Remove(downloadDest)
		return fmt.Errorf("download verification failed: %w", err)
	}

	if isZip {
		log.Printf("Extracting %s to %s...", downloadDest, newExe)
		if err := unzipFile(downloadDest, newExe); err != nil {
			os.Remove(downloadDest)
			os.Remove(newExe)
			return fmt.Errorf("failed to unzip update: %w", err)
		}
		os.Remove(downloadDest)
	}

	os.Chmod(newExe, 0755)

	if runtime.GOOS == "windows" {
		batPath := filepath.Join(exeDir, "update.bat")
		exeName := filepath.Base(currentExe)

		var args []string
		if RestartArgsProvider != nil {
			args = RestartArgsProvider()
		}
		argsStr := strings.Join(args, " ")

		batContent := fmt.Sprintf(`@echo off
pushd "%%~dp0"
set /a retries=0
:loop
set /a retries+=1
if %%retries%% geq 30 goto fail
timeout /t 1 >nul
move /y "%s" "%s"
if errorlevel 1 goto loop
start "" "%s" %s
exit

:fail
echo Failed to update after 30 retries.
pause
`, filepath.Base(newExe), exeName, exeName, argsStr)

		if err := os.WriteFile(batPath, []byte(batContent), 0755); err != nil {
			return fmt.Errorf("failed to create update script: %w", err)
		}

		log.Printf("Starting update script and exiting...")
		cmd := exec.Command("cmd", "/c", "start", "/min", "cmd", "/c", "update.bat")
		cmd.Dir = exeDir
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start update script: %w", err)
		}

		os.Exit(0)
		return nil
	}

	// Linux/Unix: Rename running file and restart
	oldExe := currentExe + ".old"
	if err := os.Rename(currentExe, oldExe); err != nil {
		return fmt.Errorf("failed to rename current exe: %w", err)
	}

	if err := os.Rename(newExe, currentExe); err != nil {
		os.Rename(oldExe, currentExe)
		return fmt.Errorf("failed to rename new exe: %w", err)
	}

	log.Printf("Restarting process...")
	var args []string
	if RestartArgsProvider != nil {
		args = RestartArgsProvider()
	}
	startNewProcess(currentExe, args)
	os.Exit(0)
	return nil
}

func startNewProcess(exePath string, args []string) {
	// Simple restart logic: start independent process
	var attr os.ProcAttr
	attr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
	// os.Args[0] is usually the program name.
	// We should construct new args.
	// os.StartProcess expects argv to include the program name as first element?
	// Yes, argv[0].
	newArgs := append([]string{exePath}, args...)

	_, err := os.StartProcess(exePath, newArgs, &attr)
	if err != nil {
		log.Printf("Failed to restart process: %v", err)
	}
}

func getFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
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

func verifySHA256(filePath, expectedHash string) error {
	if expectedHash == "" {
		return nil // Skip validation if server didn't provide a hash (e.g. legacy/local test)
	}

	actualHash, err := getFileSHA256(filePath)
	if err != nil {
		return err
	}

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("hash mismatch! expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

func unzipFile(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	var targetFile *zip.File
	for _, f := range r.File {
		// Check for the executable inside zip
		if filepath.Base(f.Name) == "vpn-share-tool.exe" {
			targetFile = f
			break
		}
	}

	// Fallback to the first file if we can't find one with the exact name
	if targetFile == nil && len(r.File) > 0 {
		targetFile = r.File[0]
	}

	if targetFile == nil {
		return fmt.Errorf("no files found in zip archive")
	}

	rc, err := targetFile.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}
