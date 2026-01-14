package pipeline

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/soda92/vpn-share-tool/core/models"
)

var (
	reLocalhost  = regexp.MustCompile(`(https?://)(localhost|127\.\d{1,3}\.\d{1,3}\.\d{1,3})(:\d+)?`)
	rePrivate10  = regexp.MustCompile(`(https?://)(10\.\d{1,3}\.\d{1,3}\.\d{1,3})(:\d+)?`)
	rePrivate172 = regexp.MustCompile(`(https?://)(172\.(?:1[6-9]|2\d|3[0-1])\.\d{1,3}\.\d{1,3})(:\d+)?`)
	rePrivate192 = regexp.MustCompile(`(https?://)(192\.168\.\d{1,3}\.\d{1,3})(:\d+)?`)
)

// Reachability Cache
type reachabilityResult struct {
	reachable bool
	timestamp time.Time
}

var (
	reachCache     = make(map[string]reachabilityResult)
	reachCacheLock sync.RWMutex
	fastClient     = &http.Client{
		Timeout: 800 * time.Millisecond,
	}
)

func isReachableFast(urlStr string) bool {
	// 1. Check Cache
	reachCacheLock.RLock()
	res, ok := reachCache[urlStr]
	reachCacheLock.RUnlock()

	if ok && time.Since(res.timestamp) < 5*time.Minute {
		return res.reachable
	}

	// 2. Perform Check
	reachable := false
	req, err := http.NewRequest("HEAD", urlStr, nil)
	if err == nil {
		resp, err := fastClient.Do(req)
		if err == nil {
			resp.Body.Close()
			reachable = true
		}
	}

	// 3. Update Cache
	reachCacheLock.Lock()
	reachCache[urlStr] = reachabilityResult{
		reachable: reachable,
		timestamp: time.Now(),
	}
	reachCacheLock.Unlock()

	if !reachable {
		log.Printf("Internal URL %s is unreachable (timeout/error)", urlStr)
	}

	return reachable
}

func RewriteInternalURLs(ctx *models.ProcessingContext, body string) string {
	// 1. Rewrite specific internal URLs (e.g. 10.x.x.x)
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
			originalHost, ctxOk := ctx.ReqContext.Value(models.OriginalHostKey).(string)

			for match := range uniqueMatches {
				if _, processed := replacements[match]; processed {
					continue
				}

				if ctx.Services.MyIP != "" && strings.Contains(match, ctx.Services.MyIP) {
					continue
				}

				// Verify if the detected internal URL is actually reachable (Fast Cached Check)
				if !isReachableFast(match) {
					continue
				}

				var newProxy *models.SharedProxy
				var err error

				// Use injected CreateProxy service
				if ctx.Services.CreateProxy != nil {
					newProxy, err = ctx.Services.CreateProxy(match, 0)
				} else {
					err = fmt.Errorf("CreateProxy service not available")
				}

				if err != nil {
					log.Printf("Error creating proxy for internal URL %s: %v", match, err)
					continue
				}

				if !ctxOk {
					if ctx.Services.MyIP != "" {
						replacements[match] = fmt.Sprintf("http://%s:%d", ctx.Services.MyIP, newProxy.RemotePort)
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
