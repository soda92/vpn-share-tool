package gui

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/soda92/vpn-share-tool/core"
)

//go:embed i18n/*.json
var i18nFS embed.FS

const (
	startPort = 10081
)

func Run() {
	proxyURL := flag.String("proxy-url", "", "URL to proxy on startup")
	startMinimized := flag.Bool("minimized", false, "start minimized with windows start")
	flag.Parse()

	initI18n()
	SetAutostart(true)

	myApp := app.New()
	myWindow := myApp.NewWindow(l("vpnShareToolTitle"))

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

	go func() {
		ip := <-core.IPReadyChan
		fyne.Do(func() {
			serverStatus.SetText(fmt.Sprintf("Server running on: %s", ip))
		})
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
			// Wait for the IP address to be ready
			ip := <-core.IPReadyChan

			newProxy, err := core.ShareUrlAndGetProxy(*proxyURL)
			if err != nil {
				log.Printf("Error sharing URL from command line: %v", err)
				return
			}

			// Print the shared URL to the console
			sharedURL := fmt.Sprintf("http://%s:%d%s", ip, newProxy.RemotePort, newProxy.Path)
			fmt.Println("--- SHARED URL ---")
			fmt.Println(sharedURL)
			fmt.Println("------------------")

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
		myWindow.Hide()
	})
	myWindow.SetOnClosed(func() {
		core.Shutdown()
	})

	myWindow.Resize(fyne.NewSize(600, 400))
	if !*startMinimized && *proxyURL == "" {
		myWindow.Show()
	}
	myApp.Run()
}
