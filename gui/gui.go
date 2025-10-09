package gui

import (
	"context"
	"embed"
	"encoding/json"
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
	"github.com/Xuanwo/go-locale"
	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config"
	"github.com/fatedier/frp/server"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed i18n/*.json
var i18nFS embed.FS

const (
	serverBindAddr = "0.0.0.0"
	serverAddr     = "127.0.0.1"
	startPort      = 10081
)

// sharedProxy holds information about a shared URL.
type sharedProxy struct {
	OriginalURL string
	RemotePort  int
	Path        string
	service     *client.Service
	cancel      context.CancelFunc
	tmpFile     string
}

var (
	proxies        []*sharedProxy
	proxiesLock    sync.RWMutex
	lanIPs         []string
	serverPort     = 7000 // Start with default server port
	nextRemotePort = startPort
	localizer      *i18n.Localizer
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

	var err error
	lanIPs, err = getLanIPs()
	if err != nil {
		log.Printf(l("couldNotDetermineLanIp", map[string]interface{}{"ip": "N/A", "error": err}))
		lanIPs = []string{}
	}

	// Server section
	serverStatus := widget.NewLabel(l("startingServer"))
	go startFrps(serverStatus)

	// Client/Proxy section
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder(l("urlPlaceholder"))

	sharedListData := binding.NewStringList()

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

		newProxy, err := addAndStartProxy(rawURL)
		if err != nil {
			log.Printf(l("errorAddingProxy", map[string]interface{}{"url": rawURL, "error": err}))
			return
		}

		proxiesLock.Lock()
		proxies = append(proxies, newProxy)
		proxiesLock.Unlock()

		// Add a line for each IP address
		for _, ip := range lanIPs {
			// Append the path from the original URL.
			sharedURL := fmt.Sprintf("http://%s:%d%s", ip, newProxy.RemotePort, newProxy.Path)
			displayString := l("sharedUrlFormat", map[string]interface{}{
				"originalUrl": newProxy.OriginalURL,
				"sharedUrl":   sharedURL,
			})
			sharedListData.Append(displayString)
		}

		urlEntry.SetText("")
	}

	urlEntry.OnSubmitted = func(text string) {
		shareLogic(text)
	}

	addButton := widget.NewButton(l("shareButton"), func() {
		shareLogic(urlEntry.Text)
	})

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
		return nil, fmt.Errorf(l("invalidUrl", map[string]interface{}{"error": err}))
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

	// Each proxy needs a unique name.
	proxyName := fmt.Sprintf("web_%s_%s", strings.ReplaceAll(localHost, ".", "_"), localPort)

	// Generate config as a string for the legacy INI format
	clientCfgStr := fmt.Sprintf(`
[common]
server_addr = %s
server_port = %d

[%s]
type = tcp
local_ip = %s
local_port = %s
remote_port = %d
`, serverAddr, serverPort, proxyName, localHost, localPort, remotePort)

	// Write config to a temporary file
	tmpfile, err := ioutil.TempFile("", "frpc-*.ini")
	if err != nil {
		return nil, fmt.Errorf(l("couldNotCreateTempFile", map[string]interface{}{"error": err}))
	}
	if _, err := tmpfile.Write([]byte(clientCfgStr)); err != nil {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
		return nil, fmt.Errorf(l("couldNotWriteToTempFile", map[string]interface{}{"error": err}))
	}
	tmpfile.Close()

	// Load config from the temporary file
	clientCfg, pxyCfgs, visitorCfgs, _, err := config.LoadClientConfig(tmpfile.Name(), false)
	if err != nil {
		os.Remove(tmpfile.Name())
		return nil, fmt.Errorf(l("failedToLoadClientConfig", map[string]interface{}{"error": err}))
	}

	// FIX 1: Pass ServiceOptions by value, not by pointer.
	service, err := client.NewService(client.ServiceOptions{
		Common:      clientCfg,
		ProxyCfgs:   pxyCfgs,
		VisitorCfgs: visitorCfgs,
	})
	if err != nil {
		os.Remove(tmpfile.Name())
		return nil, fmt.Errorf(l("failedToCreateClientService", map[string]interface{}{"w": err}))
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer os.Remove(tmpfile.Name()) // Clean up file when service stops
		if err := service.Run(ctx); err != nil {
			log.Printf(l("frpcServiceExited", map[string]interface{}{"proxyName": proxyName, "error": err}))
		}
	}()

	newProxy := &sharedProxy{
		OriginalURL: rawURL,
		RemotePort:  remotePort,
		Path:        parsedURL.Path,
		service:     service,
		cancel:      cancel,
		tmpFile:     tmpfile.Name(),
	}

	return newProxy, nil
}

func startFrps(statusLabel *widget.Label) {
	proxiesLock.Lock()
	var foundPort int
	port := serverPort
	for i := 0; i < 100; i++ { // Try up to 100 ports from the base
		if isPortAvailable(port + i) {
			foundPort = port + i
			break
		}
	}
	if foundPort == 0 {
		proxiesLock.Unlock()
		statusLabel.SetText(l("errorNoServerPort"))
		return
	}
	serverPort = foundPort // Update global server port for clients to use
	proxiesLock.Unlock()

	serverCfgStr := fmt.Sprintf(`
[common]
bind_addr = %s
bind_port = %d
`, serverBindAddr, serverPort)

	tmpfile, err := ioutil.TempFile("", "frps-*.ini")
	if err != nil {
		statusLabel.SetText(l("errorCreatingTempFile", map[string]interface{}{"error": err}))
		return
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(serverCfgStr)); err != nil {
		tmpfile.Close()
		statusLabel.SetText(l("errorWritingTempFile", map[string]interface{}{"error": err}))
		return
	}
	tmpfile.Close()

	serverCfg, _, err := config.LoadServerConfig(tmpfile.Name(), false)
	if err != nil {
		statusLabel.SetText(l("errorLoadingServerConfig", map[string]interface{}{"error": err}))
		return
	}

	// FIX 2: Pass serverCfg directly to NewService.
	service, err := server.NewService(serverCfg)
	if err != nil {
		statusLabel.SetText(l("errorCreatingServerService", map[string]interface{}{"error": err}))
		return
	}

	statusLabel.SetText(l("serverRunning"))

	// FIX 3: Call Run without expecting a return value. It's a blocking call.
	service.Run(context.Background())
	log.Printf("Server service has stopped.")
	statusLabel.SetText(l("serverStopped"))
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