package core

import (
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
	sessionID := strings.TrimPrefix(r.URL.Path, "/debug/sessions/")
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

func listSessions(w http.ResponseWriter, r *http.Request) {
	var sessions []map[string]string
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionsMetadataBucket))
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

	// Sort by ID descending (newest first)
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].ID > requests[j].ID
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
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
