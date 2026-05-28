package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/soda92/vpn-share-tool/discovery/registry"
)

func HandleCreateProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL         string `json:"url"`
		NodeAddress string `json:"node_address"`
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

	nodesWithActiveProxy := make(map[string]bool)
	var mu sync.Mutex

	if req.NodeAddress == "auto_another" {
		var wg sync.WaitGroup
		for _, instance := range activeInstances {
			wg.Add(1)
			go func(addr string) {
				defer wg.Done()
				client := &http.Client{Timeout: 3 * time.Second}
				resp, err := client.Get(fmt.Sprintf("http://%s/active-proxies", addr))
				if err != nil {
					return
				}
				defer resp.Body.Close()

				var proxies []struct {
					OriginalURL string `json:"original_url"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&proxies); err == nil {
					for _, p := range proxies {
						if p.OriginalURL == req.URL {
							mu.Lock()
							nodesWithActiveProxy[addr] = true
							mu.Unlock()
							break
						}
					}
				}
			}(instance.Address)
		}
		wg.Wait()

		if len(nodesWithActiveProxy) == len(activeInstances) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "Proxy is already active on all available nodes."})
			return
		}
	}

	// 1. Gather candidate instances
	var candidates []registry.Instance
	for _, instance := range activeInstances {
		if req.NodeAddress != "" && req.NodeAddress != "auto" && req.NodeAddress != "auto_another" && instance.Address != req.NodeAddress {
			continue
		}
		if req.NodeAddress == "auto_another" && nodesWithActiveProxy[instance.Address] {
			continue
		}
		candidates = append(candidates, instance)
	}

	if len(candidates) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "No available nodes match the selection criteria."})
		return
	}

	type checkResult struct {
		address   string
		reachable bool
		err       error
	}

	resultChan := make(chan checkResult, len(candidates))
	var checkWg sync.WaitGroup

	// 2. Perform can-reach checks in parallel
	for _, instance := range candidates {
		checkWg.Add(1)
		go func(addr string) {
			defer checkWg.Done()
			canReachURL := fmt.Sprintf("http://%s/can-reach?url=%s", addr, url.QueryEscape(req.URL))
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(canReachURL)
			if err != nil {
				resultChan <- checkResult{address: addr, reachable: false, err: err}
				return
			}
			defer resp.Body.Close()

			var canReachResp struct {
				Reachable bool `json:"reachable"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&canReachResp); err != nil {
				resultChan <- checkResult{address: addr, reachable: false, err: err}
				return
			}

			resultChan <- checkResult{address: addr, reachable: canReachResp.Reachable, err: nil}
		}(instance.Address)
	}

	// Close channel when all reachability checks complete
	go func() {
		checkWg.Wait()
		close(resultChan)
	}()

	// 3. Process reachability results as they arrive and create proxy on the first success
	for res := range resultChan {
		if res.err != nil {
			log.Printf("Error checking reachability on %s: %v", res.address, res.err)
			continue
		}

		if res.reachable {
			reachableNodeFound = true
			createProxyURL := fmt.Sprintf("http://%s/proxies", res.address)
			proxyReqBody, _ := json.Marshal(map[string]string{"url": req.URL})

			postClient := &http.Client{Timeout: 10 * time.Second}
			resp, err := postClient.Post(createProxyURL, "application/json", bytes.NewBuffer(proxyReqBody))
			if err != nil {
				log.Printf("Error creating proxy on %s: %v", res.address, err)
				lastError = fmt.Sprintf("Node %s reachable but failed to connect: %v", res.address, err)
				continue
			}

			if resp.StatusCode == http.StatusCreated {
				var proxyResp struct {
					OriginalURL string `json:"original_url"`
					SharedURL   string `json:"shared_url"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&proxyResp); err != nil {
					log.Printf("Error decoding proxy response from %s: %v", res.address, err)
					resp.Body.Close()
					continue
				}
				resp.Body.Close()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(proxyResp)
				return
			} else {
				buf := new(bytes.Buffer)
				buf.ReadFrom(resp.Body)
				resp.Body.Close()
				errorMsg := buf.String()
				log.Printf("Failed to create proxy on %s. Status: %d, Body: %s", res.address, resp.StatusCode, errorMsg)
				lastError = fmt.Sprintf("Node %s reachable but refused creation (%d): %s", res.address, resp.StatusCode, errorMsg)
			}
		}
	}

	// If no instance was successfully set up
	w.Header().Set("Content-Type", "application/json")

	if reachableNodeFound {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": lastError})
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "No available instance can reach the target URL."})
	}
}
