package core

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

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

	log.Printf("Update available: %s -> %s. Downloading...", Version, info.Version)

	currentExe, err := os.Executable()
	if err != nil {
		return false, err
	}

	newExe := currentExe + ".new"
	oldExe := currentExe + ".old"

	// Download
	resp, err := http.Get(DiscoveryServerURL + info.URL)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	out, err := os.Create(newExe)
	if err != nil {
		return false, err
	}
	
	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		return false, err
	}

	log.Printf("Download complete. Replacing binary...")

	// Rename current to old
	if err := os.Rename(currentExe, oldExe); err != nil {
		return false, fmt.Errorf("failed to rename current exe: %w", err)
	}

	// Rename new to current
	if err := os.Rename(newExe, currentExe); err != nil {
		// Try to rollback
		os.Rename(oldExe, currentExe)
		return false, fmt.Errorf("failed to rename new exe: %w", err)
	}
	
	// Make executable
	os.Chmod(currentExe, 0755)

	log.Printf("Update applied successfully. Restarting...")
	
	// Prepare restart (using a bat script or just exiting if we assume service manager/user restarts)
	// User mentioned "bat script... loop".
	// For now, let's just exit. If it's a dev tool, maybe user restarts manually?
	// But "auto update" implies restart.
	// Implementing the restart logic here is complex cross-platform.
	// Since the user focused on "silent self update", exiting with a special code might be enough if wrapped?
	// Or we can try to exec.Command(currentExe).Start() and then os.Exit(0).
	
	startNewProcess(currentExe)
	os.Exit(0)
	
	return true, nil
}

func startNewProcess(exePath string) {
	// Simple restart logic: start independent process
	// TODO: Add platform specific robust restart if needed
	var attr os.ProcAttr
	attr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
	_, err := os.StartProcess(exePath, os.Args, &attr)
	if err != nil {
		log.Printf("Failed to restart process: %v", err)
	}
}
