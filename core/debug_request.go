package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"go.etcd.io/bbolt"
)


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

func getSingleRequest(w http.ResponseWriter, r *http.Request) {
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
			if string(name) == sessionsMetadataBucket {
				return nil
			}
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
			if string(name) == sessionsMetadataBucket {
				return nil
			}
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
