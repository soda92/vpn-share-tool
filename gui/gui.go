package gui

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

//go:embed i18n/*.json
var i18nFS embed.FS

// Config holds the data to be saved to a JSON file.
type Config struct {
	OriginalURLs []string `json:"original_urls"`
	AutoStart    bool     `json:"autostart,omitempty"`
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
	gconfig        Config
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

func Run() {
	startMinimized := flag.Bool("minimized", false, "start minimized with windows start")
	flag.Parse()

	initI18n()

	myApp := app.New()
	myWindow := myApp.NewWindow(l("vpnShareToolTitle"))

	// Channel to signal UI updates from the discovery server
	newProxyChan := make(chan *sharedProxy)
	go startDiscoveryServer(newProxyChan)

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

	// Setup system tray
	if desk, ok := myApp.(desktop.App); ok {
		var autostartMenuItem *fyne.MenuItem
		autostartMenuItem = fyne.NewMenuItem(l("enableAutostartMenuItem"), func() {
			gconfig.AutoStart = !gconfig.AutoStart
			autostartMenuItem.Checked = gconfig.AutoStart
			SetAutostart(gconfig.AutoStart)
			saveConfig()
		})
		autostartMenuItem.Checked = gconfig.AutoStart
		SetAutostart(gconfig.AutoStart)

		menu := fyne.NewMenu("VPN Share Tool",
			fyne.NewMenuItem(l("showMenuItem"), func() {
				myWindow.Show()
			}),
			autostartMenuItem,
			fyne.NewMenuItem(l("exitMenuItem"), func() {
				myApp.Quit()
			}),
		)
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(theme.InfoIcon()) // Using a standard icon
	}

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
	if !*startMinimized {
		myWindow.Show()
	}
	myApp.Run()
}
