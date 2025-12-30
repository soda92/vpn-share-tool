package core

import (
	"context"
	"log"
	"sync/atomic"
	"time"
)

// startStatsUpdater updates the RequestRate every second.
func startStatsUpdater(p *SharedProxy) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Calculate rate
			count := atomic.SwapInt64(&p.reqCounter, 0)
			p.mu.Lock()
			p.RequestRate = float64(count)
			p.mu.Unlock()
		case <-p.ctx.Done():
			// Context cancelled, stop the updater
			return
		}
	}
}

// startHealthChecker runs in a goroutine to periodically check if a URL is reachable.
func startHealthChecker(p *SharedProxy) {
	healthCheckTicker := time.NewTicker(1 * time.Minute) // Check more frequently
	defer healthCheckTicker.Stop()

	failureCount := 0
	const maxFailures = 3

	for {
		select {
		case <-healthCheckTicker.C:
			log.Printf("Performing health check for %s", p.OriginalURL)
			if !IsURLReachable(p.OriginalURL) {
				failureCount++
				log.Printf("Health check failed for %s (%d/%d).", p.OriginalURL, failureCount, maxFailures)
				if failureCount >= maxFailures {
					log.Printf("Health check failed for %s after %d attempts. Tearing down proxy.", p.OriginalURL, maxFailures)
					removeProxy(p)
					return // Stop this health checker goroutine
				}
			} else {
				if failureCount > 0 {
					log.Printf("Health check successful for %s after %d failures, resetting failure count.", p.OriginalURL, failureCount)
					failureCount = 0 // Reset on success
				} else {
					log.Printf("Health check successful for %s", p.OriginalURL)
				}
			}
		case <-p.ctx.Done():
			// Context cancelled, stop the health checker
			return
		}
	}
}
