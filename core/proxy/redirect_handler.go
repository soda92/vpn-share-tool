package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/soda92/vpn-share-tool/core/models"
)

func HandleRedirect(resp *http.Response, target *url.URL) error {
	if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
		location := resp.Header.Get("Location")
		if location == "" {
			return nil // No Location header, nothing to do
		}

		locationURL, err := url.Parse(location)
		if err != nil {
			log.Printf("Error parsing Location header: %v", err)
			return nil
		}

		if !locationURL.IsAbs() {
			locationURL = target.ResolveReference(locationURL)
		}

		ProxiesLock.RLock()
		var existingProxy *models.SharedProxy
		for _, p := range Proxies {
			if p.OriginalURL == locationURL.String() {
				existingProxy = p
				break
			}
		}
		ProxiesLock.RUnlock()

		originalHost, ok := resp.Request.Context().Value(models.OriginalHostKey).(string)
		if !ok {
			log.Println("Error: could not retrieve originalHost from context or it's not a string")
			return nil
		}
		hostParts := strings.Split(originalHost, ":")
		proxyHost := hostParts[0]

		if existingProxy != nil {
			newLocation := fmt.Sprintf("http://%s:%d%s", proxyHost, existingProxy.RemotePort, locationURL.RequestURI())
			resp.Header.Set("Location", newLocation)
			log.Printf("Redirecting to existing proxy: %s", newLocation)
		} else {
			log.Printf("Redirect location not proxied, creating new proxy for: %s", locationURL.String())
			// Recursively call ShareUrlAndGetProxy to create a new proxy
			newProxy, err := ShareUrlAndGetProxy(locationURL.String(), 0)
			if err != nil {
				log.Printf("Error creating new proxy for redirect: %v", err)
			} else {
				newLocation := fmt.Sprintf("http://%s:%d%s", proxyHost, newProxy.RemotePort, locationURL.RequestURI())
				resp.Header.Set("Location", newLocation)
				log.Printf("Redirecting to new proxy: %s", newLocation)
			}
		}
	}
	return nil
}
