package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/soda92/vpn-share-tool/discovery/registry"
	"github.com/soda92/vpn-share-tool/core/models"
)

func HandleUpdateProxySettings(w http.ResponseWriter, r *http.Request) {
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

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Find instances
	activeInstances := registry.GetActiveInstances()

	reqBody, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return
	}

	found := false
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 5 * time.Second}

	for _, instance := range activeInstances {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			updateURL := fmt.Sprintf("http://%s/update-settings", addr)
			resp, err := client.Post(updateURL, "application/json", bytes.NewBuffer(reqBody))
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				found = true
				log.Printf("Successfully updated settings for %s on %s", req.URL, addr)
			}
		}(instance.Address)
	}
	wg.Wait()

	if found {
		w.WriteHeader(http.StatusOK)
	} else {
		// Fallback: If no instance supported the new endpoint, maybe it's an old client?
		// We could try to map "EnableContentMod" to "/toggle-captcha"? 
		// But that's messy. Let's return 404/501 and let the UI handle the graceful degradation msg.
		http.Error(w, "Proxy not found or client does not support settings update", http.StatusNotFound)
	}
}
