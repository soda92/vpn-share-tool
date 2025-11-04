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
)


const (
	maxCapturedRequests    = 1000
)


//go:embed frontend/dist
var debugFrontend embed.FS


// CapturedRequest holds details of an intercepted HTTP request and its response.
type CapturedRequest struct {
	ID              int64       `json:"id"`
	Timestamp       time.Time   `json:"timestamp"`
	Method          string      `json:"method"`
	URL             string      `json:"url"`
	RequestHeaders  http.Header `json:"request_headers"`
	RequestBody     string      `json:"request_body"`
	ResponseStatus  int         `json:"response_status"`
	ResponseHeaders http.Header `json:"response_headers"`
	ResponseBody    string      `json:"response_body"`
}

var (
	capturedRequests     [maxCapturedRequests]*CapturedRequest
	capturedRequestsLock sync.RWMutex
	nextRequestID        int64
	captureHead          int
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
	mux.HandleFunc("/debug/sessions/", handleSession)
	mux.HandleFunc("/debug/live-requests", handleLiveRequests)
	mux.HandleFunc("/debug/clear", handleClearRequests)
	mux.HandleFunc("/debug/ws", handleDebugWS)
	mux.HandleFunc("/debug/requests/", handleSingleRequest)
	mux.HandleFunc("/debug/share-request", handleShareRequest)

	log.Println("Debug UI registered at /debug/")
}



// CaptureRequest captures the request and response for debugging.
func CaptureRequest(req *http.Request, resp *http.Response, reqBody, respBody []byte) {
	capturedRequestsLock.Lock()
	defer capturedRequestsLock.Unlock()

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

	capturedRequests[captureHead] = cr
	captureHead = (captureHead + 1) % maxCapturedRequests

	wsBroadCast(cr)
}

func handleLiveRequests(w http.ResponseWriter, r *http.Request) {
	// This handler will be used for the live view
}


func handleClearRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	capturedRequestsLock.Lock()
	defer capturedRequestsLock.Unlock()

	for i := range capturedRequests {
		capturedRequests[i] = nil
	}
	captureHead = 0
	nextRequestID = 0
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("History cleared"))
}


func handleShareRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CapturedRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Assign a new ID to ensure uniqueness for shared requests
	// This ID will be used for the shareable URL
	req.ID = time.Now().UnixNano() // Use nanoseconds for a highly unique ID

	// Save to a special "shared_requests" bucket
	sharedBucketName := "shared_requests"
	err := db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(sharedBucketName))
		if err != nil {
			return err
		}
		jsonReq, err := json.Marshal(req)
		if err != nil {
			return err
		}
		return b.Put([]byte(strconv.FormatInt(req.ID, 10)), jsonReq)
	})

	if err != nil {
		log.Printf("Error saving shared request: %v", err)
		http.Error(w, "Failed to save request for sharing", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"id": req.ID})
}
