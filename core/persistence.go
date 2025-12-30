package core

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

type ProxyConfigItem struct {
	OriginalURL   string `json:"original_url"`
	RemotePort    int    `json:"remote_port"`
	EnableDebug   bool   `json:"enable_debug"`
	EnableCaptcha bool   `json:"enable_captcha"`
}

func getConfigFile() (string, error) {
	if DebugStoragePath != "" {
		// Ensure the directory exists
		if err := os.MkdirAll(DebugStoragePath, 0755); err != nil {
			return "", err
		}
		return filepath.Join(DebugStoragePath, "proxies.json"), nil
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
			OriginalURL:   p.OriginalURL,
			RemotePort:    p.RemotePort,
			EnableDebug:   p.EnableDebug,
			EnableCaptcha: p.EnableCaptcha,
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
		proxy.SetEnableDebug(item.EnableDebug)
		proxy.SetEnableCaptcha(item.EnableCaptcha)
	}
}
