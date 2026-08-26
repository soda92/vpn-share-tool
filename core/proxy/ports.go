package proxy

import (
	"fmt"
	"log"
	"net"
)

// isPortAvailable checks if a TCP port is available to be listened on for IPv4.
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func SelectAvailablePort(requestedPort int, startPort int, proxiesCount int) (int, error) {
	remotePort := 0

	isPortUsed := func(port int) bool {
		if APIPort > 0 && port == APIPort {
			return true
		}
		for _, p := range Proxies {
			if p.RemotePort == port {
				return true
			}
		}
		return false
	}

	// Try requested port first
	if requestedPort > 0 {
		ProxiesLock.RLock()
		inUse := isPortUsed(requestedPort)
		ProxiesLock.RUnlock()

		if !inUse && isPortAvailable(requestedPort) {
			remotePort = requestedPort
		} else {
			log.Printf("Requested port %d is not available or in use (APIPort: %d), falling back to auto-selection.", requestedPort, APIPort)
		}
	}

	if remotePort == 0 {
		port := startPort
		ProxiesLock.Lock()
		defer ProxiesLock.Unlock()

		for {
			if !isPortUsed(port) && isPortAvailable(port) {
				remotePort = port
				break
			}
			port++
			if port > startPort+1000 {
				return 0, fmt.Errorf("could not find an available port")
			}
		}
	}
	return remotePort, nil
}
