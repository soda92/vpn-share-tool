package handlers

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/soda92/vpn-share-tool/core/models"
)

type HandleToggleDebug struct {
	Proxies     []*models.SharedProxy
	ProxiesLock sync.RWMutex
}

func (h *HandleToggleDebug) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL    string `json:"url"`
		Enable bool   `json:"enable"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.ProxiesLock.RLock()
	var targetProxy *models.SharedProxy
	for _, p := range h.Proxies {
		if p.OriginalURL == req.URL {
			targetProxy = p
			break
		}
	}
	h.ProxiesLock.RUnlock()

	if targetProxy == nil {
		http.NotFound(w, r)
		return
	}

	// Update the setting using thread-safe method
	targetProxy.SetEnableDebug(req.Enable)
	log.Printf("Updated debug for %s to %v", req.URL, req.Enable)

	w.WriteHeader(http.StatusOK)
}
