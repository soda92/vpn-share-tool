package pipeline

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/soda92/vpn-share-tool/core/models"
)

func TestRewritePhisURLs(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		path           string
		createProxyErr error
		expectedBody   string
		expectedLoc    string
	}{
		{
			name: "Rewrite phisUrl in body and header",
			body: `var config = { phisUrl: "http://internal.phis/login" };`,
			path: "/phis/showView.jsp",
			expectedBody: `var config = { phisUrl: "http://127.0.0.1:20001/path" };`,
			expectedLoc:  "http://192.168.1.100:20001/path",
		},
		{
			name: "Rewrite Http.phis in body",
			body: `Http.phis = "http://internal.phis/app";`,
			path: "/phis/showView.jsp",
			expectedBody: `Http.phis = "http://127.0.0.1:20001/path";`,
			expectedLoc:  "http://192.168.1.100:20001/path",
		},
		{
			name: "Skip rewrite on different path",
			body: `var phisUrl = "http://internal.phis/login";`,
			path: "/other/page.jsp",
			expectedBody: `var phisUrl = "http://internal.phis/login";`,
		},
		{
			name: "Handle CreateProxy error",
			body: `phisUrl: "http://internal.phis/login"`,
			path: "/phis/showView.jsp",
			createProxyErr: fmt.Errorf("failed"),
			expectedBody: `phisUrl: "http://internal.phis/login"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &models.ProcessingContext{
				ReqURL: &url.URL{Path: tt.path},
				RespHeader: make(http.Header),
				Services: models.PipelineServices{
					MyIP: "192.168.1.100",
					CreateProxy: func(rawURL string, port int) (*models.SharedProxy, error) {
						if tt.createProxyErr != nil {
							return nil, tt.createProxyErr
						}
						return &models.SharedProxy{
							RemotePort: 20001,
							Path:       "/path",
						}, nil
					},
				},
				// Use models helper to set original host
				ReqContext: models.WithOriginalHost(context.Background(), "127.0.0.1:8080"),
			}

			gotBody := RewritePhisURLs(ctx, tt.body)

			if gotBody != tt.expectedBody {
				t.Errorf("RewritePhisURLs() body = %q, want %q", gotBody, tt.expectedBody)
			}

			if tt.expectedLoc != "" {
				loc := ctx.RespHeader.Get("Location")
				if loc != tt.expectedLoc {
					t.Errorf("RewritePhisURLs() Location header = %q, want %q", loc, tt.expectedLoc)
				}
			}
		})
	}
}
