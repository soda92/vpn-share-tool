package handlers

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"

	"github.com/soda92/vpn-share-tool/core/models"
)

type HandleGetActiveProxies struct {
	GetProxies func()     []*models.SharedProxy
}

func(h *HandleGetActiveProxies) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	proxies := h.GetProxies()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proxies); err != nil {
		log.Printf("Failed to encode active proxies to JSON: %v", err)
		http.Error(w, "Failed to encode active proxies", http.StatusInternalServerError)
	}
}
