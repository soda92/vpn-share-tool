package handlers

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"

	"github.com/soda92/vpn-share-tool/core/models"
)

type ToggleDebugHandler struct {
	GetProxies func() []*models.SharedProxy
}

func (h *ToggleDebugHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	proxies := h.GetProxies()
	var targetProxy *models.SharedProxy
	for _, p := range proxies {
		if p.OriginalURL == req.URL {
			targetProxy = p
			break
		}
	}

	if targetProxy == nil {
		http.NotFound(w, r)
		return
	}

	// Update the setting using thread-safe method
	targetProxy.SetEnableDebug(req.Enable)
	log.Printf("Updated debug for %s to %v", req.URL, req.Enable)

	w.WriteHeader(http.StatusOK)
}
