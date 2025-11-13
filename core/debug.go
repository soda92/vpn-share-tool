package core

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

const (
	maxCapturedRequests   = 1000
	liveSessionBucketName = "live_session"
)

//go:embed frontend/dist
var debugFrontend embed.FS

// CapturedRequest holds details of an intercepted HTTP request and its response.
type CapturedRequest struct {
	ID               int64       `json:"id"`
	Timestamp        time.Time   `json:"timestamp"`
	Method           string      `json:"method"`
	URL              string      `json:"url"`
	RequestHeaders   http.Header `json:"request_headers"`
	RequestBody      string      `json:"request_body"`
	ResponseStatus   int         `json:"response_status"`
	ResponseHeaders  http.Header `json:"response_headers"`
	ResponseBody     string      `json:"response_body"`
	Bookmarked       bool        `json:"bookmarked"`
	Note             string      `json:"note"`
	VpnShareToolMeta string      `json:"_vpnShareToolMetadata,omitempty"` // Field for HAR metadata
}

var (
	nextRequestID int64
	requestIDLock sync.Mutex
)

// RegisterDebugRoutes registers the debug UI and API routes.
func RegisterDebugRoutes(mux *http.ServeMux) {
	if err := InitDB("debug_requests.db"); err != nil {
		log.Fatalf("Failed to initialize debug database: %v", err)
	}

	// Serve the embedded frontend for the /debug/ path
	debugFS, err := fs.Sub(debugFrontend, "frontend/dist")
	if err != nil {
		log.Fatalf("Failed to create sub-filesystem for debug frontend: %v", err)
	}

	fileServer := http.FileServer(http.FS(debugFS))
	mux.Handle("/debug/", http.StripPrefix("/debug/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If the path doesn't contain a dot, it's a route, so serve index.html.
		if !strings.Contains(r.URL.Path, ".") {
			index, err := debugFS.Open("index.html")
			if err != nil {
				log.Printf("Failed to open index.html from embedded fs: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			io.Copy(w, index)
			return
		}
		// Otherwise, it's a file, so serve it.
		fileServer.ServeHTTP(w, r)
	})))

	// Add debug API endpoints
	mux.HandleFunc("/debug/sessions", handleSessions)
	mux.HandleFunc("/debug/sessions/", handleSessionOrHar)
	mux.HandleFunc("/debug/har/import", importHar)
	mux.HandleFunc("/debug/clear-live", handleClearLiveRequests)
	mux.HandleFunc("/debug/ws", handleDebugWS)
	mux.HandleFunc("/api/debug/requests/", handleSingleRequest) // New unambiguous endpoint

	log.Println("Debug UI registered at /debug/")
}

func handleSessionOrHar(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/har") {
		exportHar(w, r)
	} else {
		handleSession(w, r)
	}
}

// CaptureRequest captures the request and response for debugging.
func CaptureRequest(req *http.Request, resp *http.Response, reqBody, respBody []byte) {
	requestIDLock.Lock()
	nextRequestID++
	cr := &CapturedRequest{
		ID:              nextRequestID,
		Timestamp:       time.Now(),
		Method:          req.Method,
		URL:             req.URL.String(),
		RequestHeaders:  req.Header,
		RequestBody:     string(reqBody),
		ResponseStatus:  resp.StatusCode,
		ResponseHeaders: resp.Header,
		ResponseBody:    string(respBody),
	}
	requestIDLock.Unlock()

	err := db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(liveSessionBucketName))

		// Enforce request limit
		if b.Stats().KeyN >= maxCapturedRequests {
			c := b.Cursor()
			// Iterate and delete the oldest non-essential requests
			for k, v := c.First(); k != nil; k, v = c.Next() {
				var tempReq CapturedRequest
				if json.Unmarshal(v, &tempReq) == nil {
					if !tempReq.Bookmarked && tempReq.Note == "" {
						b.Delete(k)
						// Check if we are now under the limit
						if b.Stats().KeyN < maxCapturedRequests {
							break
						}
					}
				}
			}
		}

		// Save the new request
		jsonReq, _ := json.Marshal(cr)
		return b.Put([]byte(strconv.FormatInt(cr.ID, 10)), jsonReq)
	})

	if err != nil {
		log.Printf("Error capturing request to DB: %v", err)
		return
	}

	wsBroadCast(cr)
}

func handleClearLiveRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(liveSessionBucketName))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var req CapturedRequest
			if json.Unmarshal(v, &req) == nil {
				if !req.Bookmarked && req.Note == "" {
					b.Delete(k)
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Error clearing live requests: %v", err)
		http.Error(w, "Failed to clear requests", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Non-essential requests cleared"))
}
