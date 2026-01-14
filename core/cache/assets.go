package cache

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"runtime/trace"
	"time"

	"github.com/soda92/vpn-share-tool/core/debug"
)

func (t *CachingTransport) handleStaticAsset(req *http.Request, reqBody []byte) (*http.Response, error) {
	defer trace.StartRegion(req.Context(), "handleStaticAsset").End()
	if entry, ok := t.Cache.Get(req.URL.String()); ok {
		log.Printf("Cache HIT for static: %s", req.URL.String())
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     entry.Header,
			Body:       io.NopCloser(bytes.NewReader(entry.Body)),
			Request:    req,
		}
		debug.CaptureRequest(req, resp, reqBody, entry.Body)
		return resp, nil
	}
	log.Printf("Cache MISS for static: %s", req.URL.String())

	// Fetch
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Read Body
	var respBody []byte
	if resp.Body != nil {
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()
	}

	// Cache if 200 OK
	if resp.StatusCode == http.StatusOK && respBody != nil {
		t.Cache.Add(req.URL.String(), cacheEntry{
			Header: resp.Header,
			Body:   respBody,
		})
	}

	if respBody != nil {
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
	}
	debug.CaptureRequest(req, resp, reqBody, respBody)
	return resp, nil
}

func (t *CachingTransport) handleDynamicAsset(req *http.Request, reqBody []byte) (*http.Response, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if duration > 1*time.Second {
			log.Printf("SLOW REQUEST: %s took %v", req.URL.String(), duration)
		}
	}()
	defer trace.StartRegion(req.Context(), "handleDynamicAsset").End()
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Force a full fetch by removing cache validation headers
	// This ensures we always get the body to run the pipeline on (rewriting URLs, etc.)
	req.Header.Del("If-Modified-Since")
	req.Header.Del("If-None-Match")

	netRegion := trace.StartRegion(req.Context(), "NetworkWait")
	resp, err := transport.RoundTrip(req)
	netRegion.End()

	if err != nil {
		return nil, err
	}

	// Prevent browser caching of dynamic/modified content
	resp.Header.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	resp.Header.Del("ETag")
	resp.Header.Del("Last-Modified")
	resp.Header.Del("Expires")
	resp.Header.Del("Pragma")

	if resp.Body == nil {
		debug.CaptureRequest(req, resp, reqBody, nil)
		return resp, nil
	}

	readRegion := trace.StartRegion(req.Context(), "ReadBody")
	respBody, err := io.ReadAll(resp.Body)
	readRegion.End()
	
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}
	resp.Body.Close()

	// Decompress
	decompRegion := trace.StartRegion(req.Context(), "Decompress")
	decompressedBody, err := t.decompressBody(resp.Header.Get("Content-Encoding"), respBody)
	decompRegion.End()

	if err != nil {
		log.Printf("Error decompressing response body: %v", err)
		return nil, err
	}

	// Run Pipeline
	pipelineRegion := trace.StartRegion(req.Context(), "Pipeline")
	newBody, modified := t.runPipeline(req, resp.Header, decompressedBody)
	pipelineRegion.End()

	if modified {
		respBody = newBody
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
	} else {
		respBody = decompressedBody
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(respBody))
	debug.CaptureRequest(req, resp, reqBody, decompressedBody)
	return resp, nil
}
