package debug

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
)

var (
	ProxyProvider   func() []*models.SharedProxy
	ShareProxyFunc  func(url string, requestedPort int) (*models.SharedProxy, error)
	RemoveProxyFunc func(port int) error
	InstanceIP      func() string
)

type ProxyInfoResponse struct {
	OriginalURL   string               `json:"original_url"`
	RemotePort    int                  `json:"remote_port"`
	Path          string               `json:"path"`
	SharedURL     string               `json:"shared_url"`
	Settings      models.ProxySettings `json:"settings"`
	ActiveSystems []string             `json:"active_systems"`
	RequestRate   float64              `json:"request_rate"`
	TotalRequests int64                `json:"total_requests"`
}

func handleDebugProxies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodGet:
		var proxies []*models.SharedProxy
		if ProxyProvider != nil {
			proxies = ProxyProvider()
		}
		var res []ProxyInfoResponse
		for _, p := range proxies {
			p.Mu.RLock()
			systems := make([]string, len(p.ActiveSystems))
			copy(systems, p.ActiveSystems)
			totalReq := p.TotalRequests
			reqRate := p.RequestRate
			p.Mu.RUnlock()

			host := "127.0.0.1"
			if InstanceIP != nil && InstanceIP() != "" {
				host = InstanceIP()
			}
			sharedURL := fmt.Sprintf("http://%s:%d%s", host, p.RemotePort, p.Path)

			res = append(res, ProxyInfoResponse{
				OriginalURL:   p.OriginalURL,
				RemotePort:    p.RemotePort,
				Path:          p.Path,
				SharedURL:     sharedURL,
				Settings:      p.Settings,
				ActiveSystems: systems,
				RequestRate:   reqRate,
				TotalRequests: totalReq,
			})
		}
		if res == nil {
			res = []ProxyInfoResponse{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)

	case http.MethodPost:
		var req struct {
			URL           string `json:"url"`
			RequestedPort int    `json:"requested_port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.URL) == "" {
			http.Error(w, "Valid URL is required", http.StatusBadRequest)
			return
		}

		if ShareProxyFunc == nil {
			http.Error(w, "Proxy creation service unavailable", http.StatusServiceUnavailable)
			return
		}

		p, err := ShareProxyFunc(strings.TrimSpace(req.URL), req.RequestedPort)
		if err != nil {
			log.Printf("Failed to create proxy for %s: %v", req.URL, err)
			http.Error(w, fmt.Sprintf("Failed to create proxy: %v", err), http.StatusInternalServerError)
			return
		}

		host := "127.0.0.1"
		if InstanceIP != nil && InstanceIP() != "" {
			host = InstanceIP()
		}
		sharedURL := fmt.Sprintf("http://%s:%d%s", host, p.RemotePort, p.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ProxyInfoResponse{
			OriginalURL:   p.OriginalURL,
			RemotePort:    p.RemotePort,
			Path:          p.Path,
			SharedURL:     sharedURL,
			Settings:      p.Settings,
			ActiveSystems: p.ActiveSystems,
		})

	case http.MethodDelete:
		portStr := r.URL.Query().Get("port")
		if portStr == "" {
			var body struct {
				Port int `json:"port"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Port > 0 {
				portStr = strconv.Itoa(body.Port)
			}
		}

		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 {
			http.Error(w, "Valid port query parameter or body is required", http.StatusBadRequest)
			return
		}

		if RemoveProxyFunc == nil {
			http.Error(w, "Proxy removal service unavailable", http.StatusServiceUnavailable)
			return
		}

		if err := RemoveProxyFunc(port); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"success": true, "port": port})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleDebugLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	linesLimit := 200
	if l := r.URL.Query().Get("lines"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			linesLimit = val
			if linesLimit > 1000 {
				linesLimit = 1000
			}
		}
	}

	var logPath string
	if DebugStoragePath != "" {
		logPath = filepath.Join(DebugStoragePath, "vpn-share-tool.log")
	} else if home, err := os.UserHomeDir(); err == nil {
		logPath = filepath.Join(home, ".vpn-share-tool", "vpn-share-tool.log")
	} else {
		logPath = "vpn-share-tool.log"
	}

	file, err := os.Open(logPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"lines": []string{}})
		return
	}
	defer file.Close()

	var allLines []string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	start := 0
	if len(allLines) > linesLimit {
		start = len(allLines) - linesLimit
	}
	tailLines := allLines[start:]
	if tailLines == nil {
		tailLines = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"lines": tailLines})
}
