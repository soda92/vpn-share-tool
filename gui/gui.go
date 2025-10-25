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

const (
	startPort = 10081
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
	proxies             []*sharedProxy
	proxiesLock         sync.RWMutex
	lanIPs              []string
	nextRemotePort      = startPort
	localizer           *i18n.Localizer
	shareUrlAndGetProxy func(rawURL string) (*sharedProxy, error)
	proxyAddedChan      = make(chan *sharedProxy)
	proxyRemovedChan    = make(chan *sharedProxy)
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
func removeProxy(p *sharedProxy) {
	log.Printf("Removing proxy for unreachable URL: %s", p.OriginalURL)

	// 1. Shutdown the HTTP server
	if p.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := p.server.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down proxy server for %s: %v", p.OriginalURL, err)
		}
	}

	// 2. Remove from the global proxies slice
	proxiesLock.Lock()
	newProxies := []*sharedProxy{}
	for _, proxy := range proxies {
		if proxy != p {
			newProxies = append(newProxies, proxy)
		}
	}
	proxies = newProxies
	proxiesLock.Unlock()

	// 3. Signal the UI to update
	proxyRemovedChan <- p
}

// startHealthChecker runs in a goroutine to periodically check if a URL is reachable.
func startHealthChecker(p *sharedProxy) {
	healthCheckTicker := time.NewTicker(3 * time.Minute)
	defer healthCheckTicker.Stop()

	for range healthCheckTicker.C {
		log.Printf("Performing health check for %s", p.OriginalURL)
		if !isURLReachable(p.OriginalURL) {
			log.Printf("Health check failed for %s. Tearing down proxy.", p.OriginalURL)
			removeProxy(p)
			return // Stop this health checker goroutine
		}
		log.Printf("Health check successful for %s", p.OriginalURL)
	}
}

func Run() {
	startMinimized := flag.Bool("minimized", false, "start minimized with windows start")
	flag.Parse()

	initI18n()

	myApp := app.New()
	myWindow := myApp.NewWindow(l("vpnShareToolTitle"))

	// Start the local API server and register with the discovery server
	go startApiServer()
	go registerWithDiscoveryServer()

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

	addProxyToUI := func(newProxy *sharedProxy) {
		fyne.Do(func() {
			for _, ip := range lanIPs {
				sharedURL := fmt.Sprintf("http://%s:%d%s", ip, newProxy.RemotePort, newProxy.Path)
				displayString := l("sharedUrlFormat", map[string]interface{}{
					"originalUrl": newProxy.OriginalURL,
					"sharedUrl":   sharedURL,
				})
				sharedListData.Append(displayString)
			}
		})
		// NOTE: saveConfig() is removed from here to prevent saving during startup loops.
		// The caller is now responsible for saving the config.
	}

	removeProxyFromUI := func(p *sharedProxy) {
		fyne.Do(func() {
			currentList, _ := sharedListData.Get()
			newList := []string{}
			for _, item := range currentList {
				// The display string is "original -> shared".
				// If the original URL is in the string, we can assume it's the one to remove.
				if !strings.Contains(item, p.OriginalURL) {
					newList = append(newList, item)
				}
			}
			sharedListData.Set(newList)
		})
	}

	// Goroutine to handle UI updates from any part of the application
	go func() {
		for {
			select {
			case newProxy := <-proxyAddedChan:
				addProxyToUI(newProxy)
			case removedProxy := <-proxyRemovedChan:
				removeProxyFromUI(removedProxy)
			}
		}
	}()

	sharedList := widget.NewListWithData(
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

	// shareUrlAndGetProxy contains the core logic for creating and starting a proxy.
	// It is designed to be called from both the GUI and API handlers.
	shareUrlAndGetProxy = func(rawURL string) (*sharedProxy, error) {
		if rawURL == "" {
			return nil, fmt.Errorf("URL cannot be empty")
		}

		if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
			rawURL = "http://" + rawURL
		}

		// Prevent adding duplicate proxies
		parsedURL, err := url.Parse(rawURL)
		if err == nil {
			hostname := parsedURL.Hostname()
			proxiesLock.RLock()
			for _, p := range proxies {
				existingURL, err := url.Parse(p.OriginalURL)
				if err != nil {
					continue // Skip invalid stored URL
				}
				if existingURL.Hostname() == hostname {
					proxiesLock.RUnlock()
					log.Printf("Proxy for %s already exists, returning existing one.", rawURL)
					return p, nil // Return existing proxy instead of an error
				}
			}
			proxiesLock.RUnlock()
		}

		// Pass nil for statusLabel as this is a non-GUI context
		newProxy, err := addAndStartProxy(rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("error adding proxy for %s: %w", rawURL, err)
		}

		proxiesLock.Lock()
		proxies = append(proxies, newProxy)
		proxiesLock.Unlock()

		// Announce the new proxy to the UI
		proxyAddedChan <- newProxy

		return newProxy, nil
	}

	// Define the core sharing logic as a function to be reused.
	shareLogic := func(rawURL string) {
		go func() {
			_, err := shareUrlAndGetProxy(rawURL)
			if err != nil {
				// The error might be that it already exists, which is not a critical failure for the user.
				log.Printf("Error sharing URL: %v", err)
				fyne.Do(func() {
					serverStatus.SetText(err.Error())
				})
				return
			}

			fyne.Do(func() {
				urlEntry.SetText("")
			})
		}()
	}

	// Bug fix 2: Handle 'Enter' key in the URL entry field.
	urlEntry.OnSubmitted = func(text string) {
		shareLogic(text)
	}

	addButton := widget.NewButton(l("shareButton"), func() {
		shareLogic(urlEntry.Text)
	})

	// Config is no longer loaded on startup. The application starts with a clean state.

	// Setup system tray
	if desk, ok := myApp.(desktop.App); ok {
		autostartMenuItem := fyne.NewMenuItem(l("setupAutostartMenuItem"), func() {
			SetAutostart(true)
			fyne.CurrentApp().SendNotification(&fyne.Notification{
				Title:   l("autostartEnabledTitle"),
				Content: l("autostartEnabledContent"),
			})
		})

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

	fyne.Do(func() {
		myWindow.SetContent(container.NewBorder(topContent, nil, nil, nil, sharedList))
	})
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
