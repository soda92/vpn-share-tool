package core

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
)

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

				if MyIP != "" && strings.Contains(match, MyIP) {
					continue
				}

				newProxy, err := ShareUrlAndGetProxy(match, 0)
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
