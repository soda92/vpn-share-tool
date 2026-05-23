package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/proxy"
)

func TestE2E_ProxyTraffic(t *testing.T) {
	proxy.Reset()

	// 0. Setup Temporary Storage for the test
	tmpDir, err := os.MkdirTemp("", "vpn-share-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// Override DebugStoragePath so persistence goes to tmpDir
	debug.DebugStoragePath = tmpDir

	// 1. Setup Mock Target Server
	targetMux := http.NewServeMux()
	targetMux.HandleFunc("/200", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Target OK"))
	})
	targetMux.HandleFunc("/302", func(w http.ResponseWriter, r *http.Request) {
		// Redirect to /200
		http.Redirect(w, r, "/200", http.StatusFound)
	})
	targetMux.HandleFunc("/404", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Target Not Found"))
	})

	targetSrv := httptest.NewServer(targetMux)
	defer targetSrv.Close()

	// 2. Setup Client API Server
	MyIP = "127.0.0.1"
	Version = "test-v1"
	
	// Initialize proxy config with 0 port initially
	proxy.SetGlobalConfig(MyIP, 0, "", GetHTTPClient)
	
	apiHandler := SetupApiMux()
	apiSrv := httptest.NewServer(apiHandler)
	defer apiSrv.Close()

	// Update the global APIPort so handlers know where we are
	apiURL, _ := url.Parse(apiSrv.URL)
	var apiPort int
	fmt.Sscanf(apiURL.Port(), "%d", &apiPort)
	APIPort = apiPort
	proxy.SetGlobalConfig(MyIP, APIPort, "", GetHTTPClient)

	apiClient := apiSrv.Client()

	t.Run("Status 200 OK", func(t *testing.T) {
		sharedURL := createProxy(t, apiClient, apiSrv.URL, targetSrv.URL+"/200")
		
		resp, err := http.Get(sharedURL)
		if err != nil {
			t.Fatalf("Failed to access proxy: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "Target OK" {
			t.Errorf("Expected body 'Target OK', got %q", string(body))
		}
	})

	t.Run("Status 404 Not Found", func(t *testing.T) {
		sharedURL := createProxy(t, apiClient, apiSrv.URL, targetSrv.URL+"/404")
		
		resp, err := http.Get(sharedURL)
		if err != nil {
			t.Fatalf("Failed to access proxy: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("Status 302 Redirect Handling", func(t *testing.T) {
		// When we hit /302, it redirects to /200.
		// Our proxy should intercept the 302, see that /200 is not proxied,
		// create a new proxy for /200, and rewrite the Location header.
		
		sharedURL := createProxy(t, apiClient, apiSrv.URL, targetSrv.URL+"/302")
		
		// We use a custom client that DOES NOT follow redirects so we can inspect the Location header
		noFollowClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		resp, err := noFollowClient.Get(sharedURL)
		if err != nil {
			t.Fatalf("Failed to access proxy: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusFound {
			t.Errorf("Expected status 302, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location == "" {
			t.Fatal("Missing Location header in redirect")
		}

		// The Location should now point to a NEW proxy port on 127.0.0.1
		locURL, err := url.Parse(location)
		if err != nil {
			t.Fatalf("Invalid Location header: %v", err)
		}

		if locURL.Hostname() != "127.0.0.1" {
			t.Errorf("Expected redirect host 127.0.0.1, got %s", locURL.Hostname())
		}
		
		if locURL.Port() == "" {
			t.Error("Expected redirect to have a port")
		}

		// Now verify that the NEW proxy actually works
		resp2, err := http.Get(location)
		if err != nil {
			t.Fatalf("Failed to access auto-created redirect proxy: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("Redirect proxy returned status %d, want 200", resp2.StatusCode)
		}
		body, _ := io.ReadAll(resp2.Body)
		if string(body) != "Target OK" {
			t.Errorf("Redirect proxy returned body %q, want 'Target OK'", string(body))
		}
	})
}

func createProxy(t *testing.T, client *http.Client, apiURL string, targetURL string) string {
	reqBody, _ := json.Marshal(map[string]string{"url": targetURL})
	resp, err := client.Post(apiURL+"/proxies", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create proxy for %s: %v", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Create proxy failed with status %d", resp.StatusCode)
	}

	var data struct {
		SharedURL string `json:"shared_url"`
	}
	json.NewDecoder(resp.Body).Decode(&data)
	return data.SharedURL
}
