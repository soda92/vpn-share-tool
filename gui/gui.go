package gui

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
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
	serverAddr    = "127.0.0.1"
	serverPort    = 7000
	vhostHTTPPort = 8080
)

// sharedProxy holds information about a shared URL.
type sharedProxy struct {
	OriginalURL string
	SharedURL   string
	service     *client.Service
	cancel      context.CancelFunc
	tmpFile     string
}

var (
	proxies     []*sharedProxy
	proxiesLock sync.RWMutex
	lanIP       string
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
		// Clean up all running frpc services and temp files on exit
		proxiesLock.Lock()
		defer proxiesLock.Unlock()
		for _, p := range proxies {
			p.cancel()
			os.Remove(p.tmpFile)
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
	localPort := parsedURL.Port()
	if localPort == "" {
		if parsedURL.Scheme == "https" {
			localPort = "443"
		} else {
			localPort = "80"
		}
	}

	// Each proxy needs a unique name.
	proxyName := fmt.Sprintf("web_%s_%s", strings.ReplaceAll(localHost, ".", "_"), localPort)

	// Generate config as a string for the legacy INI format
	clientCfgStr := fmt.Sprintf(`
[common]
server_addr = %s
server_port = %d

[%s]
type = http
local_ip = %s
local_port = %s
custom_domains = %s
`, serverAddr, serverPort, proxyName, localHost, localPort, lanIP)

	// Write config to a temporary file
	tmpfile, err := ioutil.TempFile("", "frpc-*.ini")
	if err != nil {
		return nil, fmt.Errorf("could not create temp file: %w", err)
	}
	if _, err := tmpfile.Write([]byte(clientCfgStr)); err != nil {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
		return nil, fmt.Errorf("could not write to temp file: %w", err)
	}
	tmpfile.Close()

	// Load config from the temporary file
	clientCfg, pxyCfgs, visitorCfgs, _, err := config.LoadClientConfig(tmpfile.Name(), false)
	if err != nil {
		os.Remove(tmpfile.Name())
		return nil, fmt.Errorf("failed to load client config from file: %w", err)
	}

	// FIX 1: Pass ServiceOptions by value, not by pointer.
	service, err := client.NewService(client.ServiceOptions{
		Common:      clientCfg,
		ProxyCfgs:   pxyCfgs,
		VisitorCfgs: visitorCfgs,
	})
	if err != nil {
		os.Remove(tmpfile.Name())
		return nil, fmt.Errorf("failed to create client service: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer os.Remove(tmpfile.Name()) // Clean up file when service stops
		if err := service.Run(ctx); err != nil {
			log.Printf("frpc service [%s] exited with error: %v", proxyName, err)
		}
	}()

	newProxy := &sharedProxy{
		OriginalURL: rawURL,
		SharedURL:   fmt.Sprintf("http://%s:%d", lanIP, vhostHTTPPort),
		service:     service,
		cancel:      cancel,
		tmpFile:     tmpfile.Name(),
	}

	return newProxy, nil
}

func startFrps(statusLabel *widget.Label) {
	serverCfgStr := fmt.Sprintf(`
[common]
bind_addr = %s
bind_port = %d
vhost_http_port = %d
`, serverAddr, serverPort, vhostHTTPPort)

	tmpfile, err := ioutil.TempFile("", "frps-*.ini")
	if err != nil {
		statusLabel.SetText(fmt.Sprintf("Error creating temp file: %v", err))
		return
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(serverCfgStr)); err != nil {
		tmpfile.Close()
		statusLabel.SetText(fmt.Sprintf("Error writing temp file: %v", err))
		return
	}
	tmpfile.Close()

	serverCfg, _, err := config.LoadServerConfig(tmpfile.Name(), false)
	if err != nil {
		statusLabel.SetText(fmt.Sprintf("Error loading server config: %v", err))
		return
	}

	// FIX 2: Pass serverCfg directly to NewService.
	service, err := server.NewService(serverCfg)
	if err != nil {
		statusLabel.SetText(fmt.Sprintf("Error creating server service: %v", err))
		return
	}

	statusLabel.SetText(fmt.Sprintf("Server running. Share base URL: http://%s:%d", lanIP, vhostHTTPPort))

	// FIX 3: Call Run without expecting a return value. It's a blocking call.
	service.Run(context.Background())
	log.Printf("Server service has stopped.")
	statusLabel.SetText("Server stopped.")
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
