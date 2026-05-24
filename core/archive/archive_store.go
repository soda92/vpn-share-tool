package archive

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/models"
	"go.etcd.io/bbolt"
)

const (
	metadataBucketName = "archive_sessions_metadata"
)

type SessionMetadata struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	ProxyURL  string    `json:"proxy_url"`
}

type ArchivedResponse struct {
	Timestamp       time.Time           `json:"timestamp"`
	Method          string              `json:"method"`
	URL             string              `json:"url"`
	RequestHeaders  http.Header         `json:"request_headers"`
	RequestBody     string              `json:"request_body"`
	ResponseStatus  int                 `json:"response_status"`
	ResponseHeaders http.Header         `json:"response_headers"`
	ResponseBody    string              `json:"response_body"`
	IsBase64        bool                `json:"is_base64"`
}

var (
	// Map to track active recording proxies and synchronize accesses
	recordingProxies     = make(map[int]string)
	recordingProxiesLock sync.Mutex

	// Track assets already pre-fetched in active session
	prefetchedURLs sync.Map
)

// StartSession initializes a recording session for a proxy port
func StartSession(p *models.SharedProxy, name string) (string, error) {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	if p.IsRecording {
		return p.ActiveSessionID, fmt.Errorf("proxy is already recording session %s", p.ActiveSessionID)
	}

	db := debug.GetDB()
	if db == nil {
		return "", fmt.Errorf("debug database not initialized")
	}

	sessionID := uuid.New().String()

	err := db.Update(func(tx *bbolt.Tx) error {
		// Create the metadata bucket if it doesn't exist
		metaBucket, err := tx.CreateBucketIfNotExists([]byte(metadataBucketName))
		if err != nil {
			return err
		}

		// Create a bucket for the session resources
		_, err = tx.CreateBucketIfNotExists([]byte("archive_session_" + sessionID))
		if err != nil {
			return err
		}

		// Save metadata
		meta := SessionMetadata{
			ID:        sessionID,
			Name:      name,
			CreatedAt: time.Now(),
			ProxyURL:  p.OriginalURL,
		}
		data, err := json.Marshal(meta)
		if err != nil {
			return err
		}

		return metaBucket.Put([]byte(sessionID), data)
	})

	if err != nil {
		return "", err
	}

	p.IsRecording = true
	p.ActiveSessionID = sessionID

	recordingProxiesLock.Lock()
	recordingProxies[p.RemotePort] = sessionID
	recordingProxiesLock.Unlock()

	log.Printf("Started archiving session '%s' (ID: %s) on port %d", name, sessionID, p.RemotePort)
	return sessionID, nil
}

// StopSession stops recording for a proxy port
func StopSession(p *models.SharedProxy) error {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	if !p.IsRecording {
		return fmt.Errorf("proxy is not currently recording")
	}

	sessionID := p.ActiveSessionID
	log.Printf("Stopped archiving session %s on port %d", sessionID, p.RemotePort)

	p.IsRecording = false
	p.ActiveSessionID = ""

	recordingProxiesLock.Lock()
	delete(recordingProxies, p.RemotePort)
	recordingProxiesLock.Unlock()

	// Clean up prefetchedURLs for this session
	prefetchedURLs.Range(func(key, value interface{}) bool {
		if kStr, ok := key.(string); ok && strings.HasPrefix(kStr, sessionID+"#") {
			prefetchedURLs.Delete(key)
		}
		return true
	})

	return nil
}

