package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/bbolt"
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

func main() {
	fmt.Println("==================================================")
	fmt.Println("          VPN Share Tool Archive Viewer")
	fmt.Println("==================================================")

	exePath, err := os.Executable()
	var currentDir string
	if err == nil {
		currentDir = filepath.Dir(exePath)
	} else {
		currentDir = "."
	}

	dbPath := filepath.Join(currentDir, "archive.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// Fallback to checking local folder directly
		dbPath = "archive.db"
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			log.Fatalf("Database file not found: archive.db. Please ensure archive.db is in the same directory as this viewer.")
		}
	}

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var meta SessionMetadata
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("archive_sessions_metadata"))
		if b == nil {
			return fmt.Errorf("no archive metadata bucket found")
		}
		c := b.Cursor()
		k, v := c.First()
		if k == nil {
			return fmt.Errorf("no archive session metadata found")
		}
		return json.Unmarshal(v, &meta)
	})
	if err != nil {
		log.Fatalf("Failed to read archive metadata: %v", err)
	}

	fmt.Printf("Loading Archive: %s\n", meta.Name)
	fmt.Printf("Original URL:    %s\n", meta.ProxyURL)

	// Bind to a random free port on local loopback
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to bind to local port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	playbackURL := fmt.Sprintf("http://%s", addr)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve index or resolve URL relative to original domain
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

		targetTimestamp := time.Now().UnixNano()

		resp, err := FindResource(db, meta.ID, targetURL, targetTimestamp)
		if err != nil {
			log.Printf("Resource not found: %s", targetURL)
			serveCustom404(w, targetURL, meta.ID, targetTimestamp)
			return
		}

		for k, values := range resp.ResponseHeaders {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(resp.ResponseStatus)

		var body []byte
		if resp.IsBase64 {
			body, _ = base64.StdEncoding.DecodeString(resp.ResponseBody)
		} else {
			body = []byte(resp.ResponseBody)
		}

		contentType := strings.ToLower(resp.ResponseHeaders.Get("Content-Type"))

		if strings.Contains(contentType, "text/html") {
			bodyStr := string(body)
			formattedTime := resp.Timestamp.Format("2006-01-02 15:04:05")
			bannerHTML := fmt.Sprintf(`
<div id="wayback-banner" style="position:fixed; top:0; left:0; width:100%%; height:45px; background:#f4f4f9; border-bottom:2px solid #673ab7; z-index:2147483647; display:flex; align-items:center; justify-content:space-between; padding:0 20px; font-family:sans-serif; font-size:14px; color:#333; box-shadow:0 2px 5px rgba(0,0,0,0.15); box-sizing:border-box;">
	<div><strong>VPN Archive Playback (Offline Viewer)</strong> | Site: <span style="font-weight:bold;">%s</span> | Snapshot Date: <span style="color:#673ab7; font-weight:bold;">%s</span></div>
	<div>
		<button onclick="document.getElementById('wayback-banner').style.display='none'" style="background:none; border:none; color:#999; font-weight:bold; cursor:pointer; font-size:18px; line-height:1;">&times;</button>
	</div>
</div>
<div style="height: 45px; width: 100%%; display: block; box-sizing: border-box;"></div>
`, meta.Name, formattedTime)

			if idx := strings.Index(bodyStr, "<body>"); idx != -1 {
				bodyStr = bodyStr[:idx+6] + bannerHTML + bodyStr[idx+6:]
			} else {
				bodyStr = bannerHTML + bodyStr
			}
			body = []byte(bodyStr)
		}

		w.Write(body)
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("Starting local web server at %s...\n", playbackURL)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to run: %v", err)
		}
	}()

	time.Sleep(300 * time.Millisecond)
	openBrowser(playbackURL)

	fmt.Println("\nServer running. Press Ctrl+C to close and exit...")
	select {}
}

func FindResource(db *bbolt.DB, sessionID string, targetURL string, targetTimestamp int64) (*ArchivedResponse, error) {
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

		if closestKey == nil {
			strippedTarget := stripQuery(targetURL)
			for k, v := c.Seek([]byte(strippedTarget)); k != nil; k, v = c.Next() {
				keyStr := string(k)
				parts := strings.Split(keyStr, "#")
				if len(parts) < 2 {
					continue
				}
				keyURL := parts[0]

				if stripQuery(keyURL) != strippedTarget {
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
			return fmt.Errorf("not found")
		}
		closestValCopy := make([]byte, len(closestVal))
		copy(closestValCopy, closestVal)
		match = closestValCopy
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

func stripQuery(urlStr string) string {
	if idx := strings.Index(urlStr, "?"); idx != -1 {
		return urlStr[:idx]
	}
	return urlStr
}

func serveCustom404(w http.ResponseWriter, urlStr string, sessionID string, timestamp int64) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)

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
	</style>
</head>
<body>
	<div class="card">
		<h1>Offline Archive: Resource Not Found</h1>
		<p>The following resource was not captured during the recording session:</p>
		<p><code>%%s</code></p>
		<p>To record it, make sure to browse/trigger this specific page or AJAX endpoint while the recording session is running.</p>
	</div>
</body>
</html>
`, urlStr)

	w.Write([]byte(html))
}

func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // Linux
		cmd = "xdg-open"
		args = []string{url}
	}

	exec.Command(cmd, args...).Start()
}
