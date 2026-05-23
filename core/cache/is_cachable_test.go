package cache

import (
	"testing"
)

func TestIsCacheable(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/img/logo.png", true},
		{"/font/awesome.woff2", true},
		{"/js/jquery.min.js", true},
		{"/js/bootstrap.bundle.js", true},
		{"/js/moment.js", true},
		{"/style/main.css", false}, // CSS is not cacheable (URL rewriting)
		{"/api/data.json", false},
		{"/index.html", false},
		{"/js/app.js", false}, // Non-library JS is not cacheable
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsCacheable(tt.path); got != tt.expected {
				t.Errorf("IsCacheable(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}
