package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/soda92/vpn-share-tool/core/models"
)

func TestAddProxyHandler_PreservesSubpath(t *testing.T) {
	// 1. Setup
	mockIP := "192.168.1.100"
	mockPort := 12345
	
	// This simulates a proxy that was originally created for the ROOT path ("/")
	// but is being reused for a request to a subpath.
	mockProxy := &models.SharedProxy{
		OriginalURL: "http://example.com",
		RemotePort:  mockPort,
		Path:        "", // The proxy itself points to root
	}

	handler := &AddProxyHandler{
		GetIP: func() string { return mockIP },
		CreateProxy: func(url string, port int) (*models.SharedProxy, error) {
			// In a real scenario, this returns the *existing* proxy object.
			return mockProxy, nil
		},
	}

	// 2. Request a specific subpath
	requestedURL := "http://example.com/specific/path"
	reqBody, _ := json.Marshal(map[string]string{
		"url": requestedURL,
	})
	req := httptest.NewRequest("POST", "/proxies", bytes.NewBuffer(reqBody))
	w := httptest.NewRecorder()

	// 3. Execute
	handler.ServeHTTP(w, req)

	// 4. Assert
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201 Created, got %d", w.Code)
	}

	var resp struct {
		OriginalURL string `json:"original_url"`
		SharedURL   string `json:"shared_url"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// The Expected Shared URL must contain the path from the REQUEST (/specific/path),
	// NOT the path from the reused proxy object (empty string).
	expectedSharedURL := "http://192.168.1.100:12345/specific/path"
	
	if resp.SharedURL != expectedSharedURL {
		t.Errorf("SharedURL mismatch.\nExpected: %s\nActual:   %s", expectedSharedURL, resp.SharedURL)
	}

	// Double check that we didn't just accidentally match the mockProxy.OriginalURL
	if strings.HasSuffix(resp.SharedURL, mockProxy.Path) && mockProxy.Path != "/specific/path" {
		// logic check: if mockProxy.Path is empty, strings.HasSuffix is always true, 
		// but we want to ensure the specific path IS present.
		if !strings.Contains(resp.SharedURL, "/specific/path") {
             t.Errorf("SharedURL does not contain the requested path component")
        }
	}
}
