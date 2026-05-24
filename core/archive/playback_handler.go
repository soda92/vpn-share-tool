package archive

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/models"
	"go.etcd.io/bbolt"
)

// RegisterArchiveRoutes registers archive-related API and playback routes
func RegisterArchiveRoutes(mux *http.ServeMux, getProxies func() []*models.SharedProxy) {
	mux.HandleFunc("/archive/sessions", handleArchiveSessions)
	mux.HandleFunc("/archive/sessions/", handleSingleArchiveSession)
	mux.HandleFunc("/archive/history", handleArchiveHistory)
	mux.HandleFunc("/archive/toggle-recording", func(w http.ResponseWriter, r *http.Request) {
		handleToggleRecording(w, r, getProxies)
	})
	// Capture all wildcard subpaths for playback and AJAX
	mux.HandleFunc("/archive/view/", handlePlaybackView)
	mux.HandleFunc("/archive/ajax/", handlePlaybackAjax)
	// Catch-all route to resolve absolute-path relative assets using Referer headers
	mux.HandleFunc("/", handlePlaybackFallback)
}

func handleArchiveSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessions, err := ListSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func handleSingleArchiveSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	sessionID := strings.TrimPrefix(r.URL.Path, "/archive/sessions/")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := DeleteSession(sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type ToggleReq struct {
	ProxyPort int    `json:"proxy_port"`
	Name      string `json:"name"`
	Enable    bool   `json:"enable"`
}

func handleToggleRecording(w http.ResponseWriter, r *http.Request, getProxies func() []*models.SharedProxy) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ToggleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	proxies := getProxies()
	var targetProxy *models.SharedProxy
	for _, p := range proxies {
		if p.RemotePort == req.ProxyPort {
			targetProxy = p
			break
		}
	}

	if targetProxy == nil {
		http.Error(w, fmt.Sprintf("Proxy on port %d not found", req.ProxyPort), http.StatusNotFound)
		return
	}

	var sessionID string
	var err error

	if req.Enable {
		sessionID, err = StartSession(targetProxy, req.Name)
	} else {
		err = StopSession(targetProxy)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"session_id": sessionID,
		"recording":  req.Enable,
	})
}

// Helper to parse playback paths: /archive/{type}/{sessionID}/{timestamp}/{originalURL}
func parsePlaybackPath(path string, query string, prefix string) (string, int64, string, error) {
	remaining := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(remaining, "/", 3)
	if len(parts) < 3 {
		return "", 0, "", fmt.Errorf("invalid playback path")
	}

	sessionID := parts[0]
	timestamp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid timestamp format: %w", err)
	}

	originalURL := parts[2]
	// Clean protocol collapsed slash (e.g. http:/some-site -> http://some-site)
	if strings.HasPrefix(originalURL, "http:/") && !strings.HasPrefix(originalURL, "http://") {
		originalURL = "http://" + strings.TrimPrefix(originalURL, "http:/")
	} else if strings.HasPrefix(originalURL, "https:/") && !strings.HasPrefix(originalURL, "https://") {
		originalURL = "https://" + strings.TrimPrefix(originalURL, "https:/")
	}

	// Reattach query params
	if query != "" {
		originalURL = originalURL + "?" + query
	}

	return sessionID, timestamp, originalURL, nil
}

