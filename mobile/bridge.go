package mobile

import (
	"encoding/json"
	"github.com/soda92/vpn-share-tool/core"
)

// Start initializes the core services.
func Start() {
	go core.StartApiServer()
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

// PollEvents waits for an event from the core and returns it as a JSON string.
func PollEvents() string {
	type Event struct {
		Type  string      `json:"type"`
		Proxy interface{} `json:"proxy"`
	}

	select {
	case p := <-core.ProxyAddedChan:
		event := Event{Type: "added", Proxy: p}
		data, _ := json.Marshal(event)
		return string(data)
	case p := <-core.ProxyRemovedChan:
				event := Event{Type: "removed", Proxy: p}
				data, _ := json.Marshal(event)
				return string(data)
			case ip := <-core.IPReadyChan:
				event := struct {
					Type string `json:"type"`
					IP   string `json:"ip"`
				}{"ip_ready", ip}
				data, _ := json.Marshal(event)
				return string(data)
		}}
