package core

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/handlers"
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

	addProxyHandler := &handlers.AddProxyHandler{
		GetIP:       func() string { return MyIP },
		CreateProxy: ShareUrlAndGetProxy,
	}
	canReachHandler := &handlers.CanReachHandler{
		IsURLReachable: IsURLReachable,
	}
	servicesHandler := &handlers.ServicesHandler{
		GetProxies:  GetProxies,
		MyIP:        MyIP,
	}

	activeProxiesHandler := &handlers.GetActiveProxiesHandler{
		GetProxies: GetProxies,
	}

	toggleDebugHandler := &handlers.ToggleDebugHandler{
		GetProxies:  GetProxies,
	}
	toggleCaptchaHandler := &handlers.ToggleCaptchaHandler{
		GetProxies:  GetProxies,
	}

	triggerUpdatehandler := &handlers.TriggerUpdateHandler{
		TriggerUpdate: TriggerUpdate,
	}

	// Start the HTTP server to provide the list of services
	mux := http.NewServeMux()
	mux.Handle("/services", servicesHandler)
	mux.Handle("/proxies", addProxyHandler)
	mux.Handle("/can-reach", canReachHandler)
	mux.Handle("/active-proxies", activeProxiesHandler)
	mux.Handle("/toggle-debug", toggleDebugHandler)
	mux.Handle("/toggle-captcha", toggleCaptchaHandler)
	mux.Handle("/trigger-update", triggerUpdatehandler)

	debug.RegisterDebugRoutes(mux)

	apiServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", apiPort),
		Handler: mux,
	}

	log.Printf("Starting API server on port %d", apiPort)

	// Restore saved proxies
	LoadProxies()

	go registerWithDiscoveryServer(apiPort)
	if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("API server stopped with error: %w", err)
	}
	return nil
}
