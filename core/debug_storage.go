package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
		_, err := tx.CreateBucketIfNotExists([]byte(sessionsMetadataBucket))
		return err
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
