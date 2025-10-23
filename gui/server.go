package gui

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"fyne.io/fyne/v2/widget"
)

func startDiscoveryServer(newProxyChan chan<- *sharedProxy) {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", discoveryPort))
	if err != nil {
		log.Printf("Discovery server failed to resolve UDP address: %v", err)
		return
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Printf("Discovery server failed to listen on UDP port: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Discovery server listening on port %d", discoveryPort)

	buf := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Discovery server error reading from UDP: %v", err)
			continue
		}

		msg := string(buf[:n])
		if !strings.HasPrefix(msg, discoveryReqPrefix) {
			continue
		}

		targetURL := strings.TrimPrefix(msg, discoveryReqPrefix)
		log.Printf("Discovery: received request for URL: %s", targetURL)

		// Check if the requested URL is already a proxied URL.
		parsedDiscoveryURL, parseErr := url.Parse(targetURL)
		if parseErr != nil {
			log.Printf("Discovery: could not parse target URL %s: %v", targetURL, parseErr)
			continue
		}
		hostname := parsedDiscoveryURL.Hostname()
		isProxiedURL := false
		for _, ip := range lanIPs {
			if hostname == ip {
				isProxiedURL = true
				break
			}
		}
		if isProxiedURL {
			log.Printf("Discovery: ignoring request for already proxied URL %s", targetURL)
			continue
		}

		// Check if this instance can reach the URL
		if !isURLReachable(targetURL) {
			continue
		}

		// Check if a proxy for this host already exists
		parsedURL, err := url.Parse(targetURL)
		if err != nil {
			continue
		}
		hostname = parsedURL.Hostname()

		proxiesLock.RLock()
		var existingProxy *sharedProxy
		for _, p := range proxies {
			if strings.Contains(p.OriginalURL, hostname) {
				existingProxy = p
				break
			}
		}
		proxiesLock.RUnlock()

		var proxyToRespond *sharedProxy
		if existingProxy != nil {
			log.Printf("Discovery: found existing proxy for host %s", hostname)
			proxyToRespond = existingProxy
		} else {
			log.Printf("Discovery: no existing proxy for %s, creating a new one...", targetURL)
			// Cannot update status label from here, so pass nil
			newProxy, err := addAndStartProxy(targetURL, nil)
			if err != nil {
				log.Printf("Discovery: failed to create new proxy: %v", err)
				continue
			}
			proxiesLock.Lock()
			proxies = append(proxies, newProxy)
			proxiesLock.Unlock()
			proxyToRespond = newProxy

			// Bug fix 1: Signal the main UI thread to update the list.
			newProxyChan <- newProxy
		}

		// Respond to the client
		if len(lanIPs) > 0 {
			proxyURL := fmt.Sprintf("http://%s:%d%s", lanIPs[0], proxyToRespond.RemotePort, proxyToRespond.Path)
			responseMsg := discoveryRespPrefix + proxyURL
			_, err = conn.WriteToUDP([]byte(responseMsg), remoteAddr)
			if err != nil {
				log.Printf("Discovery: failed to send response: %v", err)
			}
			log.Printf("Discovery: sent response '%s' to %s", responseMsg, remoteAddr)
		}
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
