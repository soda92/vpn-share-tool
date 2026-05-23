package mobile

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/soda92/vpn-share-tool/core"
	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/proxy"
)

// EventCallback is the type for the Dart callback function.
type EventCallback interface {
	OnEvent(eventJSON string)
}

var (
	eventCallbackMu sync.Mutex
	eventCallback   EventCallback
	unifiedEventChan = make(chan string, 100)
)

// SetEventCallback registers a Dart function to be called when events occur.
func SetEventCallback(cb EventCallback) {
	eventCallbackMu.Lock()
	eventCallback = cb
	eventCallbackMu.Unlock()
	log.Println("Event callback registered.")
}

// GetNextEvent blocks until an event is available and returns it.
// This is used by the FFI bridge to avoid thread-safety issues with callbacks.
func GetNextEvent() string {
	return <-unifiedEventChan
}

func init() {
	// Start goroutines to listen for core events and push them to both the callback and the unified channel.
	go func() {
		for p := range proxy.ProxyAddedChan {
			event := struct {
				Type  string      `json:"type"`
				Proxy interface{} `json:"proxy"`
			}{"added", p}
			data, _ := json.Marshal(event)
			eventJSON := string(data)

			// Push to unified channel (for Linux FFI)
			unifiedEventChan <- eventJSON

			// Call callback (for Android Gomobile)
			eventCallbackMu.Lock()
			if eventCallback != nil {
				eventCallback.OnEvent(eventJSON)
			}
			eventCallbackMu.Unlock()
		}
	}()

	go func() {
		for p := range proxy.ProxyRemovedChan {
			event := struct {
				Type  string      `json:"type"`
				Proxy interface{} `json:"proxy"`
			}{"removed", p}
			data, _ := json.Marshal(event)
			eventJSON := string(data)

			unifiedEventChan <- eventJSON

			eventCallbackMu.Lock()
			if eventCallback != nil {
				eventCallback.OnEvent(eventJSON)
			}
			eventCallbackMu.Unlock()
		}
	}()

	go func() {
		for ip := range proxy.IPReadyChan {
			event := struct {
				Type string `json:"type"`
				IP   string `json:"ip"`
			}{"ip_ready", ip}
			data, _ := json.Marshal(event)
			eventJSON := string(data)

			unifiedEventChan <- eventJSON

			eventCallbackMu.Lock()
			if eventCallback != nil {
				eventCallback.OnEvent(eventJSON)
			}
			eventCallbackMu.Unlock()
		}
	}()
}

// StartApiServerWithPort starts the API server on the given port.
func StartApiServerWithPort(apiPort int) {
	go func() {
		if err := core.StartApiServer(apiPort); err != nil {
			log.Printf("Failed to start API server: %v", err)
			eventCallbackMu.Lock()
			if eventCallback != nil {
				event := struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				}{"error", fmt.Sprintf("Failed to start API server: %v", err)}
				data, _ := json.Marshal(event)
				eventCallback.OnEvent(string(data))
			}
			eventCallbackMu.Unlock()
			return
		}
	}()
}

// initializes the core services
func StartGoBackendWithPort(port int) {
	StartApiServerWithPort(port)
}

// ShareURL shares a URL.
func ShareURL(url string) {
	go proxy.ShareUrlAndGetProxy(url, 0)
}

// GetProxies returns the list of shared proxies as a JSON string.
func GetProxies() string {
	proxies := proxy.GetProxies()
	data, _ := json.Marshal(proxies)
	return string(data)
}

// GetIP returns the IP address from the discovery server.
func GetIP() string {
	return core.MyIP
}

// SetDeviceIP allows the mobile side to manually set the device IP.
// This is used because gomobile cannot reliably detect interface IPs on Android/iOS.
func SetDeviceIP(ip string) {
	core.SetMyIP(ip)
}

// SetStoragePath sets the directory where the app should store its files (e.g., database).

func SetStoragePath(path string) {
	debug.DebugStoragePath = path
}
