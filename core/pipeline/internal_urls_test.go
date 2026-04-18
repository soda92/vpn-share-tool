package pipeline

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/soda92/vpn-share-tool/core/models"
)

func TestRewriteInternalURLs(t *testing.T) {
	// Setup reachability cache for tests to avoid real network calls
	reachCacheLock.Lock()
	reachCache["http://10.0.0.1"] = reachabilityResult{reachable: true, timestamp: time.Now()}
	reachCache["http://192.168.1.1"] = reachabilityResult{reachable: true, timestamp: time.Now()}
	reachCache["http://127.0.0.1:3000"] = reachabilityResult{reachable: true, timestamp: time.Now()}
	reachCache["http://10.0.0.2"] = reachabilityResult{reachable: false, timestamp: time.Now()} // Unreachable
	reachCacheLock.Unlock()

	tests := []struct {
		name         string
		body         string
		contentType  string
		expectedBody string
	}{
		{
			name:         "Rewrite 10.x.x.x URL",
			body:         `var api = "http://10.0.0.1/api";`,
			contentType:  "application/javascript",
			expectedBody: `var api = "http://127.0.0.1:20001/api";`,
		},
		{
			name:         "Rewrite 192.168.x.x URL",
			body:         `href="http://192.168.1.1/home"`,
			contentType:  "text/html",
			expectedBody: `href="http://127.0.0.1:20001/home"`,
		},
		{
			name:         "Rewrite localhost URL",
			body:         `fetch("http://127.0.0.1:3000/data")`,
			contentType:  "application/json",
			expectedBody: `fetch("http://127.0.0.1:20001/data")`,
		},
		{
			name:         "Skip unreachable URL",
			body:         `http://10.0.0.2/api`,
			contentType:  "text/plain",
			expectedBody: `http://10.0.0.2/api`,
		},
		{
			name:         "Skip non-internal URL",
			body:         `http://google.com`,
			contentType:  "text/html",
			expectedBody: `http://google.com`,
		},
		{
			name:         "Skip non-text content type",
			body:         `http://10.0.0.1/api`,
			contentType:  "image/png",
			expectedBody: `http://10.0.0.1/api`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &models.ProcessingContext{
				ReqURL: &url.URL{Path: "/test"},
				RespHeader: http.Header{
					"Content-Type": []string{tt.contentType},
				},
				Services: models.PipelineServices{
					MyIP: "192.168.1.100",
					CreateProxy: func(rawURL string, port int) (*models.SharedProxy, error) {
						return &models.SharedProxy{
							RemotePort: 20001,
						}, nil
					},
				},
				ReqContext: models.WithOriginalHost(context.Background(), "127.0.0.1:8080"),
			}

			got := RewriteInternalURLs(ctx, tt.body)

			if got != tt.expectedBody {
				t.Errorf("RewriteInternalURLs() = %q, want %q", got, tt.expectedBody)
			}
		})
	}
}
