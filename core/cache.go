package core

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"sync"
)

// cacheEntry holds the cached response data and headers.
type cacheEntry struct {
	Header http.Header
	Body   []byte
}

// CachingTransport is an http.RoundTripper that caches responses for static assets.
type CachingTransport struct {
	Transport http.RoundTripper
	Cache     sync.Map // Using sync.Map for concurrent access
}

// RoundTrip implements the http.RoundTripper interface.
func (t *CachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// We only cache GET requests.
	if req.Method != http.MethodGet {
		return t.Transport.RoundTrip(req)
	}

	// Don't cache AJAX requests.
	if req.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		return t.Transport.RoundTrip(req)
	}

	// Check if the file extension is cacheable.
	ext := filepath.Ext(req.URL.Path)
	isCacheable := false
	switch ext {
	case ".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".ico":
		isCacheable = true
	}

	if !isCacheable {
		return t.Transport.RoundTrip(req)
	}

	// If the item is in the cache, return it.
	if entry, ok := t.Cache.Load(req.URL.String()); ok {
		log.Printf("Cache HIT for: %s", req.URL.String())
		cached := entry.(cacheEntry)
		// Create a new response from the cached data.
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     cached.Header,
			Body:       io.NopCloser(bytes.NewReader(cached.Body)),
			Request:    req,
		}
		return resp, nil
	}

	log.Printf("Cache MISS for: %s", req.URL.String())
	// If not in the cache, make the request.
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Read the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close() // We've read it, so close it.

	// Create a new body for the original response.
	resp.Body = io.NopCloser(bytes.NewReader(body))

	// Cache the response.
	entry := cacheEntry{
		Header: resp.Header,
		Body:   body,
	}
	t.Cache.Store(req.URL.String(), entry)

	return resp, nil
}
