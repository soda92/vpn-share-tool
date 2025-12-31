package core

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/soda92/vpn-share-tool/core/models"
)

// startStatsUpdater updates the RequestRate every second.
func startStatsUpdater(p *models.SharedProxy) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	const alpha = 0.3 // Smoothing factor for EMA (0 < alpha < 1)

	for {
		select {
		case <-ticker.C:
			// Calculate rate
			count := atomic.SwapInt64(&p.ReqCounter, 0)
			currentRate := float64(count)

			p.Mu.Lock()
			// EMA: NewVal = (Current * alpha) + (OldVal * (1 - alpha))
			p.RequestRate = (currentRate * alpha) + (p.RequestRate * (1 - alpha))

			// Snap to 0 if negligible to avoid long tails of 0.000001
			if p.RequestRate < 0.01 {
				p.RequestRate = 0
			}
			p.Mu.Unlock()
		case <-p.Ctx.Done():
			// Context cancelled, stop the updater
			return
		}
	}
}

// startHealthChecker runs in a goroutine to periodically check if a URL is reachable.
func startHealthChecker(p *models.SharedProxy) {
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
		case <-p.Ctx.Done():
			// Context cancelled, stop the health checker
			return
		}
	}
}
