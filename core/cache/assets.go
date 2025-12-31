package cache

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/soda92/vpn-share-tool/core/debug"
)

func (t *CachingTransport) handleStaticAsset(req *http.Request, reqBody []byte) (*http.Response, error) {
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
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.Body == nil {
		debug.CaptureRequest(req, resp, reqBody, nil)
		return resp, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}
	resp.Body.Close()

	// Decompress
	decompressedBody, err := t.decompressBody(resp.Header.Get("Content-Encoding"), respBody)
	if err != nil {
		log.Printf("Error decompressing response body: %v", err)
		return nil, err
	}

	// Run Pipeline
	newBody, modified := t.runPipeline(req, resp.Header, decompressedBody)

	if modified {
		respBody = newBody
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
	} else {
		respBody = decompressedBody // Use decompressed version if we want to serve plain text, or keep original? 
		// The original logic replaced body with decompressed version regardless if not modified? 
		// Wait, original logic:
		// if modified { respBody = newBody ... } else { respBody = decompressedBody; Del(Encoding) }
		// So it always serves decompressed.
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(respBody))
	debug.CaptureRequest(req, resp, reqBody, decompressedBody)
	return resp, nil
}
