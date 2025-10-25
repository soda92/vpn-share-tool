package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

const (
	apiPort          = 10080
	discoverySrvPort = "45679"
	SERVER_IP        = "192.168.0.81"
)

// servicesHandler provides the list of currently shared proxies as a JSON response.
func servicesHandler(w http.ResponseWriter, r *http.Request) {
	ProxiesLock.RLock()
	defer ProxiesLock.RUnlock()

	lanIPs, err := GetLanIPs()
	if err != nil {
		log.Printf("Could not get LAN IPs for services handler: %v", err)
	}

	// We need to construct the response with accessible URLs.
	// Since this handler will be called from another machine, we use the LAN IPs.
	type sharedURLInfo struct {
		OriginalURL string `json:"original_url"`
		SharedURL   string `json:"shared_url"`
	}

	// Initialize with a non-nil empty slice to ensure the JSON output is `[]` instead of `null`.
	response := make([]sharedURLInfo, 0)
	if len(lanIPs) > 0 {
		// Just use the first LAN IP for the response. The client can substitute it if needed.
		ip := lanIPs[0]
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

func registerWithDiscoveryServer() {
	// This loop ensures we keep trying to register if the connection fails
	for {
		serverAddr := net.JoinHostPort(SERVER_IP, discoverySrvPort)
		conn, err := net.Dial("tcp", serverAddr)
		if err != nil {
			log.Printf("Failed to connect to discovery server at %s: %v. Retrying in 1 minute.", serverAddr, err)
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
			if response := scanner.Text(); response != "OK" {
				log.Printf("Failed to register with discovery server, response: %s.", response)
				return // Exit closure, trigger reconnect
			}
			log.Printf("Successfully registered with discovery server.")

			// 2. Heartbeat Loop
			heartbeatTicker := time.NewTicker(1 * time.Minute)
			defer heartbeatTicker.Stop()

			for range heartbeatTicker.C {
				heartbeatMsg := fmt.Sprintf("HEARTBEAT %d\n", apiPort)
				if _, err := conn.Write([]byte(heartbeatMsg)); err != nil {
					log.Printf("Failed to send HEARTBEAT: %v", err)
					return // Exit closure, trigger reconnect
				}
				log.Println("Sent heartbeat to discovery server.")

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

	lanIPs, err := GetLanIPs()
	if err != nil {
		log.Printf("Could not get LAN IPs for add proxy handler: %v", err)
	}

	// Construct the response containing the full proxy details
	// This ensures the client gets the externally accessible URL.
	var sharedURL string
	if len(lanIPs) > 0 {
		sharedURL = fmt.Sprintf("http://%s:%d%s", lanIPs[0], newProxy.RemotePort, newProxy.Path)
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

// StartApiServer starts the HTTP server to provide the API endpoints.
func StartApiServer() {
	// Start the HTTP server to provide the list of services
	mux := http.NewServeMux()
	mux.HandleFunc("/services", servicesHandler)
	mux.HandleFunc("/proxies", addProxyHandler)
	mux.HandleFunc("/can-reach", canReachHandler)
	apiServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", apiPort),
		Handler: mux,
	}

	log.Printf("Starting API server on port %d", apiPort)
	go registerWithDiscoveryServer()
	if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("API server stopped with error: %v", err)
	}
}
