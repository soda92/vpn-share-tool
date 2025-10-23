package gui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
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
				if strings.Contains(p.OriginalURL, hostname) {
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
	go loadConfig(shareLogic, serverStatus)

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
		for _, p := range proxies {
			if p.server != nil {
				go p.server.Shutdown(context.Background())
			}
		}
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

		// Check if this instance can reach the URL
		if !isURLReachable(targetURL) {
			continue
		}

		// Check if a proxy for this host already exists
		parsedURL, err := url.Parse(targetURL)
		if err != nil {
			continue
		}
		hostname := parsedURL.Hostname()

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