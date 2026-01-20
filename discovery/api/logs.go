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

const maxLogLines = 1000

var (
	logStore = make(map[string][]string)
	logMutex sync.RWMutex

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	// Hub for broadcasting logs to viewers
	logHub = &LogHub{
		viewers: make(map[string]map[*websocket.Conn]bool),
	}
)

type LogHub struct {
	viewers map[string]map[*websocket.Conn]bool // address -> set of connections
	lock    sync.RWMutex
}

func (h *LogHub) Register(address string, conn *websocket.Conn) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if h.viewers[address] == nil {
		h.viewers[address] = make(map[*websocket.Conn]bool)
	}
	h.viewers[address][conn] = true
}

func (h *LogHub) Unregister(address string, conn *websocket.Conn) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if set, ok := h.viewers[address]; ok {
		delete(set, conn)
		if len(set) == 0 {
			delete(h.viewers, address)
		}
	}
}

func (h *LogHub) Broadcast(address string, logs string) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if set, ok := h.viewers[address]; ok {
		for conn := range set {
			// Write message non-blocking or with timeout?
			// For simplicity, blocking write, but in production use a write pump.
			// Here we just ignore errors (handled by reader/pinger)
			_ = conn.WriteMessage(websocket.TextMessage, []byte(logs))
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
	defer conn.Close()

	// Register viewer
	logHub.Register(address, conn)
	defer logHub.Unregister(address, conn)

	// Send existing logs first?
	logMutex.RLock()
	existing, ok := logStore[address]
	logMutex.RUnlock()
	if ok {
		// Join them back to string for transmission
		fullLog := strings.Join(existing, "\n")
		conn.WriteMessage(websocket.TextMessage, []byte(fullLog))
	}

	// Keep connection alive
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Printf("log view WS read error: %v", err)
			break
		}
	}
}
