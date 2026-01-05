package proxy

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/models"
)

type ProxyConfigItem struct {
	OriginalURL string               `json:"original_url"`
	RemotePort  int                  `json:"remote_port"`
	Settings    models.ProxySettings `json:"settings"`
	// Legacy fields for migration
	LegacyEnableDebug   bool `json:"enable_debug,omitempty"`
	LegacyEnableCaptcha bool `json:"enable_captcha,omitempty"`
}

func getConfigFile() (string, error) {
	if debug.DebugStoragePath != "" {
		// Ensure the directory exists
		if err := os.MkdirAll(debug.DebugStoragePath, 0755); err != nil {
			return "", err
		}
		return filepath.Join(debug.DebugStoragePath, "proxies.json"), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "vpn-share-tool")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "proxies.json"), nil
}

func SaveProxies() {
	file, err := getConfigFile()
	if err != nil {
		log.Printf("Failed to get config file path: %v", err)
		return
	}

	ProxiesLock.RLock()
	defer ProxiesLock.RUnlock()

	var config []ProxyConfigItem
	for _, p := range Proxies {
		config = append(config, ProxyConfigItem{
			OriginalURL: p.OriginalURL,
			RemotePort:  p.RemotePort,
			Settings:    p.Settings,
		})
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal proxy config: %v", err)
		return
	}

	if err := os.WriteFile(file, data, 0644); err != nil {
		log.Printf("Failed to save proxy config: %v", err)
	}
}

func LoadProxies() {
	file, err := getConfigFile()
	if err != nil {
		log.Printf("Failed to get config file path: %v", err)
		return
	}

	data, err := os.ReadFile(file)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Failed to read proxy config: %v", err)
		}
		return
	}

	var config []ProxyConfigItem
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Failed to unmarshal proxy config: %v", err)
		return
	}

	log.Printf("Loading %d proxies from config...", len(config))
	for _, item := range config {
		log.Printf("Restoring proxy: %s -> :%d", item.OriginalURL, item.RemotePort)
		// We use the new requestedPort parameter (0 for now, will update ShareUrlAndGetProxy next)
		proxy, err := ShareUrlAndGetProxy(item.OriginalURL, item.RemotePort)
		if err != nil {
			log.Printf("Failed to restore proxy for %s: %v", item.OriginalURL, err)
			continue
		}
		
		// Restore settings
		// Check if Settings is populated, otherwise try legacy
		if item.Settings == (models.ProxySettings{}) {
			// Zero value settings, check if we should migrate legacy
			if item.LegacyEnableDebug {
				// Legacy EnableDebug mapped to what? Maybe EnableContentMod?
				// Since legacy debug script is controlled by EnableDebug flag in RunPipeline...
				// But we removed EnableDebug from SharedProxy model!
				// So we must map it to Settings.EnableContentMod? Or ignore it?
				// The prompt said "Legacy Debug Script" is enabled by "EnableDebug" property in Settings? No.
				// In RunPipeline:
				/*
					// 2. Debug Script (Legacy Flag)
					if ctx.Proxy.GetEnableDebug() { ... }
				*/
				// But I removed GetEnableDebug() method and field!
				// So RunPipeline is broken now. I need to fix it.
			}
			
			// Map legacy to new settings
			proxy.Settings.EnableContentMod = item.LegacyEnableCaptcha || item.LegacyEnableDebug
			proxy.Settings.EnableUrlRewrite = true // Default true
		} else {
			proxy.Settings = item.Settings
		}
	}
}
