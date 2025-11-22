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

	found := false
	for _, instance := range activeInstances {
		toggleURL := fmt.Sprintf("http://%s/toggle-debug", instance.Address)
		reqBody, err := json.Marshal(req)
		if err != nil {
			log.Printf("Error marshalling toggle request body for %s: %v", instance.Address, err)
			continue
		}
		resp, err := http.Post(toggleURL, "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			log.Printf("Error sending toggle to %s: %v", instance.Address, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			found = true
			log.Printf("Successfully toggled debug for %s on %s", req.URL, instance.Address)
			// We continue in case multiple instances have it?
			// Usually only one.
			break
		}
	}

	if found {
		w.WriteHeader(http.StatusOK)
	} else {
		http.Error(w, "Proxy not found on any instance", http.StatusNotFound)
	}
}
