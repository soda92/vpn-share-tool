package gui

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config"
	"github.com/fatedier/frp/server"
)

const (
	serverAddr = "127.0.0.1"
	serverPort = 7000
)

// sharedProxy holds information about a shared URL.
type sharedProxy struct {
	OriginalURL string
	SharedURL   string
	service     *client.Service
	cancel      context.CancelFunc
}

var (
	proxies        []*sharedProxy
	proxiesLock    sync.RWMutex
	lanIP          string
	nextRemotePort = 8081 // Starting port for shared URLs
)

func Run() {
	myApp := app.New()
	myWindow := myApp.NewWindow("VPN Share Tool")

	var err error
	lanIP, err = getLanIP()
	if err != nil {
		lanIP = "127.0.0.1" // Fallback
		log.Printf("Could not determine LAN IP, falling back to %s: %v", lanIP, err)
	}

	// Server section
	serverStatus := widget.NewLabel("Starting server...")
	go startFrps(serverStatus)

	// Client/Proxy section
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("http://internal.site.com or http://localhost:3000")

	sharedListData := binding.NewStringList()

	sharedList := widget.NewListWithData(
		sharedListData,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			o.(*widget.Label).Bind(i.(binding.String))
		},
	)

	addButton := widget.NewButton("Share", func() {
		rawURL := urlEntry.Text
		if rawURL == "" {
			return
		}

		newProxy, err := addAndStartProxy(rawURL)
		if err != nil {
			log.Printf("Error adding proxy for %s: %v", rawURL, err)
			// Optionally, show this error in the UI
			return
		}

		proxiesLock.Lock()
		proxies = append(proxies, newProxy)
		proxiesLock.Unlock()

		sharedListData.Append(fmt.Sprintf("%s -> %s", newProxy.OriginalURL, newProxy.SharedURL))
		urlEntry.SetText("")
	})

	myWindow.SetContent(container.NewVBox(
		widget.NewLabel("Local Server Status"),
		serverStatus,
		widget.NewSeparator(),
		widget.NewLabel("Add a URL to share"),
		urlEntry,
		addButton,
		widget.NewSeparator(),
		widget.NewLabel("Shared URLs (accessible on your LAN)"),
		sharedList,
	))

	myWindow.SetOnClosed(func() {
		// Clean up all running frpc services on exit
		proxiesLock.Lock()
		defer proxiesLock.Unlock()
		for _, p := range proxies {
			p.cancel()
		}
	})

	myWindow.Resize(fyne.NewSize(600, 400))
	myWindow.ShowAndRun()
}

func addAndStartProxy(rawURL string) (*sharedProxy, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	localHost := parsedURL.Hostname()
	localPortStr := parsedURL.Port()
	if localPortStr == "" {
		if parsedURL.Scheme == "https" {
			localPortStr = "443"
		} else {
			localPortStr = "80"
		}
	}
	localPort, err := strconv.Atoi(localPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port in URL: %w", err)
	}

	proxiesLock.Lock()
	remotePort := nextRemotePort
	nextRemotePort++
	proxiesLock.Unlock()

	proxyName := fmt.Sprintf("web_%s_%d", strings.ReplaceAll(localHost, ".", "_"), remotePort)

	// Build config structs directly
	cfg := &config.ClientCommonConf{}
	config.MustLoadDefaultClientConf(cfg)
	cfg.ServerAddr = serverAddr
	cfg.ServerPort = serverPort

	pxyCfgs := make(map[string]config.ProxyConf)
	pxyCfgs[proxyName] = &config.HTTPProxyConf{
		BaseProxyConf: config.BaseProxyConf{
			ProxyName: proxyName,
			ProxyType: "http",
			LocalSvrConf: config.LocalSvrConf{
				LocalIP:   localHost,
				LocalPort: localPort,
			},
		},
		RemotePort: remotePort,
	}

	service, err := client.NewService(cfg, pxyCfgs, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create client service: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := service.Run(ctx); err != nil {
			log.Printf("frpc service [%s] exited with error: %v", proxyName, err)
		}
	}()

	newProxy := &sharedProxy{
		OriginalURL: rawURL,
		SharedURL:   fmt.Sprintf("http://%s:%d", lanIP, remotePort),
		service:     service,
		cancel:      cancel,
	}

	return newProxy, nil
}

func startFrps(statusLabel *widget.Label) {
	// Build server config struct directly
	cfg := config.GetDefaultServerConf()
	cfg.BindAddr = serverAddr
	cfg.BindPort = serverPort

	service, err := server.NewService(cfg)
	if err != nil {
		statusLabel.SetText(fmt.Sprintf("Error: %v", err))
		return
	}

	statusLabel.SetText(fmt.Sprintf("Server running on %s:%d. Ready to share.", serverAddr, serverPort))
	if err = service.Run(context.Background()); err != nil {
		statusLabel.SetText(fmt.Sprintf("Server failed: %v", err))
	}
}

// getLanIP finds the local IP address of the machine.
func getLanIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no suitable LAN IP found")
}
