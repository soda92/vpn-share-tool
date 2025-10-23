package gui

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Xuanwo/go-locale"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed i18n/*.json
var i18nFS embed.FS

// cacheEntry holds the cached response data and headers.
type cacheEntry struct {
	Header http.Header
	Body   []byte
}

// cachingTransport is an http.RoundTripper that caches responses for static assets.
type cachingTransport struct {
	Transport http.RoundTripper
	Cache     sync.Map // Using sync.Map for concurrent access
}

// RoundTrip implements the http.RoundTripper interface.
func (t *cachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// We only cache GET requests.
	if req.Method != http.MethodGet {
		return t.Transport.RoundTrip(req)
	}

	// Don't cache AJAX requests.
	if req.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		return t.Transport.RoundTrip(req)
	}

	// Check if the file extension is cacheable.
	ext := filepath.Ext(req.URL.Path)
	isCacheable := false
	switch ext {
	case ".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".ico":
		isCacheable = true
	}

	if !isCacheable {
		return t.Transport.RoundTrip(req)
	}

	// If the item is in the cache, return it.
	if entry, ok := t.Cache.Load(req.URL.String()); ok {
		log.Printf("Cache HIT for: %s", req.URL.String())
		cached := entry.(cacheEntry)
		// Create a new response from the cached data.
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     cached.Header,
			Body:       io.NopCloser(bytes.NewReader(cached.Body)),
			Request:    req,
		}
		return resp, nil
	}

	log.Printf("Cache MISS for: %s", req.URL.String())
	// If not in the cache, make the request.
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Read the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close() // We've read it, so close it.

	// Create a new body for the original response.
	resp.Body = io.NopCloser(bytes.NewReader(body))

	// Cache the response.
	entry := cacheEntry{
		Header: resp.Header,
		Body:   body,
	}
	t.Cache.Store(req.URL.String(), entry)

	return resp, nil
}

// Config holds the data to be saved to a JSON file.
type Config struct {
	OriginalURLs []string `json:"original_urls"`
}

const (
	configFile          = "vpn_share_config.json"
	startPort           = 10081
	discoveryPort       = 45678
	discoveryReqPrefix  = "DISCOVER_REQ:"
	discoveryRespPrefix = "DISCOVER_RESP:"
)

// sharedProxy holds information about a shared URL.
type sharedProxy struct {
	OriginalURL string
	RemotePort  int
	Path        string
	handler     *httputil.ReverseProxy
	server      *http.Server
}

var (
	proxies        []*sharedProxy
	proxiesLock    sync.RWMutex
	lanIPs         []string
	nextRemotePort = startPort
	localizer      *i18n.Localizer
)

// saveConfig saves the current list of original URLs to the config file.
func saveConfig() {
	proxiesLock.RLock()
	defer proxiesLock.RUnlock()

	var urls []string
	for _, p := range proxies {
		urls = append(urls, p.OriginalURL)
	}

	config := Config{OriginalURLs: urls}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal config to JSON: %v", err)
		return
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		log.Printf("Failed to write config file: %v", err)
	}
}

// loadConfig loads URLs from the config file and re-initializes the proxies.
func loadConfig(shareFunc func(string), statusLabel *widget.Label) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return // No config file yet
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Printf("Failed to read config file: %v", err)
		return
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Failed to unmarshal config JSON: %v", err)
		return
	}

	log.Printf("Loading %d URLs from config...", len(config.OriginalURLs))
	for _, u := range config.OriginalURLs {
		shareFunc(u)
	}
	statusLabel.SetText(l("serverRunning"))
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

func initI18n() {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	bundle.LoadMessageFileFS(i18nFS, "i18n/en.json")
	bundle.LoadMessageFileFS(i18nFS, "i18n/zh.json")

	langTag, err := locale.Detect()
	if err != nil {
		log.Printf("Failed to detect locale, falling back to English: %v", err)
		langTag = language.English
	}

	matcher := language.NewMatcher([]language.Tag{
		language.English,
		language.Chinese,
	})
	tag, _, _ := matcher.Match(langTag)

	localizer = i18n.NewLocalizer(bundle, tag.String())
}

func l(messageID string, templateData ...map[string]interface{}) string {
	var data map[string]interface{}
	if len(templateData) > 0 {
		data = templateData[0]
	}
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
	})
	if err != nil {
		log.Printf("Failed to localize message '%s': %v", messageID, err)
		return messageID // Fallback to message ID
	}
	return msg
}