// Record captures the HTTP request and response into the active session bucket
func Record(p *models.SharedProxy, req *http.Request, resp *http.Response, reqBody, respBody []byte) {
	p.Mu.RLock()
	isRecording := p.IsRecording
	sessionID := p.ActiveSessionID
	p.Mu.RUnlock()

	if !isRecording || sessionID == "" {
		return
	}

	// 1. Filter out videos based on Content-Type or Extension
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "video/") {
		log.Printf("[Archive] Skipping video content-type: %s", contentType)
		return
	}

	urlPath := strings.ToLower(req.URL.Path)
	ext := filepath.Ext(urlPath)
	videoExts := map[string]bool{
		".mp4":  true,
		".webm": true,
		".mov":  true,
		".mkv":  true,
		".avi":  true,
		".flv":  true,
		".wmv":  true,
		".m4v":  true,
		".3gp":  true,
	}
	if videoExts[ext] {
		log.Printf("[Archive] Skipping video file extension: %s", ext)
		return
	}

	// Capture response details
	method := req.Method
	urlStr := req.URL.String()

	reqHeaders := make(http.Header)
	for k, v := range req.Header {
		reqHeaders[k] = v
	}

	respStatus := resp.StatusCode
	respHeaders := make(http.Header)
	for k, v := range resp.Header {
		respHeaders[k] = v
	}

	// Copy body asynchronously to database
	go func() {
		saveResourceRecord(sessionID, method, urlStr, reqHeaders, reqBody, respStatus, respHeaders, respBody)

		// Proactively pre-fetch static assets to resolve browser cache issues
		if strings.Contains(contentType, "text/html") {
			go prefetchAssets(sessionID, req, respBody)
		} else if strings.Contains(contentType, "text/css") {
			go prefetchCSSAssets(sessionID, req, respBody)
		}
	}()
}

func saveResourceRecord(sessionID string, method string, urlStr string, reqHeaders http.Header, reqBody []byte, respStatus int, respHeaders http.Header, respBody []byte) {
	db := debug.GetDB()
	if db == nil {
		return
	}

	contentType := strings.ToLower(respHeaders.Get("Content-Type"))
	isBase64 := false
	var responseBody string

	if strings.HasPrefix(contentType, "image/") || !utf8.Valid(respBody) {
		responseBody = base64.StdEncoding.EncodeToString(respBody)
		isBase64 = true
	} else {
		responseBody = string(respBody)
	}

	timestamp := time.Now()
	entry := ArchivedResponse{
		Timestamp:       timestamp,
		Method:          method,
		URL:             urlStr,
		RequestHeaders:  reqHeaders,
		RequestBody:     string(reqBody),
		ResponseStatus:  respStatus,
		ResponseHeaders: respHeaders,
		ResponseBody:    responseBody,
		IsBase64:        isBase64,
	}

	err := db.Update(func(tx *bbolt.Tx) error {
		bucketName := "archive_session_" + sessionID
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("archive session bucket %s not found", bucketName)
		}

		// Key format: url + "#" + timestamp_nanoseconds
		key := urlStr + "#" + strconv.FormatInt(timestamp.UnixNano(), 10)
		value, err := json.Marshal(entry)
		if err != nil {
			return err
		}

		return b.Put([]byte(key), value)
	})

	if err != nil {
		log.Printf("[Archive] Error writing resource to DB: %v", err)
	} else {
		// Store in prefetchedURLs to prevent pre-fetching what the browser already requested
		prefetchedURLs.Store(sessionID+"#"+urlStr, true)
		log.Printf("[Archive] Recorded resource: %s %s (%d bytes)", method, urlStr, len(respBody))
	}
}

