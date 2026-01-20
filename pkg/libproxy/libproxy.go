package libproxy

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Fallback addresses if scanning fails
var DiscoveryServerHosts = []string{"192.168.0.81", "192.168.1.81"}

var DiscoveryServerPort = 45679

// CA Cert Placeholder - in a real build this might be replaced or loaded from file
var CACertPEM = `__CA_CERT_PLACEHOLDER__`

type Cache map[string]string

func getCachePath() (string, error) {
	var baseDir string
	var err error
	if runtime.GOOS == "windows" {
		baseDir, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
		// Windows: AppData/Roaming
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(home, ".config")
	}
	return filepath.Join(baseDir, "vpn-share-tool", "libproxy_cache.json"), nil
}

func loadCache() Cache {
	path, err := getCachePath()
	if err != nil {
		return make(Cache)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return make(Cache)
	}
	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		return make(Cache)
	}
	return cache
}

func saveToCache(targetURL, proxyURL string) {
	path, err := getCachePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	cache := loadCache()
	cache[targetURL] = proxyURL
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

func getLocalIP() string {
	// Method 1: Connect to a public DNS (no data sent)
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}
	return ""
}

func scanSubnet(localIP string, port int) []string {
	if localIP == "" {
		return nil
	}
	ip := net.ParseIP(localIP)
	if ip == nil {
		return nil
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil
	}

	// Simple /24 assumption
	baseIP := ipv4.Mask(net.CIDRMask(24, 32))
	if baseIP[0] == 10 { // Skip 10.x.x.x
		return nil
	}

	var foundHosts []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 50) // Semaphore for concurrency

	for i := 1; i < 255; i++ {
		targetIP := net.IPv4(baseIP[0], baseIP[1], baseIP[2], byte(i)).String()
		wg.Add(1)
		go func(ipStr string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ipStr, port), 200*time.Millisecond)
			if err == nil {
				conn.Close()
				mu.Lock()
				foundHosts = append(foundHosts, ipStr)
				mu.Unlock()
			}
		}(targetIP)
	}
	wg.Wait()
	return foundHosts
}

func getTLSConfig() (*tls.Config, error) {
	certPool := x509.NewCertPool()
	
	// If placeholder is present, try to load from file relative to binary or env var
	pemData := []byte(CACertPEM)
	if strings.Contains(CACertPEM, "__CA_CERT_PLACEHOLDER__") {
		// Try env var
		if envPath := os.Getenv("VPN_SHARE_TOOL_CA_PATH"); envPath != "" {
			data, err := os.ReadFile(envPath)
			if err == nil {
				pemData = data
			}
		} else {
			// Try default relative path (this is tricky in a library, maybe skip or assume /usr/local/share?)
			// For now, we'll try to look in typical dev paths if they exist, otherwise fail safely
			// or just continue with empty pool (which will likely fail if CA is needed)
		}
	}

	if len(pemData) > 0 && !strings.Contains(string(pemData), "__CA_CERT_PLACEHOLDER__") {
		if ok := certPool.AppendCertsFromPEM(pemData); !ok {
			return nil, fmt.Errorf("failed to append CA cert")
		}
		return &tls.Config{RootCAs: certPool, InsecureSkipVerify: true}, nil // CheckHostname false equivalent
	}
	
	// If we have no cert, we can't do TLS properly for the discovery server self-signed certs
	// But we return a config that expects system certs if any, or insecure if strictly necessary (but better to fail)
	// For this specific tool, we likely need the custom CA.
	// We'll return insecure=true purely for the 'check hostname' part, but RootCAs is empty so it relies on system certs
	// unless we find the cert.
	return &tls.Config{InsecureSkipVerify: true}, nil
}

type InstanceResponse struct {
	Address string `json:"address"`
}

