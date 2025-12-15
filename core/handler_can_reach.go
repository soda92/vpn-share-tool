package core

import (
	_ "embed"
	"encoding/json"
	"net/http"
)

func canReachHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "url query parameter is required", http.StatusBadRequest)
		return
	}

	reachable := IsURLReachable(targetURL)

	response := struct {
		Reachable bool `json:"reachable"`
	}{
		Reachable: reachable,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
