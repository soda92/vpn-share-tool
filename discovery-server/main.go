package main

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

//go:embed all:frontend/dist
var frontendDist embed.FS

const (
	listenPort      = "45679"
	httpListenPort  = "8080"
	storageFilePath = "tagged_urls.json"
)

type Instance struct {
	Address  string    `json:"address"`
	LastSeen time.Time `json:"last_seen"`
}

type TaggedURL struct {
	ID        string    `json:"id"`
	Tag       string    `json:"tag"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	instances       = make(map[string]Instance)
	mutex           = &sync.Mutex{}
	cleanupInterval = 1 * time.Minute
	staleTimeout    = 5 * time.Minute

	taggedURLs      = make(map[string]TaggedURL)
	taggedURLsMutex = &sync.Mutex{}
)

func loadTaggedURLs() {
	taggedURLsMutex.Lock()
	defer taggedURLsMutex.Unlock()

	data, err := os.ReadFile(storageFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No tagged URLs file found, starting fresh.")
			return
		}
		log.Printf("Error reading tagged URLs file: %v", err)
		return
	}

	if err := json.Unmarshal(data, &taggedURLs); err != nil {
		log.Printf("Error unmarshaling tagged URLs: %v", err)
	}
	log.Printf("Loaded %d tagged URLs from %s", len(taggedURLs), storageFilePath)
}

func saveTaggedURLs() error {
	taggedURLsMutex.Lock()
	defer taggedURLsMutex.Unlock()

	data, err := json.MarshalIndent(taggedURLs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tagged URLs: %w", err)
	}

	if err := os.WriteFile(storageFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tagged URLs file: %w", err)
	}
	return nil
}

func main() {
	loadTaggedURLs()

	// Start TCP server for vpn-share-tool instances
	go startTCPServer()

	// Start the automatic proxy creator
	go startAutoProxyCreator()

	// Start HTTP server for the web UI
	startHTTPServer()
}

func startTCPServer() {
	log.Printf("Starting discovery TCP server on port %s", listenPort)
	listener, err := net.Listen("tcp", ":"+listenPort)
	if err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}
	defer listener.Close()

	// Periodically clean up stale instances
	go cleanupStaleInstances()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func startHTTPServer() {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/create-proxy", handleCreateProxy)
	mux.HandleFunc("/instances", handleGetInstances)
	mux.HandleFunc("/tagged-urls", handleTaggedURLs)
	mux.HandleFunc("/tagged-urls/", handleTaggedURLs)

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

func handleCreateProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	activeInstances := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		activeInstances = append(activeInstances, instance)
	}
	mutex.Unlock()

	for _, instance := range activeInstances {
		// Check if the instance can reach the URL
		canReachURL := fmt.Sprintf("http://%s/can-reach?url=%s", instance.Address, url.QueryEscape(req.URL))
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(canReachURL)
		if err != nil {
			log.Printf("Error checking reachability on %s: %v", instance.Address, err)
			continue
		}

		var canReachResp struct {
			Reachable bool `json:"reachable"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&canReachResp); err != nil {
			log.Printf("Error decoding reachability response from %s: %v", instance.Address, err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if canReachResp.Reachable {
			// This instance can reach the URL, so create the proxy here
			createProxyURL := fmt.Sprintf("http://%s/proxies", instance.Address)
			proxyReqBody, _ := json.Marshal(map[string]string{"url": req.URL})
			resp, err := http.Post(createProxyURL, "application/json", bytes.NewBuffer(proxyReqBody))
			if err != nil {
				log.Printf("Error creating proxy on %s: %v", instance.Address, err)
				continue
			}

			if resp.StatusCode == http.StatusCreated {
				var proxyResp struct {
					OriginalURL string `json:"original_url"`
					SharedURL   string `json:"shared_url"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&proxyResp); err != nil {
					log.Printf("Error decoding proxy response from %s: %v", instance.Address, err)
					resp.Body.Close()
					continue
				}
				resp.Body.Close()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(proxyResp)
				return
			} else {
				resp.Body.Close()
			}
		}
	}

	// If no instance can reach the URL
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "No available instance can reach the target URL."})
}

func handleGetInstances(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()

	activeInstances := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		activeInstances = append(activeInstances, instance)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(activeInstances); err != nil {
		log.Printf("Failed to encode instances to JSON: %v", err)
		http.Error(w, "Failed to encode instances", http.StatusInternalServerError)
	}
}

func handleGetAllProxies(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	activeInstances := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		activeInstances = append(activeInstances, instance)
	}
	mutex.Unlock()

	type ProxyInfo struct {
		OriginalURL string `json:"original_url"`
		RemotePort  int    `json:"remote_port"`
		Path        string `json:"path"`
		SharedURL   string `json:"shared_url"`
	}

	allProxies := make([]ProxyInfo, 0)

	for _, instance := range activeInstances {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://%s/active-proxies", instance.Address))
		if err != nil {
			log.Printf("Failed to get active proxies from %s: %v", instance.Address, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var proxies []ProxyInfo
			if err := json.NewDecoder(resp.Body).Decode(&proxies); err != nil {
				log.Printf("Failed to decode active proxies from %s: %v", instance.Address, err)
				continue
			}
			// Add the server address to each proxy
			for i := range proxies {
				host, _, _ := net.SplitHostPort(instance.Address)
				proxies[i].SharedURL = fmt.Sprintf("http://%s:%d%s", host, proxies[i].RemotePort, proxies[i].Path)
			}
			allProxies = append(allProxies, proxies...)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(allProxies); err != nil {
		log.Printf("Failed to encode all proxies to JSON: %v", err)
		http.Error(w, "Failed to encode all proxies", http.StatusInternalServerError)
	}
}

func handleTaggedURLs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getTaggedURLs(w, r)
	case http.MethodPost:
		postTaggedURL(w, r)
	case http.MethodPut:
		putTaggedURL(w, r)
	case http.MethodDelete:
		deleteTaggedURL(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getTaggedURLs(w http.ResponseWriter, r *http.Request) {
	taggedURLsMutex.Lock()
	urls := make([]TaggedURL, 0, len(taggedURLs))
	for _, u := range taggedURLs {
		urls = append(urls, u)
	}
	taggedURLsMutex.Unlock()

	// Concurrently fetch all active proxies to enrich the response
	mutex.Lock()
	activeInstances := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		activeInstances = append(activeInstances, instance)
	}
	mutex.Unlock()

	type ProxyInfo struct {
		OriginalURL string `json:"original_url"`
		RemotePort  int    `json:"remote_port"`
		Path        string `json:"path"`
		SharedURL   string `json:"shared_url"`
	}

	allProxies := make(map[string]string)
	var wg sync.WaitGroup
	var proxyMutex sync.Mutex

	for _, instance := range activeInstances {
		wg.Add(1)
		go func(instance Instance) {
			defer wg.Done()
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(fmt.Sprintf("http://%s/active-proxies", instance.Address))
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var proxies []ProxyInfo
				if err := json.NewDecoder(resp.Body).Decode(&proxies); err == nil {
					proxyMutex.Lock()
					for _, p := range proxies {
						host, _, _ := net.SplitHostPort(instance.Address)
						allProxies[p.OriginalURL] = fmt.Sprintf("http://%s:%d%s", host, p.RemotePort, p.Path)
					}
					proxyMutex.Unlock()
				}
			}
		}(instance)
	}
	wg.Wait()

	// Enrich the tagged URLs with their proxy status
	type EnrichedTaggedURL struct {
		TaggedURL
		ProxyURL string `json:"proxy_url,omitempty"`
	}

	enrichedUrls := make([]EnrichedTaggedURL, len(urls))
	for i, u := range urls {
		enrichedUrls[i] = EnrichedTaggedURL{TaggedURL: u}
		if proxyURL, ok := allProxies[u.URL]; ok {
			enrichedUrls[i].ProxyURL = proxyURL
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(enrichedUrls); err != nil {
		log.Printf("Failed to encode tagged URLs: %v", err)
		http.Error(w, "Failed to encode URLs", http.StatusInternalServerError)
	}
}

func postTaggedURL(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		Tag string `json:"tag"`
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if reqBody.Tag == "" || reqBody.URL == "" {
		http.Error(w, "Tag and URL are required", http.StatusBadRequest)
		return
	}

	newURL := TaggedURL{
		ID:        uuid.New().String(),
		Tag:       reqBody.Tag,
		URL:       reqBody.URL,
		CreatedAt: time.Now(),
	}

	taggedURLsMutex.Lock()
	taggedURLs[newURL.ID] = newURL
	taggedURLsMutex.Unlock()

	if err := saveTaggedURLs(); err != nil {
		log.Printf("Error saving tagged URLs: %v", err)
		http.Error(w, "Failed to save URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newURL)
}

func putTaggedURL(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/tagged-urls/")
	var reqBody struct {
		Tag string `json:"tag"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if reqBody.Tag == "" {
		http.Error(w, "Tag is required", http.StatusBadRequest)
		return
	}

	taggedURLsMutex.Lock()
	defer taggedURLsMutex.Unlock()

	if urlToUpdate, ok := taggedURLs[id]; ok {
		urlToUpdate.Tag = reqBody.Tag
		taggedURLs[id] = urlToUpdate
	} else {
		http.NotFound(w, r)
		return
	}

	if err := saveTaggedURLs(); err != nil {
		log.Printf("Error saving tagged URLs: %v", err)
		http.Error(w, "Failed to save URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteTaggedURL(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/tagged-urls/")

	taggedURLsMutex.Lock()
	defer taggedURLsMutex.Unlock()

	if _, ok := taggedURLs[id]; ok {
		delete(taggedURLs, id)
	} else {
		http.NotFound(w, r)
		return
	}

	if err := saveTaggedURLs(); err != nil {
		log.Printf("Error saving tagged URLs: %v", err)
		http.Error(w, "Failed to save URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().(*net.TCPAddr).IP.String()
	log.Printf("Accepted connection from %s", remoteAddr)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message := scanner.Text()
		parts := strings.Split(message, " ")
		command := parts[0]

		mutex.Lock()
		switch command {
		case "REGISTER":
			if len(parts) < 2 {
				log.Printf("Invalid REGISTER command from %s", remoteAddr)
				break
			}
			apiPort := parts[1]
			instanceAddress := net.JoinHostPort(remoteAddr, apiPort)
			instances[instanceAddress] = Instance{
				Address:  instanceAddress,
				LastSeen: time.Now(),
			}
			log.Printf("Registered instance: %s", instanceAddress)
			response := fmt.Sprintf("OK %s\n", remoteAddr)
			if _, err := conn.Write([]byte(response)); err != nil {
				log.Printf("Error writing to %s: %v", remoteAddr, err)
				mutex.Unlock()
				return
			}

		case "LIST":
			var activeInstances []Instance
			for _, instance := range instances {
				activeInstances = append(activeInstances, instance)
			}
			mutex.Unlock() // Unlock early

			data, err := json.Marshal(activeInstances)
			if err != nil {
				log.Printf("Failed to marshal instance list: %v", err)
				continue // Skip to next loop iteration
			}
			if _, err := conn.Write(data); err != nil {
				log.Printf("Error writing to %s: %v", remoteAddr, err)
				return // Exit function
			}
			if _, err := conn.Write([]byte("\n")); err != nil {
				log.Printf("Error writing to %s: %v", remoteAddr, err)
				return // Exit function
			}
			continue // Continue to next loop iteration to avoid double-unlock

		case "HEARTBEAT":
			if len(parts) < 2 {
				log.Printf("Invalid HEARTBEAT command from %s", remoteAddr)
				break
			}
			apiPort := parts[1]
			instanceAddress := net.JoinHostPort(remoteAddr, apiPort)
			if _, ok := instances[instanceAddress]; ok {
				instances[instanceAddress] = Instance{
					Address:  instanceAddress,
					LastSeen: time.Now(),
				}
				log.Printf("Heartbeat from: %s", instanceAddress)
				if _, err := conn.Write([]byte("OK\n")); err != nil {
					log.Printf("Error writing to %s: %v", remoteAddr, err)
					mutex.Unlock()
					return
				}
			} else {
				log.Printf("Heartbeat from unregistered instance: %s", instanceAddress)
				if _, err := conn.Write([]byte("ERR_NOT_REGISTERED\n")); err != nil {
					log.Printf("Error writing to %s: %v", remoteAddr, err)
					mutex.Unlock()
					return
				}
			}

		default:
			log.Printf("Unknown command from %s: %s", remoteAddr, command)
		}
		mutex.Unlock()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from connection from %s: %v", remoteAddr, err)
	}
	log.Printf("Connection from %s closed", remoteAddr)
}

func cleanupStaleInstances() {
	for {
		time.Sleep(cleanupInterval)
		mutex.Lock()
		log.Println("Running cleanup of stale instances...")
		for addr, instance := range instances {
			if time.Since(instance.LastSeen) > staleTimeout {
				log.Printf("Removing stale instance: %s", addr)
				delete(instances, addr)
			}
		}
		mutex.Unlock()
	}
}

func startAutoProxyCreator() {
	// Initial delay to allow instances to register
	time.Sleep(30 * time.Second)

	for {
		log.Println("Running auto-proxy creator...")

		taggedURLsMutex.Lock()
		urlsToCheck := make([]TaggedURL, 0, len(taggedURLs))
		for _, u := range taggedURLs {
			urlsToCheck = append(urlsToCheck, u)
		}
		taggedURLsMutex.Unlock()

		// This is a simplified version. A more robust implementation would be needed here.
		// For now, we just log the intent.
		for _, u := range urlsToCheck {
			log.Printf("Auto-proxy check for: %s (%s)", u.Tag, u.URL)
			// In a real implementation, you would get all active proxies,
			// check if a proxy for u.URL exists, and if not, call the create-proxy logic.
		}

		time.Sleep(10 * time.Minute)
	}
}
