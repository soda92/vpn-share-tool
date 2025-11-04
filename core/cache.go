package core

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed injector.js
var injectorScript []byte

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
	// Read the request body for capturing
	var reqBody []byte
	if req.Body != nil {
		var err error
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody)) // Restore body for the actual request
	}

	// We only cache GET requests for static assets.
	ext := filepath.Ext(req.URL.Path)
	isCacheable := false
	if req.Method == http.MethodGet {
		switch ext {
		case ".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".ico":
			isCacheable = true
		}
	}

	// If cacheable, check the cache first.
	if isCacheable {
		if entry, ok := t.Cache.Load(req.URL.String()); ok {
			log.Printf("Cache HIT for: %s", req.URL.String())
			cached := entry.(cacheEntry)
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     cached.Header,
				Body:       io.NopCloser(bytes.NewReader(cached.Body)),
				Request:    req,
			}
			// Capture the cached response
			CaptureRequest(req, resp, reqBody, cached.Body)
			return resp, nil
		}
		log.Printf("Cache MISS for: %s", req.URL.String())
	}

	// If not cacheable or not in cache, make the request.
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Read the response body for capturing and caching
	var respBody []byte
	if resp.Body != nil {
		var err error
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response body: %v", err)
			return nil, err
		}
		// Inject script if the content is HTML and debug info is available
		if strings.Contains(resp.Header.Get("Content-Type"), "text/html") && MyIP != "" && ApiPort != 0 {
			debugURL := fmt.Sprintf("http://%s:%d/debug", MyIP, ApiPort)
			script := strings.Replace(string(injectorScript), "__DEBUG_URL__", debugURL, 1)
			
			// Use strings.Replace for a safer and cleaner injection
			bodyStr := string(respBody)
			injectionHTML := "<script>" + string(script) + "</script>"
			newBodyStr := strings.Replace(bodyStr, "</body>", injectionHTML+"</body>", 1)
			respBody = []byte(newBodyStr)

			// Manually delete Content-Length header to avoid conflicts
			resp.Header.Del("Content-Length")
		}
		resp.Body = io.NopCloser(bytes.NewBuffer(respBody)) // Restore body for the client
	}

	// Capture the request and response
	CaptureRequest(req, resp, reqBody, respBody)

	// If cacheable, store the response in the cache.
	if isCacheable && resp.StatusCode == http.StatusOK {
		entry := cacheEntry{
			Header: resp.Header,
			Body:   respBody,
		}
		t.Cache.Store(req.URL.String(), entry)
	}

	return resp, nil
}
