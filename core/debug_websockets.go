package core

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)


var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections
	},
}

var wsClients = make(map[*websocket.Conn]bool)
var wsMutex = &sync.Mutex{}

func handleDebugWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to websocket: %v", err)
		return
	}
	defer conn.Close()

	wsMutex.Lock()
	wsClients[conn] = true
	wsMutex.Unlock()

	// Keep the connection open
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
	}

	wsMutex.Lock()
	delete(wsClients, conn)
	wsMutex.Unlock()
}

func wsBroadCast(cr *CapturedRequest) {
	// Broadcast the new request to all WebSocket clients
	wsMutex.Lock()
	defer wsMutex.Unlock()
	for client := range wsClients {
		if err := client.WriteJSON(cr); err != nil {
			log.Printf("Error writing to websocket client: %v", err)
			client.Close()
			delete(wsClients, client)
		}
	}
}