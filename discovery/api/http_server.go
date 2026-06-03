package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/soda92/vpn-share-tool/discovery/proxy"
	"github.com/soda92/vpn-share-tool/discovery/registry"
	"github.com/soda92/vpn-share-tool/discovery/resources"
	"github.com/soda92/vpn-share-tool/discovery/store"
	"github.com/soheilhy/cmux"
)

const (
	httpListenPort = "8080"
)

var (
	SharePath = "/sambashare/VPN共享工具"
)

type updateInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	Sha256  string `json:"sha256"`
}

var (
	reVersionExe    = regexp.MustCompile(`^vpn-share-tool_v(\d+)([a-z]+)\.exe$`)
	reVersionZip    = regexp.MustCompile(`^vpn-share-tool_v(\d+)([a-z]+)\.zip$`)
	reVersionAppExe = regexp.MustCompile(`^vpn-share-tool-app_v(\d+)([a-z]+)\.exe$`)
	reVersionAppZip = regexp.MustCompile(`^vpn-share-tool-app_v(\d+)([a-z]+)\.zip$`)
)

func handleLatestVersion(w http.ResponseWriter, r *http.Request) {
	handleVersionRequest(w, r, false)
}

func handleLatestAppVersion(w http.ResponseWriter, r *http.Request) {
	handleVersionRequest(w, r, true)
}

func handleVersionRequest(w http.ResponseWriter, r *http.Request, isApp bool) {
	format := r.URL.Query().Get("format")
	useZip := format == "zip"

	var regexes []*regexp.Regexp
	if isApp {
		if useZip {
			// Primary: zip, Fallback: exe
			regexes = []*regexp.Regexp{reVersionAppZip, reVersionAppExe, reVersionZip, reVersionExe}
		} else {
			// Primary: exe, Fallback: zip
			regexes = []*regexp.Regexp{reVersionAppExe, reVersionAppZip, reVersionExe, reVersionZip}
		}
	} else {
		if useZip {
			// Primary: zip, Fallback: exe
			regexes = []*regexp.Regexp{reVersionZip, reVersionExe}
		} else {
			// Primary: exe, Fallback: zip
			regexes = []*regexp.Regexp{reVersionExe, reVersionZip}
		}
	}

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
		Sha256  string
	}

	var versions []version

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		var matches []string
		for _, re := range regexes {
			m := re.FindStringSubmatch(e.Name())
			if len(m) == 3 {
				matches = m
				break
			}
		}
		if len(matches) == 3 {
			// Check if corresponding .sha256 file exists (marking copy completion)
			sha256Path := filepath.Join(SharePath, e.Name()+".sha256")
			shaBytes, err := os.ReadFile(sha256Path)
			var hash string
			if err != nil {
				if isApp {
					// Skip if .sha256 doesn't exist (copy incomplete) for app
					continue
				}
				// For updater, it's ok if there's no .sha256 file
			} else {
				hash = strings.TrimSpace(string(shaBytes))
				fields := strings.Fields(hash)
				if len(fields) > 0 {
					hash = fields[0]
				}

				// Verify that the actual hash of the file on disk matches the expected hash in the .sha256 file.
				// This prevents serving an incomplete file during release copy.
				exePath := filepath.Join(SharePath, e.Name())
				if err := verifyLocalFileHash(exePath, hash); err != nil {
					log.Printf("Version %s has .sha256 file but actual file verification failed: %v", e.Name(), err)
					continue
				}
			}

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
				Sha256:  hash,
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
		Sha256:  latest.Sha256,
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

var (
	sessions      = make(map[string]string) // sessionToken -> username
	sessionsMutex = &sync.RWMutex{}
)

func generateSessionToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return uuid.New().String()
	}
	return hex.EncodeToString(b)
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sessionsMutex.RLock()
		_, exists := sessions[cookie.Value]
		sessionsMutex.RUnlock()

		if !exists {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !store.VerifyPassword(creds.Username, creds.Password) {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	token := generateSessionToken()
	sessionsMutex.Lock()
	sessions[token] = creds.Username
	sessionsMutex.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Can be true if running strictly TLS, but since -insecure is an option, false is safer/easier
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 30, // 30 days
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "username": creds.Username})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		sessionsMutex.Lock()
		delete(sessions, cookie.Value)
		sessionsMutex.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleCheckAuth(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
		return
	}

	sessionsMutex.RLock()
	username, exists := sessions[cookie.Value]
	sessionsMutex.RUnlock()

	if !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": true, "username": username})
}


