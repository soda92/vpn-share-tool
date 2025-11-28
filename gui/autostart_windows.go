//go:build windows

package gui

import (
	"log"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	appName    = "VPNShareTool"
)

func SetAutostart(enable bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		log.Printf("Failed to open registry key: %v", err)
		return
	}
	defer key.Close()

	// Clear old or existing entries
	names, err := key.ReadValueNames(0)
	if err == nil {
		for _, name := range names {
			if strings.Contains(strings.ToLower(name), "vpn-share-tool") || name == appName {
				err = key.DeleteValue(name)
				if err != nil && err != registry.ErrNotExist {
					log.Printf("Failed to delete autostart registry value %s: %v", name, err)
				}
			}
		}
	}

	if !enable {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Printf("Failed to get executable path: %v", err)
		return
	}

	// Add quotes around path to handle spaces, and add the -minimized flag.
	value := `"` + exePath + `" -minimized`
	err = key.SetStringValue(appName, value)
	if err != nil {
		log.Printf("Failed to set autostart registry value: %v", err)
	}
}

func IsAutostartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.READ)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(appName)
	return err == nil
}
