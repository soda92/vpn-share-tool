package mobile

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/soda92/vpn-share-tool/core"
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

    // The API server will now be started by the explicit Start() call from Dart.
    // Remove the automatic start in init() to avoid race conditions or double starts.
    // The error handling will be done in the Start() function.
}

// Start initializes the core services and returns an error string if API server fails to start.
func Start() string { // Change return type to string
	if err := core.StartApiServer(); err != nil {
		log.Printf("Failed to start API server: %v", err) // Log the error
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
		return err.Error() // Return the error string
	}
	return "" // Return empty string on success
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
