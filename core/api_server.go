package core

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

// StartApiServer starts the HTTP server to provide the API endpoints.
func StartApiServer(apiPort int) error {
	APIPort = apiPort

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
	mux.HandleFunc("/toggle-debug", handleToggleDebug)
	mux.HandleFunc("/trigger-update", handleTriggerUpdate)
	mux.HandleFunc("/toggle-captcha", handleToggleCaptcha)

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
