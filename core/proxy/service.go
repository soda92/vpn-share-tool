package proxy

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
	"sync/atomic"
	"time"

	"github.com/soda92/vpn-share-tool/core/cache"
	"github.com/soda92/vpn-share-tool/core/models"
	"github.com/soda92/vpn-share-tool/core/pipeline"
)

type captchaAdapter struct{}

func (c *captchaAdapter) Solve(data []byte) string { return SolveCaptcha(data) }
func (c *captchaAdapter) Store(ip, sol string)     { StoreCaptchaSolution(ip, sol) }
func (c *captchaAdapter) Get(ip string) string     { return GetCaptchaSolution(ip) }
func (c *captchaAdapter) Clear(ip string)          { ClearCaptchaSolution(ip) }

const (
	startPort = 10081
)

var (
	Proxies            []*models.SharedProxy
	ProxiesLock        sync.RWMutex
	ProxyAddedChan     = make(chan *models.SharedProxy)
	ProxyRemovedChan   = make(chan *models.SharedProxy)
	IPReadyChan        = make(chan string, 1)
	MyIP               string
	APIPort            int
	DiscoveryServerURL string
	HTTPClientProvider func() *http.Client
)

func SetGlobalConfig(ip string, port int, discoveryURL string, clientProvider func() *http.Client) {
	MyIP = ip
	APIPort = port
	DiscoveryServerURL = discoveryURL
	HTTPClientProvider = clientProvider
}

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
func removeProxy(p *models.SharedProxy) {
	log.Printf("Removing proxy for unreachable URL: %s", p.OriginalURL)

	// 0. Cancel the context to stop background tasks (Stats, HealthCheck)
	if p.Cancel != nil {
		p.Cancel()
	}

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
	newProxies := []*models.SharedProxy{}
	for _, proxy := range Proxies {
		if proxy != p {
			newProxies = append(newProxies, proxy)
		}
	}
	Proxies = newProxies
	ProxiesLock.Unlock()

	// 3. Signal the UI to update
	ProxyRemovedChan <- p

	// 4. Persist changes
	SaveProxies()
}

func ShareUrlAndGetProxy(rawURL string, requestedPort int) (*models.SharedProxy, error) {
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

	// Prevent adding duplicate Proxies by checking inside the lock
	host := target.Host
	for _, p := range Proxies {
		existingURL, err := url.Parse(p.OriginalURL)
		if err != nil {
			continue // Skip invalid stored URL
		}
		if existingURL.Host == host {
			log.Printf("Proxy for %s already exists, returning existing one.", rawURL)
			ProxiesLock.Unlock()
			return p, nil
		}
	}
	ProxiesLock.Unlock()

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	// proxy.Transport will be assigned later after creating SharedProxy

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
			var existingProxy *models.SharedProxy
			for _, p := range Proxies {
				if p.OriginalURL == locationURL.String() {
					existingProxy = p
					break
				}
			}
			ProxiesLock.RUnlock()

			originalHost, ok := resp.Request.Context().Value(models.OriginalHostKey).(string)
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
				newProxy, err := ShareUrlAndGetProxy(locationURL.String(), 0)
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

	// Try requested port first
	if requestedPort > 0 {
		isUsed := false
		for _, p := range Proxies {
			if p.RemotePort == requestedPort {
				isUsed = true
				break
			}
		}
		if !isUsed && isPortAvailable(requestedPort) {
			remotePort = requestedPort
		} else {
			log.Printf("Requested port %d is not available or in use, falling back to auto-selection.", requestedPort)
		}
	}

	ProxiesLock.Lock()
	if remotePort == 0 {
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
	}
	ProxiesLock.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	// Pre-create the struct to allow closure capture
	newProxy := &models.SharedProxy{
		OriginalURL: rawURL,
		RemotePort:  remotePort,
		Path:        target.Path,
		Handler:     proxy,
		Settings: models.ProxySettings{
			EnableContentMod: true,
			EnableUrlRewrite: true,
		},
		Ctx:    ctx,
		Cancel: cancel,
	}
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", remotePort),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Update metrics
			atomic.AddInt64(&newProxy.ReqCounter, 1)
			atomic.AddInt64(&newProxy.TotalRequests, 1)
			ctx := context.WithValue(r.Context(), models.OriginalHostKey, r.Host)
			proxy.ServeHTTP(w, r.WithContext(ctx))
		}),
	}
	newProxy.Server = server
	// Assign transport here to pass the newProxy reference
	proxy.Transport = cache.NewCachingTransport(nil, newProxy, &captchaAdapter{}, func(ctx *models.ProcessingContext, body string) string {
		// Populate Services
		ctx.Services = models.PipelineServices{
			CreateProxy: ShareUrlAndGetProxy,
			MyIP:        MyIP,
			APIPort:     APIPort,
		}
		return pipeline.RunPipeline(ctx, body, pipeline.GetDefaultProcessors())
	})

	go func() {
		log.Printf("Starting proxy for %s on port %d", rawURL, remotePort)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Proxy for %s on port %d stopped: %v", rawURL, remotePort, err)
		}
		log.Printf("Proxy for %s on port %d stopped gracefully.", rawURL, remotePort)
	}()

	go startHealthChecker(newProxy)
	go startStatsUpdater(newProxy)
	go StartSystemDetector(newProxy)
	ProxiesLock.Lock()
	Proxies = append(Proxies, newProxy)
	ProxiesLock.Unlock()

	ProxyAddedChan <- newProxy

	SaveProxies()

	return newProxy, nil
}

func Shutdown() {
	ProxiesLock.Lock()
	proxiesToShutdown := make([]*models.SharedProxy, len(Proxies))
	copy(proxiesToShutdown, Proxies)
	ProxiesLock.Unlock()

	var wg sync.WaitGroup
	for _, p := range proxiesToShutdown {
		// Cancel background tasks (Stats, HealthCheck)
		if p.Cancel != nil {
			p.Cancel()
		}

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

func GetProxies() []*models.SharedProxy {
	ProxiesLock.RLock()
	defer ProxiesLock.RUnlock()
	// Return a copy to ensure thread safety for the caller
	result := make([]*models.SharedProxy, len(Proxies))
	copy(result, Proxies)
	return result
}
