package core

import (
	"embed"
	"encoding/json"
	"io/fs"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed frontend/dist
var debugFrontend embed.FS

const maxCapturedRequests = 1000

// CapturedRequest holds details of an intercepted HTTP request and its response.
type CapturedRequest struct {
	ID           int64             `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	RequestHeaders  http.Header       `json:"request_headers"`
	RequestBody  string            `json:"request_body"`
	ResponseStatus int               `json:"response_status"`
	ResponseHeaders http.Header       `json:"response_headers"`
	ResponseBody string            `json:"response_body"`
}

var (
	capturedRequests     []*CapturedRequest
	capturedRequestsLock sync.RWMutex
	nextRequestID        int64
)

// RegisterDebugRoutes registers the debug UI and API routes.
func RegisterDebugRoutes(mux *http.ServeMux) {
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
	mux.HandleFunc("/debug/requests", handleDebugRequests)
	mux.HandleFunc("/debug/clear", handleClearRequests)

	log.Println("Debug UI registered at /debug/")
}

// CaptureRequest captures the request and response for debugging.
func CaptureRequest(req *http.Request, resp *http.Response, reqBody, respBody []byte) {
	capturedRequestsLock.Lock()
	defer capturedRequestsLock.Unlock()

	nextRequestID++
	if len(capturedRequests) >= maxCapturedRequests {
		// Remove the oldest request if the limit is reached
		capturedRequests = capturedRequests[1:]
	}

	cr := &CapturedRequest{
		ID:           nextRequestID,
		Timestamp:    time.Now(),
		Method:       req.Method,
		URL:          req.URL.String(),
		RequestHeaders:  req.Header,
		RequestBody:  string(reqBody),
		ResponseStatus: resp.StatusCode,
		ResponseHeaders: resp.Header,
		ResponseBody: string(respBody),
	}
	capturedRequests = append(capturedRequests, cr)
}

func handleDebugRequests(w http.ResponseWriter, r *http.Request) {
	capturedRequestsLock.RLock()
	defer capturedRequestsLock.RUnlock()

	// Check if an ID is present in the URL path, e.g., /debug/requests/123
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) > 3 && parts[3] != "" {
		idStr := parts[3]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid request ID", http.StatusBadRequest)
			return
		}

		for _, req := range capturedRequests {
			if req.ID == id {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(req); err != nil {
					log.Printf("Failed to encode captured request to JSON: %v", err)
					http.Error(w, "Failed to encode captured request", http.StatusInternalServerError)
				}
				return
			}
		}

		http.NotFound(w, r)
		return
	}

	// If no ID, return all requests
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(capturedRequests); err != nil {
		log.Printf("Failed to encode captured requests to JSON: %v", err)
		http.Error(w, "Failed to encode captured requests", http.StatusInternalServerError)
	}
}

func handleClearRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	capturedRequestsLock.Lock()
	defer capturedRequestsLock.Unlock()

	capturedRequests = []*CapturedRequest{}
	nextRequestID = 0
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("History cleared"))
}