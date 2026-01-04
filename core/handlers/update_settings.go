package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/soda92/vpn-share-tool/core/models"
)

type UpdateSettingsHandler struct {
	GetProxies func() []*models.SharedProxy
}

func (h *UpdateSettingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL      string               `json:"url"`
		Settings models.ProxySettings `json:"settings"`
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

	targetProxy.Mu.Lock()
	targetProxy.Settings = req.Settings
	targetProxy.Mu.Unlock()

	log.Printf("Updated settings for %s: %+v", req.URL, req.Settings)

	w.WriteHeader(http.StatusOK)
}
