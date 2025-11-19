package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
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

func getTaggedURLs(w http.ResponseWriter, r *http.Request) {
	taggedURLsMutex.Lock()
	urls := make([]TaggedURL, 0, len(taggedURLs))
	for _, u := range taggedURLs {
		urls = append(urls, u)
	}
	taggedURLsMutex.Unlock()

	// Concurrently fetch all active proxies to enrich the response
	mutex.Lock()
	activeInstances := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		activeInstances = append(activeInstances, instance)
	}
	mutex.Unlock()

	type ProxyInfo struct {
		OriginalURL string `json:"original_url"`
		RemotePort  int    `json:"remote_port"`
		Path        string `json:"path"`
		SharedURL   string `json:"shared_url"`
	}

	allProxies := make(map[string]string)
	var wg sync.WaitGroup
	var proxyMutex sync.Mutex

	for _, instance := range activeInstances {
		wg.Add(1)
		go func(instance Instance) {
			defer wg.Done()
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(fmt.Sprintf("http://%s/active-proxies", instance.Address))
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var proxies []ProxyInfo
				if err := json.NewDecoder(resp.Body).Decode(&proxies); err == nil {
					proxyMutex.Lock()
					for _, p := range proxies {
						host, _, _ := net.SplitHostPort(instance.Address)
						allProxies[p.OriginalURL] = fmt.Sprintf("http://%s:%d%s", host, p.RemotePort, p.Path)
					}
					proxyMutex.Unlock()
				}
			}
		}(instance)
	}
	wg.Wait()

	// Enrich the tagged URLs with their proxy status
	type EnrichedTaggedURL struct {
		TaggedURL
		ProxyURL string `json:"proxy_url,omitempty"`
	}

	enrichedUrls := make([]EnrichedTaggedURL, len(urls))
	for i, u := range urls {
		enrichedUrls[i] = EnrichedTaggedURL{TaggedURL: u}
		if proxyURL, ok := allProxies[u.URL]; ok {
			enrichedUrls[i].ProxyURL = proxyURL
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
	if ok {
		urlToUpdate.Tag = reqBody.Tag
		taggedURLs[id] = urlToUpdate
	}
	taggedURLsMutex.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

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
	if ok {
		delete(taggedURLs, id)
	}
	taggedURLsMutex.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := saveTaggedURLs(); err != nil {
		log.Printf("Error saving tagged URLs: %v", err)
		http.Error(w, "Failed to save URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
