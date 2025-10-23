package gui

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"fyne.io/fyne/v2/widget"
	"github.com/grandcat/zeroconf"
)

const (
	apiPort = 10080
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

// startMdnsServer registers the API service with mDNS and starts the API server.
func startMdnsServer() {
	// Find suitable network interfaces for mDNS advertising.
	ifaces := getSuitableInterfaces()
	if len(ifaces) == 0 {
		log.Printf("Warning: No suitable network interface found for mDNS. Using all.")
	}

	// Register the service via mDNS
	server, err := zeroconf.Register(
		"VPN Share Tool API",       // service instance name
		"_vpnshare-api._tcp",       // service type and protocol
		"local.",                   // domain
		apiPort,                    // port
		[]string{"version=1.0"},    // TXT records
		ifaces,                     // interfaces
	)
	if err != nil {
		log.Fatalf("Failed to register mDNS service: %v", err)
	}
	defer server.Shutdown()
	log.Printf("mDNS service registered and advertising on port %d", apiPort)


	// Start the HTTP server to provide the list of services
	mux := http.NewServeMux()
	mux.HandleFunc("/services", servicesHandler)
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