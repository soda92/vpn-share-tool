package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
)

const httpListenPort = "8080"

func startHTTPServer() {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/create-proxy", handleCreateProxy)
	mux.HandleFunc("/instances", handleGetInstances)
	mux.HandleFunc("/tagged-urls", handleTaggedURLs)
	mux.HandleFunc("/tagged-urls/", handleTaggedURLs)
	mux.HandleFunc("/cluster-proxies", handleClusterProxies)

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
	if err := http.ListenAndServe(":"+httpListenPort, mux); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
