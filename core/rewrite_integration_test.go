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
	"strings"
	"testing"

	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/proxy"
)

func TestE2E_PipelineRewrite(t *testing.T) {
	proxy.Reset()

	// 0. Setup Temporary Storage
	tmpDir, err := os.MkdirTemp("", "vpn-share-rewrite-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	debug.DebugStoragePath = tmpDir

	// 1. Setup Client API Server (Shared for subtests)
	MyIP = "127.0.0.1"
	Version = "test-v1"
	proxy.SetGlobalConfig(MyIP, 0, "", GetHTTPClient)
	
	apiHandler := SetupApiMux()
	apiSrv := httptest.NewServer(apiHandler)
	defer apiSrv.Close()

	apiURL, _ := url.Parse(apiSrv.URL)
	var apiPort int
	fmt.Sscanf(apiURL.Port(), "%d", &apiPort)
	APIPort = apiPort
	proxy.SetGlobalConfig(MyIP, APIPort, "", GetHTTPClient)

	apiClient := apiSrv.Client()

	t.Run("Rewrite JS Content", func(t *testing.T) {
		// Use a dedicated mock server for this subtest to ensure a unique port/host
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/javascript")
			w.Write([]byte(`Ehr.openChrome = function(url){ var executableFullPath = "D:\\pb\\chromerun.exe "+url; };`))
		}))
		defer srv.Close()

		sharedURL := createProxyInTest(t, apiClient, apiSrv.URL, srv.URL+"/legacy.js")
		
		// MANUALLY Enable HIS system and ContentMod
		proxy.ProxiesLock.Lock()
		found := false
		for _, p := range proxy.Proxies {
			if strings.Contains(p.OriginalURL, srv.URL) {
				p.ActiveSystems = []string{"HIS"}
				p.Settings.EnableContentMod = true
				found = true
				break
			}
		}
		proxy.ProxiesLock.Unlock()
		
		if !found {
			t.Fatal("Failed to find created proxy")
		}

		resp, err := http.Get(sharedURL)
		if err != nil {
			t.Fatalf("Failed to access proxy: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		content := string(body)

		if !strings.Contains(content, `window.open(url, "_blank"); return;`) {
			t.Errorf("JS content was not rewritten correctly. Got:\n%s", content)
		}
	})

	t.Run("Rewrite HTML Content", func(t *testing.T) {
		// Use a dedicated mock server for this subtest to ensure a unique port/host
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><body><script>if ( window.showModalDialog == undefined ) { alert('legacy'); }</script></body></html>`))
		}))
		defer srv.Close()

		sharedURL := createProxyInTest(t, apiClient, apiSrv.URL, srv.URL+"/page.html")
		
		proxy.ProxiesLock.Lock()
		for _, p := range proxy.Proxies {
			if strings.Contains(p.OriginalURL, srv.URL) {
				p.ActiveSystems = []string{"HIS"}
				p.Settings.EnableContentMod = true
			}
		}
		proxy.ProxiesLock.Unlock()

		resp, err := http.Get(sharedURL)
		if err != nil {
			t.Fatalf("Failed to access proxy: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		content := string(body)

		if !strings.Contains(content, "if(true)") && strings.Contains(content, "window.showModalDialog") {
			t.Errorf("HTML content was not rewritten correctly. Got:\n%s", content)
		}
	})
}

func createProxyInTest(t *testing.T, client *http.Client, apiURL string, targetURL string) string {
	reqBody, _ := json.Marshal(map[string]string{"url": targetURL})
	resp, err := client.Post(apiURL+"/proxies", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
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
