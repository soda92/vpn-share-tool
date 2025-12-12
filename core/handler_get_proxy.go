package core

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
)


func handleGetActiveProxies(w http.ResponseWriter, r *http.Request) {
	proxies := GetProxies()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proxies); err != nil {
		log.Printf("Failed to encode active proxies to JSON: %v", err)
		http.Error(w, "Failed to encode active proxies", http.StatusInternalServerError)
	}
}
