package cache

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soda92/vpn-share-tool/core/models"
)

type mockRoundTripper struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestCachingTransport_StaticCache(t *testing.T) {
	callCount := 0
	mock := &mockRoundTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			callCount++
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString("image-data")),
				Request:    req,
			}, nil
		},
	}

	transport := NewCachingTransport(mock, &models.SharedProxy{}, nil, nil)

	req := httptest.NewRequest("GET", "http://example.com/logo.png", nil)

	// 1. First request (MISS)
	resp1, _ := transport.RoundTrip(req)
	body1, _ := io.ReadAll(resp1.Body)
	if string(body1) != "image-data" {
		t.Errorf("Expected body 'image-data', got %q", string(body1))
	}

	// 2. Second request (HIT)
	resp2, _ := transport.RoundTrip(req)
	body2, _ := io.ReadAll(resp2.Body)
	if string(body2) != "image-data" {
		t.Errorf("Expected cached body 'image-data', got %q", string(body2))
	}

	if callCount != 1 {
		t.Errorf("Expected only 1 call to mock roundtripper, got %d", callCount)
	}
}

func TestCachingTransport_DecompressGzip(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte("compressed-data"))
	gw.Close()

	mock := &mockRoundTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			header := make(http.Header)
			header.Set("Content-Encoding", "gzip")
			return &http.Response{
				StatusCode: 200,
				Header:     header,
				Body:       io.NopCloser(bytes.NewReader(buf.Bytes())),
				Request:    req,
			}, nil
		},
	}

	transport := NewCachingTransport(mock, &models.SharedProxy{}, nil, func(ctx *models.ProcessingContext, body string) string {
		return body + "-processed"
	})

	// Use a dynamic path to trigger handleDynamicAsset which runs the pipeline
	req := httptest.NewRequest("GET", "http://example.com/index.html", nil)

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "compressed-data-processed" {
		t.Errorf("Expected decompressed and processed body, got %q", string(body))
	}
}
