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

	lru "github.com/hashicorp/golang-lru/v2"
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
	Cache     *lru.Cache[string, cacheEntry]
	Proxy     *SharedProxy
}

func NewCachingTransport(transport http.RoundTripper, proxy *SharedProxy) *CachingTransport {
	cache, err := lru.New[string, cacheEntry](2560)
	if err != nil {
		// This should not happen with a static size
		panic(err)
	}
	return &CachingTransport{
		Transport: transport,
		Cache:     cache,
		Proxy:     proxy,
	}
}

func (t *CachingTransport) injectDebugScript(body string, header http.Header) string {
	if t.Proxy != nil && t.Proxy.GetEnableDebug() && strings.Contains(header.Get("Content-Type"), "text/html") && MyIP != "" && ApiPort != 0 {
		debugURL := fmt.Sprintf("http://%s:%d/debug", MyIP, ApiPort)
		script := strings.Replace(string(injectorScript), "__DEBUG_URL__", debugURL, 1)
		injectionHTML := "<script>" + string(script) + "</script>"
		return strings.Replace(body, "</body>", injectionHTML+"</body>", 1)
	}
	return body
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
		if entry, ok := t.Cache.Get(req.URL.String()); ok {
			log.Printf("Cache HIT for: %s", req.URL.String())

			// We have the clean body. We must re-apply any injections (like the debugger script)
			// because the EnableDebug flag might have changed.
			bodyStr := string(entry.Body)
			
			bodyStr = t.injectDebugScript(bodyStr, entry.Header)
			
			// Convert back to bytes
			finalBody := []byte(bodyStr)

			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     entry.Header,
				Body:       io.NopCloser(bytes.NewReader(finalBody)),
				Request:    req,
			}

			// Update Content-Length if changed (though for chunked/compressed it might not matter, but good practice)
			if len(finalBody) != len(entry.Body) {
				resp.Header.Del("Content-Length")
			}

			// Capture the cached response (using clean body)
			CaptureRequest(req, resp, reqBody, entry.Body)
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
	var decompressedBody []byte // Declare here for scope access

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

		decompressedBody, err = io.ReadAll(reader)
		if err != nil {
			log.Printf("Error decompressing response body: %v", err)
			return nil, err
		}

		// Now, use decompressedBody for all manipulations
		bodyStr := string(decompressedBody)
		originalBodyStr := bodyStr

		// 1. Inject script (Only if enabled)
		bodyStr = t.injectDebugScript(bodyStr, resp.Header)

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
			resp.Header.Del("Content-Encoding")
		}

		// Restore the body for the client
		resp.Body = io.NopCloser(bytes.NewBuffer(respBody))

		// Capture the request and the clean (decompressed) response body
		CaptureRequest(req, resp, reqBody, decompressedBody)
	} else {
		// If there was no body, capture the request anyway
		CaptureRequest(req, resp, reqBody, nil)
	}

	// If cacheable, store the response in the cache.
	if isCacheable && resp.StatusCode == http.StatusOK {
		entry := cacheEntry{
			Header: resp.Header,
			// Store the clean body in cache so we can re-inject (or not) on next hit
			Body: decompressedBody,
		}
		t.Cache.Add(req.URL.String(), entry)
	}

	return resp, nil
}
