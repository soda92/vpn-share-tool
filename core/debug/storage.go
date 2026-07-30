package debug

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

var db *bbolt.DB

func GetDB() *bbolt.DB {
	return db
}

const sessionsMetadataBucket = "sessions_metadata"

func InitDB(dbPath string) error {
	var err error
	db, err = bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return err
	}

	return db.Update(func(tx *bbolt.Tx) error {
		// Ensure metadata bucket exists
		_, err := tx.CreateBucketIfNotExists([]byte(sessionsMetadataBucket))
		if err != nil {
			return err
		}

		// Ensure live session bucket exists and clean it
		_, err = tx.CreateBucketIfNotExists([]byte(liveSessionBucketName))
		if err != nil {
			return err
		}

		// Initialize nextRequestID from the largest existing ID in any bucket
		nextRequestID = 0
		tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			b.ForEach(func(k, v []byte) error {
				id, _ := strconv.ParseInt(string(k), 10, 64)
				if id > nextRequestID {
					nextRequestID = id
				}
				return nil
			})
			return nil
		})

		return nil
	})
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
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/debug/sessions/")
	sessionID = strings.TrimPrefix(sessionID, "/debug/sessions/")
	sessionID = strings.TrimSuffix(sessionID, "/requests")

	switch r.Method {
	case http.MethodGet:
		getSessionRequests(w, r, sessionID)
	case http.MethodPut:
		updateSession(w, r, sessionID)
	case http.MethodDelete:
		deleteSession(w, r, sessionID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func listSessions(w http.ResponseWriter, _ *http.Request) {
	sessions := make([]map[string]string, 0)
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionsMetadataBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			// Do not list the internal live session
			if string(k) != liveSessionBucketName {
				sessions = append(sessions, map[string]string{"id": string(k), "name": string(v)})
			}
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
		// Create new bucket for the session
		destBucket, err := tx.CreateBucket([]byte(sessionID))
		if err != nil {
			return err
		}

		// Copy from live session bucket
		sourceBucket := tx.Bucket([]byte(liveSessionBucketName))
		err = sourceBucket.ForEach(func(k, v []byte) error {
			return destBucket.Put(k, v)
		})
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

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": sessionID, "name": reqBody.Name})
}

func getSessionRequests(w http.ResponseWriter, r *http.Request, sessionID string) {
	// Parse query parameters
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	search := strings.ToLower(r.URL.Query().Get("search"))
	hideErrors := r.URL.Query().Get("hide_errors") == "true"
	typesStr := r.URL.Query().Get("types")

	page := 0
	limit := 0
	var err error
	if pageStr != "" {
		if page, err = strconv.Atoi(pageStr); err != nil || page < 1 {
			page = 1
		}
	}
	if limitStr != "" {
		if limit, err = strconv.Atoi(limitStr); err != nil || limit < 1 {
			limit = 50
		}
	}

	var allowedTypes []string
	if typesStr != "" {
		allowedTypes = strings.Split(typesStr, ",")
	}

	var requests []*CapturedRequestSummary
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionID))
		if b == nil {
			return fmt.Errorf("session not found")
		}
		return b.ForEach(func(k, v []byte) error {
			var req CapturedRequestSummary
			if err := json.Unmarshal(v, &req); err == nil {
				// 1. Filter by Hide Errors
				if hideErrors && req.ResponseStatus >= 400 {
					return nil
				}

				// 2. Filter by Search Query
				if search != "" && !strings.Contains(strings.ToLower(req.URL), search) {
					return nil
				}

				// 3. Filter by Resource Types
				if len(allowedTypes) > 0 {
					matchedType := false
					reqType := getResourceType(&req)
					for _, t := range allowedTypes {
						if t == "ALL" || t == reqType {
							matchedType = true
							break
						}
					}
					if !matchedType {
						return nil
					}
				}

				requests = append(requests, &req)
			}
			return nil
		})
	})

	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Sort by ID descending (newest first)
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].ID > requests[j].ID
	})

	total := len(requests)
	var paginatedRequests []*CapturedRequestSummary

	if page > 0 && limit > 0 {
		startIndex := (page - 1) * limit
		if startIndex < total {
			endIndex := startIndex + limit
			if endIndex > total {
				endIndex = total
			}
			paginatedRequests = requests[startIndex:endIndex]
		} else {
			paginatedRequests = []*CapturedRequestSummary{}
		}
	} else {
		paginatedRequests = requests
	}

	w.Header().Set("Content-Type", "application/json")

	type GetRequestsResponse struct {
		Requests []*CapturedRequestSummary `json:"requests"`
		Total    int                      `json:"total"`
		Page     int                      `json:"page"`
		Limit    int                      `json:"limit"`
	}

	resp := GetRequestsResponse{
		Requests: paginatedRequests,
		Total:    total,
		Page:     page,
		Limit:    limit,
	}

	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		json.NewEncoder(gz).Encode(resp)
	} else {
		json.NewEncoder(w).Encode(resp)
	}
}

func getResourceType(req *CapturedRequestSummary) string {
	url := strings.ToLower(req.URL)
	if strings.Contains(url, ".js") || strings.Contains(url, "javascript") {
		return "JS"
	}
	if strings.Contains(url, ".css") {
		return "CSS"
	}
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico"} {
		if strings.Contains(url, ext) {
			return "IMG"
		}
	}
	if strings.Contains(url, ".html") || strings.Contains(url, ".htm") {
		return "DOC"
	}

	if req.ResponseHeaders != nil {
		contentType := strings.ToLower(req.ResponseHeaders.Get("Content-Type"))
		if contentType != "" {
			if strings.Contains(contentType, "javascript") {
				return "JS"
			}
			if strings.Contains(contentType, "css") {
				return "CSS"
			}
			if strings.Contains(contentType, "image") {
				return "IMG"
			}
			if strings.Contains(contentType, "html") {
				return "DOC"
			}
			if strings.Contains(contentType, "json") || strings.Contains(contentType, "xml") {
				return "XHR"
			}
		}
	}
	return "OTHER"
}

func updateSession(w http.ResponseWriter, r *http.Request, sessionID string) {
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

func deleteSession(w http.ResponseWriter, r *http.Request, sessionID string) {
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
