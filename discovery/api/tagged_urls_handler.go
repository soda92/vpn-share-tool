package api

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
	"github.com/soda92/vpn-share-tool/discovery/proxy"
	"github.com/soda92/vpn-share-tool/discovery/store"
	"github.com/soda92/vpn-share-tool/discovery/utils"
)

func HandleTaggedURLs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getTaggedURLs(w, r)
	case http.MethodPost:
		postTaggedURL(w, r)
	case http.MethodPut:
		putTaggedURL(w, r)
	case http.MethodDelete:
		deleteTaggedURL(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getTaggedURLs(w http.ResponseWriter, r *http.Request) {
	urls := store.GetTaggedURLs()

	// Use shared aggregator to fetch proxies from all instances
	allProxies, _ := proxy.FetchAllClusterProxies()

	// Enrich the tagged URLs with their proxy status
	type EnrichedTaggedURL struct {
		store.TaggedURL
		ProxyURL      string               `json:"proxy_url,omitempty"`
		Settings      models.ProxySettings `json:"settings"`
		ActiveSystems []string             `json:"active_systems"`
		RequestRate   float64              `json:"request_rate"`
		TotalRequests int64                `json:"total_requests"`
	}

	enrichedUrls := make([]EnrichedTaggedURL, len(urls))
	for i, u := range urls {
		enrichedUrls[i] = EnrichedTaggedURL{TaggedURL: u}
		// Check against Hostname (keys in allProxies are normalized hostnames)
		if proxyInfo, ok := allProxies[utils.NormalizeHost(u.URL)]; ok {
			enrichedUrls[i].ProxyURL = proxyInfo.SharedURL
			enrichedUrls[i].Settings = proxyInfo.Settings
			enrichedUrls[i].ActiveSystems = proxyInfo.ActiveSystems
			enrichedUrls[i].RequestRate = proxyInfo.RequestRate
			enrichedUrls[i].TotalRequests = proxyInfo.TotalRequests
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(enrichedUrls); err != nil {
		log.Printf("Failed to encode tagged URLs: %v", err)
		http.Error(w, "Failed to encode URLs", http.StatusInternalServerError)
	}
}

func postTaggedURL(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		Tag string `json:"tag"`
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if reqBody.Tag == "" || reqBody.URL == "" {
		http.Error(w, "Tag and URL are required", http.StatusBadRequest)
		return
	}

	newURL, err := store.AddTaggedURL(reqBody.Tag, reqBody.URL)
	if err != nil {
		log.Printf("Error saving tagged URLs: %v", err)
		http.Error(w, "Failed to save URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newURL)
}

func putTaggedURL(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/tagged-urls/")
	var reqBody struct {
		Tag string `json:"tag"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if reqBody.Tag == "" {
		http.Error(w, "Tag is required", http.StatusBadRequest)
		return
	}

	if err := store.UpdateTaggedURL(id, reqBody.Tag); err != nil {
		log.Printf("Error updating tagged URL: %v", err)
		if err.Error() == "not found" {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Failed to update URL", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteTaggedURL(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/tagged-urls/")

	if err := store.DeleteTaggedURL(id); err != nil {
		log.Printf("Error deleting tagged URL: %v", err)
		if err.Error() == "not found" {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Failed to delete URL", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
