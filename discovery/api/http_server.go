package api

import (
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/soda92/vpn-share-tool/discovery/proxy"
	"github.com/soda92/vpn-share-tool/discovery/registry"
	"github.com/soda92/vpn-share-tool/discovery/resources"
	"github.com/soheilhy/cmux"
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
	// Since we upgraded client to use HTTPS for discovery URL construction,
	// but the client LISTEN port (API) is HTTP or HTTPS?
	// core/server.go: StartApiServer uses http.ListenAndServe (plaintext).
	// So we should use http:// here.
	// Check core/server.go: StartApiServer listens on apiPort.
	// It does NOT wrap in TLS.
	// So targetURL is http://
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

func handleGetInstances(w http.ResponseWriter, r *http.Request) {
	activeInstances := registry.GetActiveInstances()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(activeInstances); err != nil {
		log.Printf("Failed to encode instances to JSON: %v", err)
		http.Error(w, "Failed to encode instances", http.StatusInternalServerError)
	}
}

func StartHTTPServer(insecure bool) {
	// Protected Mux for Dashboard and Management APIs
	protectedMux := http.NewServeMux()

	// API routes (Protected)
	protectedMux.HandleFunc("/create-proxy", proxy.HandleCreateProxy)
	protectedMux.HandleFunc("/instances", handleGetInstances)
	protectedMux.HandleFunc("/tagged-urls", HandleTaggedURLs)
	protectedMux.HandleFunc("/tagged-urls/", HandleTaggedURLs)
	protectedMux.HandleFunc("/cluster-proxies", proxy.HandleClusterProxies)
	protectedMux.HandleFunc("/update-proxy-settings", HandleUpdateProxySettings)
	protectedMux.HandleFunc("/trigger-update-remote", handleTriggerUpdateRemote)

	// Serve the Vue frontend (Protected)
	fsys, err := fs.Sub(frontendDist, "dist")
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
	rootMux.HandleFunc("/solve-captcha", handleSolveCaptchaRequest)

	// Delegate everything else to Protected Mux
	rootMux.Handle("/", BasicAuth(protectedMux))

	if insecure {
		log.Printf("Starting discovery HTTP server (INSECURE) on port %s", httpListenPort)
		server := &http.Server{
			Addr:    ":" + httpListenPort,
			Handler: rootMux,
		}
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("HTTP Server error: %v", err)
		}
		return
	}

	log.Printf("Starting discovery HTTP server on port %s", httpListenPort)

	// Load embedded certs
	cert, err := tls.X509KeyPair(resources.ServerCert, resources.ServerKey)
	if err != nil {
		log.Fatalf("Failed to load embedded server certs: %v", err)
	}

	log.Printf("Embedded TLS certificates found. Serving HTTPS and HTTP Redirect on same port.")

	// Create the main listener
	l, err := net.Listen("tcp", ":"+httpListenPort)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", httpListenPort, err)
	}

	// Create a multiplexer
	m := cmux.New(l)

	// Match TLS connections (HTTPS)
	httpsL := m.Match(cmux.TLS())

	// Match anything else as HTTP (for redirection)
	httpL := m.Match(cmux.Any())

	// Start HTTPS Server
	go func() {
		server := &http.Server{
			Handler:   rootMux,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		}
		if err := server.ServeTLS(httpsL, "", ""); err != nil {
			log.Printf("HTTPS Server error: %v", err)
		}
	}()

	// Start HTTP Redirect Server
	go func() {
		redirectServer := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				target := "https://" + r.Host + r.RequestURI
				http.Redirect(w, r, target, http.StatusMovedPermanently)
			}),
		}
		if err := redirectServer.Serve(httpL); err != nil {
			log.Printf("HTTP Redirect Server error: %v", err)
		}
	}()

	// Start multiplexing
	if err := m.Serve(); err != nil {
		log.Fatalf("Multiplexer error: %v", err)
	}
}
