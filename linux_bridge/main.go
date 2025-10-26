package main

import (
	"C"
	"encoding/json"
	"log"
	"sync"

	"github.com/soda92/vpn-share-tool/core"
)
import "fmt"

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

}

//export StartApiServerWithPort
func StartApiServerWithPort(port int) {
	go func() {
		if err := core.StartApiServer(port); err != nil {
			log.Printf("Failed to start API server: %v", err)
			eventCallbackMu.Lock()
			if eventCallback != nil {
				event := struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				}{"error", fmt.Sprintf("Failed to start API server: %v", err)}
				data, _ := json.Marshal(event)
				eventCallback(string(data))
			}
			eventCallbackMu.Unlock()
		}
	}()
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
