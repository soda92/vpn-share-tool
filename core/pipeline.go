package core

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
)

//go:embed solver_script.js
var SolverScript []byte

var (
	rePhisUrl            = regexp.MustCompile(`phisUrl\s*:\s*['"](.*?)['"]`)
	reHttpPhis           = regexp.MustCompile(`Http\.phis\s*=\s*['"](.*?)['"]`)
	reStopItBlock        = regexp.MustCompile(`function\s+_stopIt\(e\)\s*\{[\s\S]*?return\s+false;\s*\}`)
	reShowModalCheck     = regexp.MustCompile(`if\s*\(\s*window\.showModalDialog\s*==\s*undefined\s*\)`)
	reWindowOpenFallback = regexp.MustCompile(`window\.open\(url,obj,"width="\+w\+",height="\+h\+",modal=yes,toolbar=no,menubar=no,scrollbars=yes,resizeable=no,location=no,status=no"\);`)
	reEhrOpenChrome      = regexp.MustCompile(`Ehr\.openChrome\s*=\s*function\s*\(\s*url\s*\)\s*\{`)
	reEhrWindowOpen      = regexp.MustCompile(`window\.open\(\s*url\s*,\s*""\s*,\s*[^;]+\);`)
	reLocalhost          = regexp.MustCompile(`(https?://)(localhost|127\.\d{1,3}\.\d{1,3}\.\d{1,3})(:\d+)?`)
	rePrivate10          = regexp.MustCompile(`(https?://)(10\.\d{1,3}\.\d{1,3}\.\d{1,3})(:\d+)?`)
	rePrivate172         = regexp.MustCompile(`(https?://)(172\.(?:1[6-9]|2\d|3[0-1])\.\d{1,3}\.\d{1,3})(:\d+)?`)
	rePrivate192         = regexp.MustCompile(`(https?://)(192\.168\.\d{1,3}\.\d{1,3})(:\d+)?`)
	reCaptchaImage       = regexp.MustCompile(`<img[^>]+src=["']/phis/app/login/voCode["']`)
)

var DefaultProcessors = []ContentProcessor{
	InjectDebugScript,
	FixLegacyJS,
	RewriteInternalURLs,
	RewritePhisURLs,
	InjectCaptchaSolver,
}

type ProcessingContext struct {
	ReqURL     *url.URL
	ReqContext context.Context
	RespHeader http.Header
	Proxy      *models.SharedProxy
}

type ContentProcessor func(ctx *ProcessingContext, body string) string

func InjectCaptchaSolver(ctx *ProcessingContext, body string) string {
	if ctx.Proxy != nil && ctx.Proxy.GetEnableCaptcha() && reCaptchaImage.MatchString(body) {
		log.Println("Injecting Captcha Solver Script")

		return strings.Replace(body, "</body>", `<script>`+string(SolverScript)+`</script>`+"</body>", 1)
	}
	return body
}

func RunPipeline(ctx *ProcessingContext, body string, processors []ContentProcessor) string {
	// Skip processing for specific dynamic JS patterns or large libraries
	path := strings.ToLower(ctx.ReqURL.Path)
	if path == "*.js" {
		return body
	}
	if !ctx.Proxy.GetEnableCaptcha() {
		return body
	}

	for _, p := range processors {
		body = p(ctx, body)
	}
	return body
}

func InjectDebugScript(ctx *ProcessingContext, body string) string {
	if ctx.Proxy != nil && ctx.Proxy.GetEnableDebug() && strings.Contains(ctx.RespHeader.Get("Content-Type"), "text/html") && MyIP != "" && APIPort != 0 {
		debugURL := fmt.Sprintf("http://%s:%d/debug", MyIP, APIPort)
		script := strings.Replace(string(injectorScript), "__DEBUG_URL__", debugURL, 1)
		injectionHTML := "<script>" + string(script) + "</script>"
		return strings.Replace(body, "</body>", injectionHTML+"</body>", 1)
	}
	return body
}

func FixLegacyJS(ctx *ProcessingContext, body string) string {
	// Remove disable_backspace script using regex
	body = reStopItBlock.ReplaceAllString(body, "")

	// Replace openModalDialog logic
	body = reShowModalCheck.ReplaceAllString(body, "if(true)")
	body = reWindowOpenFallback.ReplaceAllString(body, `window.open(url, "_blank");`)
	body = reEhrOpenChrome.ReplaceAllString(body, `Ehr.openChrome = function(url){ window.open(url, "_blank"); return;`)
	body = reEhrWindowOpen.ReplaceAllString(body, `window.open(url, "_blank");`)
	return body
}

func RewritePhisURLs(ctx *ProcessingContext, body string) string {
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
				newProxy, err = ShareUrlAndGetProxy(originalPhisURL, 0)
				if err == nil {
					// We created a proxy for the anti-phishing redirect destination.
					// Now we should rewrite the Location header to point to our proxy.
					sharedURL := fmt.Sprintf("http://%s:%d%s", MyIP, newProxy.RemotePort, newProxy.Path)
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
