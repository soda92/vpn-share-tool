package discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type ProxyInfo struct {
	OriginalURL   string  `json:"original_url"`
	RemotePort    int     `json:"remote_port"`
	Path          string  `json:"path"`
	SharedURL     string  `json:"shared_url"`
	EnableDebug   bool    `json:"enable_debug"`
	EnableCaptcha bool    `json:"enable_captcha"`
	RequestRate   float64 `json:"request_rate"`
	TotalRequests int64   `json:"total_requests"`
}

// fetchAllClusterProxies queries all registered instances for their active proxies.
// It returns a map where the key is the normalized Hostname of the OriginalURL,
// and the value is the ProxyInfo struct.
// It also returns a raw list of all proxies for display purposes.
func fetchAllClusterProxies() (map[string]ProxyInfo, []ProxyInfo) {
	mutex.Lock()
	activeInstances := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		activeInstances = append(activeInstances, instance)
	}
	mutex.Unlock()

	hostnameMap := make(map[string]ProxyInfo)
	var rawList []ProxyInfo
	var wg sync.WaitGroup
	var resultMutex sync.Mutex

	for _, instance := range activeInstances {
		wg.Add(1)
		go func(instance Instance) {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(fmt.Sprintf("http://%s/active-proxies", instance.Address))
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var proxies []ProxyInfo
				if err := json.NewDecoder(resp.Body).Decode(&proxies); err == nil {
					host, _, _ := net.SplitHostPort(instance.Address)
					resultMutex.Lock()
					for _, p := range proxies {
						sharedURL := fmt.Sprintf("http://%s:%d%s", host, p.RemotePort, p.Path)
						p.SharedURL = sharedURL // Enrich struct

						rawList = append(rawList, p)

						// Store by normalized host for tagging matching
						key := normalizeHost(p.OriginalURL)
						hostnameMap[key] = p
					}
					resultMutex.Unlock()
				}
			}
		}(instance)
	}
	wg.Wait()

	return hostnameMap, rawList
}

func handleClusterProxies(w http.ResponseWriter, r *http.Request) {
	_, rawList := fetchAllClusterProxies()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(rawList); err != nil {
		log.Printf("Failed to encode cluster proxies: %v", err)
		http.Error(w, "Failed to encode proxies", http.StatusInternalServerError)
	}
}