func handleGetInstances(w http.ResponseWriter, r *http.Request) {
	activeInstances := registry.GetActiveInstances()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(activeInstances); err != nil {
		log.Printf("Failed to encode instances to JSON: %v", err)
		http.Error(w, "Failed to encode instances", http.StatusInternalServerError)
	}
}

func SentryPanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				sentry.CurrentHub().Recover(err)
				sentry.Flush(2 * time.Second)
				panic(err)
			}
		}()
		next.ServeHTTP(w, r)
	})
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
	protectedMux.HandleFunc("/logs", handleGetLogs)
	protectedMux.HandleFunc("/debug-panic", func(w http.ResponseWriter, r *http.Request) {
		panic("Sentry Go Test Panic!")
	})

	protectedHandler := RequireAuth(protectedMux)

	// Root Mux
	rootMux := http.NewServeMux()

	// Public Routes (for Auto-Update)
	rootMux.HandleFunc("/latest-version", handleLatestVersion)
	rootMux.HandleFunc("/latest-app-version", handleLatestAppVersion)
	rootMux.Handle("/download/", http.StripPrefix("/download/", http.FileServer(http.Dir(SharePath))))
	rootMux.HandleFunc("/solve-captcha", handleSolveCaptchaRequest)
	rootMux.HandleFunc("/upload-logs", handleUploadLogs)

	// Auth routes
	rootMux.HandleFunc("/login", handleLogin)
	rootMux.HandleFunc("/logout", handleLogout)
	rootMux.HandleFunc("/check-auth", handleCheckAuth)

	// Register specific protected API endpoints directly on rootMux
	rootMux.Handle("/create-proxy", protectedHandler)
	rootMux.Handle("/instances", protectedHandler)
	rootMux.Handle("/tagged-urls", protectedHandler)
	rootMux.Handle("/tagged-urls/", protectedHandler)
	rootMux.Handle("/cluster-proxies", protectedHandler)
	rootMux.Handle("/update-proxy-settings", protectedHandler)
	rootMux.Handle("/trigger-update-remote", protectedHandler)
	rootMux.Handle("/logs", protectedHandler)
	rootMux.Handle("/debug-panic", protectedHandler)

	// Serve the Vue frontend (Public)
	fsys, err := fs.Sub(frontendDist, "dist")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(fsys))
	rootMux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fsys.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if os.IsNotExist(err) {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}))

	// Wrap root handler with Sentry recovery middleware
	mainHandler := SentryPanicRecovery(rootMux)

	if insecure {
		log.Printf("Starting discovery HTTP server (INSECURE) on port %s", httpListenPort)
		server := &http.Server{
			Addr:    ":" + httpListenPort,
			Handler: mainHandler,
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
			Handler:   mainHandler,
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

type cacheEntry struct {
	Hash    string
	Size    int64
	ModTime time.Time
}

var (
	verifiedHashesCache     = make(map[string]cacheEntry)
	verifiedHashesCacheLock sync.RWMutex
)

func verifyLocalFileHash(filePath string, expectedHash string) error {
	fi, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	verifiedHashesCacheLock.RLock()
	cached, ok := verifiedHashesCache[filePath]
	verifiedHashesCacheLock.RUnlock()

	if ok && cached.Size == fi.Size() && cached.ModTime.Equal(fi.ModTime()) && strings.EqualFold(cached.Hash, expectedHash) {
		return nil
	}

	// Not cached or modified, compute hash
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	// Cache the verified hash along with size and mod time
	verifiedHashesCacheLock.Lock()
	verifiedHashesCache[filePath] = cacheEntry{
		Hash:    actualHash,
		Size:    fi.Size(),
		ModTime: fi.ModTime(),
	}
	verifiedHashesCacheLock.Unlock()

	return nil
}
