package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/soda92/vpn-share-tool/core/models"
	"github.com/soda92/vpn-share-tool/discovery/registry"
)

type ProxyInfo struct {
	OriginalURL   string               `json:"original_url"`
	RemotePort    int                  `json:"remote_port"`
	Path          string               `json:"path"`
	SharedURL     string               `json:"shared_url"`
	Settings      models.ProxySettings `json:"settings"`
	ActiveSystems []string             `json:"active_systems"`
	RequestRate   float64              `json:"request_rate"`
	TotalRequests int64                `json:"total_requests"`
}

func normalizeHost(u string) string {
	if !strings.HasPrefix(u, "http") {
		u = "http://" + u
	}
	u = strings.ReplaceAll(u, "localhost", "127.0.0.1")
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	return parsed.Host
}

// FetchAllClusterProxies queries all active instances for their proxy lists.
// Returns a map of normalized URL host -> ProxyInfo AND a flat list of all proxies
func FetchAllClusterProxies() (map[string]ProxyInfo, []ProxyInfo) {
	activeInstances := registry.GetActiveInstances()
	hostnameMap := make(map[string]ProxyInfo)
	var rawList []ProxyInfo
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, instance := range activeInstances {
		wg.Add(1)
		go func(inst registry.Instance) {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(fmt.Sprintf("http://%s/active-proxies", inst.Address))
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var proxies []ProxyInfo
				if err := json.NewDecoder(resp.Body).Decode(&proxies); err == nil {
					host, _, _ := net.SplitHostPort(inst.Address)
					mu.Lock()
					for _, p := range proxies {
						sharedURL := fmt.Sprintf("http://%s:%d%s", host, p.RemotePort, p.Path)
						p.SharedURL = sharedURL // Enrich struct

						rawList = append(rawList, p)

						// Store by normalized host for tagging matching
						key := normalizeHost(p.OriginalURL)
						hostnameMap[key] = p
					}
					mu.Unlock()
				}
			}
		}(instance)
	}
	wg.Wait()

	return hostnameMap, rawList
}

func HandleClusterProxies(w http.ResponseWriter, r *http.Request) {
	_, rawList := FetchAllClusterProxies()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(rawList); err != nil {
		log.Printf("Failed to encode cluster proxies: %v", err)
		http.Error(w, "Failed to encode proxies", http.StatusInternalServerError)
	}
}
