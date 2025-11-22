package core

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	reachableClient http.Client
	once            sync.Once
)

func IsURLReachable(targetURL string) bool {
	once.Do(func() {
		reachableClient = http.Client{
			Timeout: 10 * time.Second,
		}
	})
	// Use HEAD request for efficiency
	req, err := http.NewRequest("HEAD", targetURL, nil)
	if err != nil {
		log.Printf("Discovery: could not create request for %s: %v", targetURL, err)
		return false
	}

	resp, err := reachableClient.Do(req)
	if err != nil {
		log.Printf("Discovery: URL %s is not reachable: %v", targetURL, err)
		return false
	}
	defer resp.Body.Close()

	// Any status code (even 401/403) means the server is alive.
	log.Printf("Discovery: URL %s is reachable with status %d", targetURL, resp.StatusCode)
	return true
}

// GetLocalIPs returns a list of local non-loopback IPv4 addresses.
// It prefers 192.168.x.x, but returns others if found.
func GetLocalIPs() ([]string, error) {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			ip = ip.To4()
			if ip == nil {
				continue
			}

			// Filter logic: We generally want private IPs.
			// For now, we just collect all valid IPv4.
			ips = append(ips, ip.String())
		}
	}
	return ips, nil
}

// ScanSubnet scans the /24 subnet of the given IP for a TCP service on the specified port.
// It skips 10.x.x.x networks as requested.
func ScanSubnet(localIP string, port string) []string {
	ip := net.ParseIP(localIP)
	if ip == nil {
		return nil
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}

	// Skip 10.x.x.x
	if ip4[0] == 10 {
		log.Printf("Skipping scan for 10.x.x.x network: %s", localIP)
		return nil
	}

	// Base IP (e.g., 192.168.1.)
	baseIP := fmt.Sprintf("%d.%d.%d.", ip4[0], ip4[1], ip4[2])

	var foundServers []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Scan 1 to 254
	semaphore := make(chan struct{}, 50) // Limit concurrency to 50

	for i := 1; i < 255; i++ {
		targetIP := fmt.Sprintf("%s%d", baseIP, i)
		// Skip self if needed, but sometimes server runs on same machine
		// if targetIP == localIP { continue }

		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			address := net.JoinHostPort(target, port)
			conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
			if err == nil {
				conn.Close()
				mu.Lock()
				foundServers = append(foundServers, target)
				mu.Unlock()
			}
		}(targetIP)
	}
	wg.Wait()
	return foundServers
}