func Run() {
	initI18n()

	myApp := app.New()
	myWindow := myApp.NewWindow(l("vpnShareToolTitle"))

	// Channel to signal UI updates from the discovery server
	newProxyChan := make(chan *sharedProxy)
	go startDiscoveryServer(newProxyChan)

	// Setup system tray
	if desk, ok := myApp.(desktop.App); ok {
		menu := fyne.NewMenu("VPN Share Tool",
			fyne.NewMenuItem(l("showMenuItem"), func() {
				myWindow.Show()
			}),
			fyne.NewMenuItem(l("exitMenuItem"), func() {
				myApp.Quit()
			}),
		)
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(theme.InfoIcon()) // Using a standard icon
	}

	var err error
	lanIPs, err = getLanIPs()
	if err != nil {
		log.Printf(l("couldNotDetermineLanIp", map[string]interface{}{"ip": "N/A", "error": err}))
		lanIPs = []string{}
	}

	// Server section
	serverStatus := widget.NewLabel(l("startingServer"))

	// Client/Proxy section
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder(l("urlPlaceholder"))

	sharedListData := binding.NewStringList()

	// Function to add a proxy to the UI list and save config.
	// This is thread-safe due to Fyne's data binding capabilities.
	addProxyToUI := func(newProxy *sharedProxy) {
		for _, ip := range lanIPs {
			sharedURL := fmt.Sprintf("http://%s:%d%s", ip, newProxy.RemotePort, newProxy.Path)
			displayString := l("sharedUrlFormat", map[string]interface{}{
				"originalUrl": newProxy.OriginalURL,
				"sharedUrl":   sharedURL,
			})
			sharedListData.Append(displayString)
		}
		saveConfig()
	}

	// Goroutine to handle UI updates from the discovery server channel
	go func() {
		for newProxy := range newProxyChan {
			addProxyToUI(newProxy)
		}
	}()

	var sharedList *widget.List
	sharedList = widget.NewListWithData(
		sharedListData,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			o.(*widget.Label).Bind(i.(binding.String))
		},
	)

	sharedList.OnSelected = func(id widget.ListItemID) {
		itemText, err := sharedListData.GetValue(id)
		if err != nil {
			return
		}

		parts := strings.Split(itemText, " -> ")
		if len(parts) < 2 {
			return
		}
		urlToCopy := parts[1]

		myWindow.Clipboard().SetContent(urlToCopy)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   l("copiedTitle"),
			Content: l("copiedContent"),
		})

		// Unselect the item so it can be clicked again.
		sharedList.Unselect(id)
	}

	// Define the core sharing logic as a function to be reused.
	shareLogic := func(rawURL string) {
		if rawURL == "" {
			return
		}

		if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
			rawURL = "http://" + rawURL
		}

		// Prevent adding duplicate proxies from the UI
		parsedURL, err := url.Parse(rawURL)
		if err == nil {
			hostname := parsedURL.Hostname()
			proxiesLock.RLock()
			var exists bool
			for _, p := range proxies {
					existingURL, err := url.Parse(p.OriginalURL)
					if err != nil {
						log.Printf("Skipping invalid stored URL: %s", p.OriginalURL)
						continue
					}
					if existingURL.Hostname() == hostname {
						exists = true
						break
					}
				}
			proxiesLock.RUnlock()
			if exists {
				log.Printf("Proxy for %s already exists.", rawURL)
				return // Already exists, do nothing.
			}
		}

		newProxy, err := addAndStartProxy(rawURL, serverStatus)
		if err != nil {
			log.Printf(l("errorAddingProxy", map[string]interface{}{"url": rawURL, "error": err}))
			serverStatus.SetText(l("errorAddingProxy", map[string]interface{}{"url": rawURL, "error": err}))
			return
		}

		proxiesLock.Lock()
		proxies = append(proxies, newProxy)
		proxiesLock.Unlock()

		addProxyToUI(newProxy)
		urlEntry.SetText("")
	}

	// Bug fix 2: Handle 'Enter' key in the URL entry field.
	urlEntry.OnSubmitted = func(text string) {
		shareLogic(text)
	}

	addButton := widget.NewButton(l("shareButton"), func() {
		shareLogic(urlEntry.Text)
	})

	// Load config on startup
	loadConfig(shareLogic, serverStatus)

	topContent := container.NewVBox(
		widget.NewLabel(l("localServerStatusLabel")),
		serverStatus,
		widget.NewSeparator(),
		widget.NewLabel(l("addUrlLabel")),
		urlEntry,
		addButton,
		widget.NewSeparator(),
		widget.NewLabel(l("sharedUrlsLabel")),
	)

	myWindow.SetContent(container.NewBorder(topContent, nil, nil, nil, sharedList))
	// Intercept close to hide window instead of quitting
	myWindow.SetCloseIntercept(func() {
		myWindow.Hide()
	})
	myWindow.SetOnClosed(func() {
		// Clean up all running proxy servers on exit
		proxiesLock.Lock()
		defer proxiesLock.Unlock()
		var wg sync.WaitGroup
		for _, p := range proxies {
			if p.server != nil {
				wg.Add(1)
				go func(s *http.Server) {
					defer wg.Done()
					// Give server 5 seconds to shutdown gracefully
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := s.Shutdown(ctx); err != nil {
						log.Printf("Error shutting down proxy server: %v", err)
					}
				}(p.server)
			}
		}
		wg.Wait()
	})

	myWindow.Resize(fyne.NewSize(600, 400))
	myWindow.ShowAndRun()
}

func isURLReachable(targetURL string) bool {
	client := http.Client{
		Timeout: 3 * time.Second,
	}
	// Use HEAD request for efficiency
	req, err := http.NewRequest("HEAD", targetURL, nil)
	if err != nil {
		log.Printf("Discovery: could not create request for %s: %v", targetURL, err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Discovery: URL %s is not reachable: %v", targetURL, err)
		return false
	}
	defer resp.Body.Close()

	// Any status code (even 401/403) means the server is alive.
	log.Printf("Discovery: URL %s is reachable with status %d", targetURL, resp.StatusCode)
	return true
}

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

// getLanIPs finds all suitable local private IP addresses of the machine.
func getLanIPs() ([]string, error) {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip := ipnet.IP.To4(); ip != nil && ip.IsPrivate() {
				ips = append(ips, ip.String())
			}
		}
	}

	if len(ips) == 0 {
		// If no private IP was found, try to find any usable IP that is not link-local.
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ip := ipnet.IP.To4(); ip != nil && !ip.IsLinkLocalUnicast() {
					ips = append(ips, ip.String())
				}
			}
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf(l("noSuitableLanIpFound"))
	}
	return ips, nil
}