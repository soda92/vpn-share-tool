package core

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
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
	mux.Handle("/debug/", http.StripPrefix("/debug/", http.FileServer(http.FS(debugFS))))

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