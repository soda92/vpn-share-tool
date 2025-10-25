package core

import (
	"log"
	"net/http"
	"time"
)

// SendHeartbeat sends a heartbeat to the discovery server.
func SendHeartbeat() {
	// Implement heartbeat logic here. For now, just log.
	log.Println("Sent heartbeat to discovery server.")
}

func IsURLReachable(targetURL string) bool {
	client := http.Client{
		Timeout: 3 * time.Second,
	}
	// Use HEAD request for efficiency
	req, err := http.NewRequest("HEAD", targetURL, nil)
	if err != nil {
		log.Printf("Discovery: could not create request for %s: %v", targetURL, err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Discovery: URL %s is not reachable: %v", targetURL, err)
		return false
	}
	defer resp.Body.Close()

	// Any status code (even 401/403) means the server is alive.
	log.Printf("Discovery: URL %s is reachable with status %d", targetURL, resp.StatusCode)
	return true
}
