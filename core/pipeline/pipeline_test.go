package pipeline

import (
	"context"
	"net/url"
	"testing"

	"github.com/soda92/vpn-share-tool/core/models"
)

func TestRunPipeline(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		body         string
		settings     models.ProxySettings
		activeSys    []string
		expectedBody string
	}{
		{
			name: "Disable rewriters by settings",
			path: "/legacy.js",
			body: `Ehr.openChrome = function(url){ ... };`,
			settings: models.ProxySettings{
				EnableContentMod: false,
			},
			activeSys:    []string{"HIS"},
			expectedBody: `Ehr.openChrome = function(url){ ... };`,
		},
		{
			name: "Skip processing for dynamic streaming JS",
			path: "*.js",
			body: `any content`,
			settings: models.ProxySettings{
				EnableContentMod: true,
			},
			expectedBody: `any content`,
		},
		{
			name: "Enable rewriter for HIS",
			path: "/lib.js",
			body: `Ehr.openChrome = function(url){`,
			settings: models.ProxySettings{
				EnableContentMod: true,
			},
			activeSys:    []string{"HIS"},
			expectedBody: `Ehr.openChrome = function(url){ window.open(url, "_blank"); return;`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &models.ProcessingContext{
				ReqURL:     &url.URL{Path: tt.path},
				ReqContext: context.Background(),
				Proxy: &models.SharedProxy{
					Settings:      tt.settings,
					ActiveSystems: tt.activeSys,
				},
			}

			got := RunPipeline(ctx, tt.body)

			if got != tt.expectedBody {
				t.Errorf("RunPipeline() = %q, want %q", got, tt.expectedBody)
			}
		})
	}
}
