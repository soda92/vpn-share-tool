package proxy

import (
	"fmt"
	"log"
	"net"
)

// isPortAvailable checks if a TCP port is available to be listened on.
func isPortAvailable(port int) bool {
	address := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func SelectAvailablePort(requestedPort int, startPort int, proxiesCount int) (int, error) {
	remotePort := 0

	// Try requested port first
	if requestedPort > 0 {
		isUsed := false
		ProxiesLock.RLock()
		for _, p := range Proxies {
			if p.RemotePort == requestedPort {
				isUsed = true
				break
			}
		}
		ProxiesLock.RUnlock()
		
		if !isUsed && isPortAvailable(requestedPort) {
			remotePort = requestedPort
		} else {
			log.Printf("Requested port %d is not available or in use, falling back to auto-selection.", requestedPort)
		}
	}

	if remotePort == 0 {
		port := startPort
		ProxiesLock.Lock()
		defer ProxiesLock.Unlock()
		
		for {
			isUsed := false
			for _, p := range Proxies {
				if p.RemotePort == port {
					isUsed = true
					break
				}
			}
			// Release lock temporarily to check port availability (avoid holding lock during system call)
			// But wait, checking usage requires lock. 
			// We can check usage in memory quickly.
			// isPortAvailable might take a few ms.
			// It is safer to keep lock if we want to guarantee no race condition within our own app,
			// but we can't guarantee other apps don't take it.
			
			if !isUsed && isPortAvailable(port) {
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
