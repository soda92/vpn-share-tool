package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeHost takes a URL string and returns a normalized host:port string.
// It converts "localhost" to "127.0.0.1" to ensure consistency.
// It handles cases with or without scheme.
func NormalizeHost(u string) string {
	// If scheme is missing, parse might fail or treat it as path. Add valid scheme.
	if !strings.Contains(u, "://") {
		u = "http://" + u
	}

	parsed, err := url.Parse(u)
	if err != nil {
		// Fallback: just return the original if parsing fails completely,
		// though likely it's just the input string.
		return u
	}

	host := parsed.Hostname()
	port := parsed.Port()

	// Normalize localhost
	if host == "localhost" {
		host = "127.0.0.1"
	}

	if port != "" {
		return fmt.Sprintf("%s:%s", host, port)
	}
	return host
}
