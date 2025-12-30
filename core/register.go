package core

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

func registerWithDiscoveryServer(apiPort int) {
	// This loop ensures we keep trying to register if the connection fails
	for {
		// 1. Discover Server IPs
		var candidateIPs []string

		// If MyIP is set (Mobile pushed it, or Desktop detected it), use it to scan.
		if MyIP != "" {
			log.Printf("MyIP is set to %s. Scanning subnet...", MyIP)
			found := ScanSubnet(MyIP, discoverySrvPort)
			if len(found) > 0 {
				log.Printf("Found servers via scanning: %v", found)
				candidateIPs = append(candidateIPs, found...)
			}
		} else {
			// If MyIP is not set, try to detect local IPs (Desktop mode)
			log.Println("MyIP not set. Attempting to detect local IPs...")
			localIPs, err := GetLocalIPs()
			if err == nil && len(localIPs) > 0 {
				for _, ip := range localIPs {
					// Heuristic: Prefer 192.168.x.x for setting MyIP initially if finding nothing else
					if strings.HasPrefix(ip, "192.168.") && MyIP == "" {
						SetMyIP(ip)
					}

					log.Printf("Scanning subnet of %s...", ip)
					found := ScanSubnet(ip, discoverySrvPort)
					if len(found) > 0 {
						log.Printf("Found servers via scanning %s: %v", ip, found)
						candidateIPs = append(candidateIPs, found...)
					}
				}
				// If we still haven't set MyIP but found IPs, just pick the first one
				if MyIP == "" && len(localIPs) > 0 {
					SetMyIP(localIPs[0])
				}
			}
		}

		// Append hardcoded fallbacks at the end
		candidateIPs = append(candidateIPs, ServerIPs...)

		var conn net.Conn
		var err error
		var serverAddr string

		// Try to connect to candidates
		for _, ip := range candidateIPs {
			// If ip already has port, don't add it again (ScanSubnet returns generic IPs usually, but let's be safe)
			if strings.Contains(ip, ":") {
				serverAddr = ip
			} else {
				serverAddr = net.JoinHostPort(ip, discoverySrvPort)
			}

			log.Printf("Trying to connect to discovery server at %s...", serverAddr)

			// Prepare TLS config
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(rootCACert)
			tlsConfig := &tls.Config{RootCAs: caCertPool}
			dialer := &net.Dialer{Timeout: 2 * time.Second}

			// Try TLS first
			conn, err = tls.DialWithDialer(dialer, "tcp", serverAddr, tlsConfig)
			if err == nil {
				log.Printf("Connected to discovery server at %s (TLS)", serverAddr)

				host, _, _ := net.SplitHostPort(serverAddr)
				DiscoveryServerURL = fmt.Sprintf("https://%s:8080", host)

				break
			}

			// Fallback to Plaintext
			conn, err = dialer.Dial("tcp", serverAddr)
			if err == nil {
				log.Printf("Connected to discovery server at %s (Plaintext)", serverAddr)

				host, _, _ := net.SplitHostPort(serverAddr)
				DiscoveryServerURL = fmt.Sprintf("http://%s:8080", host)

				break
			}
		}
		if err != nil {
			log.Printf("Failed to connect to any discovery server. Retrying in 5 seconds.")
			time.Sleep(5 * time.Second)
			continue
		}

		// Use a closure to manage the connection lifecycle.
		// This makes resource management (like closing connections and stopping tickers) cleaner.
		func(conn net.Conn) {
			defer conn.Close()
			scanner := bufio.NewScanner(conn)

			// 1. Initial Registration
			registerMsg := fmt.Sprintf("REGISTER %d %s\n", apiPort, Version)
			if _, err := conn.Write([]byte(registerMsg)); err != nil {
				log.Printf("Failed to send REGISTER command: %v", err)
				return // Exit closure, trigger reconnect
			}

			if !scanner.Scan() {
				log.Printf("Did not receive response from server after REGISTER.")
				return // Exit closure, trigger reconnect
			}
			response := scanner.Text()
			parts := strings.Split(response, " ")
			if len(parts) == 2 && parts[0] == "OK" {
				// Server confirmed our IP. We can trust it, or keep our own.
				// For now, let's respect the server's view as it's the source of truth for the network.
				detectedIP := parts[1]
				if MyIP != detectedIP {
					log.Printf("Server sees us as %s (Local was %s). Updating.", detectedIP, MyIP)
					SetMyIP(detectedIP)
				}
				log.Printf("Successfully registered with discovery server. My IP is %s", MyIP)
				IPReadyChan <- MyIP // Signal that the IP is ready
			} else {
				log.Printf("Failed to register with discovery server, response: %s.", response)
				return // Exit closure, trigger reconnect
			}

			// 2. Heartbeat Loop
			heartbeatTicker := time.NewTicker(5 * time.Second)
			defer heartbeatTicker.Stop()

			for range heartbeatTicker.C {
				heartbeatMsg := fmt.Sprintf("HEARTBEAT %d\n", apiPort)
				if _, err := conn.Write([]byte(heartbeatMsg)); err != nil {
					log.Printf("Failed to send HEARTBEAT: %v", err)
					return // Exit closure, trigger reconnect
				}
				// log.Println("Sent heartbeat to discovery server.") // Reduce log noise

				// Wait for and process server response
				if !scanner.Scan() {
					log.Printf("Did not receive response from server after HEARTBEAT.")
					return // Exit closure, trigger reconnect
				}

				response := scanner.Text()
				switch response {
				case "OK":
					// All good
				case "ERR_NOT_REGISTERED":
					log.Printf("Heartbeat failed: instance not registered. Re-registering...")
					return // Exit closure, trigger reconnect and re-register
				default:
					log.Printf("Unknown response from server after HEARTBEAT: %s", response)
					return // Exit closure, trigger reconnect
				}
			}
		}(conn)

		log.Printf("Connection to discovery server lost. Retrying...")
		time.Sleep(5 * time.Second)
	}
}