func handlePlaybackView(w http.ResponseWriter, r *http.Request) {
	sessionID, timestamp, targetURL, err := parsePlaybackPath(r.URL.Path, r.URL.RawQuery, "/archive/view/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	servePlaybackResource(w, r, sessionID, timestamp, targetURL, true)
}

func handlePlaybackAjax(w http.ResponseWriter, r *http.Request) {
	sessionID, timestamp, targetURL, err := parsePlaybackPath(r.URL.Path, r.URL.RawQuery, "/archive/ajax/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	servePlaybackResource(w, r, sessionID, timestamp, targetURL, false)
}

func servePlaybackResource(w http.ResponseWriter, r *http.Request, sessionID string, targetTimestamp int64, targetURL string, injectBanner bool) {
	// Disable banner injection inside iframes/frames
	dest := strings.ToLower(r.Header.Get("Sec-Fetch-Dest"))
	if dest == "iframe" || dest == "frame" {
		injectBanner = false
	}

	resp, err := FindResource(sessionID, targetURL, targetTimestamp)
	if err != nil {
		log.Printf("[Playback] Resource not found: %s at %d. Error: %v", targetURL, targetTimestamp, err)
		if trySimulateRender(w, r, targetURL) {
			return
		}
		serveCustom404(w, targetURL, sessionID, targetTimestamp)
		return
	}

	// Set Headers
	for k, values := range resp.ResponseHeaders {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	// Inject CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.WriteHeader(resp.ResponseStatus)

	var body []byte
	if resp.IsBase64 {
		body, _ = base64.StdEncoding.DecodeString(resp.ResponseBody)
	} else {
		body = []byte(resp.ResponseBody)
	}

	contentType := strings.ToLower(resp.ResponseHeaders.Get("Content-Type"))

	// Inject Wayback Interceptor script & floating banner if HTML
	if strings.Contains(contentType, "text/html") {
		bodyStr := string(body)

		// 1. Script injection
		interceptorScript := fmt.Sprintf(`
<script>
// Playback AJAX interception script
(function() {
	const sessionID = "%s";
	const timestamp = "%d";
	const originalPageURL = "%s";

	const origFetch = window.fetch;
	window.fetch = function(input, init) {
		let url = typeof input === 'string' ? input : (input && input.url);
		if (url) {
			let targetURL = new URL(url, originalPageURL).href;
			if (!targetURL.includes('/archive/')) {
				url = "/archive/ajax/" + sessionID + "/" + timestamp + "/" + targetURL.replace("://", ":/");
			}
		}
		return origFetch(url, init);
	};

	const origOpen = XMLHttpRequest.prototype.open;
	XMLHttpRequest.prototype.open = function(method, url, ...args) {
		if (url) {
			let targetURL = new URL(url, originalPageURL).href;
			if (!targetURL.includes('/archive/')) {
				url = "/archive/ajax/" + sessionID + "/" + timestamp + "/" + targetURL.replace("://", ":/");
			}
		}
		return origOpen.call(this, method, url, ...args);
	};
})();
</script>
`, sessionID, targetTimestamp, targetURL)

		// Locate injection point (after head)
		if idx := strings.Index(bodyStr, "<head>"); idx != -1 {
			bodyStr = bodyStr[:idx+6] + interceptorScript + bodyStr[idx+6:]
		} else {
			bodyStr = interceptorScript + bodyStr
		}

		// 2. Banner Injection
		if injectBanner {
			formattedTime := time.Unix(0, targetTimestamp).Format("2006-01-02 15:04:05")
			bannerHTML := fmt.Sprintf(`
<div id="wayback-banner" style="position:fixed; top:0; left:0; width:100%%; height:45px; background:#f4f4f9; border-bottom:2px solid #673ab7; z-index:2147483647; display:flex; align-items:center; justify-content:space-between; padding:0 20px; font-family:sans-serif; font-size:14px; color:#333; box-shadow:0 2px 5px rgba(0,0,0,0.15); box-sizing:border-box;">
	<div><strong>VPN Archive Playback</strong> | Snapshot: <span style="color:#673ab7; font-weight:bold;">%s</span></div>
	<div>
		<a href="/debug/#/archives" target="_blank" style="color:#673ab7; text-decoration:none; font-weight:bold; margin-right:15px; font-size:13px;">View Dashboard</a>
		<button onclick="document.getElementById('wayback-banner').style.display='none'" style="background:none; border:none; color:#999; font-weight:bold; cursor:pointer; font-size:18px; line-height:1;">&times;</button>
	</div>
</div>
<div id="wayback-banner-spacer" style="height: 45px; width: 100%%; display: block; box-sizing: border-box;"></div>
<script>
if (window.self !== window.top) {
	const banner = document.getElementById('wayback-banner');
	const spacer = document.getElementById('wayback-banner-spacer');
	if (banner) banner.style.setProperty('display', 'none', 'important');
	if (spacer) spacer.style.setProperty('display', 'none', 'important');
}
</script>
`, formattedTime)

			// Locate <body> or insert at start
			if idx := strings.Index(bodyStr, "<body>"); idx != -1 {
				bodyStr = bodyStr[:idx+6] + bannerHTML + bodyStr[idx+6:]
			} else {
				bodyStr = bannerHTML + bodyStr
			}
		}

		body = []byte(bodyStr)
	}

	w.Write(body)
}

// FindResource scans bbolt keys to find the snapshot of targetURL closest to targetTimestamp
func FindResource(sessionID string, targetURL string, targetTimestamp int64) (*ArchivedResponse, error) {
	db := debug.GetDB()
	if db == nil {
		return nil, fmt.Errorf("debug database not initialized")
	}

	var match []byte
	err := db.View(func(tx *bbolt.Tx) error {
		bucketName := "archive_session_" + sessionID
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("archive session not found")
		}

		prefix := targetURL + "#"
		c := b.Cursor()

		var closestKey []byte
		var closestVal []byte
		minDiff := int64(math.MaxInt64)

		// Seek to prefix start
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			keyStr := string(k)
			parts := strings.Split(keyStr, "#")
			if len(parts) < 2 {
				continue
			}

			keyTime, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
			if err != nil {
				continue
			}

			diff := int64(math.Abs(float64(keyTime - targetTimestamp)))
			if diff < minDiff {
				minDiff = diff
				closestKey = k
				closestVal = v
			}
		}

		// If closestKey was not found, try fuzzy lookup by stripping query parameters
		if closestKey == nil {
			strippedTarget := stripQuery(targetURL)
			
			// Seek to the start of strippedTarget prefix
			for k, v := c.Seek([]byte(strippedTarget)); k != nil; k, v = c.Next() {
				keyStr := string(k)
				parts := strings.Split(keyStr, "#")
				if len(parts) < 2 {
					continue
				}
				keyURL := parts[0]
				
				// Check if this key matches strippedTarget when its query is stripped
				if stripQuery(keyURL) != strippedTarget {
					// Since keys are sorted, once we pass the prefix we can stop
					if !strings.HasPrefix(keyURL, strippedTarget) {
						break
					}
					continue
				}

				keyTime, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
				if err != nil {
					continue
				}

				diff := int64(math.Abs(float64(keyTime - targetTimestamp)))
				if diff < minDiff {
					minDiff = diff
					closestKey = k
					closestVal = v
				}
			}
		}

		if closestKey == nil {
			return fmt.Errorf("no archive matches found for URL: %s", targetURL)
		}

		match = closestVal
		return nil
	})

	if err != nil {
		return nil, err
	}

	var resp ArchivedResponse
	if err := json.Unmarshal(match, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func serveCustom404(w http.ResponseWriter, urlStr string, sessionID string, timestamp int64) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Access-Control-Allow-Origin", "*")

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<title>Resource Not Archived</title>
	<style>
		body { font-family: sans-serif; background-color: #f8f9fa; color: #333; text-align: center; padding: 50px; }
		.card { background: white; padding: 40px; border-radius: 8px; box-shadow: 0 4px 10px rgba(0,0,0,0.05); display: inline-block; max-width: 600px; border-top: 4px solid #673ab7; }
		h1 { color: #673ab7; font-size: 24px; margin-top: 0; }
		p { font-size: 15px; line-height: 1.5; color: #666; }
		code { background: #f1f3f5; padding: 4px 8px; border-radius: 4px; font-family: monospace; font-size: 14px; word-break: break-all; }
		.btn { display: inline-block; margin-top: 20px; padding: 10px 20px; background-color: #673ab7; color: white; text-decoration: none; border-radius: 4px; font-weight: bold; }
		.btn:hover { background-color: #512da8; }
	</style>
</head>
<body>
	<div class="card">
		<h1>Offline Archive: Resource Not Found</h1>
		<p>The following resource was not captured during the recording session:</p>
		<p><code>%s</code></p>
		<p>To record it, make sure to browse/trigger this specific page or AJAX endpoint while the recording session is running.</p>
		<a href="/debug/#/archives" class="btn">Back to Archives</a>
	</div>
</body>
</html>
`, urlStr)

	w.Write([]byte(html))
}

type SnapshotHistoryEntry struct {
	SessionID   string    `json:"session_id"`
	SessionName string    `json:"session_name"`
	Timestamp   int64     `json:"timestamp"`
	Formatted   string    `json:"formatted"`
	PlaybackURL string    `json:"playback_url"`
}

func handleArchiveHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "URL parameter is required", http.StatusBadRequest)
		return
	}

	db := debug.GetDB()
	if db == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var history []SnapshotHistoryEntry

	sessions, err := ListSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = db.View(func(tx *bbolt.Tx) error {
		for _, session := range sessions {
			bucketName := "archive_session_" + session.ID
			b := tx.Bucket([]byte(bucketName))
			if b == nil {
				continue
			}

			prefix := targetURL + "#"
			c := b.Cursor()
			for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
				parts := strings.Split(string(k), "#")
				if len(parts) < 2 {
					continue
				}
				ts, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
				if err != nil {
					continue
				}

				history = append(history, SnapshotHistoryEntry{
					SessionID:   session.ID,
					SessionName: session.Name,
					Timestamp:   ts,
					Formatted:   time.Unix(0, ts).Format("2006-01-02 15:04:05"),
					PlaybackURL: fmt.Sprintf("/archive/view/%s/%d/%s", session.ID, ts, strings.Replace(targetURL, "://", ":/", 1)),
				})
			}
		}
		return nil
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func handlePlaybackFallback(w http.ResponseWriter, r *http.Request) {
	referer := r.Header.Get("Referer")
	if referer != "" {
		// Try to parse referer to see if it is an archive page request
		refURL, err := url.Parse(referer)
		if err == nil {
			var prefix string
			if strings.Contains(refURL.Path, "/archive/view/") {
				prefix = "/archive/view/"
			} else if strings.Contains(refURL.Path, "/archive/ajax/") {
				prefix = "/archive/ajax/"
			}

			if prefix != "" {
				idx := strings.Index(refURL.Path, prefix)
				remaining := refURL.Path[idx+len(prefix):]
				parts := strings.SplitN(remaining, "/", 3)
				if len(parts) >= 3 {
					sessionID := parts[0]
					timestamp, err := strconv.ParseInt(parts[1], 10, 64)
					if err == nil {
						refererOriginalURL := parts[2]
						if strings.HasPrefix(refererOriginalURL, "http:/") && !strings.HasPrefix(refererOriginalURL, "http://") {
							refererOriginalURL = "http://" + strings.TrimPrefix(refererOriginalURL, "http:/")
						} else if strings.HasPrefix(refererOriginalURL, "https:/") && !strings.HasPrefix(refererOriginalURL, "https://") {
							refererOriginalURL = "https://" + strings.TrimPrefix(refererOriginalURL, "https:/")
						}

						// Parse the referer original URL
						refBase, err := url.Parse(refererOriginalURL)
						if err == nil {
							// Resolve the requested path relative to the referer's original base URL
							resolvedURL := refBase.ResolveReference(r.URL).String()
							log.Printf("[Playback Fallback] Resolved %s relative to referer %s -> %s", r.URL.String(), refererOriginalURL, resolvedURL)

							// If it's a page navigation request (Accept contains text/html), redirect the browser
							// to the proper /archive/view/... path to preserve the session and timestamp context.
							accept := strings.ToLower(r.Header.Get("Accept"))
							if strings.Contains(accept, "text/html") {
								redirectURL := fmt.Sprintf("/archive/view/%s/%d/%s", sessionID, timestamp, strings.Replace(resolvedURL, "://", ":/", 1))
								http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
								return
							}

							servePlaybackResource(w, r, sessionID, timestamp, resolvedURL, false)
							return
						}
					}
				}
			}
		}
	}

	http.NotFound(w, r)
}

func stripQuery(urlStr string) string {
	if idx := strings.Index(urlStr, "?"); idx != -1 {
		return urlStr[:idx]
	}
	return urlStr
}

func trySimulateRender(w http.ResponseWriter, r *http.Request, targetURL string) bool {
	// Parse URL path to check extension
	parsed, err := url.Parse(targetURL)
	var ext string
	if err == nil {
		ext = strings.ToLower(filepath.Ext(parsed.Path))
	}

	accept := strings.ToLower(r.Header.Get("Accept"))

	// 1. CSS
	if ext == ".css" || strings.Contains(accept, "text/css") {
		w.Header().Set("Content-Type", "text/css")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("/* simulated placeholder */"))
		return true
	}

	// 2. JavaScript
	if ext == ".js" || ext == ".mjs" || strings.Contains(accept, "javascript") {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("// simulated placeholder"))
		return true
	}

	// 3. Images (PNG, JPG, ICO, SVG, etc.)
	isImage := false
	imageExtensions := map[string]bool{
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".ico":  true,
		".svg":  true,
		".webp": true,
		".bmp":  true,
	}
	if imageExtensions[ext] || strings.Contains(accept, "image/") {
		isImage = true
	}

	if isImage {
		// 1x1 pixel transparent PNG base64
		const transparentPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
		imgBytes, _ := base64.StdEncoding.DecodeString(transparentPNG)
		
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		w.Write(imgBytes)
		return true
	}

	return false
}
