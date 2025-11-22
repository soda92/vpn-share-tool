package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func handleToggleDebugProxy(w http.ResponseWriter, r *http.Request) {
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

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Find the instance hosting this proxy
	mutex.Lock()
	activeInstances := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		activeInstances = append(activeInstances, instance)
	}
	mutex.Unlock()

	reqBody, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return
	}

	found := false
	for _, instance := range activeInstances {
		toggleURL := fmt.Sprintf("http://%s/toggle-debug", instance.Address)
		resp, err := http.Post(toggleURL, "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			// Log but continue to other instances
			// It's possible an instance is down or just doesn't have this proxy
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			found = true
			log.Printf("Successfully toggled debug for %s on %s", req.URL, instance.Address)
		}
	}

	if found {
		w.WriteHeader(http.StatusOK)
	} else {
		// If we iterated all and found none, return 404. 
		// Note: If connection failed to the *only* instance hosting it, we technically return 404 here
		// which might be misleading ("Not found" vs "Found but unreachable"), but acceptable for now.
		http.Error(w, "Proxy not found on any reachable instance", http.StatusNotFound)
	}
}
