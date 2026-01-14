package core

import (
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/handlers"
	"github.com/soda92/vpn-share-tool/core/proxy"
	"github.com/soda92/vpn-share-tool/core/register"
	"github.com/soda92/vpn-share-tool/core/resources"
	"github.com/soda92/vpn-share-tool/core/utils"
)

// StartApiServer starts the HTTP server to provide the API endpoints.
func StartApiServer(apiPort int) error {
	APIPort = apiPort

	// Try to auto-detect IP on startup for Desktop/CLI usage
	if MyIP == "" {
		ips, err := utils.GetLocalIPs()
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

	proxy.SetGlobalConfig(MyIP, APIPort, DiscoveryServerURL, GetHTTPClient)

	addProxyHandler := &handlers.AddProxyHandler{
		GetIP:       func() string { return MyIP },
		CreateProxy: proxy.ShareUrlAndGetProxy,
	}
	canReachHandler := &handlers.CanReachHandler{
		IsURLReachable: utils.IsURLReachable,
	}
	servicesHandler := &handlers.ServicesHandler{
		GetProxies: proxy.GetProxies,
		MyIP:       MyIP,
	}

	activeProxiesHandler := &handlers.GetActiveProxiesHandler{
		GetProxies: proxy.GetProxies,
	}

	updateSettingsHandler := &handlers.UpdateSettingsHandler{
		GetProxies: proxy.GetProxies,
	}

	triggerUpdateHandler := &handlers.TriggerUpdateHandler{
		TriggerUpdate: TriggerUpdate,
	}

	// Start the HTTP server to provide the list of services
	mux := http.NewServeMux()
	mux.Handle("/services", servicesHandler)
	mux.Handle("/proxies", addProxyHandler)
	mux.Handle("/can-reach", canReachHandler)
	mux.Handle("/active-proxies", activeProxiesHandler)
	mux.Handle("/update-settings", updateSettingsHandler)
	mux.Handle("/trigger-update", triggerUpdateHandler)

	// Profiling endpoints
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	debug.RegisterDebugRoutes(mux)

	apiServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", apiPort),
		Handler: mux,
	}

	log.Printf("Starting API server on port %d", apiPort)

	// Restore saved proxies
	proxy.LoadProxies()

	regCfg := register.Config{
		MyIP:              MyIP,
		SetMyIP:           SetMyIP,
		Version:           Version,
		APIPort:           apiPort,
		DiscoverySrvPort:  discoverySrvPort,
		FallbackServerIPs: ServerIPs,
		RootCACert:        resources.RootCACert,
		IPReadyChan:       proxy.IPReadyChan,
		UpdateDiscoveryURL: func(url string) {
			DiscoveryServerURL = url
			proxy.SetGlobalConfig(MyIP, APIPort, DiscoveryServerURL, GetHTTPClient)
		},
	}
	go register.Start(regCfg)

	if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("API server stopped with error: %w", err)
	}
	return nil
}
