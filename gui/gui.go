package gui

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/soda92/vpn-share-tool/core"
)

//go:embed i18n/*.json
var i18nFS embed.FS

//go:embed version.txt
var versionFile string

var Version = "dev"

const (
	startPort = 10081
)

func checkUpdate(w fyne.Window) {
	info, err := core.CheckForUpdates()
	if err != nil {
		log.Printf("Failed to check for updates: %v", err)
		return
	}

	if info.Version != Version && Version != "dev" {
		dialog.ShowConfirm(
			l("updateAvailableTitle"),
			l("updateAvailableContent", map[string]interface{}{"version": info.Version}),
			func(b bool) {
				if b {
					// Perform update via core logic
					if err := core.ApplyUpdate(info); err != nil {
						dialog.ShowError(err, w)
					}
					// ApplyUpdate exits on success, so we only reach here on error
				}
			},
			w,
		)
	}
}

// safeMultiWriter writes to multiple writers, ignoring errors from individual writers
type safeMultiWriter struct {
	writers []io.Writer
}

func (t *safeMultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, _ = w.Write(p) // Ignore errors (e.g. from closed stdout)
	}
	return len(p), nil
}

func Run() {
	// Setup Logging
	logFile, err := os.OpenFile("vpn-share-tool.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		// Just print to stdout if file fails
		fmt.Printf("Failed to open log file: %v\n", err)
	} else {
		log.SetOutput(&safeMultiWriter{writers: []io.Writer{os.Stdout, logFile}})
	}

	// Clean up update script if present (from previous update).
	// We ignore the error because the file usually doesn't exist, which is fine.
	os.Remove("update.bat")

	proxyURL := flag.String("proxy-url", "", "URL to proxy on startup")
	startMinimized := flag.Bool("minimized", false, "start minimized with windows start")
	flag.Parse()

	if v := strings.TrimSpace(versionFile); v != "" {
		Version = v
	}
	core.Version = Version

	log.Printf("Starting VPN Share Tool version %s", Version)

	initI18n()
	SetAutostart(true)

	myApp := app.New()
	myWindow := myApp.NewWindow(l("vpnShareToolTitle") + " " + Version)
	isVisible := !*startMinimized // Track visibility state

	// Setup restart args provider for updates
	core.SetRestartArgsProvider(func() []string {
		if !isVisible {
			return []string{"--minimized"}
		}
		return nil
	})

	// Find an available port for the API server
	apiPort, err := findAvailablePort(startPort)
	if err != nil {
		log.Fatalf("Failed to find available API port: %v", err)
	}

	// Start the local API server and register with the discovery server
	go func() {
		if err := core.StartApiServer(apiPort); err != nil {
			log.Fatalf("API server stopped with error: %v", err)
		}
	}()

	// Server section
	serverStatus := widget.NewLabel(l("startingServer"))

	// Channel to signal the startup proxy logic that IP is ready
	startupProxyChan := make(chan string, 1)

	go func() {
		for ip := range core.IPReadyChan {
			localIP := ip
			fyne.Do(func() {
				serverStatus.SetText(fmt.Sprintf("Server running on: %s", localIP))
				// Check for updates after connection is established
				checkUpdate(myWindow)
			})

			// Signal startup proxy logic if it's waiting
			select {
			case startupProxyChan <- ip:
			default:
			}
		}
	}()

	// Client/Proxy section
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder(l("urlPlaceholder"))

	sharedListData := binding.NewStringList()

	addProxyToUI := func(newProxy *core.SharedProxy) {
		fyne.Do(func() {
			if core.MyIP != "" {
				sharedURL := fmt.Sprintf("http://%s:%d%s", core.MyIP, newProxy.RemotePort, newProxy.Path)
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

	if *proxyURL != "" {
		go func() {
			// Wait for the IP address to be ready via our local broadcast channel
			ip := <-startupProxyChan

			newProxy, err := core.ShareUrlAndGetProxy(*proxyURL)
			if err != nil {
				log.Printf("Error sharing URL from command line: %v", err)
				return
			}

			// Print the shared URL to the console
			sharedURL := fmt.Sprintf("http://%s:%d%s", ip, newProxy.RemotePort, newProxy.Path)
			log.Println("--- SHARED URL ---")
			log.Println(sharedURL)
			log.Println("------------------")

			// Also add it to the UI list for consistency
			addProxyToUI(newProxy)
		}()
	}

	removeProxyFromUI := func(p *core.SharedProxy) {
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
			case newProxy := <-core.ProxyAddedChan:
				addProxyToUI(newProxy)
			case removedProxy := <-core.ProxyRemovedChan:
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

	shareLogic := func(rawURL string) {
		go func() {
			_, err := core.ShareUrlAndGetProxy(rawURL)
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
		menu := fyne.NewMenu("VPN Share Tool",
			fyne.NewMenuItem(l("showMenuItem"), func() {
				isVisible = true
				myWindow.Show()
			}),
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
		isVisible = false
		myWindow.Hide()
	})
	myWindow.SetOnClosed(func() {
		core.Shutdown()
	})

	myWindow.Resize(fyne.NewSize(600, 400))
	if !*startMinimized && *proxyURL == "" {
		isVisible = true // Explicitly set true (redundant but safe)
		myWindow.Show()
	}
	myApp.Run()
}
