package register

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/soda92/vpn-share-tool/core/utils"
)

func Start(cfg Config) {
	currentIP := cfg.MyIP
	// Local Mode flag to avoid repeated signals/logs
	inLocalMode := false

	for {
		// 0. Try Cached IP first
		cachedIP := loadDiscoveryCache()
		if cachedIP != "" {
			conn, err := connectToDiscoveryServer(cachedIP, cfg)
			if err == nil {
				// Connected successfully
				inLocalMode = false
				manageConnection(conn, cfg)
				log.Printf("Connection to discovery server lost. Retrying...")
				time.Sleep(5 * time.Second)
				continue
			}
		}

		// 1. Discover Server IPs
		var candidateIPs []string
		var newIP string
		candidateIPs, newIP = discoverServers(currentIP, cfg)

		// Update currentIP if discovery found a local IP
		if newIP != "" && newIP != currentIP {
			currentIP = newIP
			if cfg.SetMyIP != nil {
				cfg.SetMyIP(currentIP)
			}
		}

		var conn net.Conn
		var err error
		var connectedIP string

		for _, ip := range candidateIPs {
			conn, err = connectToDiscoveryServer(ip, cfg)
			if err == nil {
				connectedIP = ip
				break
			}
		}

		if err != nil {
			log.Printf("Failed to connect to any discovery server.")

			// Local IP Mode Logic
			if !inLocalMode && currentIP != "" {
				log.Printf("Entering Local IP Mode (no discovery server). Using IP: %s", currentIP)
				// Ensure it is set
				if cfg.SetMyIP != nil {
					cfg.SetMyIP(currentIP)
				}
				// Signal the app to start
				if cfg.IPReadyChan != nil {
					// Use a non-blocking send or separate goroutine to avoid blocking if channel is full/unbuffered
					// But usually this is a buffered channel or consumer is ready.
					// Assuming safe to send.
					select {
					case cfg.IPReadyChan <- currentIP:
					default:
						log.Println("IPReadyChan blocked, skipping signal in Local Mode")
					}
				}
				inLocalMode = true
			}

			log.Printf("Retrying in 5 seconds.")
			time.Sleep(5 * time.Second)
			continue
		}

		// Connected successfully
		inLocalMode = false
		saveDiscoveryCache(connectedIP)

		manageConnection(conn, cfg)

		log.Printf("Connection to discovery server lost. Retrying...")
		time.Sleep(5 * time.Second)
	}
}

func discoverServers(myIP string, cfg Config) ([]string, string) {
	var candidateIPs []string
	detectedIP := myIP

	if detectedIP != "" {
		log.Printf("MyIP is set to %s. Scanning subnet...", detectedIP)
		found := utils.ScanSubnet(detectedIP, cfg.DiscoverySrvPort)
		if len(found) > 0 {
			log.Printf("Found servers via scanning: %v", found)
			candidateIPs = append(candidateIPs, found...)
		}
	} else {
		// If MyIP is not set, try to detect local IPs (Desktop mode)
		log.Println("MyIP not set. Attempting to detect local IPs...")
		localIPs, err := utils.GetLocalIPs()
		if err == nil && len(localIPs) > 0 {
			for _, ip := range localIPs {
				// Heuristic: Prefer 192.168.x.x for setting MyIP initially if finding nothing else
				if strings.HasPrefix(ip, "192.168.") && detectedIP == "" {
					detectedIP = ip
				}

				log.Printf("Scanning subnet of %s...", ip)
				found := utils.ScanSubnet(ip, cfg.DiscoverySrvPort)
				if len(found) > 0 {
					log.Printf("Found servers via scanning %s: %v", ip, found)
					candidateIPs = append(candidateIPs, found...)
				}
			}
			// If we still haven't set MyIP but found IPs, just pick the first one
			if detectedIP == "" && len(localIPs) > 0 {
				detectedIP = localIPs[0]
			}
		}
	}

	// Append hardcoded fallbacks at the end
	candidateIPs = append(candidateIPs, cfg.FallbackServerIPs...)

	return candidateIPs, detectedIP
}

func connectToDiscoveryServer(ip string, cfg Config) (net.Conn, error) {
	var serverAddr string
	// If ip already has port, don't add it again
	if strings.Contains(ip, ":") {
		serverAddr = ip
	} else {
		serverAddr = net.JoinHostPort(ip, cfg.DiscoverySrvPort)
	}

	log.Printf("Trying to connect to discovery server at %s...", serverAddr)

	// Prepare TLS config
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(cfg.RootCACert)
	tlsConfig := &tls.Config{RootCAs: caCertPool}
	dialer := &net.Dialer{Timeout: 2 * time.Second}

	// Try TLS first
	conn, err := tls.DialWithDialer(dialer, "tcp", serverAddr, tlsConfig)
	if err == nil {
		log.Printf("Connected to discovery server at %s (TLS)", serverAddr)
		host, _, _ := net.SplitHostPort(serverAddr)
		if cfg.UpdateDiscoveryURL != nil {
			url := fmt.Sprintf("https://%s:8080", host)
			cfg.UpdateDiscoveryURL(url)
		}
		return conn, nil
	}
	return nil, err
}

func manageConnection(conn net.Conn, cfg Config) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)

	// 1. Initial Registration
	registerMsg := fmt.Sprintf("REGISTER %d %s\n", cfg.APIPort, cfg.Version)
	if _, err := conn.Write([]byte(registerMsg)); err != nil {
		log.Printf("Failed to send REGISTER command: %v", err)
		return
	}

	if !scanner.Scan() {
		log.Printf("Did not receive response from server after REGISTER.")
		return
	}
	response := scanner.Text()
	parts := strings.Split(response, " ")
	if len(parts) == 2 && parts[0] == "OK" {
		// Server confirmed our IP. We can trust it, or keep our own.
		// For now, let's respect the server's view as it's the source of truth for the network.
		serverDetectedIP := parts[1]
		if cfg.MyIP != serverDetectedIP {
			log.Printf("Server sees us as %s (Local was %s). Updating.", serverDetectedIP, cfg.MyIP)
			if cfg.SetMyIP != nil {
				cfg.SetMyIP(serverDetectedIP)
			}
		}
		log.Printf("Successfully registered with discovery server. My IP is %s", serverDetectedIP)
		if cfg.IPReadyChan != nil {
			// Signal that the IP is ready
			select {
			case cfg.IPReadyChan <- serverDetectedIP:
			default:
			}
		}
	} else {
		log.Printf("Failed to register with discovery server, response: %s.", response)
		return
	}

	// 2. Heartbeat Loop
	heartbeatTicker := time.NewTicker(5 * time.Second)
	defer heartbeatTicker.Stop()

	for range heartbeatTicker.C {
		heartbeatMsg := fmt.Sprintf("HEARTBEAT %d\n", cfg.APIPort)
		if _, err := conn.Write([]byte(heartbeatMsg)); err != nil {
			log.Printf("Failed to send HEARTBEAT: %v", err)
			return
		}

		// Wait for and process server response
		if !scanner.Scan() {
			log.Printf("Did not receive response from server after HEARTBEAT.")
			return
		}

		response := scanner.Text()
		switch response {
		case "OK":
			// All good
		case "ERR_NOT_REGISTERED":
			log.Printf("Heartbeat failed: instance not registered. Re-registering...")
			return
		default:
			log.Printf("Unknown response from server after HEARTBEAT: %s", response)
			return
		}
	}
}
