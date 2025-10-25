package mobile

import (
	"encoding/json"
	"github.com/soda92/vpn-share-tool/core"
	"log"
	"sync"
	"time"
)

// EventCallback is the type for the Dart callback function.
type EventCallback interface {
	OnEvent(eventJSON string)
}

var (
	eventCallbackMu sync.Mutex
	eventCallback   EventCallback
)

// SetEventCallback registers a Dart function to be called when events occur.
// This function is called from Dart via gomobile.
func SetEventCallback(cb EventCallback) {
	eventCallbackMu.Lock()
	eventCallback = cb
	eventCallbackMu.Unlock()
	log.Println("Event callback registered from Dart.")
}

func init() {
	// Start goroutines to listen for core events and push them to Dart.
	go func() {
		for p := range core.ProxyAddedChan {
			eventCallbackMu.Lock()
			if eventCallback != nil {
				event := struct {
					Type  string      `json:"type"`
					Proxy interface{} `json:"proxy"`
				}{"added", p}
				data, _ := json.Marshal(event)
				eventCallback.OnEvent(string(data))
			}
			eventCallbackMu.Unlock()
		}
	}()

	go func() {
		for p := range core.ProxyRemovedChan {
			eventCallbackMu.Lock()
			if eventCallback != nil {
				event := struct {
					Type  string      `json:"type"`
					Proxy interface{} `json:"proxy"`
				}{"removed", p}
				data, _ := json.Marshal(event)
				eventCallback.OnEvent(string(data))
			}
			eventCallbackMu.Unlock()
		}
	}()

	go func() {
		for ip := range core.IPReadyChan {
			eventCallbackMu.Lock()
			if eventCallback != nil {
				event := struct {
					Type string `json:"type"`
					IP   string `json:"ip"`
				}{"ip_ready", ip}
				data, _ := json.Marshal(event)
				eventCallback.OnEvent(string(data))
			}
			eventCallbackMu.Unlock()
		}
	}()

    // Start a goroutine for heartbeats and API server
    go func() {
        core.StartApiServer()
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for range ticker.C {
            core.SendHeartbeat()
        }
    }()
}

// Start initializes the core services.
func Start() {
	// The API server is now started in init() and heartbeats are sent automatically
	// This function is still needed for gomobile bind to generate a Start() function
}

// ShareURL shares a URL.
func ShareURL(url string) {
	go core.ShareUrlAndGetProxy(url)
}

// GetProxies returns the list of shared proxies as a JSON string.
func GetProxies() string {
	proxies := core.GetProxies()
	data, _ := json.Marshal(proxies)
	return string(data)
}

// GetIP returns the IP address from the discovery server.
func GetIP() string {
	return core.MyIP
}
