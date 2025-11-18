package core

import (
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	reachableClient http.Client
	once            sync.Once
)

func IsURLReachable(targetURL string) bool {
	once.Do(func() {
		reachableClient = http.Client{
			Timeout: 10 * time.Second,
		}
	})
	// Use HEAD request for efficiency
	req, err := http.NewRequest("HEAD", targetURL, nil)
	if err != nil {
		log.Printf("Discovery: could not create request for %s: %v", targetURL, err)
		return false
	}

	resp, err := reachableClient.Do(req)
	if err != nil {
		log.Printf("Discovery: URL %s is not reachable: %v", targetURL, err)
		return false
	}
	defer resp.Body.Close()

	// Any status code (even 401/403) means the server is alive.
	log.Printf("Discovery: URL %s is reachable with status %d", targetURL, resp.StatusCode)
	return true
}
