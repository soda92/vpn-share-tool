package core

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"log"
	// "mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

//go:embed injector.js
var injectorScript []byte

var rePhisUrl = regexp.MustCompile(`phisUrl\s*:\s*['"](.*?)['"]`)
var reHttpPhis = regexp.MustCompile(`Http\.phis\s*=\s*['"](.*?)['"]`)

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

		// Decompress body if necessary
		var reader io.ReadCloser
		switch resp.Header.Get("Content-Encoding") {
		case "gzip":
			reader, err = gzip.NewReader(bytes.NewReader(respBody))
			if err != nil {
				log.Printf("Error creating gzip reader: %v", err)
				return nil, err
			}
			defer reader.Close()
		case "deflate":
			reader = flate.NewReader(bytes.NewReader(respBody))
			defer reader.Close()
		default:
			reader = io.NopCloser(bytes.NewReader(respBody))
		}

		decompressedBody, err := io.ReadAll(reader)
		if err != nil {
			log.Printf("Error decompressing response body: %v", err)
			return nil, err
		}

		// Now, use decompressedBody for all manipulations
		bodyStr := string(decompressedBody)
		originalBodyStr := bodyStr

		// 1. Inject script
		if strings.Contains(resp.Header.Get("Content-Type"), "text/html") && MyIP != "" && ApiPort != 0 {
			debugURL := fmt.Sprintf("http://%s:%d/debug", MyIP, ApiPort)
			script := strings.Replace(string(injectorScript), "__DEBUG_URL__", debugURL, 1)
			injectionHTML := "<script>" + string(script) + "</script>"
			bodyStr = strings.Replace(bodyStr, "</body>", injectionHTML+"</body>", 1)
		}

		// 2. Handle Http.phis replacement
		if strings.Contains(req.URL.Path, "showView.jsp") {
			// Regex for Http.phis = '...'
			matchesHttpPhis := reHttpPhis.FindStringSubmatch(bodyStr)

			// Regex for phisUrl:'...'
			matchesPhisUrl := rePhisUrl.FindStringSubmatch(bodyStr)

			var originalPhisURL string
			var foundMatch bool

			if len(matchesPhisUrl) > 1 {
				originalPhisURL = matchesPhisUrl[1]
				foundMatch = true
			} else if len(matchesHttpPhis) > 1 {
				originalPhisURL = matchesHttpPhis[1]
				foundMatch = true
			}

			if foundMatch {
				log.Printf("Found phis URL: %s", originalPhisURL)

				newProxy, err := ShareUrlAndGetProxy(originalPhisURL)
				if err != nil {
					log.Printf("Error creating proxy for phis URL: %v", err)
				} else {
					originalHost, ok := req.Context().Value(originalHostKey).(string)
					if !ok {
						log.Printf("Error: originalHost not found in request context for URL %s", req.URL.String())
					} else {
						hostParts := strings.Split(originalHost, ":")
						// Construct the new URL, preserving the path from the original
						newProxyURL := fmt.Sprintf("http://%s:%d%s", hostParts[0], newProxy.RemotePort, newProxy.Path)

						log.Printf("Replacing phis URL with: %s", newProxyURL)
						bodyStr = strings.Replace(bodyStr, originalPhisURL, newProxyURL, 1)
					}
				}
			}
		}

		// If body was modified, update respBody and headers
		if bodyStr != originalBodyStr {
			respBody = []byte(bodyStr)
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
		} else {
			// If no modifications, the captured body should still be the decompressed one
			respBody = decompressedBody
		}

		// Restore the body for the client
		resp.Body = io.NopCloser(bytes.NewBuffer(respBody))

		// Capture the request and the final (potentially modified and decompressed) response
		CaptureRequest(req, resp, reqBody, respBody)
	} else {
		// If there was no body, capture the request anyway
		CaptureRequest(req, resp, reqBody, nil)
	}

	// If cacheable, store the response in the cache.
	if isCacheable && resp.StatusCode == http.StatusOK {
		entry := cacheEntry{
			Header: resp.Header,
			Body:   respBody, // This is the final, correct body
		}
		t.Cache.Store(req.URL.String(), entry)
	}

	return resp, nil
}
