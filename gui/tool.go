package gui

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// getSuitableInterfaces iterates through network interfaces and returns a slice of suitable interfaces for LAN communication.
func getSuitableInterfaces() []net.Interface {
	var suitableInterfaces []net.Interface
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Failed to get network interfaces: %v", err)
		return nil
	}

	for _, i := range interfaces {
		// Skip docker, down, loopback, and non-multicast interfaces
		if strings.Contains(i.Name, "docker") || (i.Flags&net.FlagUp == 0) || (i.Flags&net.FlagLoopback != 0) || (i.Flags&net.FlagMulticast == 0) {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ip := ipnet.IP.To4(); ip != nil && strings.HasPrefix(ip.String(), "192.168.") {
					suitableInterfaces = append(suitableInterfaces, i)
					break // Found a suitable IP on this interface, move to the next interface
				}
			}
		}
	}
	return suitableInterfaces
}

// getLanIPs finds all suitable local private IP addresses of the machine.
func getLanIPs() ([]string, error) {
	var ips []string
	suitableInterfaces := getSuitableInterfaces()

	for _, i := range suitableInterfaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ip := ipnet.IP.To4(); ip != nil && strings.HasPrefix(ip.String(), "192.168.") {
					ips = append(ips, ip.String())
				}
			}
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf(l("noSuitableLanIpFound"))
	}
	return ips, nil
}

func isURLReachable(targetURL string) bool {
	client := http.Client{
		Timeout: 3 * time.Second,
	}
	// Use HEAD request for efficiency
	req, err := http.NewRequest("HEAD", targetURL, nil)
	if err != nil {
		log.Printf("Discovery: could not create request for %s: %v", targetURL, err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Discovery: URL %s is not reachable: %v", targetURL, err)
		return false
	}
	defer resp.Body.Close()

	// Any status code (even 401/403) means the server is alive.
	log.Printf("Discovery: URL %s is reachable with status %d", targetURL, resp.StatusCode)
	return true
}
