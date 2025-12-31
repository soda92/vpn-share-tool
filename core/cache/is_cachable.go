package cache

import (
	"path/filepath"
	"strings"
)

// IsCacheable determines if a request path points to a static asset that should be cached
// and NOT processed by the modification pipeline.
//
// Rules:
// 1. Images and fonts are always cacheable.
// 2. CSS is NOT cacheable (treated as dynamic for URL rewriting).
// 3. JS is cacheable ONLY if it matches known libraries (jquery, bootstrap, moment).
func IsCacheable(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".woff", ".woff2", ".ttf", ".eot":
		return true
	case ".js":
		lowerPath := strings.ToLower(path)
		// Add more libraries here as needed
		if strings.Contains(lowerPath, "jquery") ||
			strings.Contains(lowerPath, "bootstrap") ||
			strings.Contains(lowerPath, "moment") {
			return true
		}
		return false
	default:
		return false
	}
}
