package core

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	startPort = 10081
)

type SharedProxy struct {
	OriginalURL string `json:"original_url"`
	RemotePort  int    `json:"remote_port"`
	Path        string `json:"path"`
	Handler     *httputil.ReverseProxy `json:"-"`
	Server      *http.Server `json:"-"`
}

var (
	Proxies          []*SharedProxy
	ProxiesLock      sync.RWMutex
	NextRemotePort   = startPort
	ProxyAddedChan   = make(chan *SharedProxy)
	ProxyRemovedChan = make(chan *SharedProxy)
	IPReadyChan      = make(chan string, 1)
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

// removeProxy shuts down a proxy server and removes it from the list.
func removeProxy(p *SharedProxy) {
	log.Printf("Removing proxy for unreachable URL: %s", p.OriginalURL)

	// 1. Shutdown the HTTP server
	if p.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := p.Server.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down proxy server for %s: %v", p.OriginalURL, err)
		}
	}

	// 2. Remove from the global Proxies slice
	ProxiesLock.Lock()
	newProxies := []*SharedProxy{}
	for _, proxy := range Proxies {
		if proxy != p {
			newProxies = append(newProxies, proxy)
		}
	}
	Proxies = newProxies
	ProxiesLock.Unlock()

	// 3. Signal the UI to update
	ProxyRemovedChan <- p
}

// startHealthChecker runs in a goroutine to periodically check if a URL is reachable.
func startHealthChecker(p *SharedProxy) {
	healthCheckTicker := time.NewTicker(3 * time.Minute)
	defer healthCheckTicker.Stop()

	for range healthCheckTicker.C {
		log.Printf("Performing health check for %s", p.OriginalURL)
		if !IsURLReachable(p.OriginalURL) {
			log.Printf("Health check failed for %s. Tearing down proxy.", p.OriginalURL)
			removeProxy(p)
			return // Stop this health checker goroutine
		}
		log.Printf("Health check successful for %s", p.OriginalURL)
	}
}

func AddAndStartProxy(rawURL string) (*SharedProxy, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &CachingTransport{
		Transport: http.DefaultTransport,
	}

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	ProxiesLock.Lock()
	var remotePort int
	for {
		port := NextRemotePort
		NextRemotePort++
		if isPortAvailable(port) {
			remotePort = port
			break
		}
	}
	ProxiesLock.Unlock()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", remotePort),
		Handler: proxy,
	}

	go func() {
		log.Printf("Starting proxy for %s on port %d", rawURL, remotePort)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Proxy for %s on port %d stopped: %v", rawURL, remotePort, err)
		}
		log.Printf("Proxy for %s on port %d stopped gracefully.", rawURL, remotePort)
	}()

	newProxy := &SharedProxy{
		OriginalURL: rawURL,
		RemotePort:  remotePort,
		Path:        target.Path,
		Handler:     proxy,
		Server:      server,
	}

	go startHealthChecker(newProxy)

	return newProxy, nil
}

func ShareUrlAndGetProxy(rawURL string) (*SharedProxy, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "http://" + rawURL
	}

	// Prevent adding duplicate Proxies
	parsedURL, err := url.Parse(rawURL)
	if err == nil {
		hostname := parsedURL.Hostname()
		ProxiesLock.RLock()
		for _, p := range Proxies {
			existingURL, err := url.Parse(p.OriginalURL)
			if err != nil {
				continue // Skip invalid stored URL
			}
			if existingURL.Hostname() == hostname {
				ProxiesLock.RUnlock()
				log.Printf("Proxy for %s already exists, returning existing one.", rawURL)
				return p, nil // Return existing proxy instead of an error
			}
		}
		ProxiesLock.RUnlock()
	}

	newProxy, err := AddAndStartProxy(rawURL)
	if err != nil {
		return nil, fmt.Errorf("error adding proxy for %s: %w", rawURL, err)
	}

	ProxiesLock.Lock()
	Proxies = append(Proxies, newProxy)
	ProxiesLock.Unlock()

	// Announce the new proxy to the UI
	ProxyAddedChan <- newProxy

	return newProxy, nil
}

func Shutdown() {
	ProxiesLock.Lock()
	defer ProxiesLock.Unlock()
	var wg sync.WaitGroup
	for _, p := range Proxies {
		if p.Server != nil {
			wg.Add(1)
			go func(s *http.Server) {
				defer wg.Done()
				// Give server 5 seconds to shutdown gracefully
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := s.Shutdown(ctx); err != nil {
					log.Printf("Error shutting down proxy server: %v", err)
				}
			}(p.Server)
		}
	}
	wg.Wait()
}

func GetProxies() []*SharedProxy {
	ProxiesLock.RLock()
	defer ProxiesLock.RUnlock()
	return Proxies
}
