package proxy

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/soda92/vpn-share-tool/core/models"
	"github.com/soda92/vpn-share-tool/core/pipeline"
)

// StartSystemDetector runs periodically to detect which systems are active on the proxy target
func StartSystemDetector(p *models.SharedProxy) {
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes (or once at start)
	defer ticker.Stop()

	// Initial check
	detectSystems(p)

	for {
		select {
		case <-ticker.C:
			detectSystems(p)
		case <-p.Ctx.Done():
			return
		}
	}
}

func detectSystems(p *models.SharedProxy) {
	detected := []string{}
	baseURL := p.OriginalURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	baseParsed, err := url.Parse(baseURL)
	if err != nil {
		log.Printf("Invalid base URL for detection: %s", baseURL)
		return
	}

	for _, sys := range pipeline.DefinedSystems {
		for _, probe := range sys.ProbeURLs {
			probeURL, err := url.Parse(probe)
			if err != nil {
				continue
			}
			targetURL := baseParsed.ResolveReference(probeURL).String()

			// We can use IsURLReachable from utils, but we might want more specific check (200 OK)
			// IsURLReachable returns true for 403/401 too.
			// For asset probing, we usually expect 200.
			if checkProbe(targetURL) {
				log.Printf("Detected system %s on %s", sys.Name, p.OriginalURL)
				detected = append(detected, sys.ID)
				break // Found one probe, system matches
			}
		}
	}

	p.Mu.Lock()
	p.ActiveSystems = detected
	p.Mu.Unlock()
}

func checkProbe(targetURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("HEAD", targetURL, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
