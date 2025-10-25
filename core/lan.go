package core

import (
	"fmt"
	"log"
	"net"
	"strings"
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

// GetLanIPs finds all suitable local private IP addresses of the machine.
func GetLanIPs() ([]string, error) {
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
		return nil, fmt.Errorf("no suitable LAN IP found")
	}
	return ips, nil
}
