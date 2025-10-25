package main

import "C"
import (
	"encoding/json"
	"github.com/soda92/vpn-share-tool/core"
)

//export Start
func Start() {
	go core.StartApiServer()
}

//export ShareURL
func ShareURL(url *C.char) {
	go core.ShareUrlAndGetProxy(C.GoString(url))
}

//export PollEvents
func PollEvents() *C.char {
	type Event struct {
		Type  string      `json:"type"`
		Proxy interface{} `json:"proxy"`
	}

	select {
	case p := <-core.ProxyAddedChan:
		event := Event{Type: "added", Proxy: p}
		data, _ := json.Marshal(event)
		return C.CString(string(data))
	case p := <-core.ProxyRemovedChan:
		event := Event{Type: "removed", Proxy: p}
		data, _ := json.Marshal(event)
		return C.CString(string(data))
	case ip := <-core.IPReadyChan:
		event := struct {
			Type string `json:"type"`
			IP   string `json:"ip"`
		}{"ip_ready", ip}
		data, _ := json.Marshal(event)
		return C.CString(string(data))
	}
}

func main() {}