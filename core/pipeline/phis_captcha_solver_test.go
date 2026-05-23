package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/soda92/vpn-share-tool/core/models"
	"github.com/soda92/vpn-share-tool/core/resources"
)

func TestInjectCaptchaSolver(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		shouldInject bool
	}{
		{
			name:         "Inject on matching img src",
			body:         `<html><body><img src="/phis/app/login/voCode"></body></html>`,
			shouldInject: true,
		},
		{
			name:         "Inject with other attributes",
			body:         `<html><body><img id="captcha" src='/phis/app/login/voCode' class="img"></body></html>`,
			shouldInject: true,
		},
		{
			name:         "No inject on missing image",
			body:         `<html><body><img src="/other/img.png"></body></html>`,
			shouldInject: false,
		},
		{
			name:         "No inject if pattern not exact",
			body:         `<html><body><img src="/phis/login/voCode"></body></html>`,
			shouldInject: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &models.ProcessingContext{
				ReqContext: context.Background(),
			}

			got := InjectCaptchaSolver(ctx, tt.body)

			if tt.shouldInject {
				if !strings.Contains(got, string(resources.SolverScript)) {
					t.Errorf("InjectCaptchaSolver() failed to inject solver script")
				}
				if !strings.Contains(got, "</body>") {
					t.Errorf("InjectCaptchaSolver() failed to preserve </body>")
				}
			} else {
				if got != tt.body {
					t.Errorf("InjectCaptchaSolver() modified body when it shouldn't have")
				}
			}
		})
	}
}
