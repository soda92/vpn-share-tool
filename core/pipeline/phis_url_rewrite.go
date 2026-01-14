package pipeline

import (
	"fmt"
	"log"
	"regexp"
	"runtime/trace"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
)

var (
	rePhisUrl  = regexp.MustCompile(`phisUrl\s*:\s*['"](.*?)['"]`)
	reHttpPhis = regexp.MustCompile(`Http\.phis\s*=\s*['"](.*?)['"]`)
)

func RewritePhisURLs(ctx *models.ProcessingContext, body string) string {
	defer trace.StartRegion(ctx.ReqContext, "RewritePhisURLs").End()
	if strings.Contains(ctx.ReqURL.Path, "showView.jsp") {
		matchesHttpPhis := reHttpPhis.FindStringSubmatch(body)
		matchesPhisUrl := rePhisUrl.FindStringSubmatch(body)

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

			var newProxy *models.SharedProxy
			var err error

			if originalPhisURL != "" {
				// Use the CreateProxy service injected in the context
				if ctx.Services.CreateProxy != nil {
					newProxy, err = ctx.Services.CreateProxy(originalPhisURL, 0)
				} else {
					err = fmt.Errorf("CreateProxy service not available")
				}

				if err == nil && newProxy != nil {
					// We created a proxy for the anti-phishing redirect destination.
					// Now we should rewrite the Location header to point to our proxy.
					sharedURL := fmt.Sprintf("http://%s:%d%s", ctx.Services.MyIP, newProxy.RemotePort, newProxy.Path)
					ctx.RespHeader.Set("Location", sharedURL)
					log.Printf("Rewrote anti-phishing redirect to: %s", sharedURL)
				}
			}

			if err != nil {
				log.Printf("Error creating proxy for phis URL: %v", err)
			} else if newProxy != nil {
				originalHost, ok := ctx.ReqContext.Value(models.OriginalHostKey).(string)
				if !ok {
					log.Printf("Error: originalHost not found in request context for URL %s", ctx.ReqURL.String())
				} else {
					hostParts := strings.Split(originalHost, ":")
					newProxyURL := fmt.Sprintf("http://%s:%d%s", hostParts[0], newProxy.RemotePort, newProxy.Path)

					log.Printf("Replacing phis URL with: %s", newProxyURL)
					body = strings.ReplaceAll(body, originalPhisURL, newProxyURL)
				}
			}
		}
	}
	return body
}
