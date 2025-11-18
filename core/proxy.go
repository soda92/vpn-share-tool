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

type contextKey string

const originalHostKey contextKey = "originalHost"

type SharedProxy struct {
	OriginalURL string                 `json:"original_url"`
	RemotePort  int                    `json:"remote_port"`
	Path        string                 `json:"path"`
	Handler     *httputil.ReverseProxy `json:"-"`
	Server      *http.Server           `json:"-"`
}

var (
	Proxies          []*SharedProxy
	ProxiesLock      sync.RWMutex
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
	healthCheckTicker := time.NewTicker(1 * time.Minute) // Check more frequently
	defer healthCheckTicker.Stop()

	failureCount := 0
	const maxFailures = 3

	for range healthCheckTicker.C {
		log.Printf("Performing health check for %s", p.OriginalURL)
		if !IsURLReachable(p.OriginalURL) {
			failureCount++
			log.Printf("Health check failed for %s (%d/%d).", p.OriginalURL, failureCount, maxFailures)
			if failureCount >= maxFailures {
				log.Printf("Health check failed for %s after %d attempts. Tearing down proxy.", p.OriginalURL, maxFailures)
				removeProxy(p)
				return // Stop this health checker goroutine
			}
		} else {
			if failureCount > 0 {
				log.Printf("Health check successful for %s after %d failures, resetting failure count.", p.OriginalURL, failureCount)
				failureCount = 0 // Reset on success
			} else {
				log.Printf("Health check successful for %s", p.OriginalURL)
			}
		}
	}
}

func ShareUrlAndGetProxy(rawURL string) (*SharedProxy, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "http://" + rawURL
	}

	// Replace localhost with 127.0.0.1 for Android compatibility
	rawURL = strings.ReplaceAll(rawURL, "localhost", "127.0.0.1")

	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	ProxiesLock.Lock()
	defer ProxiesLock.Unlock()

	// Prevent adding duplicate Proxies by checking inside the lock
	hostname := target.Hostname()
	for _, p := range Proxies {
		existingURL, err := url.Parse(p.OriginalURL)
		if err != nil {
			continue // Skip invalid stored URL
		}
		if existingURL.Hostname() == hostname {
			log.Printf("Proxy for %s already exists, returning existing one.", rawURL)
			return p, nil
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	proxy.Transport = NewCachingTransport(client.Transport)

	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
			location := resp.Header.Get("Location")
			if location == "" {
				return nil // No Location header, nothing to do
			}

			locationURL, err := url.Parse(location)
			if err != nil {
				log.Printf("Error parsing Location header: %v", err)
				return nil
			}

			if !locationURL.IsAbs() {
				locationURL = target.ResolveReference(locationURL)
			}

			ProxiesLock.RLock()
			var existingProxy *SharedProxy
			for _, p := range Proxies {
				if p.OriginalURL == locationURL.String() {
					existingProxy = p
					break
				}
			}
			ProxiesLock.RUnlock()

			originalHost, ok := resp.Request.Context().Value(originalHostKey).(string)
			if !ok {
				log.Println("Error: could not retrieve originalHost from context or it's not a string")
				return nil
			}
			hostParts := strings.Split(originalHost, ":")
			proxyHost := hostParts[0]

			if existingProxy != nil {
				newLocation := fmt.Sprintf("http://%s:%d%s", proxyHost, existingProxy.RemotePort, locationURL.RequestURI())
				resp.Header.Set("Location", newLocation)
				log.Printf("Redirecting to existing proxy: %s", newLocation)
			} else {
				log.Printf("Redirect location not proxied, creating new proxy for: %s", locationURL.String())
				newProxy, err := ShareUrlAndGetProxy(locationURL.String())
				if err != nil {
					log.Printf("Error creating new proxy for redirect: %v", err)
				} else {
					newLocation := fmt.Sprintf("http://%s:%d%s", proxyHost, newProxy.RemotePort, locationURL.RequestURI())
					resp.Header.Set("Location", newLocation)
					log.Printf("Redirecting to new proxy: %s", newLocation)
				}
			}
		}
		return nil
	}

	remotePort := 0
	port := startPort
	for {
		isUsed := false
		for _, p := range Proxies {
			if p.RemotePort == port {
				isUsed = true
				break
			}
		}
		if !isUsed && isPortAvailable(port) {
			remotePort = port
			break
		}
		port++
		if port > startPort+1000 {
			return nil, fmt.Errorf("could not find an available port")
		}
	}

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", remotePort),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), originalHostKey, r.Host)
			proxy.ServeHTTP(w, r.WithContext(ctx))
		}),
	}

	newProxy := &SharedProxy{
		OriginalURL: rawURL,
		RemotePort:  remotePort,
		Path:        target.Path,
		Handler:     proxy,
		Server:      server,
	}

	go func() {
		log.Printf("Starting proxy for %s on port %d", rawURL, remotePort)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Proxy for %s on port %d stopped: %v", rawURL, remotePort, err)
		}
		log.Printf("Proxy for %s on port %d stopped gracefully.", rawURL, remotePort)
	}()

	go startHealthChecker(newProxy)

	Proxies = append(Proxies, newProxy)

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
