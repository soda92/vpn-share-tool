package archive

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/soda92/vpn-share-tool/core/debug"
	"go.etcd.io/bbolt"
)

// ServeStandalone spins up a web server at the root of a port to host a specific archived session
func ServeStandalone(sessionID string, port int) error {
	// 1. Initialize DB if not already initialized
	db := debug.GetDB()
	if db == nil {
		dbPath := "debug_requests.db"
		if home, err := os.UserHomeDir(); err == nil {
			dbPath = filepath.Join(home, ".vpn-share-tool", "debug_requests.db")
		}
		log.Printf("[Archive Standalone] Opening database at %s", dbPath)
		if err := debug.InitDB(dbPath); err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		db = debug.GetDB()
	}

	// 2. Lookup session metadata to get original ProxyURL
	var meta SessionMetadata
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(metadataBucketName))
		if b == nil {
			return fmt.Errorf("no archive sessions found")
		}
		data := b.Get([]byte(sessionID))
		if data == nil {
			return fmt.Errorf("session %s not found", sessionID)
		}
		return json.Unmarshal(data, &meta)
	})
	if err != nil {
		return err
	}

	log.Printf("[Archive Standalone] Serving archived site '%s' (original URL: %s)", meta.Name, meta.ProxyURL)
	log.Printf("[Archive Standalone] Listening on http://localhost:%d", port)

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Resolve URL relative to the original domain
			var targetURL string
			base, err := url.Parse(meta.ProxyURL)
			if err != nil {
				targetURL = meta.ProxyURL + r.URL.Path
				if r.URL.RawQuery != "" {
					targetURL += "?" + r.URL.RawQuery
				}
			} else {
				targetURL = base.ResolveReference(r.URL).String()
			}

			// We query using current timestamp to always retrieve the latest snapshot available
			targetTimestamp := time.Now().UnixNano()

			resp, err := FindResource(sessionID, targetURL, targetTimestamp)
			if err != nil {
				log.Printf("[Archive Standalone] Resource not found: %s. Error: %v", targetURL, err)
				serveCustom404(w, targetURL, sessionID, targetTimestamp)
				return
			}

			// Set original Headers
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

			// Inject wayback banner for HTML files
			if strings.Contains(contentType, "text/html") {
				bodyStr := string(body)

				formattedTime := resp.Timestamp.Format("2006-01-02 15:04:05")
				bannerHTML := fmt.Sprintf(`
<div id="wayback-banner" style="position:fixed; top:0; left:0; width:100%%; height:45px; background:#f4f4f9; border-bottom:2px solid #673ab7; z-index:2147483647; display:flex; align-items:center; justify-content:space-between; padding:0 20px; font-family:sans-serif; font-size:14px; color:#333; box-shadow:0 2px 5px rgba(0,0,0,0.15); box-sizing:border-box;">
	<div><strong>VPN Archive Playback (Standalone)</strong> | Site: <span style="font-weight:bold;">%s</span> | Snapshot Date: <span style="color:#673ab7; font-weight:bold;">%s</span></div>
	<div>
		<button onclick="document.getElementById('wayback-banner').style.display='none'" style="background:none; border:none; color:#999; font-weight:bold; cursor:pointer; font-size:18px; line-height:1;">&times;</button>
	</div>
</div>
<div style="height: 45px; width: 100%%; display: block; box-sizing: border-box;"></div>
`, meta.Name, formattedTime)

				// Locate <body> or insert at start
				if idx := strings.Index(bodyStr, "<body>"); idx != -1 {
					bodyStr = bodyStr[:idx+6] + bannerHTML + bodyStr[idx+6:]
				} else {
					bodyStr = bannerHTML + bodyStr
				}
				body = []byte(bodyStr)
			}

			w.Write(body)
		}),
	}

	return server.ListenAndServe()
}
