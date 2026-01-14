package register

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/soda92/vpn-share-tool/core/debug"
)

type DiscoveryCache struct {
	LastServerIP string `json:"last_server_ip"`
}

func getDiscoveryCacheFile() (string, error) {
	if debug.DebugStoragePath != "" {
		if err := os.MkdirAll(debug.DebugStoragePath, 0755); err != nil {
			return "", err
		}
		return filepath.Join(debug.DebugStoragePath, "discovery_cache.json"), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "vpn-share-tool")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "discovery_cache.json"), nil
}

func loadDiscoveryCache() string {
	file, err := getDiscoveryCacheFile()
	if err != nil {
		return ""
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}

	var cache DiscoveryCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return ""
	}
	return cache.LastServerIP
}

func saveDiscoveryCache(ip string) {
	file, err := getDiscoveryCacheFile()
	if err != nil {
		log.Printf("Failed to get cache file path: %v", err)
		return
	}

	cache := DiscoveryCache{LastServerIP: ip}
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}

	if err := os.WriteFile(file, data, 0644); err != nil {
		log.Printf("Failed to save discovery cache: %v", err)
	}
}
