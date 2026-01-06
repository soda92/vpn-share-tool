package cache

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	_ "embed"
	"io"
	"log"
	"net/http"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/soda92/vpn-share-tool/core/models"
)

// cacheEntry holds the cached response data and headers.
type cacheEntry struct {
	Header http.Header
	Body   []byte
}

// CaptchaProvider defines the interface for captcha operations.
type CaptchaProvider interface {
	Solve(imgData []byte) string
	Store(clientIP, solution string)
	Get(clientIP string) string
	Clear(clientIP string)
}

// CachingTransport is an http.RoundTripper that caches responses for static assets.
type CachingTransport struct {
	Transport       http.RoundTripper
	Cache           *lru.Cache[string, cacheEntry]
	Proxy           *models.SharedProxy
	CaptchaProvider CaptchaProvider
	Processor       StringProcessor
}

func NewCachingTransport(transport http.RoundTripper, proxy *models.SharedProxy, captchaProvider CaptchaProvider, processor StringProcessor) *CachingTransport {
	cache, err := lru.New[string, cacheEntry](2560)
	if err != nil {
		// This should not happen with a static size
		panic(err)
	}
	return &CachingTransport{
		Transport:       transport,
		Cache:           cache,
		Proxy:           proxy,
		CaptchaProvider: captchaProvider,
		Processor:       processor,
	}
}

func (t *CachingTransport) readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewBuffer(reqBody)) // Restore body for the actual request
	return reqBody, nil
}

func (t *CachingTransport) decompressBody(encoding string, body []byte) ([]byte, error) {
	var reader io.ReadCloser
	var err error
	switch encoding {
	case "gzip":
		reader, err = gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
	case "deflate":
		reader = flate.NewReader(bytes.NewReader(body))
		defer reader.Close()
	default:
		return body, nil
	}
	return io.ReadAll(reader)
}
