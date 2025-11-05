package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"go.etcd.io/bbolt"
)


func handleSingleRequest(w http.ResponseWriter, r *http.Request) {
	// New path: /api/debug/requests/{sessionID}/{requestID}
	path := strings.TrimPrefix(r.URL.Path, "/api/debug/requests/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid request path. Expected /api/debug/requests/{sessionID}/{requestID}", http.StatusBadRequest)
		return
	}
	sessionID := parts[0]
	idStr := parts[1]

	switch r.Method {
	case http.MethodGet:
		getSingleRequest(w, r, sessionID, idStr)
	case http.MethodPut:
		updateSingleRequest(w, r, sessionID, idStr)
	case http.MethodDelete:
		deleteSingleRequest(w, r, sessionID, idStr)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getSingleRequest(w http.ResponseWriter, r *http.Request, sessionID, idStr string) {
	var foundRequest *CapturedRequest
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionID))
		if b == nil {
			return fmt.Errorf("session not found")
		}
		data := b.Get([]byte(idStr))
		if data == nil {
			return fmt.Errorf("request not found")
		}
		var req CapturedRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return fmt.Errorf("failed to unmarshal request: %v", err)
		}
		foundRequest = &req
		return nil
	})

	if err != nil {
		log.Printf("Error getting single request: %v", err)
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(foundRequest)
}

func updateSingleRequest(w http.ResponseWriter, r *http.Request, sessionID, idStr string) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionID))
		if b == nil {
			return fmt.Errorf("session not found")
		}

		// Get existing request
		data := b.Get([]byte(idStr))
		if data == nil {
			return fmt.Errorf("request not found")
		}
		var req CapturedRequest
		if err := json.Unmarshal(data, &req); err != nil {
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
		return b.Put([]byte(idStr), updatedData)
	})

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
		} else {
			log.Printf("Error updating request: %v", err)
			http.Error(w, "Failed to update request", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

func deleteSingleRequest(w http.ResponseWriter, r *http.Request, sessionID, idStr string) {
	err := db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionID))
		if b == nil {
			return fmt.Errorf("session not found")
		}
		if b.Get([]byte(idStr)) == nil {
			return fmt.Errorf("request not found")
		}
		return b.Delete([]byte(idStr))
	})

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
		} else {
			log.Printf("Error deleting request: %v", err)
			http.Error(w, "Failed to delete request", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
