package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// servicesHandler provides the list of currently shared proxies as a JSON response.
func servicesHandler(w http.ResponseWriter, r *http.Request) {
	ProxiesLock.RLock()
	defer ProxiesLock.RUnlock()

	// Initialize with a non-nil empty slice to ensure the JSON output is `[]` instead of `null`.
	response := make([]sharedURLInfo, 0)
	if MyIP != "" {
		// Just use the first LAN IP for the response. The client can substitute it if needed.
		ip := MyIP
		for _, p := range Proxies {
			sharedURL := fmt.Sprintf("http://%s:%d%s", ip, p.RemotePort, p.Path)
			response = append(response, sharedURLInfo{
				OriginalURL: p.OriginalURL,
				SharedURL:   sharedURL,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode services to JSON: %v", err)
		http.Error(w, "Failed to encode services", http.StatusInternalServerError)
	}
}
