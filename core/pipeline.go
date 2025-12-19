package core

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

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
	Proxy      *SharedProxy
}

type ContentProcessor func(ctx *ProcessingContext, body string) string

func InjectCaptchaSolver(ctx *ProcessingContext, body string) string {
	if ctx.Proxy != nil && ctx.Proxy.GetEnableCaptcha() && reCaptchaImage.MatchString(body) {
		log.Println("Injecting Captcha Solver Script")
		
		solverScript := `
<script>
(function() {
    var checkInterval;
    
    function startPolling() {
        if (checkInterval) clearInterval(checkInterval);
        
        var attempts = 0;
        var maxAttempts = 60; // 60 seconds
        
        // Reset input on polling start (new image)
        var input = document.getElementById('verifyCode');
        if (input) input.value = '';

        checkInterval = setInterval(function() {
            attempts++;
            if (attempts > maxAttempts) {
                clearInterval(checkInterval);
                return;
            }

            fetch('/_proxy/captcha-solution')
                .then(function(res) {
                    if (res.ok) return res.text();
                    throw new Error('Not ready');
                })
                .then(function(code) {
                    // Check if input is empty (to avoid overwriting user edits if they started typing?)
                    // But if we just started polling, we cleared it.
                    if (code && code.trim() !== "") {
                        var input = document.getElementById('verifyCode');
                        if (input) {
                            input.value = code;
                            console.log('Auto-filled Captcha: ' + code);

                            var event = new Event('input', { bubbles: true });
                            input.dispatchEvent(event);

                            clearInterval(checkInterval);
                        }
                    }
                })
                .catch(function(e) { console.error('Captcha solution fetch error:', e);});
        }, 500); // Poll faster
    }

    // Start on load
    startPolling();

    // Restart on image click
    var img = document.getElementById('img');
    if (img) {
        img.addEventListener('click', function() {
            console.log('Captcha refreshed, restarting solver...');
            // Wait a bit for the new request to trigger
            setTimeout(startPolling, 500);
        });
    }
})();
</script>`
		return strings.Replace(body, "</body>", solverScript+"</body>", 1)
	}
	return body
}


func RunPipeline(ctx *ProcessingContext, body string, processors []ContentProcessor) string {
	for _, p := range processors {
		body = p(ctx, body)
	}
	return body
}

func InjectDebugScript(ctx *ProcessingContext, body string) string {
	if ctx.Proxy != nil && ctx.Proxy.GetEnableDebug() && strings.Contains(ctx.RespHeader.Get("Content-Type"), "text/html") && MyIP != "" && ApiPort != 0 {
		debugURL := fmt.Sprintf("http://%s:%d/debug", MyIP, ApiPort)
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

func RewriteInternalURLs(ctx *ProcessingContext, body string) string {
	contentType := ctx.RespHeader.Get("Content-Type")
	if strings.Contains(contentType, "text/") ||
		strings.Contains(contentType, "application/javascript") ||
		strings.Contains(contentType, "application/json") ||
		strings.Contains(ctx.ReqURL.Path, ".jsp") {

		regexes := []*regexp.Regexp{reLocalhost, rePrivate10, rePrivate172, rePrivate192}

		uniqueMatches := make(map[string]bool)
		for _, re := range regexes {
			matches := re.FindAllString(body, -1)
			for _, match := range matches {
				uniqueMatches[match] = true
			}
		}

		if len(uniqueMatches) > 0 {
			replacements := make(map[string]string)
			originalHost, ctxOk := ctx.ReqContext.Value(originalHostKey).(string)

			for match := range uniqueMatches {
				if _, processed := replacements[match]; processed {
					continue
				}

				if MyIP != "" && strings.Contains(match, MyIP) {
					continue
				}

				newProxy, err := ShareUrlAndGetProxy(match)
				if err != nil {
					log.Printf("Error creating proxy for internal URL %s: %v", match, err)
					continue
				}

				if !ctxOk {
					if MyIP != "" {
						replacements[match] = fmt.Sprintf("http://%s:%d", MyIP, newProxy.RemotePort)
					}
				} else {
					hostParts := strings.Split(originalHost, ":")
					proxyHost := hostParts[0]
					replacements[match] = fmt.Sprintf("http://%s:%d", proxyHost, newProxy.RemotePort)
				}
			}

			for oldURL, newURL := range replacements {
				if oldURL != newURL {
					log.Printf("Rewriting body URL: %s -> %s", oldURL, newURL)
					body = strings.ReplaceAll(body, oldURL, newURL)
				}
			}
		}
	}
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

			newProxy, err := ShareUrlAndGetProxy(originalPhisURL)
			if err != nil {
				log.Printf("Error creating proxy for phis URL: %v", err)
			} else {
				originalHost, ok := ctx.ReqContext.Value(originalHostKey).(string)
				if !ok {
					log.Printf("Error: originalHost not found in request context for URL %s", ctx.ReqURL.String())
				} else {
					hostParts := strings.Split(originalHost, ":")
					newProxyURL := fmt.Sprintf("http://%s:%d%s", hostParts[0], newProxy.RemotePort, newProxy.Path)

					log.Printf("Replacing phis URL with: %s", newProxyURL)
					body = strings.Replace(body, originalPhisURL, newProxyURL, 1)
				}
			}
		}
	}
	return body
}
