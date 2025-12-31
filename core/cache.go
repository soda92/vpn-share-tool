package core

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/soda92/vpn-share-tool/core/models"
	"github.com/soda92/vpn-share-tool/core/debug"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

//go:embed injector.js
var injectorScript []byte

//go:embed calendar.unpacked.js
var calendarScript []byte

// cacheEntry holds the cached response data and headers.
type cacheEntry struct {
	Header http.Header
	Body   []byte
}

// CachingTransport is an http.RoundTripper that caches responses for static assets.
type CachingTransport struct {
	Transport http.RoundTripper
	Cache     *lru.Cache[string, cacheEntry]
	Proxy     *models.SharedProxy
}

func NewCachingTransport(transport http.RoundTripper, proxy *models.SharedProxy) *CachingTransport {
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

	// Intercept calendar.js
	if strings.Contains(req.URL.Path, "calendar.js") {
		log.Printf("Intercepting calendar.js request: %s", req.URL.String())
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(calendarScript)),
			Request:    req,
		}
		resp.Header.Set("Content-Type", "application/javascript")
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(calendarScript)))

		debug.CaptureRequest(req, resp, reqBody, calendarScript)
		return resp, nil
	}

	// Determine if the asset is "Static" (Cache, No Modification)
	isStatic := IsCacheable(req.URL.Path)

	// Intercept Captcha Image
	if strings.HasSuffix(req.URL.Path, "voCode") && t.Proxy != nil && t.Proxy.GetEnableCaptcha() {
		transport := t.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		resp, err := transport.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		var respBody []byte
		if resp.Body != nil {
			respBody, err = io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			resp.Body.Close()
		}

		// Solve and Store (Async to avoid blocking image load)
		if len(respBody) > 0 {
			clientIP := getClientIP(req)
			// Clear old solution immediately to prevent JS from picking up stale data
			ClearCaptchaSolution(clientIP)

			go func(data []byte, ip string) {
				solution := SolveCaptcha(data)
				if solution != "" {
					StoreCaptchaSolution(ip, solution)
				}
			}(respBody, clientIP)
		}

		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		debug.CaptureRequest(req, resp, reqBody, respBody)
		return resp, nil
	}

	// Intercept Captcha Solution Poll
	if strings.HasSuffix(req.URL.Path, "/_proxy/captcha-solution") {
		clientIP := getClientIP(req)

		solution := GetCaptchaSolution(clientIP)
		if solution != "" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(solution)),
				Request:    req,
			}, nil
		}
		// Not ready yet
		return &http.Response{
			StatusCode: http.StatusNotFound, // JS will retry
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString("Not found")),
			Request:    req,
		}, nil
	}

	// 1. STATIC ASSETS: Cache Strategy (No Pipeline)
	if isStatic {
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

	// 2. DYNAMIC/APP ASSETS: Pipeline Strategy (No Cache)
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	var respBody []byte
	var decompressedBody []byte

	if resp.Body != nil {
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response body: %v", err)
			return nil, err
		}
		resp.Body.Close()

		// Decompress
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

		// Run Pipeline
		var bodyStr string
		contentType := resp.Header.Get("Content-Type")
		isGBK := false
		if _, params, err := mime.ParseMediaType(contentType); err == nil {
			charset := strings.ToLower(params["charset"])
			isGBK = charset == "gbk" || charset == "gb2312"
		}

		if isGBK {
			reader := transform.NewReader(bytes.NewReader(decompressedBody), simplifiedchinese.GBK.NewDecoder())
			decoded, err := io.ReadAll(reader)
			if err != nil {
				log.Printf("Error decoding GBK body: %v", err)
				bodyStr = string(decompressedBody)
			} else {
				bodyStr = string(decoded)
			}
		} else {
			bodyStr = string(decompressedBody)
		}

		originalBodyStr := bodyStr

		ctx := &ProcessingContext{
			ReqURL:     req.URL,
			ReqContext: req.Context(),
			RespHeader: resp.Header,
			Proxy:      t.Proxy,
		}

		bodyStr = RunPipeline(ctx, bodyStr, DefaultProcessors)

		// Update response if modified
		if bodyStr != originalBodyStr {
			respBody = []byte(bodyStr)
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")

			if isGBK {
				if mediaType, params, err := mime.ParseMediaType(contentType); err == nil {
					params["charset"] = "utf-8"
					resp.Header.Set("Content-Type", mime.FormatMediaType(mediaType, params))
				}
			}
		} else {
			respBody = decompressedBody
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
		}

		resp.Body = io.NopCloser(bytes.NewBuffer(respBody))
		debug.CaptureRequest(req, resp, reqBody, decompressedBody)
	} else {
		debug.CaptureRequest(req, resp, reqBody, nil)
	}

	return resp, nil
}
