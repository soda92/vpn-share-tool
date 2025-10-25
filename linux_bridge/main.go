package main

import "C"
import (
	"encoding/json"
	"github.com/soda92/vpn-share-tool/core"
	"log"
	"sync"
	"time"
)

// EventCallback is the type for the Dart callback function.
type EventCallback func(eventJSON string)

var (
	eventCallbackMu sync.Mutex
	eventCallback   EventCallback
)

//export SetEventCallback
func SetEventCallback(cb EventCallback) {
	eventCallbackMu.Lock()
	eventCallback = cb
	eventCallbackMu.Unlock()
	log.Println("Event callback registered from Dart (FFI).")
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
				eventCallback(string(data))
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
				eventCallback(string(data))
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
				eventCallback(string(data))
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

//export Start
func Start() {
	// The API server is now started in init() and heartbeats are sent automatically
	// This function is still needed for gomobile bind to generate a Start() function
}

//export ShareURL
func ShareURL(url *C.char) {
	go core.ShareUrlAndGetProxy(C.GoString(url))
}

//export GetProxies
func GetProxies() *C.char {
	proxies := core.GetProxies()
	data, _ := json.Marshal(proxies)
	return C.CString(string(data))
}

//export GetIP
func GetIP() *C.char {
	return C.CString(core.MyIP)
}

func main() {}