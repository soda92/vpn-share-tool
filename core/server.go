package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type sharedURLInfo struct {
	OriginalURL string `json:"original_url"`
	SharedURL   string `json:"shared_url"`
}

const (
	discoverySrvPort = "45679"
)

var (
	// Fallback IPs if scanning fails
	SERVER_IPs = []string{"192.168.0.81", "192.168.1.81"}
	ApiPort    int
	MyIP       string
)

// SetMyIP allows external packages (like mobile bridge) to set the client IP.
func SetMyIP(ip string) {
	MyIP = ip
	log.Printf("Device IP set to: %s", MyIP)
	// Trigger a signal? For now just logging.
}

// servicesHandler provides the list of currently shared proxies as a JSON response.
func servicesHandler(w http.ResponseWriter, r *http.Request) {
	ProxiesLock.RLock()
	defer ProxiesLock.RUnlock()

	// Initialize with a non-nil empty slice to ensure the JSON output is `[]` instead of `null`.
	response := make([]sharedURLInfo, 0)
	if MyIP != "" {
		// Just use the first LAN IP for the response. The client can substitute it if needed.
		ip := MyIP
		for _, p := range Proxies {
			sharedURL := fmt.Sprintf("http://%s:%d%s", ip, p.RemotePort, p.Path)
			response = append(response, sharedURLInfo{
				OriginalURL: p.OriginalURL,
				SharedURL:   sharedURL,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode services to JSON: %v", err)
		http.Error(w, "Failed to encode services", http.StatusInternalServerError)
	}
}

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
		candidateIPs = append(candidateIPs, SERVER_IPs...)

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
			conn, err = net.DialTimeout("tcp", serverAddr, 2*time.Second)
			if err == nil {
				log.Printf("Connected to discovery server at %s", serverAddr)
				break
			}
		}

		if err != nil {
			log.Printf("Failed to connect to any discovery server. Retrying in 1 minute.")
			time.Sleep(1 * time.Minute)
			continue
		}

		// Use a closure to manage the connection lifecycle.
		// This makes resource management (like closing connections and stopping tickers) cleaner.
		func(conn net.Conn) {
			defer conn.Close()
			scanner := bufio.NewScanner(conn)

			// 1. Initial Registration
			registerMsg := fmt.Sprintf("REGISTER %d\n", apiPort)
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

func canReachHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "url query parameter is required", http.StatusBadRequest)
		return
	}

	reachable := IsURLReachable(targetURL)

	response := struct {
		Reachable bool `json:"reachable"`
	}{
		Reachable: reachable,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func addProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	newProxy, err := ShareUrlAndGetProxy(req.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create proxy: %v", err), http.StatusInternalServerError)
		return
	}

	// Construct the response containing the full proxy details
	// This ensures the client gets the externally accessible URL.
	var sharedURL string
	if MyIP != "" {
		sharedURL = fmt.Sprintf("http://%s:%d%s", MyIP, newProxy.RemotePort, newProxy.Path)
	}

	type sharedURLInfo struct {
		OriginalURL string `json:"original_url"`
		SharedURL   string `json:"shared_url"`
	}

	response := sharedURLInfo{
		OriginalURL: newProxy.OriginalURL,
		SharedURL:   sharedURL,
	}

	// Respond with the details of the new proxy
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func handleGetActiveProxies(w http.ResponseWriter, r *http.Request) {
	proxies := GetProxies()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proxies); err != nil {
		log.Printf("Failed to encode active proxies to JSON: %v", err)
		http.Error(w, "Failed to encode active proxies", http.StatusInternalServerError)
	}
}

// StartApiServer starts the HTTP server to provide the API endpoints.
func StartApiServer(apiPort int) error {
	ApiPort = apiPort
	
	// Try to auto-detect IP on startup for Desktop/CLI usage
	if MyIP == "" {
		ips, err := GetLocalIPs()
		if err == nil {
			for _, ip := range ips {
				if strings.HasPrefix(ip, "192.168.") {
					SetMyIP(ip)
					break
				}
			}
			// If no 192.168 found, take first
			if MyIP == "" && len(ips) > 0 {
				SetMyIP(ips[0])
			}
		}
	}

	// Start the HTTP server to provide the list of services
	mux := http.NewServeMux()
	mux.HandleFunc("/services", servicesHandler)
	mux.HandleFunc("/proxies", addProxyHandler)
	mux.HandleFunc("/can-reach", canReachHandler)
	mux.HandleFunc("/active-proxies", handleGetActiveProxies)

	RegisterDebugRoutes(mux)

	apiServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", apiPort),
		Handler: mux,
	}

	log.Printf("Starting API server on port %d", apiPort)
	go registerWithDiscoveryServer(apiPort)
	if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("API server stopped with error: %w", err)
	}
	return nil
}