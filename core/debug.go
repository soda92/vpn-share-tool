package core

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

var db *bbolt.DB

const (
	maxCapturedRequests = 1000
	sessionsMetadataBucket = "sessions_metadata"
)

func InitDB(dbPath string) error {
	var err error
	db, err = bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return err
	}

	return db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(sessionsMetadataBucket))
		return err
	})
}

//go:embed frontend/dist
var debugFrontend embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections
	},
}

var wsClients = make(map[*websocket.Conn]bool)
var wsMutex = &sync.Mutex{}

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

func handleDebugWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to websocket: %v", err)
		return
	}
	defer conn.Close()

	wsMutex.Lock()
	wsClients[conn] = true
	wsMutex.Unlock()

	// Keep the connection open
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
	}

	wsMutex.Lock()
	delete(wsClients, conn)
	wsMutex.Unlock()
}

// CaptureRequest captures the request and response for debugging.
func CaptureRequest(req *http.Request, resp *http.Response, reqBody, respBody []byte) {
	capturedRequestsLock.Lock()
	defer capturedRequestsLock.Unlock()

	nextRequestID++

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

	capturedRequests[captureHead] = cr
	captureHead = (captureHead + 1) % maxCapturedRequests

	// Broadcast the new request to all WebSocket clients
	wsMutex.Lock()
	defer wsMutex.Unlock()
	for client := range wsClients {
		if err := client.WriteJSON(cr); err != nil {
			log.Printf("Error writing to websocket client: %v", err)
			client.Close()
			delete(wsClients, client)
		}
	}
}

func handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listSessions(w, r)
	case http.MethodPost:
		createSession(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getSessionRequests(w, r)
	case http.MethodPut:
		updateSession(w, r)
	case http.MethodDelete:
		deleteSession(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleLiveRequests(w http.ResponseWriter, r *http.Request) {
	// This handler will be used for the live view
}

func listSessions(w http.ResponseWriter, r *http.Request) {
	var sessions []map[string]string
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionsMetadataBucket))
		return b.ForEach(func(k, v []byte) error {
			sessions = append(sessions, map[string]string{"id": string(k), "name": string(v)})
			return nil
		})
	})

	if err != nil {
		http.Error(w, "Failed to list sessions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func createSession(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil || reqBody.Name == "" {
		http.Error(w, "Session name is required", http.StatusBadRequest)
		return
	}

	sessionID := uuid.New().String()

	err := db.Update(func(tx *bbolt.Tx) error {
		// Create bucket for the session
		_, err := tx.CreateBucket([]byte(sessionID))
		if err != nil {
			return err
		}

		// Add to metadata
		metaBucket := tx.Bucket([]byte(sessionsMetadataBucket))
		return metaBucket.Put([]byte(sessionID), []byte(reqBody.Name))
	})

	if err != nil {
		log.Printf("Error creating session: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Now, copy live requests to the new session bucket
	capturedRequestsLock.RLock()
	defer capturedRequestsLock.RUnlock()

	err = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionID))
		for _, req := range capturedRequests {
			if req != nil {
				jsonReq, _ := json.Marshal(req)
				if err := b.Put([]byte(strconv.FormatInt(req.ID, 10)), jsonReq); err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Error saving requests to session: %v", err)
		// Don't block the user, session is created, but requests might be missing
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": sessionID, "name": reqBody.Name})
}

func getSessionRequests(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/debug/sessions/")
	sessionID = strings.TrimSuffix(sessionID, "/requests")

	var requests []*CapturedRequest
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionID))
		if b == nil {
			return fmt.Errorf("session not found")
		}
		return b.ForEach(func(k, v []byte) error {
			var req CapturedRequest
			if err := json.Unmarshal(v, &req); err == nil {
				requests = append(requests, &req)
			}
			return nil
		})
	})

	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

func updateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/debug/sessions/")
	var reqBody struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil || reqBody.Name == "" {
		http.Error(w, "Session name is required", http.StatusBadRequest)
		return
	}

	err := db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionsMetadataBucket))
		// Check if session exists before renaming
		if b.Get([]byte(sessionID)) == nil {
			return fmt.Errorf("not found")
		}
		return b.Put([]byte(sessionID), []byte(reqBody.Name))
	})

	if err != nil {
		if err.Error() == "not found" {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Failed to rename session", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

func deleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/debug/sessions/")
	err := db.Update(func(tx *bbolt.Tx) error {
		metaBucket := tx.Bucket([]byte(sessionsMetadataBucket))
		if err := metaBucket.Delete([]byte(sessionID)); err != nil {
			return err
		}
		return tx.DeleteBucket([]byte(sessionID))
	})

	if err != nil {
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func handleSingleRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getSingleRequest(w, r)
	case http.MethodPut:
		updateSingleRequest(w, r)
	case http.MethodDelete:
		deleteSingleRequest(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func updateSingleRequest(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/debug/requests/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		var bucketName, reqData []byte
		// Find the request and its bucket
		tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			if string(name) == sessionsMetadataBucket { return nil }
			data := b.Get([]byte(strconv.FormatInt(id, 10)))
			if data != nil {
				bucketName = name
				reqData = data
				return fmt.Errorf("found")
			}
			return nil
		})

		if bucketName == nil {
			return fmt.Errorf("not found")
		}

		var req CapturedRequest
		if err := json.Unmarshal(reqData, &req); err != nil {
			return err
		}

		// Apply updates
		if bookmarked, ok := updates["bookmarked"].(bool); ok {
			req.Bookmarked = bookmarked
		}
		if note, ok := updates["note"].(string); ok {
			req.Note = note
		}

		// Save back to the database
		updatedData, err := json.Marshal(req)
		if err != nil {
			return err
		}
		b := tx.Bucket(bucketName)
		return b.Put([]byte(strconv.FormatInt(id, 10)), updatedData)
	})

	if err != nil {
		if err.Error() == "not found" {
			http.NotFound(w, r)
		} else {
			log.Printf("Error updating request: %v", err)
			http.Error(w, "Failed to update request", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

func deleteSingleRequest(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/debug/requests/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		var bucketName []byte
		// Find the request's bucket
		tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			if string(name) == sessionsMetadataBucket { return nil }
			if b.Get([]byte(strconv.FormatInt(id, 10))) != nil {
				bucketName = name
				return fmt.Errorf("found")
			}
			return nil
		})

		if bucketName == nil {
			return fmt.Errorf("not found")
		}

		b := tx.Bucket(bucketName)
		return b.Delete([]byte(strconv.FormatInt(id, 10)))
	})

	if err != nil {
		if err.Error() == "not found" {
			http.NotFound(w, r)
		} else {
			log.Printf("Error deleting request: %v", err)
			http.Error(w, "Failed to delete request", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
	idStr := strings.TrimPrefix(r.URL.Path, "/debug/requests/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	var foundRequest *CapturedRequest
	err = db.View(func(tx *bbolt.Tx) error {
		// Iterate over all session buckets to find the request
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			if string(name) == sessionsMetadataBucket { // Skip metadata bucket
				return nil
			}
			data := b.Get([]byte(strconv.FormatInt(id, 10)))
			if data != nil {
				var req CapturedRequest
				if err := json.Unmarshal(data, &req); err == nil {
					foundRequest = &req
				}
				return fmt.Errorf("found") // Stop iteration
			}
			return nil
		})
	})

	if err != nil && err.Error() != "found" {
		log.Printf("Error searching for single request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if foundRequest == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(foundRequest)
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