func prefetchAssets(sessionID string, origReq *http.Request, htmlBody []byte) {
	html := string(htmlBody)
	reSrc := regexp.MustCompile(`(?i)\bsrc=["']([^"']+)["']`)
	reHref := regexp.MustCompile(`(?i)\bhref=["']([^"']+)["']`)
	reSrcset := regexp.MustCompile(`(?i)\bsrcset=["']([^"']+)["']`)
	reUrl := regexp.MustCompile(`(?i)\burl\(\s*['"]?([^'")]+)['"]?\s*\)`)

	uniqueURLs := make(map[string]bool)

	// Check if it is a static asset by file extension (mainly for href link filtering)
	staticExtensions := map[string]bool{
		".css":   true,
		".js":    true,
		".mjs":   true,
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".gif":   true,
		".svg":   true,
		".ico":   true,
		".webp":  true,
		".woff":  true,
		".woff2": true,
		".ttf":   true,
		".otf":   true,
	}

	addURL := func(assetURLRaw string, checkExtension bool) {
		assetURLRaw = strings.TrimSpace(assetURLRaw)
		if assetURLRaw == "" || strings.HasPrefix(assetURLRaw, "#") || strings.HasPrefix(assetURLRaw, "data:") || strings.HasPrefix(assetURLRaw, "javascript:") {
			return
		}

		// Parse URL relative to origReq.URL
		resolved, err := origReq.URL.Parse(assetURLRaw)
		if err != nil {
			return
		}

		resolved.Fragment = ""
		resolvedURL := resolved.String()

		ext := strings.ToLower(filepath.Ext(resolved.Path))
		
		// Filter out video extensions to prevent DB bloating
		videoExts := map[string]bool{
			".mp4":  true,
			".webm": true,
			".mov":  true,
			".mkv":  true,
			".avi":  true,
			".flv":  true,
			".wmv":  true,
			".m4v":  true,
			".3gp":  true,
		}
		if videoExts[ext] {
			return
		}

		if checkExtension {
			if !staticExtensions[ext] {
				return
			}
		}

		uniqueURLs[resolvedURL] = true
	}

	// 1. Extract from src attributes
	if matches := reSrc.FindAllStringSubmatch(html, -1); matches != nil {
		for _, m := range matches {
			if len(m) >= 2 {
				addURL(m[1], false)
			}
		}
	}

	// 2. Extract from href attributes
	if matches := reHref.FindAllStringSubmatch(html, -1); matches != nil {
		for _, m := range matches {
			if len(m) >= 2 {
				addURL(m[1], true)
			}
		}
	}

	// 3. Extract from srcset attributes
	if matches := reSrcset.FindAllStringSubmatch(html, -1); matches != nil {
		for _, m := range matches {
			if len(m) >= 2 {
				parts := strings.Split(m[1], ",")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					subParts := strings.Fields(part)
					if len(subParts) > 0 {
						addURL(subParts[0], false)
					}
				}
			}
		}
	}

	// 4. Extract from url(...) in style blocks/attributes
	if matches := reUrl.FindAllStringSubmatch(html, -1); matches != nil {
		for _, m := range matches {
			if len(m) >= 2 {
				addURL(m[1], false)
			}
		}
	}

	if len(uniqueURLs) == 0 {
		return
	}

	log.Printf("[Archive Pre-fetch] Found %d static assets to pre-fetch for %s", len(uniqueURLs), origReq.URL.String())
	fetchAndRecordAssets(sessionID, origReq, uniqueURLs)
}

func prefetchCSSAssets(sessionID string, origReq *http.Request, cssBody []byte) {
	css := string(cssBody)
	reUrl := regexp.MustCompile(`(?i)\burl\(\s*['"]?([^'")]+)['"]?\s*\)`)

	uniqueURLs := make(map[string]bool)

	addURL := func(assetURLRaw string) {
		assetURLRaw = strings.TrimSpace(assetURLRaw)
		if assetURLRaw == "" || strings.HasPrefix(assetURLRaw, "#") || strings.HasPrefix(assetURLRaw, "data:") || strings.HasPrefix(assetURLRaw, "javascript:") {
			return
		}

		resolved, err := origReq.URL.Parse(assetURLRaw)
		if err != nil {
			return
		}

		resolved.Fragment = ""
		resolvedURL := resolved.String()

		ext := strings.ToLower(filepath.Ext(resolved.Path))
		
		// Filter out video extensions
		videoExts := map[string]bool{
			".mp4":  true,
			".webm": true,
			".mov":  true,
			".mkv":  true,
			".avi":  true,
			".flv":  true,
			".wmv":  true,
			".m4v":  true,
			".3gp":  true,
		}
		if videoExts[ext] {
			return
		}

		uniqueURLs[resolvedURL] = true
	}

	if matches := reUrl.FindAllStringSubmatch(css, -1); matches != nil {
		for _, m := range matches {
			if len(m) >= 2 {
				addURL(m[1])
			}
		}
	}

	if len(uniqueURLs) == 0 {
		return
	}

	log.Printf("[CSS Pre-fetch] Found %d assets to pre-fetch for %s", len(uniqueURLs), origReq.URL.String())
	fetchAndRecordAssets(sessionID, origReq, uniqueURLs)
}

