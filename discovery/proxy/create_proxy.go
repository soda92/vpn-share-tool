package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
	"github.com/soda92/vpn-share-tool/discovery/registry"
)

func HandleCreateProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	activeInstances := registry.GetActiveInstances()

	var lastError string
	reachableNodeFound := false

	for _, instance := range activeInstances {
		// Check if the instance can reach the URL
		canReachURL := fmt.Sprintf("http://%s/can-reach?url=%s", instance.Address, url.QueryEscape(req.URL))
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(canReachURL)
		if err != nil {
			log.Printf("Error checking reachability on %s: %v", instance.Address, err)
			continue
		}

		var canReachResp struct {
			Reachable bool `json:"reachable"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&canReachResp); err != nil {
			log.Printf("Error decoding reachability response from %s: %v", instance.Address, err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if canReachResp.Reachable {
			reachableNodeFound = true
			// This instance can reach the URL, so create the proxy here
			createProxyURL := fmt.Sprintf("http://%s/proxies", instance.Address)
			proxyReqBody, _ := json.Marshal(map[string]string{"url": req.URL})

			postClient := &http.Client{Timeout: 10 * time.Second}
			resp, err := postClient.Post(createProxyURL, "application/json", bytes.NewBuffer(proxyReqBody))
			if err != nil {
				log.Printf("Error creating proxy on %s: %v", instance.Address, err)
				lastError = fmt.Sprintf("Node %s reachable but failed to connect: %v", instance.Address, err)
				continue
			}

			if resp.StatusCode == http.StatusCreated {
				var proxyResp struct {
					OriginalURL string `json:"original_url"`
					SharedURL   string `json:"shared_url"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&proxyResp); err != nil {
					log.Printf("Error decoding proxy response from %s: %v", instance.Address, err)
					resp.Body.Close()
					continue
				}
				resp.Body.Close()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(proxyResp)
				return
			} else {
				// Read error body
				buf := new(bytes.Buffer)
				buf.ReadFrom(resp.Body)
				resp.Body.Close()
				errorMsg := buf.String()
				log.Printf("Failed to create proxy on %s. Status: %d, Body: %s", instance.Address, resp.StatusCode, errorMsg)
				lastError = fmt.Sprintf("Node %s reachable but refused creation (%d): %s", instance.Address, resp.StatusCode, errorMsg)
			}
		}
	}

	// If no instance can reach the URL
	w.Header().Set("Content-Type", "application/json")

	if reachableNodeFound {
		w.WriteHeader(http.StatusBadGateway) // Or 502/500
		json.NewEncoder(w).Encode(map[string]string{"error": lastError})
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "No available instance can reach the target URL."})
	}
}
