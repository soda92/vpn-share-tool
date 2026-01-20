package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	maxLogLines = 1000
	writeWait   = 10 * time.Second
	pongWait    = 60 * time.Second
	pingPeriod  = (pongWait * 9) / 10
)

var (
	logStore = make(map[string][]string)
	logMutex sync.RWMutex

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	// Hub for broadcasting logs to viewers
	logHub = &LogHub{
		viewers: make(map[string]map[*Client]bool),
	}
)

type Client struct {
	hub  *LogHub
	conn *websocket.Conn
	send chan []byte
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type LogHub struct {
	viewers map[string]map[*Client]bool // address -> set of clients
	lock    sync.RWMutex
}

func (h *LogHub) Register(address string, client *Client) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if h.viewers[address] == nil {
		h.viewers[address] = make(map[*Client]bool)
	}
	h.viewers[address][client] = true
}

func (h *LogHub) Unregister(address string, client *Client) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if set, ok := h.viewers[address]; ok {
		if _, ok := set[client]; ok {
			delete(set, client)
			close(client.send)
			if len(set) == 0 {
				delete(h.viewers, address)
			}
		}
	}
}

func (h *LogHub) Broadcast(address string, logs string) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if set, ok := h.viewers[address]; ok {
		for client := range set {
			select {
			case client.send <- []byte(logs):
			default:
				close(client.send)
				delete(set, client)
			}
		}
	}
}

type LogEntry struct {
	Address string `json:"address"`
	Logs    string `json:"logs"` // New logs to append
}

func handleUploadLogs(w http.ResponseWriter, r *http.Request) {
	// Support both HTTP POST and WebSocket
	if r.Header.Get("Upgrade") == "websocket" {
		handleWSUpload(w, r)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var entry LogEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	processLogs(entry.Address, entry.Logs)
	w.WriteHeader(http.StatusOK)
}

func handleWSUpload(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade log upload WS: %v", err)
		return
	}
	defer conn.Close()

	for {
		// Expect JSON messages with address and logs
		// Or if the connection is dedicated to one address, we could shake hands first.
		// For simplicity, let's assume the client sends the same JSON structure as POST.
		var entry LogEntry
		err := conn.ReadJSON(&entry)
		if err != nil {
			log.Printf("log upload WS read error: %v", err)
			break
		}
		processLogs(entry.Address, entry.Logs)
	}
}

func processLogs(address, logs string) {
	if address == "" {
		return
	}

	// 1. Store
	logMutex.Lock()
	lines := strings.Split(strings.ReplaceAll(logs, "\r\n", "\n"), "\n")
	existing := logStore[address]
	existing = append(existing, lines...)
	if len(existing) > maxLogLines {
		existing = existing[len(existing)-maxLogLines:]
	}
	logStore[address] = existing
	logMutex.Unlock()

	// 2. Broadcast
	logHub.Broadcast(address, logs)
}

func handleGetLogs(w http.ResponseWriter, r *http.Request) {
	// Support WebSocket for real-time viewing
	if r.Header.Get("Upgrade") == "websocket" {
		handleWSView(w, r)
		return
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "Address query parameter is required", http.StatusBadRequest)
		return
	}

	logMutex.RLock()
	logs, ok := logStore[address]
	logMutex.RUnlock()

	if !ok {
		logs = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"address": address,
		"logs":    logs,
		"ts":      time.Now(),
	})
}

func handleWSView(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "Address required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade log view WS: %v", err)
		return
	}

	client := &Client{hub: logHub, conn: conn, send: make(chan []byte, 256)}
	client.hub.Register(address, client)

	// Send existing logs
	logMutex.RLock()
	existing, ok := logStore[address]
	logMutex.RUnlock()
	if ok {
		fullLog := strings.Join(existing, "\n")
		select {
		case client.send <- []byte(fullLog):
		default:
		}
	}

	go client.writePump()

	// readPump equivalent logic
	defer func() {
		client.hub.Unregister(address, client)
		client.conn.Close()
	}()

	client.conn.SetReadLimit(512)
	client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error { client.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
	}
}
