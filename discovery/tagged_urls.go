package discovery

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type TaggedURL struct {
	ID        string    `json:"id"`
	Tag       string    `json:"tag"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	taggedURLs      = make(map[string]TaggedURL)
	taggedURLsMutex = &sync.Mutex{}
)

func handleTaggedURLs(w http.ResponseWriter, r *http.Request) {
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

func normalizeHost(u string) string {
	if !strings.HasPrefix(u, "http") {
		u = "http://" + u
	}
	// Replace localhost with 127.0.0.1 to match proxy behavior
	u = strings.ReplaceAll(u, "localhost", "127.0.0.1")
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	return parsed.Host
}

func getTaggedURLs(w http.ResponseWriter, r *http.Request) {
	taggedURLsMutex.Lock()
	urls := make([]TaggedURL, 0, len(taggedURLs))
	for _, u := range taggedURLs {
		urls = append(urls, u)
	}
	taggedURLsMutex.Unlock()

	// Use shared aggregator to fetch proxies from all instances
	allProxies, _ := fetchAllClusterProxies()

	// Enrich the tagged URLs with their proxy status
	type EnrichedTaggedURL struct {
		TaggedURL
		ProxyURL      string  `json:"proxy_url,omitempty"`
		EnableDebug   bool    `json:"enable_debug"`
		EnableCaptcha bool    `json:"enable_captcha"`
		RequestRate   float64 `json:"request_rate"`
		TotalRequests int64   `json:"total_requests"`
	}

	enrichedUrls := make([]EnrichedTaggedURL, len(urls))
	for i, u := range urls {
		enrichedUrls[i] = EnrichedTaggedURL{TaggedURL: u}
		// Check against Hostname (keys in allProxies are normalized hostnames)
		if proxyInfo, ok := allProxies[normalizeHost(u.URL)]; ok {
			enrichedUrls[i].ProxyURL = proxyInfo.SharedURL
			enrichedUrls[i].EnableDebug = proxyInfo.EnableDebug
			enrichedUrls[i].EnableCaptcha = proxyInfo.EnableCaptcha
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

	newURL := TaggedURL{
		ID:        uuid.New().String(),
		Tag:       reqBody.Tag,
		URL:       reqBody.URL,
		CreatedAt: time.Now(),
	}

	taggedURLsMutex.Lock()
	taggedURLs[newURL.ID] = newURL
	taggedURLsMutex.Unlock()

	if err := saveTaggedURLs(); err != nil {
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

	taggedURLsMutex.Lock()
	urlToUpdate, ok := taggedURLs[id]
	if !ok {
		taggedURLsMutex.Unlock()
		http.NotFound(w, r)
		return
	}
	urlToUpdate.Tag = reqBody.Tag
	taggedURLs[id] = urlToUpdate
	taggedURLsMutex.Unlock()

	if err := saveTaggedURLs(); err != nil {
		log.Printf("Error saving tagged URLs: %v", err)
		http.Error(w, "Failed to save URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteTaggedURL(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/tagged-urls/")

	taggedURLsMutex.Lock()
	_, ok := taggedURLs[id]
	if !ok {
		taggedURLsMutex.Unlock()
		http.NotFound(w, r)
		return
	}
	delete(taggedURLs, id)
	taggedURLsMutex.Unlock()

	if err := saveTaggedURLs(); err != nil {
		log.Printf("Error saving tagged URLs: %v", err)
		http.Error(w, "Failed to save URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