func fetchAndRecordAssets(sessionID string, origReq *http.Request, uniqueURLs map[string]bool) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for assetURL := range uniqueURLs {
		// Dedup check using prefetchedURLs map
		key := sessionID + "#" + assetURL
		if _, loaded := prefetchedURLs.LoadOrStore(key, true); loaded {
			continue
		}

		go func(urlStr string) {
			req, err := http.NewRequest("GET", urlStr, nil)
			if err != nil {
				return
			}

			// Copy original request headers (cookies/session state)
			headersToCopy := []string{"Cookie", "User-Agent", "Authorization", "Accept-Language"}
			for _, h := range headersToCopy {
				if val := origReq.Header.Get(h); val != "" {
					req.Header.Set(h, val)
				}
			}
			// Set Referer to the parent request's URL
			req.Header.Set("Referer", origReq.URL.String())

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[Archive Pre-fetch] Failed to pre-fetch %s: %v", urlStr, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Printf("[Archive Pre-fetch] Non-OK status for %s: %d", urlStr, resp.StatusCode)
				return
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			// Record directly to active session database
			saveResourceRecord(sessionID, "GET", urlStr, req.Header, []byte(""), resp.StatusCode, resp.Header, bodyBytes)

			// Recursively pre-fetch if this asset is a CSS or HTML
			cType := strings.ToLower(resp.Header.Get("Content-Type"))
			if strings.Contains(cType, "text/html") {
				go prefetchAssets(sessionID, req, bodyBytes)
			} else if strings.Contains(cType, "text/css") {
				go prefetchCSSAssets(sessionID, req, bodyBytes)
			}
		}(assetURL)
	}
}

// ListSessions returns a list of all saved archive sessions
func ListSessions() ([]SessionMetadata, error) {
	db := debug.GetDB()
	if db == nil {
		return nil, fmt.Errorf("debug database not initialized")
	}

	var list []SessionMetadata
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(metadataBucketName))
		if b == nil {
			return nil // No sessions created yet
		}

		return b.ForEach(func(k, v []byte) error {
			var meta SessionMetadata
			if err := json.Unmarshal(v, &meta); err == nil {
				list = append(list, meta)
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return list, nil
}

// DeleteSession deletes all data associated with a session
func DeleteSession(sessionID string) error {
	db := debug.GetDB()
	if db == nil {
		return fmt.Errorf("debug database not initialized")
	}

	// Clean up prefetchedURLs for this session
	prefetchedURLs.Range(func(key, value interface{}) bool {
		if kStr, ok := key.(string); ok && strings.HasPrefix(kStr, sessionID+"#") {
			prefetchedURLs.Delete(key)
		}
		return true
	})

	return db.Update(func(tx *bbolt.Tx) error {
		// 1. Remove from metadata bucket
		metaBucket := tx.Bucket([]byte(metadataBucketName))
		if metaBucket != nil {
			if err := metaBucket.Delete([]byte(sessionID)); err != nil {
				return err
			}
		}

		// 2. Delete the session bucket itself
		bucketName := "archive_session_" + sessionID
		if tx.Bucket([]byte(bucketName)) != nil {
			return tx.DeleteBucket([]byte(bucketName))
		}
		return nil
	})
}
