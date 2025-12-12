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

func BasicAuth(next http.Handler) http.Handler {
	user := os.Getenv("BASIC_AUTH_USER")
	pass := os.Getenv("BASIC_AUTH_PASS")

	if user == "" || pass == "" {
		log.Println("Warning: BASIC_AUTH_USER/PASS not set. Web UI is unsecured.")
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func startHTTPServer() {
	// Protected Mux for Dashboard and Management APIs
	protectedMux := http.NewServeMux()
	
	// API routes (Protected)
	protectedMux.HandleFunc("/create-proxy", handleCreateProxy)
	protectedMux.HandleFunc("/instances", handleGetInstances)
	protectedMux.HandleFunc("/tagged-urls", handleTaggedURLs)
	protectedMux.HandleFunc("/tagged-urls/", handleTaggedURLs)
	protectedMux.HandleFunc("/cluster-proxies", handleClusterProxies)
	protectedMux.HandleFunc("/toggle-debug-proxy", handleToggleDebugProxy)
	protectedMux.HandleFunc("/trigger-update-remote", handleTriggerUpdateRemote)

	// Serve the Vue frontend (Protected)
	fsys, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(fsys))
	protectedMux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fsys.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if os.IsNotExist(err) {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}))

	// Root Mux
	rootMux := http.NewServeMux()
	
	// Public Routes (for Auto-Update)
	rootMux.HandleFunc("/latest-version", handleLatestVersion)
	rootMux.Handle("/download/", http.StripPrefix("/download/", http.FileServer(http.Dir(SharePath))))
	
	// Delegate everything else to Protected Mux
	rootMux.Handle("/", BasicAuth(protectedMux))

	log.Printf("Starting discovery HTTP server on port %s", httpListenPort)
	
	// Check for certificates
	certFile := "certs/server.crt"
	keyFile := "certs/server.key"
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			log.Printf("TLS certificates found. Serving HTTPS.")
			if err := http.ListenAndServeTLS(":"+httpListenPort, certFile, keyFile, rootMux); err != nil {
				log.Fatalf("Failed to start HTTPS server: %v", err)
			}
			return
		}
	}

	log.Printf("No certificates found. Serving HTTP (Insecure).")
	if err := http.ListenAndServe(":"+httpListenPort, rootMux); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
