package core

import (
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

	if info.Version == Version {
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
	exeName := filepath.Base(currentExe)
	newExe := filepath.Join(exeDir, exeName+".new")

	// Get restart args
	var args []string
	if RestartArgsProvider != nil {
		args = RestartArgsProvider()
	}
	argsStr := strings.Join(args, " ")

	// Download
	log.Printf("Downloading %s to %s...", info.URL, newExe)
	resp, err := http.Get(DiscoveryServerURL + info.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(newExe)
	if err != nil {
		return err
	}
	
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}
	out.Close()

	// Make executable
	os.Chmod(newExe, 0755)

	if runtime.GOOS == "windows" {
		// Windows: Use batch script to handle file locking
		batPath := filepath.Join(exeDir, "update.bat")
		batContent := fmt.Sprintf(`@echo off
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
		// Run batch script detached
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
	
	// Rename current to old
	if err := os.Rename(currentExe, oldExe); err != nil {
		return fmt.Errorf("failed to rename current exe: %w", err)
	}

	// Rename new to current
	if err := os.Rename(newExe, currentExe); err != nil {
		// Try to rollback
		os.Rename(oldExe, currentExe)
		return fmt.Errorf("failed to rename new exe: %w", err)
	}
	
	log.Printf("Restarting process...")
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
