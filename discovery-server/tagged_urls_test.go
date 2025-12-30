package main

import "testing"

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Standard URL",
			input:    "http://example.com",
			expected: "example.com",
		},
		{
			name:     "URL with Port",
			input:    "http://example.com:8080",
			expected: "example.com:8080",
		},
		{
			name:     "Localhost normalization",
			input:    "http://localhost:3000",
			expected: "127.0.0.1:3000",
		},
		{
			name:     "127.0.0.1 preservation",
			input:    "http://127.0.0.1:3000",
			expected: "127.0.0.1:3000",
		},
		{
			name:     "Missing scheme adds http",
			input:    "example.com:9090",
			expected: "example.com:9090",
		},
		{
			name:     "Missing scheme with localhost",
			input:    "localhost:8080",
			expected: "127.0.0.1:8080",
		},
		{
			name:     "Different ports on same host are distinct",
			input:    "http://127.0.0.1:9090",
			expected: "127.0.0.1:9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeHost(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeHost(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeHostCollision(t *testing.T) {
	// Explicitly test the regression scenario:
	// Two URLs on the same host but different ports must produce different normalized keys.
	url1 := "http://localhost:8080"
	url2 := "http://localhost:9090"

	norm1 := normalizeHost(url1)
	norm2 := normalizeHost(url2)

	if norm1 == norm2 {
		t.Errorf("Collision detected! %s and %s normalized to same key: %s", url1, url2, norm1)
	}

	// Test localhost vs 127.0.0.1 matching
	url3 := "http://127.0.0.1:8080"
	norm3 := normalizeHost(url3)

	if norm1 != norm3 {
		t.Errorf("Mismatch! %s and %s should normalize to same key but got %s and %s", url1, url3, norm1, norm3)
	}
}
