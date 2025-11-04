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
			re := regexp.MustCompile(`Http\.phis *= *'(.*?)' *;`)
			matches := re.FindStringSubmatch(bodyStr)

			if len(matches) > 1 {
				originalPhisURL := matches[1]
				log.Printf("Found Http.phis URL: %s", originalPhisURL)

				newProxy, err := ShareUrlAndGetProxy(originalPhisURL)
				if err != nil {
					log.Printf("Error creating proxy for Http.phis: %v", err)
				} else {
					originalHost := req.Context().Value(originalHostKey).(string)
					hostParts := strings.Split(originalHost, ":")
					newProxyURL := fmt.Sprintf("http://%s:%d", hostParts[0], newProxy.RemotePort)

					log.Printf("Replacing Http.phis URL with: %s", newProxyURL)
					bodyStr = strings.Replace(bodyStr, originalPhisURL, newProxyURL, 1)
				}
			} else {
				// Manual check as a fallback
				originalPhisURL := "http://10.216.11.24:8306/phis"
				if strings.Contains(bodyStr, originalPhisURL) {
					newProxy, err := ShareUrlAndGetProxy(originalPhisURL)
					if err != nil {
						log.Printf("Error creating proxy for Http.phis: %v", err)
					} else {
						originalHost := req.Context().Value(originalHostKey).(string)
						hostParts := strings.Split(originalHost, ":")
						newProxyURL := fmt.Sprintf("http://%s:%d", hostParts[0], newProxy.RemotePort)

						log.Printf("Replacing Http.phis URL with: %s", newProxyURL)
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
			// If not modified, use the original (potentially compressed) body
			respBody = decompressedBody
		}

		resp.Body = io.NopCloser(bytes.NewBuffer(respBody))
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