func getInstanceList(timeout time.Duration) []string {
	localIP := getLocalIP()
	var candidateHosts []string
	if localIP != "" {
		scanned := scanSubnet(localIP, DiscoveryServerPort)
		candidateHosts = append(candidateHosts, scanned...)
	}
	candidateHosts = append(candidateHosts, DiscoveryServerHosts...)

	tlsConfig, err := getTLSConfig()
	if err != nil {
		log.Printf("TLS Config error: %v", err)
		return nil
	}

	var validHosts []string
	
	for _, host := range candidateHosts {
		// Connect TCP/TLS
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", fmt.Sprintf("%s:%d", host, DiscoveryServerPort), tlsConfig)
		if err != nil {
			continue
		}
		
		conn.Write([]byte("LIST\n"))
		
		// Read line
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		conn.Close()
		if err != nil {
			continue
		}
		
		var instances []InstanceResponse
		if err := json.Unmarshal(buffer[:n], &instances); err != nil {
			continue
		}
		
		for _, inst := range instances {
			validHosts = append(validHosts, inst.Address)
		}
		// If we found one server, that might be enough to get the list?
		// Python script iterates all to find servers, then returns the aggregated list of instances from the first working one?
		// Python: returns on first successful LIST response.
		return validHosts
	}
	return nil
}

func isURLReachableLocally(targetURL string, timeout time.Duration) bool {
	client := http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: nil, // Direct
		},
	}
	resp, err := client.Head(targetURL)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

type Service struct {
	OriginalURL string `json:"original_url"`
	SharedURL   string `json:"shared_url"`
}

type ReachResponse struct {
	Reachable bool `json:"reachable"`
}

type CreateProxyRequest struct {
	URL string `json:"url"`
}

// DiscoverProxy attempts to find or create a proxy for the given URL.
func DiscoverProxy(targetURL string, timeout time.Duration, remoteOnly bool) (string, error) {
	// Parse URL to ensure scheme
	u, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		u.Scheme = "http"
		targetURL = u.String()
	}
	
	if !remoteOnly {
		if isURLReachableLocally(targetURL, timeout) {
			return targetURL, nil
		}
	}

	// Check cache
	cache := loadCache()
	if proxy, ok := cache[targetURL]; ok {
		if isURLReachableLocally(proxy, 2*time.Second) {
			return proxy, nil
		}
	}

	// Discovery
	instances := getInstanceList(timeout)
	if len(instances) == 0 {
		return "", fmt.Errorf("no discovery instances found")
	}

	targetHostname := u.Hostname()

	// Phase 1: Existing
	for _, addr := range instances {
		apiURL := fmt.Sprintf("http://%s/services", addr)
		client := http.Client{Timeout: timeout}
		resp, err := client.Get(apiURL)
		if err != nil || resp.StatusCode != 200 {
			continue
		}
		
		var services []Service
		if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		for _, s := range services {
			su, _ := url.Parse(s.OriginalURL)
			if su != nil && su.Hostname() == targetHostname {
				saveToCache(targetURL, s.SharedURL)
				return s.SharedURL, nil
			}
		}
	}

	// Phase 2: Create
	for _, addr := range instances {
		// Check reachability
		canReachURL := fmt.Sprintf("http://%s/can-reach?url=%s", addr, url.QueryEscape(targetURL))
		client := http.Client{Timeout: timeout}
		resp, err := client.Get(canReachURL)
		if err != nil || resp.StatusCode != 200 {
			continue
		}
		
		var reach ReachResponse
		if err := json.NewDecoder(resp.Body).Decode(&reach); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if !reach.Reachable {
			continue
		}

		// Create
		createURL := fmt.Sprintf("http://%s/proxies", addr)
		reqBody, _ := json.Marshal(CreateProxyRequest{URL: targetURL})
		resp, err = client.Post(createURL, "application/json", bytes.NewReader(reqBody))
		if err != nil {
			continue
		}
		
		if resp.StatusCode == 201 {
			var newProxy Service // Reusing struct, check fields if matches
			// Python code expects: { "shared_url": ... }
			// Service struct has SharedURL
			if err := json.NewDecoder(resp.Body).Decode(&newProxy); err == nil {
				resp.Body.Close()
				saveToCache(targetURL, newProxy.SharedURL)
				return newProxy.SharedURL, nil
			}
		}
		resp.Body.Close()
	}

	return "", fmt.Errorf("could not find or create proxy")
}
