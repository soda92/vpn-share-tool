package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	httpListenPort = "8080"
	SharePath      = "/sambashare/VPN共享工具"
)

type updateInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

var reVersion = regexp.MustCompile(`vpn-share-tool_v(\d+)([a-z]+)\.exe`)

func handleLatestVersion(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(SharePath)
	if err != nil {
		log.Printf("Failed to read share path: %v", err)
		http.Error(w, "Failed to check for updates", http.StatusInternalServerError)
		return
	}

	type version struct {
		Counter int
		Suffix  string
		Full    string
		File    string
	}

	var versions []version

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matches := reVersion.FindStringSubmatch(e.Name())
		if len(matches) == 3 {
			counter, err := strconv.Atoi(matches[1])
			if err != nil {
				log.Printf("Failed to parse version counter from %s: %v", e.Name(), err)
				continue
			}
			suffix := matches[2]
			versions = append(versions, version{
				Counter: counter,
				Suffix:  suffix,
				Full:    fmt.Sprintf("v%d%s", counter, suffix),
				File:    e.Name(),
			})
		}
	}

	if len(versions) == 0 {
		http.Error(w, "No versions found", http.StatusNotFound)
		return
	}

	sort.Slice(versions, func(i, j int) bool {
		if versions[i].Counter != versions[j].Counter {
			return versions[i].Counter > versions[j].Counter
		}
		// Compare suffixes (length then alphabetic)
		// e.g. 'aa' > 'z'.
		if len(versions[i].Suffix) != len(versions[j].Suffix) {
			return len(versions[i].Suffix) > len(versions[j].Suffix)
		}
		return versions[i].Suffix > versions[j].Suffix
	})

	latest := versions[0]
	resp := updateInfo{
		Version: latest.Full,
		URL:     fmt.Sprintf("/download/%s", latest.File),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleTriggerUpdateRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Address == "" {
		http.Error(w, "Address is required", http.StatusBadRequest)
		return
	}

	// Proxy the request to the client instance
	targetURL := fmt.Sprintf("http://%s/trigger-update", req.Address)
	log.Printf("Triggering update for %s", targetURL)

	resp, err := http.Post(targetURL, "application/json", nil)
	if err != nil {
		log.Printf("Failed to trigger update on %s: %v", targetURL, err)
		http.Error(w, fmt.Sprintf("Failed to trigger update: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Instance returned status %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func startHTTPServer() {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/create-proxy", handleCreateProxy)
	mux.HandleFunc("/instances", handleGetInstances)
	mux.HandleFunc("/tagged-urls", handleTaggedURLs)
	mux.HandleFunc("/tagged-urls/", handleTaggedURLs)
	mux.HandleFunc("/cluster-proxies", handleClusterProxies)
	mux.HandleFunc("/toggle-debug-proxy", handleToggleDebugProxy)
	
	// Update routes
	mux.HandleFunc("/latest-version", handleLatestVersion)
	mux.HandleFunc("/trigger-update-remote", handleTriggerUpdateRemote)
	mux.Handle("/download/", http.StripPrefix("/download/", http.FileServer(http.Dir(SharePath))))

	// Serve the Vue frontend
	fsys, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(fsys))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the requested file exists
		_, err := fsys.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if os.IsNotExist(err) {
			// If not, serve index.html for SPA routing
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}))

	log.Printf("Starting discovery HTTP server on port %s", httpListenPort)
	
	// Check for certificates
	certFile := "certs/server.crt"
	keyFile := "certs/server.key"
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			log.Printf("TLS certificates found. Serving HTTPS.")
			if err := http.ListenAndServeTLS(":"+httpListenPort, certFile, keyFile, mux); err != nil {
				log.Fatalf("Failed to start HTTPS server: %v", err)
			}
			return
		}
	}

	log.Printf("No certificates found. Serving HTTP (Insecure).")
	if err := http.ListenAndServe(":"+httpListenPort, mux); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
