package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

	log.Printf("Loading %d proxies from config using differential port allocation...", len(config))

	type pendingProxy struct {
		item         ProxyConfigItem
		assignedPort int
	}

	pending := make([]pendingProxy, len(config))
	claimedPorts := make(map[int]bool)

	// Pass 1: Find all available ports corresponding to original requests
	for i, item := range config {
		pending[i] = pendingProxy{item: item, assignedPort: 0}
		if item.RemotePort > 0 {
			if !claimedPorts[item.RemotePort] && isPortAvailable(item.RemotePort) {
				pending[i].assignedPort = item.RemotePort
				claimedPorts[item.RemotePort] = true
			}
		}
	}

	restoreProxySettings := func(proxy *models.SharedProxy, item ProxyConfigItem) {
		if item.Settings == (models.ProxySettings{}) {
			proxy.Settings.EnableContentMod = item.LegacyEnableCaptcha || item.LegacyEnableDebug
			proxy.Settings.EnableDebugScript = item.LegacyEnableDebug
			proxy.Settings.EnableUrlRewrite = true
		} else {
			proxy.Settings = item.Settings
		}
	}

	// Pass 2 Phase 1: Restore proxies that claimed their original ports
	for _, entry := range pending {
		if entry.assignedPort > 0 {
			log.Printf("Restoring proxy on original port: %s -> :%d", entry.item.OriginalURL, entry.assignedPort)
			proxy, err := ShareUrlAndGetProxy(entry.item.OriginalURL, entry.assignedPort)
			if err != nil {
				log.Printf("Failed to restore proxy for %s on port %d: %v", entry.item.OriginalURL, entry.assignedPort, err)
				continue
			}
			restoreProxySettings(proxy, entry.item)
		}
	}

	// Pass 2 Phase 2: Re-assign ports for proxies whose original ports were unavailable
	for _, entry := range pending {
		if entry.assignedPort == 0 {
			log.Printf("Restoring proxy with new port (original :%d unavailable): %s", entry.item.RemotePort, entry.item.OriginalURL)
			proxy, err := ShareUrlAndGetProxy(entry.item.OriginalURL, 0)
			if err != nil {
				log.Printf("Failed to restore proxy for %s: %v", entry.item.OriginalURL, err)
				continue
			}
			restoreProxySettings(proxy, entry.item)
		}
	}
}

var (
	portHistory         = make(map[string]int)
	portHistoryLock     sync.RWMutex
	loadPortHistoryOnce sync.Once
)

func getPortHistoryFile() (string, error) {
	if debug.DebugStoragePath != "" {
		if err := os.MkdirAll(debug.DebugStoragePath, 0755); err != nil {
			return "", err
		}
		return filepath.Join(debug.DebugStoragePath, "port_history.json"), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "vpn-share-tool")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "port_history.json"), nil
}

func loadPortHistory() {
	file, err := getPortHistoryFile()
	if err != nil {
		return
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return
	}

	portHistoryLock.Lock()
	defer portHistoryLock.Unlock()
	_ = json.Unmarshal(data, &portHistory)
}

func savePortHistory() {
	file, err := getPortHistoryFile()
	if err != nil {
		return
	}

	portHistoryLock.RLock()
	data, err := json.MarshalIndent(portHistory, "", "  ")
	portHistoryLock.RUnlock()

	if err != nil {
		return
	}
	_ = os.WriteFile(file, data, 0644)
}

func normalizeTargetHost(u string) string {
	if !strings.Contains(u, "://") {
		u = "http://" + u
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if host == "localhost" {
		host = "127.0.0.1"
	}
	if port != "" {
		return fmt.Sprintf("%s:%s", host, port)
	}
	return host
}

func RecordPortHistory(rawURL string, port int) {
	if rawURL == "" || port <= 0 {
		return
	}
	loadPortHistoryOnce.Do(loadPortHistory)

	hostKey := normalizeTargetHost(rawURL)

	portHistoryLock.Lock()
	if portHistory == nil {
		portHistory = make(map[string]int)
	}
	portHistory[rawURL] = port
	if hostKey != "" {
		portHistory[hostKey] = port
	}
	portHistoryLock.Unlock()

	savePortHistory()
}

func GetHistoricalPort(rawURL string) int {
	loadPortHistoryOnce.Do(loadPortHistory)

	portHistoryLock.RLock()
	defer portHistoryLock.RUnlock()

	if port, ok := portHistory[rawURL]; ok && port > 0 {
		return port
	}

	hostKey := normalizeTargetHost(rawURL)
	if hostKey != "" {
		if port, ok := portHistory[hostKey]; ok && port > 0 {
			return port
		}
	}
	return 0
}
