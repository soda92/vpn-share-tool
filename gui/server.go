package gui

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"fyne.io/fyne/v2/widget"
)

const (
	apiPort          = 10080
	discoverySrvPort = "45679"
)

// servicesHandler provides the list of currently shared proxies as a JSON response.
func servicesHandler(w http.ResponseWriter, r *http.Request) {
	proxiesLock.RLock()
	defer proxiesLock.RUnlock()

	// We need to construct the response with accessible URLs.
	// Since this handler will be called from another machine, we use the LAN IPs.
	type sharedURLInfo struct {
		OriginalURL string `json:"original_url"`
		SharedURL   string `json:"shared_url"`
	}

	var response []sharedURLInfo
	if len(lanIPs) > 0 {
		// Just use the first LAN IP for the response. The client can substitute it if needed.
		ip := lanIPs[0]
		for _, p := range proxies {
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
		serverAddr := net.JoinHostPort("192.168.1.81", discoverySrvPort)
		conn, err := net.Dial("tcp", serverAddr)
		if err != nil {
			log.Printf("Failed to connect to discovery server at %s: %v. Retrying in 1 minute.", serverAddr, err)
			time.Sleep(1 * time.Minute)
			continue
		}

		log.Printf("Connected to discovery server: %s", serverAddr)

		// Initial registration
		registerMsg := fmt.Sprintf("REGISTER %d\n", apiPort)
		_, err = conn.Write([]byte(registerMsg))
		if err != nil {
			log.Printf("Failed to send REGISTER command: %v", err)
			conn.Close()
			continue
		}
		log.Printf("Successfully registered with discovery server.")

		// Heartbeat loop
		heartbeatTicker := time.NewTicker(1 * time.Minute)
		defer heartbeatTicker.Stop()

		for range heartbeatTicker.C {
			heartbeatMsg := fmt.Sprintf("HEARTBEAT %d\n", apiPort)
			_, err := conn.Write([]byte(heartbeatMsg))
			if err != nil {
				log.Printf("Failed to send HEARTBEAT: %v. Reconnecting...", err)
				conn.Close()
				break // Break inner loop to trigger reconnection
			}
			log.Println("Sent heartbeat to discovery server.")
		}
	}
}

func canReachHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "url query parameter is required", http.StatusBadRequest)
		return
	}

	reachable := isURLReachable(targetURL)

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

	// This function will be created in gui.go
	newProxy, err := shareUrlAndGetProxy(req.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create proxy: %v", err), http.StatusInternalServerError)
		return
	}

	// Save the config in case a new proxy was added
	saveConfig()

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

// startApiServer starts the HTTP server to provide the API endpoints.
func startApiServer() {
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
	if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("API server stopped with error: %v", err)
	}
}


func addAndStartProxy(rawURL string, statusLabel *widget.Label) (*sharedProxy, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf(l("invalidUrl", map[string]interface{}{"error": err}))
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &cachingTransport{
		Transport: http.DefaultTransport,
	}

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	proxiesLock.Lock()
	var remotePort int
	for {
		port := nextRemotePort
		nextRemotePort++
		if isPortAvailable(port) {
			remotePort = port
			break
		}
	}
	proxiesLock.Unlock()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", remotePort),
		Handler: proxy,
	}

	go func() {
		if statusLabel != nil {
			statusLabel.SetText(l("serverRunning"))
		}
		log.Printf("Starting proxy for %s on port %d", rawURL, remotePort)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Proxy for %s on port %d stopped: %v", rawURL, remotePort, err)
			if statusLabel != nil {
				statusLabel.SetText(l("serverStopped"))
			}
		}
		log.Printf("Proxy for %s on port %d stopped gracefully.", rawURL, remotePort)
	}()

	newProxy := &sharedProxy{
		OriginalURL: rawURL,
		RemotePort:  remotePort,
		Path:        target.Path,
		handler:     proxy,
		server:      server,
	}

	return newProxy, nil
}