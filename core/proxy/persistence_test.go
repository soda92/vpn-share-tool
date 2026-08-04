package proxy

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/models"
)

func TestLoadProxies_DifferentialAllocation(t *testing.T) {
	// Setup temporary directory for config
	tmpDir, err := os.MkdirTemp("", "proxy_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origDebugPath := debug.DebugStoragePath
	debug.DebugStoragePath = tmpDir
	defer func() { debug.DebugStoragePath = origDebugPath }()

	// Select 3 consecutive ports
	start := 19500
	port1 := start
	port2 := start + 1
	port3 := start + 2

	// Block port1 on the system manually using a TCP listener
	ln1, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port1))
	if err != nil {
		t.Fatalf("Failed to block test port1 %d: %v", port1, err)
	}
	defer ln1.Close()

	// Prepare config: Site1 (wanted port1), Site2 (wanted port2), Site3 (wanted port3)
	configContent := fmt.Sprintf(`[
		{"original_url": "http://site1.test.local/app", "remote_port": %d},
		{"original_url": "http://site2.test.local/app", "remote_port": %d},
		{"original_url": "http://site3.test.local/app", "remote_port": %d}
	]`, port1, port2, port3)

	configFile := filepath.Join(tmpDir, "proxies.json")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Reset active proxies list
	ProxiesLock.Lock()
	oldProxies := Proxies
	Proxies = []*models.SharedProxy{}
	ProxiesLock.Unlock()

	defer func() {
		ProxiesLock.Lock()
		for _, p := range Proxies {
			if p.Cancel != nil {
				p.Cancel()
			}
		}
		Proxies = oldProxies
		ProxiesLock.Unlock()
	}()

	// Load proxies
	LoadProxies()

	ProxiesLock.RLock()
	defer ProxiesLock.RUnlock()

	if len(Proxies) != 3 {
		t.Fatalf("Expected 3 restored proxies, got %d", len(Proxies))
	}

	// Find assigned ports
	portMap := make(map[string]int)
	for _, p := range Proxies {
		portMap[p.OriginalURL] = p.RemotePort
	}

	// Site 2 should maintain its original requested port2
	if portMap["http://site2.test.local/app"] != port2 {
		t.Errorf("Expected site2 to keep original port %d, got %d", port2, portMap["http://site2.test.local/app"])
	}

	// Site 3 should maintain its original requested port3
	if portMap["http://site3.test.local/app"] != port3 {
		t.Errorf("Expected site3 to keep original port %d, got %d", port3, portMap["http://site3.test.local/app"])
	}

	// Site 1 should be assigned a new free port != port1, port2, port3
	site1Port := portMap["http://site1.test.local/app"]
	if site1Port == port1 || site1Port == port2 || site1Port == port3 {
		t.Errorf("Expected site1 to be assigned a new un-conflicting port, got %d", site1Port)
	}
}

func TestPortHistory_Reuse(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy_history_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origDebugPath := debug.DebugStoragePath
	debug.DebugStoragePath = tmpDir
	defer func() { debug.DebugStoragePath = origDebugPath }()

	testURL := "http://test-history-site.local:8080/app"

	// 1. Create a proxy with requestedPort 0
	proxy1, err := ShareUrlAndGetProxy(testURL, 0)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}
	assignedPort := proxy1.RemotePort

	// 2. Tear down / stop the proxy
	removeProxy(proxy1)

	// 3. Verify history recorded the port
	histPort := GetHistoricalPort(testURL)
	if histPort != assignedPort {
		t.Fatalf("Expected historical port %d, got %d", assignedPort, histPort)
	}

	// 4. Re-create the proxy with requestedPort 0 (simulating re-discovery when site comes back up)
	proxy2, err := ShareUrlAndGetProxy(testURL, 0)
	if err != nil {
		t.Fatalf("Failed to re-create proxy: %v", err)
	}
	defer removeProxy(proxy2)

	// 5. Verify it re-claimed the same port!
	if proxy2.RemotePort != assignedPort {
		t.Errorf("Expected re-created proxy to re-use port %d, but got %d", assignedPort, proxy2.RemotePort)
	}
}
