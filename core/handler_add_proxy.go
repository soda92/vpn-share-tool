package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
)

func addProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	newProxy, err := ShareUrlAndGetProxy(req.URL, 0)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create proxy: %v", err), http.StatusInternalServerError)
		return
	}

	// Construct the response containing the full proxy details
	// This ensures the client gets the externally accessible URL.
	var sharedURL string
	if MyIP != "" {
		sharedURL = fmt.Sprintf("http://%s:%d%s", MyIP, newProxy.RemotePort, newProxy.Path)
	}

	type sharedURLInfo struct {
		OriginalURL string `json:"original_url"`
		SharedURL   string `json:"shared_url"`
	}

	response := sharedURLInfo{
		OriginalURL: newProxy.OriginalURL,
		SharedURL:   sharedURL,
	}

	// Respond with the details of the new proxy
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
